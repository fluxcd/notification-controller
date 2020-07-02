/*
Copyright 2020 The Flux CD contributors.

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
	"encoding/json"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"github.com/fluxcd/notification-controller/api/v1alpha1"
	"github.com/fluxcd/notification-controller/internal/notifier"
	"github.com/fluxcd/pkg/recorder"
)

func (s *HTTPServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		event := &recorder.Event{}
		err = json.Unmarshal(body, event)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var allAlerts v1alpha1.AlertList
		err = s.kubeClient.List(ctx, &allAlerts, client.InNamespace(event.InvolvedObject.Namespace))
		if err != nil {
			s.logger.Error(err, "listing alerts failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// find matching alerts
		alerts := make([]v1alpha1.Alert, 0)
		for _, alert := range allAlerts.Items {
			// skip suspended alerts
			if alert.Spec.Suspend {
				continue
			}

			// filter alerts by object and severity
			for _, source := range alert.Spec.EventSources {
				if source.Namespace == "" {
					source.Namespace = alert.Namespace
				}
				if event.InvolvedObject.Name == source.Name &&
					event.InvolvedObject.Namespace == source.Namespace &&
					event.InvolvedObject.Kind == source.Kind {
					if event.Severity == alert.Spec.EventSeverity ||
						alert.Spec.EventSeverity == recorder.EventSeverityInfo {
						alerts = append(alerts, alert)
					}
				}
			}
		}

		// find providers
		alertProviders := make([]notifier.Interface, 0)
		if len(alerts) == 0 {
			s.logger.Info("Discarding event, no alerts found for the involved object",
				"object", event.InvolvedObject.Name)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// find providers
		for _, alert := range alerts {
			var provider v1alpha1.Provider
			providerName := types.NamespacedName{Namespace: alert.Namespace, Name: alert.Spec.ProviderRef.Name}

			err = s.kubeClient.Get(ctx, providerName, &provider)
			if err != nil {
				s.logger.Error(err, "failed to read provider",
					"provider", providerName.Name)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			webhook := provider.Spec.Address
			if provider.Spec.SecretRef != nil {
				var secret corev1.Secret
				secretName := types.NamespacedName{Namespace: alert.Namespace, Name: provider.Spec.SecretRef.Name}

				err = s.kubeClient.Get(ctx, secretName, &secret)
				if err != nil {
					s.logger.Error(err, "failed to read secret",
						"provider", providerName.Name,
						"secret", secretName.Name)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if address, ok := secret.Data["address"]; ok {
					webhook = string(address)
				} else {
					s.logger.Error(err, "secret does not contain an address",
						"provider", providerName.Name,
						"secret", secretName.Name)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}

			if webhook == "" {
				s.logger.Error(nil, "provider has no address",
					"provider", providerName.Name)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			factory := notifier.NewFactory(webhook, provider.Spec.Username, provider.Spec.Channel)
			sender, err := factory.Notifier(provider.Spec.Type)
			if err != nil {
				s.logger.Error(err, "failed to initialise provider",
					"provider", providerName.Name,
					"type", provider.Spec.Type)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			alertProviders = append(alertProviders, sender)
		}

		// send notifications in the background
		for _, provider := range alertProviders {
			go func(p notifier.Interface, e recorder.Event) {
				if err := p.Post(e); err != nil {
					s.logger.Error(err, "failed to send notification",
						"object", e.InvolvedObject.Namespace+"/"+e.InvolvedObject.Name)
				}
			}(provider, *event)
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
