/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/masktoken"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
	apiv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

func involvedObjectString(o corev1.ObjectReference) string {
	return fmt.Sprintf("%s/%s/%s", o.Kind, o.Namespace, o.Name)
}

func crossNSObjectRefString(o apiv1.CrossNamespaceObjectReference) string {
	return fmt.Sprintf("%s/%s/%s", o.Kind, o.Namespace, o.Name)
}

func (s *EventServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		event := r.Context().Value(eventContextKey{}).(*eventv1.Event)
		eventLogger := log.FromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Remove any internal metadata before further processing the event.
		excludeInternalMetadata(event)

		alerts, err := s.getAllAlertsForEvent(ctx, event)
		if err != nil {
			eventLogger.Error(err, "failed to get alerts for the event")
		}

		if len(alerts) == 0 {
			eventLogger.Info("discarding event, no alerts found for the involved object")
			w.WriteHeader(http.StatusAccepted)
			return
		}

		eventLogger.Info("dispatching event", "message", event.Message)

		// Dispatch notifications.
		for i := range alerts {
			alert := &alerts[i]
			alertLogger := eventLogger.WithValues(alert.Kind, client.ObjectKeyFromObject(alert))
			ctx := log.IntoContext(ctx, alertLogger)
			if err := s.dispatchNotification(ctx, event, alert); err != nil {
				alertLogger.Error(err, "failed to dispatch notification")
				s.Eventf(alert, corev1.EventTypeWarning, "NotificationDispatchFailed",
					"failed to dispatch notification for %s: %s", involvedObjectString(event.InvolvedObject), err)
			}
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *EventServer) getAllAlertsForEvent(ctx context.Context, event *eventv1.Event) ([]apiv1beta3.Alert, error) {
	var allAlerts apiv1beta3.AlertList
	err := s.kubeClient.List(ctx, &allAlerts)
	if err != nil {
		return nil, fmt.Errorf("failed listing alerts: %w", err)
	}

	return s.filterAlertsForEvent(ctx, allAlerts.Items, event), nil
}

// filterAlertsForEvent filters a given set of alerts against a given event,
// checking if the event matches with any of the alert event sources and is
// allowed by the exclusion list.
func (s *EventServer) filterAlertsForEvent(ctx context.Context, alerts []apiv1beta3.Alert, event *eventv1.Event) []apiv1beta3.Alert {
	logger := log.FromContext(ctx)

	results := make([]apiv1beta3.Alert, 0)
	for i := range alerts {
		alert := &alerts[i]
		// Skip suspended alert.
		if alert.Spec.Suspend {
			continue
		}

		alertLogger := logger.WithValues(alert.Kind, client.ObjectKeyFromObject(alert))
		ctx := log.IntoContext(ctx, alertLogger)

		// Check if the event matches any of the alert sources.
		if !s.eventMatchesAlertSources(ctx, event, alert) {
			continue
		}
		// Check if the event message is allowed for the alert based on the
		// inclusion list.
		if !s.messageIsIncluded(ctx, event.Message, alert) {
			continue
		}
		// Check if the event message is allowed for the alert based on the
		// exclusion list.
		if s.messageIsExcluded(ctx, event.Message, alert) {
			continue
		}
		results = append(results, *alert)
	}
	return results
}

// eventMatchesAlertSources returns if a given event matches with any of the
// alert sources.
func (s *EventServer) eventMatchesAlertSources(ctx context.Context, event *eventv1.Event, alert *apiv1beta3.Alert) bool {
	for _, source := range alert.Spec.EventSources {
		if source.Namespace == "" {
			source.Namespace = alert.Namespace
		}
		if s.eventMatchesAlertSource(ctx, event, alert, source) {
			return true
		}
	}
	return false
}

// messageIsIncluded returns if the given message matches with the given alert's
// inclusion rules.
func (s *EventServer) messageIsIncluded(ctx context.Context, msg string, alert *apiv1beta3.Alert) bool {
	if len(alert.Spec.InclusionList) == 0 {
		return true
	}

	for _, exp := range alert.Spec.InclusionList {
		if r, err := regexp.Compile(exp); err == nil {
			if r.Match([]byte(msg)) {
				return true
			}
		} else {
			log.FromContext(ctx).Error(err, fmt.Sprintf("failed to compile inclusion regex: %s", exp))
			s.Eventf(alert, corev1.EventTypeWarning,
				"InvalidConfig", "failed to compile inclusion regex: %s", exp)
		}
	}
	return false
}

// messageIsExcluded returns if the given message matches with the given alert's
// exclusion rules.
func (s *EventServer) messageIsExcluded(ctx context.Context, msg string, alert *apiv1beta3.Alert) bool {
	if len(alert.Spec.ExclusionList) == 0 {
		return false
	}

	for _, exp := range alert.Spec.ExclusionList {
		if r, err := regexp.Compile(exp); err == nil {
			if r.Match([]byte(msg)) {
				return true
			}
		} else {
			log.FromContext(ctx).Error(err, fmt.Sprintf("failed to compile exclusion regex: %s", exp))
			s.Eventf(alert, corev1.EventTypeWarning, "InvalidConfig",
				"failed to compile exclusion regex: %s", exp)
		}
	}
	return false
}

// dispatchNotification constructs and sends notification from the given event
// and alert data.
func (s *EventServer) dispatchNotification(ctx context.Context, event *eventv1.Event, alert *apiv1beta3.Alert) error {
	sender, notification, token, timeout, err := s.getNotificationParams(ctx, event, alert)
	if err != nil {
		return err
	}
	// Skip when either sender or notification couldn't be created.
	if sender == nil || notification == nil {
		return nil
	}

	go func(n notifier.Interface, e eventv1.Event) {
		pctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := n.Post(pctx, e); err != nil {
			maskedErrStr, maskErr := masktoken.MaskTokenFromString(err.Error(), token)
			if maskErr != nil {
				err = maskErr
			} else {
				err = errors.New(maskedErrStr)
			}
			log.FromContext(ctx).Error(err, "failed to send notification")
			s.Eventf(alert, corev1.EventTypeWarning, "NotificationDispatchFailed",
				"failed to send notification for %s: %s", involvedObjectString(event.InvolvedObject), err)
		}
	}(sender, *notification)

	return nil
}

// getNotificationParams constructs the notification parameters from the given
// event and alert, and returns a notifier, event, token and timeout for sending
// the notification. The returned event is a mutated form of the input event
// based on the alert configuration.
func (s *EventServer) getNotificationParams(ctx context.Context, event *eventv1.Event, alert *apiv1beta3.Alert) (notifier.Interface, *eventv1.Event, string, time.Duration, error) {
	// Check if event comes from a different namespace.
	if s.noCrossNamespaceRefs && event.InvolvedObject.Namespace != alert.Namespace {
		accessDenied := fmt.Errorf(
			"alert '%s/%s' can't process event from '%s', cross-namespace references have been blocked",
			alert.Namespace, alert.Name, involvedObjectString(event.InvolvedObject))
		return nil, nil, "", 0, fmt.Errorf("discarding event, access denied to cross-namespace sources: %w", accessDenied)
	}

	var provider apiv1beta3.Provider
	providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}

	err := s.kubeClient.Get(ctx, providerName, &provider)
	if err != nil {
		return nil, nil, "", 0, fmt.Errorf("failed to read provider: %w", err)
	}

	// Skip if the provider is suspended.
	if provider.Spec.Suspend {
		return nil, nil, "", 0, nil
	}

	// Create a copy of the event and combine event metadata
	notification := *event.DeepCopy()
	s.combineEventMetadata(ctx, &notification, alert)

	// Create a commit status for the given provider and event, if applicable.
	commitStatus, err := createCommitStatus(ctx, &provider, &notification, alert)
	if err != nil {
		return nil, nil, "", 0, fmt.Errorf("failed to create commit status: %w", err)
	}

	sender, token, err := createNotifier(ctx, s.kubeClient, &provider, commitStatus)
	if err != nil {
		return nil, nil, "", 0, fmt.Errorf("failed to initialize notifier for provider '%s': %w", provider.Name, err)
	}

	return sender, &notification, token, provider.GetTimeout(), nil
}

// createCommitStatus creates a commit status for the given provider and event.
// If the provider has a commitStatusExpr, it will be used to compute a commit status.
// Otherwise, a default commit status will be generated using the Provider UID and event metadata.
// If the provider is not a git provider, the commit status will be an empty string.
// If the commitStatusExpr fails to compile or is invalid, an error will be returned.
func createCommitStatus(ctx context.Context, provider *apiv1beta3.Provider, event *eventv1.Event, alert *apiv1beta3.Alert) (commitStatus string, err error) {
	if !isGitProvider(provider.Spec.Type) {
		return "", nil
	}

	if provider.Spec.CommitStatusExpr != "" {
		commitStatus, err = newCommitStatus(ctx, provider.Spec.CommitStatusExpr, event, alert, provider)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate the spec.commitStatusExpr CEL expression for the event: %w", err)
		}
	} else {
		commitStatus = generateDefaultCommitStatus(string(provider.UID), *event)
	}

	return commitStatus, nil
}

// createNotifier returns a notifier.Interface for the given Provider.
func createNotifier(ctx context.Context, kubeClient client.Client, provider *apiv1beta3.Provider, commitStatus string) (notifier.Interface, string, error) {
	logger := log.FromContext(ctx)

	webhook := provider.Spec.Address
	username := provider.Spec.Username
	proxy := provider.Spec.Proxy
	token := ""
	password := ""
	headers := make(map[string]string)

	if provider.Spec.SecretRef != nil {
		var secret corev1.Secret
		secretName := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Spec.SecretRef.Name}

		err := kubeClient.Get(ctx, secretName, &secret)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read secret: %w", err)
		}

		if val, ok := secret.Data["address"]; ok {
			if len(val) > 2048 {
				return nil, "", fmt.Errorf("invalid address in secret: address exceeds maximum length of %d bytes", 2048)
			}
			webhook = strings.TrimSpace(string(val))
		}

		if val, ok := secret.Data["password"]; ok {
			password = strings.TrimSpace(string(val))
		}

		if val, ok := secret.Data["proxy"]; ok {
			proxy = strings.TrimSpace(string(val))
			_, err := url.Parse(proxy)
			if err != nil {
				return nil, "", fmt.Errorf("invalid proxy in secret '%s': %w", proxy, err)
			}
		}

		if val, ok := secret.Data["token"]; ok {
			token = strings.TrimSpace(string(val))
		}

		if val, ok := secret.Data["username"]; ok {
			username = strings.TrimSpace(string(val))
		}

		if h, ok := secret.Data["headers"]; ok {
			err := yaml.Unmarshal(h, &headers)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read headers from secret: %w", err)
			}
		}
	}

	var certPool *x509.CertPool
	if provider.Spec.CertSecretRef != nil {
		var secret corev1.Secret
		secretName := types.NamespacedName{Namespace: provider.Namespace, Name: provider.Spec.CertSecretRef.Name}

		err := kubeClient.Get(ctx, secretName, &secret)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read cert secret: %w", err)
		}

		switch secret.Type {
		case corev1.SecretTypeOpaque, corev1.SecretTypeTLS, "":
		default:
			return nil, "", fmt.Errorf("cannot use Secret '%s' to get TLS certificate: invalid Secret type: '%s'", secret.Name, secret.Type)
		}

		caFile, ok := secret.Data["ca.crt"]
		if !ok {
			// TODO: Drop support for "caFile" field in v1 Provider API.
			caFile, ok = secret.Data["caFile"]
			if !ok {
				return nil, "", fmt.Errorf("no 'ca.crt' key found in Secret '%s'", secret.Name)
			}
			logger.Info("warning: specifying CA cert via 'caFile' is deprecated, please use 'ca.crt' instead")
		}

		certPool = x509.NewCertPool()
		ok = certPool.AppendCertsFromPEM(caFile)
		if !ok {
			return nil, "", fmt.Errorf("could not append to cert pool")
		}
	}

	if webhook == "" {
		return nil, "", fmt.Errorf("provider has no address")
	}

	options := []notifier.Option{}

	if commitStatus != "" {
		options = append(options, notifier.WithCommitStatus(commitStatus))
	}

	if proxy != "" {
		options = append(options, notifier.WithProxyURL(proxy))
	}

	if username != "" {
		options = append(options, notifier.WithUsername(username))
	}

	if provider.Spec.Channel != "" {
		options = append(options, notifier.WithChannel(provider.Spec.Channel))
	}

	if token != "" {
		options = append(options, notifier.WithToken(token))
	}

	if len(headers) > 0 {
		options = append(options, notifier.WithHeaders(headers))
	}

	if certPool != nil {
		options = append(options, notifier.WithCertPool(certPool))
	}

	if password != "" {
		options = append(options, notifier.WithPassword(password))
	}

	factory := notifier.NewFactory(webhook, options...)
	sender, err := factory.Notifier(provider.Spec.Type)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize notifier: %w", err)
	}
	return sender, token, nil
}

// eventMatchesAlertSource returns if a given event matches with the given alert
// source configuration and severity.
func (s *EventServer) eventMatchesAlertSource(ctx context.Context, event *eventv1.Event, alert *apiv1beta3.Alert, source apiv1.CrossNamespaceObjectReference) bool {
	logger := log.FromContext(ctx)

	// No match if the event and source don't have the same namespace and kind.
	if event.InvolvedObject.Namespace != source.Namespace ||
		event.InvolvedObject.Kind != source.Kind {
		return false
	}

	// No match if the alert severity doesn't match the event severity and
	// the alert severity isn't info.
	severity := alert.Spec.EventSeverity
	if event.Severity != severity && severity != eventv1.EventSeverityInfo {
		return false
	}

	// No match if the source name isn't wildcard, and source and event names
	// don't match.
	if source.Name != "*" && source.Name != event.InvolvedObject.Name {
		return false
	}

	// Match if no match labels specified.
	if source.MatchLabels == nil {
		return true
	}

	// Perform label selector matching.
	var obj metav1.PartialObjectMetadata
	obj.SetGroupVersionKind(event.InvolvedObject.GroupVersionKind())
	obj.SetName(event.InvolvedObject.Name)
	obj.SetNamespace(event.InvolvedObject.Namespace)

	if err := s.kubeClient.Get(ctx, types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}, &obj); err != nil {
		logger.Error(err, "error getting the involved object")
		s.Eventf(alert, corev1.EventTypeWarning, "SourceFetchFailed",
			"error getting source object %s", involvedObjectString(event.InvolvedObject))
		return false
	}

	sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: source.MatchLabels,
	})
	if err != nil {
		logger.Error(err, fmt.Sprintf("error using matchLabels from event source %s", crossNSObjectRefString(source)))
		s.Eventf(alert, corev1.EventTypeWarning, "InvalidConfig",
			"error using matchLabels from event source %s", crossNSObjectRefString(source))
		return false
	}

	return sel.Matches(labels.Set(obj.GetLabels()))
}

// combineEventMetadata combines all the sources of metadata for the event
// according to the precedence order defined in RFC 0008. From lowest to
// highest precedence, the sources are:
//
// 1) Event metadata keys prefixed with the Event API Group stripped of the prefix.
//
// 2) Alert .spec.eventMetadata with the keys as they are.
//
// 3) Alert .spec.summary with the key "summary".
//
// 4) Event metadata keys prefixed with the involved object's API Group stripped of the prefix.
//
// At the end of the process key conflicts are detected and a single
// info-level log is emitted to warn users about all the conflicts,
// but only if at least one conflict is found.
func (s *EventServer) combineEventMetadata(ctx context.Context, event *eventv1.Event, alert *apiv1beta3.Alert) {
	const (
		sourceEventGroup         = "involved object annotations"
		sourceAlertEventMetadata = "Alert object .spec.eventMetadata"
		sourceAlertSummary       = "Alert object .spec.summary"
		sourceObjectGroup        = "involved object controller metadata"

		summaryKey = "summary"
	)

	l := log.FromContext(ctx)
	metadata := make(map[string]string)
	metadataSources := make(map[string][]string)

	// 1) Event metadata keys prefixed with the Event API Group stripped of the prefix.
	const eventGroupPrefix = eventv1.Group + "/"
	for k, v := range event.Metadata {
		if strings.HasPrefix(k, eventGroupPrefix) {
			key := strings.TrimPrefix(k, eventGroupPrefix)
			metadata[key] = v
			metadataSources[key] = append(metadataSources[key], sourceEventGroup)
		}
	}

	// 2) Alert .spec.eventMetadata with the keys as they are.
	for k, v := range alert.Spec.EventMetadata {
		metadata[k] = v
		metadataSources[k] = append(metadataSources[k], sourceAlertEventMetadata)
	}

	// 3) Alert .spec.summary with the key "summary".
	if alert.Spec.Summary != "" {
		metadata[summaryKey] = alert.Spec.Summary
		metadataSources[summaryKey] = append(metadataSources[summaryKey], sourceAlertSummary)
		l.Info("warning: specifying an alert summary with '.spec.summary' is deprecated, use '.spec.eventMetadata.summary' instead")
	}

	// 4) Event metadata keys prefixed with the involved object's API Group stripped of the prefix.
	objectGroupPrefix := event.InvolvedObject.GroupVersionKind().Group + "/"
	for k, v := range event.Metadata {
		if strings.HasPrefix(k, objectGroupPrefix) {
			key := strings.TrimPrefix(k, objectGroupPrefix)
			metadata[key] = v
			metadataSources[key] = append(metadataSources[key], sourceObjectGroup)
		}
	}

	// Detect key conflicts and emit warnings if any.
	type keyConflict struct {
		Key     string   `json:"key"`
		Sources []string `json:"sources"`
	}
	var conflictingKeys []*keyConflict
	conflictEventAnnotations := make(map[string]string)
	for key, sources := range metadataSources {
		if len(sources) > 1 {
			conflictingKeys = append(conflictingKeys, &keyConflict{key, sources})
			conflictEventAnnotations[key] = strings.Join(sources, ", ")
		}
	}
	if len(conflictingKeys) > 0 {
		const msg = "metadata key conflicts detected (please refer to the Alert API docs and Flux RFC 0008 for more information)"
		slices.SortFunc(conflictingKeys, func(a, b *keyConflict) int { return strings.Compare(a.Key, b.Key) })
		l.Info("warning: "+msg, "conflictingKeys", conflictingKeys)
		s.AnnotatedEventf(alert, conflictEventAnnotations, corev1.EventTypeWarning, "MetadataAppendFailed", "%s", msg)
	}

	if len(metadata) > 0 {
		event.Metadata = metadata
	}
}

// excludeInternalMetadata removes any internal metadata from the given event.
func excludeInternalMetadata(event *eventv1.Event) {
	if len(event.Metadata) == 0 {
		return
	}
	objectGroup := event.InvolvedObject.GetObjectKind().GroupVersionKind().Group
	tokenKey := fmt.Sprintf("%s/%s", objectGroup, eventv1.MetaTokenKey)
	excludeList := []string{tokenKey}
	for _, key := range excludeList {
		delete(event.Metadata, key)
	}
}
