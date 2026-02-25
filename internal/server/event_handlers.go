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
	"crypto/tls"
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
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/masktoken"
	"github.com/fluxcd/pkg/runtime/secrets"

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
		var droppedCommitStatusAlerts []*apiv1beta3.Alert
		var droppedChangeRequestAlerts []*apiv1beta3.Alert
		for i := range alerts {
			alert := &alerts[i]
			alertLogger := eventLogger.WithValues(
				"alert", map[string]string{
					"name":         alert.Name,
					"namespace":    alert.Namespace,
					"providerName": alert.Spec.ProviderRef.Name,
				})
			ctx := log.IntoContext(ctx, alertLogger)
			dropped, err := s.dispatchNotification(ctx, event, alert)
			if err != nil {
				alertLogger.Error(err, "failed to dispatch notification")
				s.Eventf(alert, corev1.EventTypeWarning, "NotificationDispatchFailed",
					"failed to dispatch notification for %s: %s", involvedObjectString(event.InvolvedObject), err)
				continue
			}
			if dropped.commitStatus {
				droppedCommitStatusAlerts = append(droppedCommitStatusAlerts, alert)
			}
			if dropped.changeRequest {
				droppedChangeRequestAlerts = append(droppedChangeRequestAlerts, alert)
			}
		}

		// Log if any events were dropped due to being related to a commit status provider
		// but not having the required commit metadata key.
		if len(droppedCommitStatusAlerts) > 0 {
			var alertNames []string
			for _, alert := range droppedCommitStatusAlerts {
				alertNames = append(alertNames, fmt.Sprintf("%s/%s", alert.Namespace, alert.Name))
			}
			eventLogger.Info(
				"event dropped for commit status providers due to missing commit metadata key",
				"alerts", alertNames)
		}

		// Log if any events were dropped due to being related to a change request comment
		// provider but not having the required change request metadata key.
		if len(droppedChangeRequestAlerts) > 0 {
			var alertNames []string
			for _, alert := range droppedChangeRequestAlerts {
				alertNames = append(alertNames, fmt.Sprintf("%s/%s", alert.Namespace, alert.Name))
			}
			eventLogger.Info(
				"event dropped for change request providers due to missing change request metadata key",
				"alerts", alertNames)
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
// and alert data. The returned struct indicates if the event was dropped due
// to being related to a provider that requires a specific metadata key but the
// event didn't have that key.
func (s *EventServer) dispatchNotification(ctx context.Context,
	event *eventv1.Event, alert *apiv1beta3.Alert) (droppedProviders, error) {

	params, dropped, err := s.getNotificationParams(ctx, event, alert)
	if err != nil {
		return droppedProviders{}, err
	}
	if params == nil {
		return dropped, nil
	}

	go func(n notifier.Interface, e eventv1.Event) {
		pctx, cancel := context.WithTimeout(context.Background(), params.timeout)
		defer cancel()
		pctx = notifier.WithAlertMetadata(pctx, alert.ObjectMeta)
		if err := n.Post(pctx, e); err != nil {
			maskedErrStr, maskErr := masktoken.MaskTokenFromString(err.Error(), params.token)
			if maskErr != nil {
				err = maskErr
			} else {
				err = errors.New(maskedErrStr)
			}
			log.FromContext(ctx).Error(err, "failed to send notification")
			s.Eventf(alert, corev1.EventTypeWarning, "NotificationDispatchFailed",
				"failed to send notification for %s: %s", involvedObjectString(e.InvolvedObject), err)
		}
	}(params.sender, *params.event)

	return droppedProviders{}, nil
}

// notificationParams holds the results of the getNotificationParams function.
type notificationParams struct {
	sender  notifier.Interface
	event   *eventv1.Event
	token   string
	timeout time.Duration
}

// droppedProviders holds boolean values indicating whether the event was dropped
// due to being related to a provider that requires a specific metadata key but
// the event didn't have that key.
type droppedProviders struct {
	commitStatus  bool
	changeRequest bool
}

// getNotificationParams constructs the notification parameters from the given
// event and alert, and returns a notifier, event, token and timeout for sending
// the notification. The returned event is a mutated form of the input event
// based on the alert configuration. A struct indicating if the event was dropped
// due to being related to a provider that requires a specific metadata key but
// the event didn't have that key is also returned.
func (s *EventServer) getNotificationParams(ctx context.Context, event *eventv1.Event,
	alert *apiv1beta3.Alert) (*notificationParams, droppedProviders, error) {
	// Check if event comes from a different namespace.
	if s.noCrossNamespaceRefs && event.InvolvedObject.Namespace != alert.Namespace {
		accessDenied := fmt.Errorf(
			"alert '%s/%s' can't process event from '%s', cross-namespace references have been blocked",
			alert.Namespace, alert.Name, involvedObjectString(event.InvolvedObject))
		return nil, droppedProviders{}, fmt.Errorf("discarding event, access denied to cross-namespace sources: %w", accessDenied)
	}

	var provider apiv1beta3.Provider
	providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}

	err := s.kubeClient.Get(ctx, providerName, &provider)
	if err != nil {
		return nil, droppedProviders{}, fmt.Errorf("failed to read provider: %w", err)
	}

	// Skip if the provider is suspended.
	if provider.Spec.Suspend {
		return nil, droppedProviders{}, nil
	}

	// Skip if the event has commit status update metadata but the provider is not a git provider.
	// Git providers (github, gitlab, etc.) are the ones that set commit statuses.
	if !isCommitStatusProvider(provider.Spec.Type) && isCommitStatusUpdate(event) {
		return nil, droppedProviders{}, nil
	}

	// Skip if the provider is a commit status provider but the event doesn't have the commit metadata key
	// and the event is not a commit status update.
	if isCommitStatusProvider(provider.Spec.Type) && !hasCommitKey(event) && !isCommitStatusUpdate(event) {
		// Return true on dropped event for a commit status provider
		// when the event doesn't have the commit metadata key.
		return nil, droppedProviders{commitStatus: true}, nil
	}

	// Skip if the provider is a change request provider but the event
	// doesn't have the change request metadata key.
	if isChangeRequestProvider(provider.Spec.Type) && !hasChangeRequestKey(event) {
		// Return true on dropped event for a change request provider
		// when the event doesn't have the change request metadata key.
		return nil, droppedProviders{changeRequest: true}, nil
	}

	// Check object-level workload identity feature gate.
	if provider.Spec.ServiceAccountName != "" && !auth.IsObjectLevelWorkloadIdentityEnabled() {
		return nil, droppedProviders{}, fmt.Errorf(
			"to use spec.serviceAccountName for provider authentication please enable the %s feature gate in the controller",
			auth.FeatureGateObjectLevelWorkloadIdentity)
	}

	// Create a copy of the event and combine event metadata
	notification := *event.DeepCopy()
	s.combineEventMetadata(ctx, &notification, alert)

	// Create a commit status for the given provider and event, if applicable.
	commitStatus, err := createCommitStatus(ctx, &provider, &notification, alert)
	if err != nil {
		return nil, droppedProviders{}, fmt.Errorf("failed to create commit status: %w", err)
	}

	sender, token, err := createNotifier(ctx, s.kubeClient, &provider, commitStatus, s.tokenCache)
	if err != nil {
		return nil, droppedProviders{}, fmt.Errorf("failed to initialize notifier for provider '%s': %w", provider.Name, err)
	}

	return &notificationParams{
		sender:  sender,
		event:   &notification,
		token:   token,
		timeout: provider.GetTimeout(),
	}, droppedProviders{}, nil
}

// createCommitStatus creates a commit status for the given provider and event.
// If the provider has a commitStatusExpr, it will be used to compute a commit status.
// Otherwise, a default commit status will be generated using the Provider UID and event metadata.
// If the provider is not a git provider, the commit status will be an empty string.
// If the commitStatusExpr fails to compile or is invalid, an error will be returned.
func createCommitStatus(ctx context.Context, provider *apiv1beta3.Provider, event *eventv1.Event, alert *apiv1beta3.Alert) (commitStatus string, err error) {
	if !isCommitStatusProvider(provider.Spec.Type) {
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

// extractAuthFromSecret processes notification-controller specific keys (address, proxy, headers)
// then uses runtime/secrets to handle standard authentication keys (token, username, password, etc.).
func extractAuthFromSecret(ctx context.Context, secret *corev1.Secret) ([]notifier.Option, map[string][]byte, error) {
	options := []notifier.Option{}
	if val, ok := secret.Data["address"]; ok {
		if len(val) > 2048 {
			return nil, nil, fmt.Errorf("invalid address in secret: address exceeds maximum length of %d bytes", 2048)
		}
	}

	if val, ok := secret.Data["proxy"]; ok {
		deprecatedProxy := strings.TrimSpace(string(val))
		if _, err := url.Parse(deprecatedProxy); err != nil {
			return nil, nil, fmt.Errorf("invalid 'proxy' in secret '%s/%s'", secret.Namespace, secret.Name)
		}
		log.FromContext(ctx).Error(nil, "warning: specifying proxy with 'proxy' key in the referenced secret is deprecated, use spec.proxySecretRef with 'address' key instead. Support for the 'proxy' key will be removed in v1.")
		options = append(options, notifier.WithProxyURL(deprecatedProxy))
	}

	if h, ok := secret.Data["headers"]; ok {
		headers := make(map[string]string)
		if err := yaml.Unmarshal(h, &headers); err != nil {
			return nil, nil, fmt.Errorf("failed to read headers from secret: %w", err)
		}
		options = append(options, notifier.WithHeaders(headers))
	}

	authMethods, err := secrets.AuthMethodsFromSecret(ctx, secret)
	if err == nil && authMethods != nil {
		if authMethods.HasTokenAuth() {
			options = append(options, notifier.WithToken(string(authMethods.Token)))
		}
		if authMethods.HasBasicAuth() {
			options = append(options,
				notifier.WithUsername(authMethods.Basic.Username),
				notifier.WithPassword(authMethods.Basic.Password),
			)
		}
	}

	return options, secret.Data, nil
}

// createNotifier constructs a notifier interface from the provider configuration,
// handling authentication, proxy settings, and TLS configuration.
func createNotifier(ctx context.Context, kubeClient client.Client, provider *apiv1beta3.Provider,
	commitStatus string, tokenCache *cache.TokenCache) (notifier.Interface, string, error) {
	options := []notifier.Option{
		notifier.WithTokenClient(kubeClient),
		notifier.WithProviderUID(string(provider.UID)),
		notifier.WithProviderName(provider.Name),
		notifier.WithProviderNamespace(provider.Namespace),
	}

	if commitStatus != "" {
		options = append(options, notifier.WithCommitStatus(commitStatus))
	}

	if provider.Spec.Channel != "" {
		options = append(options, notifier.WithChannel(provider.Spec.Channel))
	}

	if provider.Spec.Username != "" {
		options = append(options, notifier.WithUsername(provider.Spec.Username))
	}

	if provider.Spec.ServiceAccountName != "" {
		options = append(options, notifier.WithServiceAccount(provider.Spec.ServiceAccountName))
	}

	if tokenCache != nil {
		options = append(options, notifier.WithTokenCache(tokenCache))
	}

	// TODO: Remove deprecated proxy handling when Provider v1 is released.
	if provider.Spec.Proxy != "" {
		log.FromContext(ctx).Error(nil, "warning: spec.proxy is deprecated, please use spec.proxySecretRef instead. Support for this field will be removed in v1.")
		options = append(options, notifier.WithProxyURL(provider.Spec.Proxy))
	}

	webhook := provider.Spec.Address
	var token string
	var secretData map[string][]byte
	var providerCertSecret, providerSecret *corev1.Secret
	var err error

	if provider.Spec.SecretRef != nil {
		providerSecret, err = getSecret(ctx, kubeClient, provider.Spec.SecretRef.Name, provider.GetNamespace())
		if err != nil {
			return nil, "", err
		}
		secretOptions, sData, err := extractAuthFromSecret(ctx, providerSecret)
		if err != nil {
			return nil, "", err
		}
		secretData = sData
		options = append(options, secretOptions...)

		if secretData != nil {
			options = append(options, notifier.WithSecretData(secretData))
		}

		if val, ok := secretData["address"]; ok {
			webhook = strings.TrimSpace(string(val))
		}
		if val, ok := secretData[secrets.KeyToken]; ok {
			token = strings.TrimSpace(string(val))
		}
	}

	if provider.Spec.ProxySecretRef != nil {
		proxySecret, err := getSecret(ctx, kubeClient, provider.Spec.ProxySecretRef.Name, provider.GetNamespace())
		if err != nil {
			return nil, "", err
		}
		proxyURL, err := secrets.ProxyURLFromSecret(ctx, proxySecret)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get proxy URL: %w", err)
		}
		options = append(options, notifier.WithProxyURL(proxyURL.String()))
	}

	if provider.Spec.CertSecretRef != nil {
		providerCertSecret, err = getSecret(ctx, kubeClient, provider.Spec.CertSecretRef.Name, provider.GetNamespace())
		if err != nil {
			return nil, "", err
		}
	}

	tlsConfig, err := getTLSConfigForProvider(ctx, providerCertSecret, providerSecret, provider.Spec.Type)
	if err != nil {
		return nil, "", err
	}

	if tlsConfig != nil {
		options = append(options, notifier.WithTLSConfig(tlsConfig))
	}
	if webhook != "" {
		options = append(options, notifier.WithURL(webhook))
	}

	factory := notifier.NewFactory(ctx, options...)
	sender, err := factory.Notifier(provider.Spec.Type)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize notifier: %w", err)
	}
	return sender, token, nil
}

// getTLSConfigForProvider - retrieves the TLS configuration from the provider's certSecretRef or secretRef.
func getTLSConfigForProvider(ctx context.Context, providerCertSecret, providerSecret *corev1.Secret, providerType string) (tlsConfig *tls.Config, err error) {
	// providerCertSecret takes precedence over providerSecret as it is explicitly specified for TLS configuration
	if providerCertSecret != nil {
		tlsConfig, err = secrets.TLSConfigFromSecret(ctx, providerCertSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS config: %w", err)
		}
		return
	}
	// if providerCertSecret is not specified, and if the provider is a commit status
	// provider then attempt to get TLS config from providerSecret if ca.crt exists
	if isCommitStatusProvider(providerType) && providerSecret != nil {
		authMethods, err := secrets.AuthMethodsFromSecret(ctx, providerSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS config: %w", err)
		}
		// only proceed to create TLS config if ca.crt exists in the secret
		if authMethods != nil && authMethods.HasTLS() {
			tlsConfig, err = secrets.TLSConfigFromSecret(ctx, providerSecret)
			if err != nil {
				return nil, fmt.Errorf("failed to get TLS config: %w", err)
			}
		}
	}
	return
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

func getSecret(ctx context.Context, c client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	ref := types.NamespacedName{Name: name, Namespace: namespace}
	if err := c.Get(ctx, ref, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret '%s': %w", ref.String(), err)
	}
	return secret, nil
}
