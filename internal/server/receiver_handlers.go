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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/go-github/v32/github"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
)

func (s *ReceiverServer) handlePayload() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		digest := url.PathEscape(strings.TrimLeft(r.RequestURI, "/hook/"))

		s.logger.Info("handling request", "digest", digest)

		var allReceivers v1beta1.ReceiverList
		err := s.kubeClient.List(ctx, &allReceivers, client.InNamespace(os.Getenv("RUNTIME_NAMESPACE")))
		if err != nil {
			s.logger.Error(err, "unable to list receivers")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivers := make([]v1beta1.Receiver, 0)
		for _, receiver := range allReceivers.Items {
			if receiver.Status.URL == fmt.Sprintf("/hook/%s", digest) && !receiver.Spec.Suspend {
				receivers = append(receivers, receiver)
			}
		}

		if len(receivers) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		withErrors := false
		for _, receiver := range receivers {
			if err := s.validate(ctx, receiver, r); err != nil {
				s.logger.Error(err, "unable to validate payload",
					"receiver", receiver.Name)
				withErrors = true
				continue
			}

			s.logger.Info("found matching receiver", "receiver", receiver.Name)
			for _, resource := range receiver.Spec.Resources {
				if err := s.annotate(ctx, resource, receiver.Namespace); err != nil {
					s.logger.Error(err, "unable to annotate resource",
						"receiver", receiver.Name)
					withErrors = true
				} else {
					s.logger.Info("resource annotated", "receiver", receiver.Name,
						"resource", resource.Name)
				}
			}
		}

		if withErrors {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (s *ReceiverServer) validate(ctx context.Context, receiver v1beta1.Receiver, r *http.Request) error {
	token, err := s.token(ctx, receiver)
	if err != nil {
		return fmt.Errorf("unable to read token, error: %w", err)
	}

	switch receiver.Spec.Type {
	case v1beta1.GenericReceiver:
		return nil
	case v1beta1.GitHubReceiver:
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
				return fmt.Errorf("the GitHub event '%s' is not authorised", event)
			}
		}

		s.logger.Info("handling GitHub event: "+event, "receiver", receiver.Name)
		return nil
	case v1beta1.GitLabReceiver:
		if r.Header.Get("X-Gitlab-Token") != token {
			return fmt.Errorf("the X-Gitlab-Token header value does not match the receiver token")
		}

		event := r.Header.Get("X-Gitlab-Event")
		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.ToLower(event) == strings.ToLower(e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the GitLab event '%s' is not authorised", event)
			}
		}

		s.logger.Info("handling GitLab event: "+event, "receiver", receiver.Name)
		return nil
	case v1beta1.BitbucketReceiver:
		_, err := github.ValidatePayload(r, []byte(token))
		if err != nil {
			return fmt.Errorf("the Bitbucket server signature header is invalid, err: %w", err)
		}

		event := r.Header.Get("X-Event-Key")

		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.ToLower(event) == strings.ToLower(e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the Bitbucket server event '%s' is not authorised", event)
			}
		}

		s.logger.Info("handling Bitbucket server event: "+event, "receiver", receiver.Name)
		return nil
	case v1beta1.HarborReceiver:
		if r.Header.Get("Authorization") != token {
			return fmt.Errorf("the Harbor Authorization header value does not match the receiver token")
		}

		s.logger.Info("handling Harbor event", "receiver", receiver.Name)
		return nil
	}

	return fmt.Errorf("recevier type '%s' not supported", receiver.Spec.Type)
}

func (s *ReceiverServer) token(ctx context.Context, receiver v1beta1.Receiver) (string, error) {
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
		return "", fmt.Errorf("invalid '%s' secret data: required field 'token'", secretName)
	}

	return token, nil
}

func (s *ReceiverServer) annotate(ctx context.Context, resource v1beta1.CrossNamespaceObjectReference, defaultNamespace string) error {
	namespace := defaultNamespace
	if resource.Namespace != "" {
		namespace = resource.Namespace
	}
	resourceName := types.NamespacedName{
		Namespace: namespace,
		Name:      resource.Name,
	}

	switch resource.Kind {
	case sourcev1.BucketKind:
		var source sourcev1.Bucket
		if err := s.kubeClient.Get(ctx, resourceName, &source); err != nil {
			return fmt.Errorf("unable to read Bucket '%s' error: %w", resourceName, err)
		}
		if source.Annotations == nil {
			source.Annotations = make(map[string]string)
		}
		source.Annotations[meta.ReconcileRequestAnnotation] = metav1.Now().String()
		if err := s.kubeClient.Update(ctx, &source); err != nil {
			return fmt.Errorf("unable to annotate Bucket '%s' error: %w", resourceName, err)
		}
	case sourcev1.GitRepositoryKind:
		var source sourcev1.GitRepository
		if err := s.kubeClient.Get(ctx, resourceName, &source); err != nil {
			return fmt.Errorf("unable to read GitRepository '%s' error: %w", resourceName, err)
		}
		if source.Annotations == nil {
			source.Annotations = make(map[string]string)
		}
		source.Annotations[meta.ReconcileRequestAnnotation] = metav1.Now().String()
		if err := s.kubeClient.Update(ctx, &source); err != nil {
			return fmt.Errorf("unable to annotate GitRepository '%s' error: %w", resourceName, err)
		}
	case sourcev1.HelmRepositoryKind:
		var source sourcev1.HelmRepository
		if err := s.kubeClient.Get(ctx, resourceName, &source); err != nil {
			return fmt.Errorf("unable to read HelmRepository '%s' error: %w", resourceName, err)
		}
		if source.Annotations == nil {
			source.Annotations = make(map[string]string)
		}
		source.Annotations[meta.ReconcileRequestAnnotation] = metav1.Now().String()
		if err := s.kubeClient.Update(ctx, &source); err != nil {
			return fmt.Errorf("unable to annotate HelmRepository '%s' error: %w", resourceName, err)
		}
	default:
		return fmt.Errorf("kind '%s not suppored", resource.Kind)
	}

	return nil
}
