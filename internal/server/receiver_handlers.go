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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"net/url"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/notification-controller/api/v1alpha1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
)

func (s *ReceiverServer) handlePayload() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pathReq := r.RequestURI
		digest := url.PathEscape(strings.TrimLeft(pathReq, "/hook/"))

		s.logger.Info("Handling request", "digest", digest)

		var allReceivers v1alpha1.ReceiverList
		err := s.kubeClient.List(context.TODO(), &allReceivers, client.InNamespace(os.Getenv("RUNTIME_NAMESPACE")))
		if err != nil {
			s.logger.Error(err, "listing receivers failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivers := make([]v1alpha1.Receiver, 0)
		for _, receiver := range allReceivers.Items {
			if receiver.Status.URL == fmt.Sprintf("/hook/%s", digest) {
				receivers = append(receivers, receiver)
			}
		}

		for _, receiver := range receivers {
			s.logger.Info("Found matching receiver", "receiver", receiver.Name)
			for _, resource := range receiver.Spec.Resources {
				switch resource.Kind {
				case "GitRepository":
					s.logger.Info("Found matching GitRepository", "name", resource.Name)

					namespace := receiver.GetNamespace()
					if resource.Namespace != "" {
						namespace = resource.Namespace
					}
					resName := types.NamespacedName{
						Namespace: namespace,
						Name:      resource.Name,
					}

					var source sourcev1.GitRepository
					err := s.kubeClient.Get(context.TODO(), resName, &source)
					if err != nil {
						s.logger.Error(err, "failed to read GitRepository",
							"receiver", receiver.Name)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					if source.Annotations == nil {
						source.Annotations = make(map[string]string)
					}
					source.Annotations[sourcev1.SyncAtAnnotation] = metav1.Now().String()
					err = s.kubeClient.Update(context.TODO(), &source)
					if err != nil {
						s.logger.Error(err, "failed to annotate GitRepository",
							"receiver", receiver.Name,
							"GitRepository", source.Name)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
			}
		}
	}
}
