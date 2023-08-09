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
	"regexp"
	"time"

	"github.com/fluxcd/pkg/runtime/conditions"
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
	apiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

func (s *EventServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		event := r.Context().Value(eventContextKey{}).(*eventv1.Event)
		eventLogger := log.FromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var allAlerts apiv1beta2.AlertList
		err := s.kubeClient.List(ctx, &allAlerts)
		if err != nil {
			eventLogger.Error(err, "listing alerts failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// find matching alerts
		alerts := make([]apiv1beta2.Alert, 0)
	each_alert:
		for _, alert := range allAlerts.Items {
			alertLogger := eventLogger.WithValues("alert", client.ObjectKeyFromObject(&alert))
			ctx := log.IntoContext(ctx, alertLogger)

			// skip suspended and not ready alerts
			isReady := conditions.IsReady(&alert)
			if alert.Spec.Suspend || !isReady {
				continue each_alert
			}

			// skip alert if the message does not match any regex from the inclusion list
			if len(alert.Spec.InclusionList) > 0 {
				var include bool
				for _, inclusionRegex := range alert.Spec.InclusionList {
					if r, err := regexp.Compile(inclusionRegex); err == nil {
						if r.Match([]byte(event.Message)) {
							include = true
							break
						}
					} else {
						alertLogger.Error(err, fmt.Sprintf("failed to compile inclusion regex: %s", inclusionRegex))
					}
				}
				if !include {
					continue each_alert
				}
			}

			// skip alert if the message matches a regex from the exclusion list
			if len(alert.Spec.ExclusionList) > 0 {
				for _, exclusionRegex := range alert.Spec.ExclusionList {
					if r, err := regexp.Compile(exclusionRegex); err == nil {
						if r.Match([]byte(event.Message)) {
							continue each_alert
						}
					} else {
						alertLogger.Error(err, fmt.Sprintf("failed to compile exclusion regex: %s", exclusionRegex))
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
			eventLogger.Info("Discarding event, no alerts found for the involved object")
			w.WriteHeader(http.StatusAccepted)
			return
		}

		eventLogger.Info(fmt.Sprintf("Dispatching event: %s", event.Message))

		// dispatch notifications
		for _, alert := range alerts {
			alertLogger := eventLogger.WithValues("alert", client.ObjectKeyFromObject(&alert))
			ctx := log.IntoContext(ctx, alertLogger)

			// verify if event comes from a different namespace
			if s.noCrossNamespaceRefs && event.InvolvedObject.Namespace != alert.Namespace {
				accessDenied := fmt.Errorf(
					"alert '%s/%s' can't process event from '%s/%s/%s', cross-namespace references have been blocked",
					alert.Namespace, alert.Name, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
				alertLogger.Error(accessDenied, "Discarding event, access denied to cross-namespace sources")
				continue
			}

			var provider apiv1beta2.Provider
			providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}

			err = s.kubeClient.Get(ctx, providerName, &provider)
			if err != nil {
				alertLogger.Error(err, "failed to read provider")
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
					alertLogger.Error(err, "failed to read secret")
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
						alertLogger.Error(err, "failed to read headers from secret")
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
					alertLogger.Error(err, "failed to read cert secret")
					continue
				}

				switch secret.Type {
				case corev1.SecretTypeOpaque, corev1.SecretTypeTLS, "":
				default:
					alertLogger.Error(nil, "cannot use secret '%s' to get TLS certificate: invalid secret type: '%s'",
						secret.Name, secret.Type)
					continue
				}

				caFile, ok := secret.Data["ca.crt"]
				if !ok {
					caFile, ok = secret.Data["caFile"]
					if !ok {
						alertLogger.Error(nil, "no 'ca.crt' key found in secret '%s'", provider.Spec.CertSecretRef.Name)
						continue
					}
					alertLogger.Info("warning: specifying CA cert via 'caFile' is deprecated, please use 'ca.crt' instead")
				}

				certPool = x509.NewCertPool()
				ok = certPool.AppendCertsFromPEM(caFile)
				if !ok {
					alertLogger.Error(nil, "could not append to cert pool")
					continue
				}
			}

			if webhook == "" {
				alertLogger.Error(nil, "provider has no address")
				continue
			}

			factory := notifier.NewFactory(webhook, proxy, username, provider.Spec.Channel, token, headers, certPool, password, string(provider.UID))
			sender, err := factory.Notifier(provider.Spec.Type)
			if err != nil {
				alertLogger.Error(err, "failed to initialize provider")
				continue
			}

			notification := *event.DeepCopy()
			s.enhanceEventWithAlertMetadata(ctx, &notification, alert)

			go func(n notifier.Interface, e eventv1.Event) {
				ctx, cancel := context.WithTimeout(context.Background(), provider.GetTimeout())
				defer cancel()
				ctx = log.IntoContext(ctx, alertLogger)
				if err := n.Post(ctx, e); err != nil {
					maskedErrStr, maskErr := masktoken.MaskTokenFromString(err.Error(), token)
					if maskErr != nil {
						err = maskErr
					} else {
						err = errors.New(maskedErrStr)
					}
					alertLogger.Error(err, "failed to send notification")
				}
			}(sender, notification)
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *EventServer) eventMatchesAlert(ctx context.Context, event *eventv1.Event, source apiv1.CrossNamespaceObjectReference, severity string) bool {
	alertLogger := log.FromContext(ctx)

	if event.InvolvedObject.Namespace == source.Namespace && event.InvolvedObject.Kind == source.Kind {
		if event.Severity == severity || severity == eventv1.EventSeverityInfo {
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
					alertLogger.Error(err, "error getting the involved object")
				}

				sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: source.MatchLabels,
				})
				if err != nil {
					alertLogger.Error(err, fmt.Sprintf("error using matchLabels from event source '%s'", source.Name))
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

func (s *EventServer) enhanceEventWithAlertMetadata(ctx context.Context, event *eventv1.Event, alert apiv1beta2.Alert) {
	meta := event.Metadata
	if meta == nil {
		meta = make(map[string]string)
	}

	for key, value := range alert.Spec.EventMetadata {
		if _, alreadyPresent := meta[key]; !alreadyPresent {
			meta[key] = value
		} else {
			log.FromContext(ctx).
				Info("metadata key found in the existing set of metadata", "key", key)
		}
	}

	if alert.Spec.Summary != "" {
		meta["summary"] = alert.Spec.Summary
	}

	if len(meta) > 0 {
		event.Metadata = meta
	}
}
