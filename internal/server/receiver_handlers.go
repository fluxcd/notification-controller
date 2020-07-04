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
	"github.com/google/go-github/v32/github"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"net/url"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/notification-controller/api/v1alpha1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1alpha1"
)

func (s *ReceiverServer) handlePayload() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		digest := url.PathEscape(strings.TrimLeft(r.RequestURI, "/hook/"))

		s.logger.Info("handling request", "digest", digest)

		var allReceivers v1alpha1.ReceiverList
		err := s.kubeClient.List(ctx, &allReceivers, client.InNamespace(os.Getenv("RUNTIME_NAMESPACE")))
		if err != nil {
			s.logger.Error(err, "unable to list receivers")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivers := make([]v1alpha1.Receiver, 0)
		for _, receiver := range allReceivers.Items {
			if receiver.Status.URL == fmt.Sprintf("/hook/%s", digest) {
				receivers = append(receivers, receiver)
			}
		}

		if len(receivers) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		for _, receiver := range receivers {
			if err := s.validate(ctx, receiver, r); err != nil {
				s.logger.Error(err, "unable to validate payload",
					"receiver", receiver.Name)
				continue
			}

			s.logger.Info("found matching receiver", "receiver", receiver.Name)
			for _, resource := range receiver.Spec.Resources {
				if err := s.annotate(ctx, resource, receiver.Namespace); err != nil {
					s.logger.Error(err, "unable to annotate resource",
						"receiver", receiver.Name)
				} else {
					s.logger.Info("resource annotated", "receiver", receiver.Name,
						"resource", resource.Name)
				}
			}
		}
	}
}

func (s *ReceiverServer) validate(ctx context.Context, receiver v1alpha1.Receiver, r *http.Request) error {
	token, err := s.token(ctx, receiver)
	if err != nil {
		return fmt.Errorf("unable to read token, error: %w", err)
	}

	switch receiver.Spec.Type {
	case v1alpha1.GenericReceiver:
		return nil
	case v1alpha1.GitHubReceiver:
		payload, err := github.ValidatePayload(r, []byte(token))
		if err != nil {
			return fmt.Errorf("the GitHub signature header is invalid, err: %w", err)
		}

		if _, err := github.ParseWebHook(github.WebHookType(r), payload); err != nil {
			return fmt.Errorf("unable to parse GitHub payload, err: %w", err)
		}

		event := github.WebHookType(r)

		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.ToLower(event) == strings.ToLower(e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("GitHub event '%s' is not authorised", event)
			}
		}

		s.logger.Info("handling GitHub event: "+event, "receiver", receiver.Name)
		return nil
	case v1alpha1.GitLabReceiver:
		return nil
	}
	return nil
}

func (s *ReceiverServer) token(ctx context.Context, receiver v1alpha1.Receiver) (string, error) {
	token := ""
	secretName := types.NamespacedName{
		Namespace: receiver.GetNamespace(),
		Name:      receiver.Spec.SecretRef.Name,
	}

	var secret corev1.Secret
	err := s.kubeClient.Get(ctx, secretName, &secret)
	if err != nil {
		return "", fmt.Errorf("unable to read token from secret '%s' error: %w", secretName, err)
	}

	if val, ok := secret.Data["token"]; ok {
		token = string(val)
	} else {
		return "", fmt.Errorf("invalid '%s' secret data: required fields 'token'", secretName)
	}

	return token, nil
}

func (s *ReceiverServer) annotate(ctx context.Context, resource v1alpha1.CrossNamespaceObjectReference, defaultNamespace string) error {
	namespace := defaultNamespace
	if resource.Namespace != "" {
		namespace = resource.Namespace
	}
	resourceName := types.NamespacedName{
		Namespace: namespace,
		Name:      resource.Name,
	}

	switch resource.Kind {
	case "GitRepository":
		var source sourcev1.GitRepository
		if err := s.kubeClient.Get(ctx, resourceName, &source); err != nil {
			return fmt.Errorf("unable to read GitRepository '%s' error: %w", resourceName, err)
		}
		if source.Annotations == nil {
			source.Annotations = make(map[string]string)
		}
		source.Annotations[sourcev1.SyncAtAnnotation] = metav1.Now().String()
		if err := s.kubeClient.Update(ctx, &source); err != nil {
			return fmt.Errorf("unable to annotate GitRepository '%s' error: %w", resourceName, err)
		}
	case "HelmRepository":
		var source sourcev1.HelmRepository
		if err := s.kubeClient.Get(ctx, resourceName, &source); err != nil {
			return fmt.Errorf("unable to read HelmRepository '%s' error: %w", resourceName, err)
		}
		if source.Annotations == nil {
			source.Annotations = make(map[string]string)
		}
		source.Annotations[sourcev1.SyncAtAnnotation] = metav1.Now().String()
		if err := s.kubeClient.Update(ctx, &source); err != nil {
			return fmt.Errorf("unable to annotate HelmRepository '%s' error: %w", resourceName, err)
		}
	default:
		return fmt.Errorf("kind '%s not suppored", resource.Kind)
	}

	return nil
}
