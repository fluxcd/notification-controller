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
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/fluxcd/pkg/runtime/conditions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/fluxcd/pkg/runtime/events"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/notification-controller/internal/notifier"
)

func (s *EventServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
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

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
				if (source.Name == "*" || event.InvolvedObject.Name == source.Name) &&
					event.InvolvedObject.Namespace == source.Namespace &&
					event.InvolvedObject.Kind == source.Kind {
					if event.Severity == alert.Spec.EventSeverity ||
						alert.Spec.EventSeverity == events.EventSeverityInfo {
						alerts = append(alerts, alert)
					}
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
			token := ""
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

				if t, ok := secret.Data["token"]; ok {
					token = string(t)
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

			factory := notifier.NewFactory(webhook, provider.Spec.Proxy, provider.Spec.Username, provider.Spec.Channel, token, certPool)
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
				if err := n.Post(e); err != nil {
					if token != "" {
						redacted := strings.ReplaceAll(err.Error(), token, "*****")
						err = errors.New(redacted)
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
