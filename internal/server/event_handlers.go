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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fluxcd/pkg/runtime/conditions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/masktoken"
	"github.com/fluxcd/pkg/runtime/events"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

func (s *EventServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		event := &events.Event{}
		err = json.Unmarshal(body, event)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cleanupMetadata(event)

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var allAlerts v1beta1.AlertList
		err = s.kubeClient.List(ctx, &allAlerts)
		if err != nil {
			s.logger.Error(err, "listing alerts failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// find matching alerts
		alerts := make([]v1beta1.Alert, 0)
	each_alert:
		for _, alert := range allAlerts.Items {
			// skip suspended and not ready alerts
			isReady := conditions.IsReady(&alert)
			if alert.Spec.Suspend || !isReady {
				continue each_alert
			}

			// skip alert if the message matches a regex from the exclusion list
			if len(alert.Spec.ExclusionList) > 0 {
				for _, exp := range alert.Spec.ExclusionList {
					if r, err := regexp.Compile(exp); err == nil {
						if r.Match([]byte(event.Message)) {
							continue each_alert
						}
					} else {
						s.logger.Error(err, fmt.Sprintf("failed to compile regex: %s", exp))
					}
				}
			}

			// filter alerts by object and severity
			for _, source := range alert.Spec.EventSources {
				if source.Namespace == "" {
					source.Namespace = alert.Namespace
				}

				if s.eventMatchesAlert(ctx, event, source, alert.Spec.EventSeverity) {
					alerts = append(alerts, alert)
				}
			}
		}

		if len(alerts) == 0 {
			s.logger.Info("Discarding event, no alerts found for the involved object",
				"reconciler kind", event.InvolvedObject.Kind,
				"name", event.InvolvedObject.Name,
				"namespace", event.InvolvedObject.Namespace)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		s.logger.Info(fmt.Sprintf("Dispatching event: %s", event.Message),
			"reconciler kind", event.InvolvedObject.Kind,
			"name", event.InvolvedObject.Name,
			"namespace", event.InvolvedObject.Namespace)

		// dispatch notifications
		for _, alert := range alerts {
			// verify if event comes from a different namespace
			if s.noCrossNamespaceRefs && event.InvolvedObject.Namespace != alert.Namespace {
				accessDenied := fmt.Errorf(
					"alert '%s/%s' can't process event from '%s/%s/%s', cross-namespace references have been blocked",
					alert.Namespace, alert.Name, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
				s.logger.Error(accessDenied, "Discarding event, access denied to cross-namespace sources")
				continue
			}

			var provider v1beta1.Provider
			providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}

			err = s.kubeClient.Get(ctx, providerName, &provider)
			if err != nil {
				s.logger.Error(err, "failed to read provider",
					"reconciler kind", v1beta1.ProviderKind,
					"name", providerName.Name,
					"namespace", providerName.Namespace)
				continue
			}

			if provider.Spec.Suspend {
				continue
			}

			webhook := provider.Spec.Address
			username := provider.Spec.Username
			proxy := provider.Spec.Proxy
			token := ""
			password := ""
			headers := make(map[string]string)
			if provider.Spec.SecretRef != nil {
				var secret corev1.Secret
				secretName := types.NamespacedName{Namespace: alert.Namespace, Name: provider.Spec.SecretRef.Name}

				err = s.kubeClient.Get(ctx, secretName, &secret)
				if err != nil {
					s.logger.Error(err, "failed to read secret",
						"reconciler kind", v1beta1.ProviderKind,
						"name", providerName.Name,
						"namespace", providerName.Namespace)
					continue
				}

				if address, ok := secret.Data["address"]; ok {
					webhook = string(address)
				}

				if p, ok := secret.Data["password"]; ok {
					password = string(p)
				}

				if p, ok := secret.Data["proxy"]; ok {
					proxy = string(p)
				}

				if t, ok := secret.Data["token"]; ok {
					token = string(t)
				}

				if u, ok := secret.Data["username"]; ok {
					username = string(u)
				}

				if h, ok := secret.Data["headers"]; ok {
					err := yaml.Unmarshal(h, &headers)
					if err != nil {
						s.logger.Error(err, "failed to read headers from secret",
							"reconciler kind", v1beta1.ProviderKind,
							"name", providerName.Name,
							"namespace", providerName.Namespace)
						continue
					}
				}
			}

			var certPool *x509.CertPool
			if provider.Spec.CertSecretRef != nil {
				var secret corev1.Secret
				secretName := types.NamespacedName{Namespace: alert.Namespace, Name: provider.Spec.CertSecretRef.Name}

				err = s.kubeClient.Get(ctx, secretName, &secret)
				if err != nil {
					s.logger.Error(err, "failed to read secret",
						"reconciler kind", v1beta1.ProviderKind,
						"name", providerName.Name,
						"namespace", providerName.Namespace)
					continue
				}

				caFile, ok := secret.Data["caFile"]
				if !ok {
					s.logger.Error(err, "failed to read secret key caFile",
						"reconciler kind", v1beta1.ProviderKind,
						"name", providerName.Name,
						"namespace", providerName.Namespace)
					continue
				}

				certPool = x509.NewCertPool()
				ok = certPool.AppendCertsFromPEM(caFile)
				if !ok {
					s.logger.Error(err, "could not append to cert pool",
						"reconciler kind", v1beta1.ProviderKind,
						"name", providerName.Name,
						"namespace", providerName.Namespace)
					continue
				}
			}

			if webhook == "" {
				s.logger.Error(nil, "provider has no address",
					"reconciler kind", v1beta1.ProviderKind,
					"name", providerName.Name,
					"namespace", providerName.Namespace)
				continue
			}

			webhookUrl, err := url.Parse(webhook)
			if err != nil {
				s.logger.Error(nil, "Error parsing webhook url",
					"reconciler kind", v1beta1.ProviderKind,
					"name", providerName.Name,
					"namespace", providerName.Namespace)
				continue
			}

			if !s.supportHttpScheme && webhookUrl.Scheme == "http" {
				s.logger.Error(nil, "http scheme is blocked",
					"reconciler kind", v1beta1.ProviderKind,
					"name", providerName.Name,
					"namespace", providerName.Namespace)
				continue
			}
			factory := notifier.NewFactory(webhook, proxy, username, provider.Spec.Channel, token, headers, certPool, password)
			sender, err := factory.Notifier(provider.Spec.Type)
			if err != nil {
				s.logger.Error(err, "failed to initialize provider",
					"reconciler kind", v1beta1.ProviderKind,
					"name", providerName.Name,
					"namespace", providerName.Namespace)
				continue
			}

			notification := *event.DeepCopy()
			if alert.Spec.Summary != "" {
				if notification.Metadata == nil {
					notification.Metadata = map[string]string{
						"summary": alert.Spec.Summary,
					}
				} else {
					notification.Metadata["summary"] = alert.Spec.Summary
				}
			}

			go func(n notifier.Interface, e events.Event) {
				ctx, cancel := context.WithTimeout(context.Background(), provider.GetTimeout())
				defer cancel()
				if err := n.Post(ctx, e); err != nil {
					maskedErrStr, maskErr := masktoken.MaskTokenFromString(err.Error(), token)
					if maskErr != nil {
						err = maskErr
					} else {
						err = errors.New(maskedErrStr)
					}
					s.logger.Error(err, "failed to send notification",
						"reconciler kind", event.InvolvedObject.Kind,
						"name", event.InvolvedObject.Name,
						"namespace", event.InvolvedObject.Namespace)
				}
			}(sender, notification)
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *EventServer) eventMatchesAlert(ctx context.Context, event *events.Event, source v1beta1.CrossNamespaceObjectReference, severity string) bool {
	if event.InvolvedObject.Namespace == source.Namespace &&
		event.InvolvedObject.Kind == source.Kind {
		if event.Severity == severity ||
			severity == events.EventSeverityInfo {

			labelMatch := true
			if source.Name == "*" && source.MatchLabels != nil {
				var obj metav1.PartialObjectMetadata
				obj.SetGroupVersionKind(event.InvolvedObject.GroupVersionKind())
				obj.SetName(event.InvolvedObject.Name)
				obj.SetNamespace(event.InvolvedObject.Namespace)

				if err := s.kubeClient.Get(ctx, types.NamespacedName{
					Namespace: event.InvolvedObject.Namespace,
					Name:      event.InvolvedObject.Name,
				}, &obj); err != nil {
					s.logger.Error(err, "error getting object", "kind", event.InvolvedObject.Kind,
						"name", event.InvolvedObject.Name, "apiVersion", event.InvolvedObject.APIVersion)
				}

				sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: source.MatchLabels,
				})
				if err != nil {
					s.logger.Error(err, fmt.Sprintf("error using matchLabels from event source '%s'", source.Name))
				}

				labelMatch = sel.Matches(labels.Set(obj.GetLabels()))
			}

			if source.Name == "*" && labelMatch || event.InvolvedObject.Name == source.Name {
				return true
			}
		}
	}

	return false
}

// TODO: move the metadata filtering function to fluxcd/pkg/runtime/events
// cleanupMetadata removes metadata entries which are not used for alerting
func cleanupMetadata(event *events.Event) {
	group := event.InvolvedObject.GetObjectKind().GroupVersionKind().Group
	excludeList := []string{fmt.Sprintf("%s/checksum", group)}

	meta := make(map[string]string)

	if event.Metadata != nil && len(event.Metadata) > 0 {
		// For backwards compatibility, include the revision without a group prefix
		revisionKey := "revision"
		if rev, ok := event.Metadata[revisionKey]; ok {
			meta[revisionKey] = rev
		}

		// Filter other meta based on group prefix, while filtering out excludes
		for key, val := range event.Metadata {
			if strings.HasPrefix(key, group) && !inList(excludeList, key) {
				newKey := strings.TrimPrefix(key, fmt.Sprintf("%s/", group))
				meta[newKey] = val
			}
		}
	}

	event.Metadata = meta
}

func inList(l []string, i string) bool {
	for _, v := range l {
		if strings.EqualFold(v, i) {
			return true
		}
	}
	return false
}
