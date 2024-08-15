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
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	cdevents "github.com/cdevents/sdk-go/pkg/api"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v63/github"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

var (
	WebhookPathIndexKey = ".metadata.webhookPath"
)

// defaultFluxAPIVersions is a map of Flux API kinds to their API versions.
var defaultFluxAPIVersions = map[string]string{
	"Bucket":          "source.toolkit.fluxcd.io/v1beta2",
	"HelmChart":       "source.toolkit.fluxcd.io/v1",
	"HelmRepository":  "source.toolkit.fluxcd.io/v1",
	"GitRepository":   "source.toolkit.fluxcd.io/v1",
	"OCIRepository":   "source.toolkit.fluxcd.io/v1beta2",
	"ImageRepository": "image.toolkit.fluxcd.io/v1beta2",
}

// IndexReceiverWebhookPath is a client.IndexerFunc that returns the Receiver's
// webhook path, if present in its status.
func IndexReceiverWebhookPath(o client.Object) []string {
	receiver := o.(*apiv1.Receiver)
	if receiver.Status.WebhookPath != "" {
		return []string{receiver.Status.WebhookPath}
	}
	return nil
}

func (s *ReceiverServer) handlePayload() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		digest := url.PathEscape(strings.TrimPrefix(r.RequestURI, apiv1.ReceiverWebhookPath))

		s.logger.Info(fmt.Sprintf("handling request: %s", digest))

		var allReceivers apiv1.ReceiverList
		err := s.kubeClient.List(ctx, &allReceivers, client.MatchingFields{
			WebhookPathIndexKey: r.RequestURI,
		}, client.Limit(1))
		if err != nil {
			s.logger.Error(err, "unable to list receivers")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(allReceivers.Items) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		receiver := allReceivers.Items[0]
		logger := s.logger.WithValues(
			"reconciler kind", apiv1.ReceiverKind,
			"name", receiver.Name,
			"namespace", receiver.Namespace)

		if receiver.Spec.Suspend || !conditions.IsReady(&receiver) {
			err := errors.New("unable to process request")
			if receiver.Spec.Suspend {
				logger.Error(err, "receiver is suspended")
			} else {
				logger.Error(err, "receiver is not ready")
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		if err := s.validate(ctx, receiver, r); err != nil {
			logger.Error(err, "unable to validate payload")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var withErrors bool
		for _, resource := range receiver.Spec.Resources {
			if err := s.requestReconciliation(ctx, logger, resource, receiver.Namespace); err != nil {
				logger.Error(err, "unable to request reconciliation")
				withErrors = true
			}
		}

		if withErrors {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func (s *ReceiverServer) validate(ctx context.Context, receiver apiv1.Receiver, r *http.Request) error {
	token, err := s.token(ctx, receiver)
	if err != nil {
		return fmt.Errorf("unable to read token, error: %w", err)
	}

	logger := s.logger.WithValues(
		"reconciler kind", apiv1.ReceiverKind,
		"name", receiver.Name,
		"namespace", receiver.Namespace)

	switch receiver.Spec.Type {
	case apiv1.GenericReceiver:
		return nil
	case apiv1.GenericHMACReceiver:
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("unable to read request body: %s", err)
		}

		err = github.ValidateSignature(r.Header.Get("X-Signature"), b, []byte(token))
		if err != nil {
			return fmt.Errorf("unable to validate HMAC signature: %s", err)
		}
		return nil
	case apiv1.GitHubReceiver:
		_, err := github.ValidatePayload(r, []byte(token))
		if err != nil {
			return fmt.Errorf("the GitHub signature header is invalid, err: %w", err)
		}

		event := github.WebHookType(r)
		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.EqualFold(event, e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the GitHub event '%s' is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling GitHub event: %s", event))
		return nil
	case apiv1.GitLabReceiver:
		if r.Header.Get("X-Gitlab-Token") != token {
			return fmt.Errorf("the X-Gitlab-Token header value does not match the receiver token")
		}

		event := r.Header.Get("X-Gitlab-Event")
		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.EqualFold(event, e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the GitLab event '%s' is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling GitLab event: %s", event))
		return nil
	case apiv1.CDEventsReceiver:
		event := r.Header.Get("Ce-Type")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("unable to read CDEvent request body: %s", err)
		}

		cdevent, err := cdevents.NewFromJsonBytes(b)
		if err != nil {
			return fmt.Errorf("unable to validate CDEvent event: %s", err)
		}

		err = cdevents.Validate(cdevent)
		if err != nil {
			return fmt.Errorf("unable to validate CDEvent event: %s", err)
		}

		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.EqualFold(event, e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the CDEvent '%s' is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling CDEvent: %s", event))
		return nil
	case apiv1.BitbucketReceiver:
		_, err := github.ValidatePayload(r, []byte(token))
		if err != nil {
			return fmt.Errorf("the Bitbucket server signature header is invalid, err: %w", err)
		}

		event := r.Header.Get("X-Event-Key")
		if len(receiver.Spec.Events) > 0 {
			allowed := false
			for _, e := range receiver.Spec.Events {
				if strings.EqualFold(event, e) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("the Bitbucket server event '%s' is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling Bitbucket server event: %s", event))
		return nil
	case apiv1.QuayReceiver:
		type payload struct {
			DockerUrl   string   `json:"docker_url"`
			UpdatedTags []string `json:"updated_tags"`
		}

		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode Quay webhook payload")
		}

		logger.Info(fmt.Sprintf("handling Quay event from %s", p.DockerUrl))
		return nil
	case apiv1.HarborReceiver:
		if r.Header.Get("Authorization") != token {
			return fmt.Errorf("the Harbor Authorization header value does not match the receiver token")
		}

		logger.Info("handling Harbor event")
		return nil
	case apiv1.DockerHubReceiver:
		type payload struct {
			PushData struct {
				Tag string `json:"tag"`
			} `json:"push_data"`
			Repository struct {
				URL string `json:"repo_url"`
			} `json:"repository"`
		}
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode DockerHub webhook payload")
		}

		logger.Info(fmt.Sprintf("handling DockerHub event from %s for tag %s", p.Repository.URL, p.PushData.Tag))
		return nil
	case apiv1.GCRReceiver:
		const tokenIndex = len("Bearer ")

		type data struct {
			Action string `json:"action"`
			Digest string `json:"digest"`
			Tag    string `json:"tag"`
		}

		type payload struct {
			Message struct {
				Data         string    `json:"data"`
				MessageID    string    `json:"messageId"`
				PublishTime  time.Time `json:"publishTime"`
				Subscription string    `json:"subscription"`
			} `json:"message"`
		}

		err := authenticateGCRRequest(&http.Client{}, r.Header.Get("Authorization"), tokenIndex)
		if err != nil {
			return fmt.Errorf("cannot authenticate GCR request: %s", err)
		}

		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode GCR webhook payload")
		}

		raw, _ := base64.StdEncoding.DecodeString(p.Message.Data)

		var d data
		err = json.Unmarshal(raw, &d)
		if err != nil {
			return fmt.Errorf("cannot decode GCR webhook body")
		}

		logger.Info(fmt.Sprintf("handling GCR event from %s for tag %s", d.Digest, d.Tag))
		return nil
	case apiv1.NexusReceiver:
		signature := r.Header.Get("X-Nexus-Webhook-Signature")
		if len(signature) == 0 {
			return fmt.Errorf("Nexus signature is missing from header")
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("cannot read Nexus payload. error: %s", err)
		}

		if !verifyHmacSignature([]byte(token), signature, b) {
			return fmt.Errorf("invalid Nexus signature")
		}
		type payload struct {
			Action         string `json:"action"`
			RepositoryName string `json:"repositoryName"`
		}
		var p payload
		if err := json.Unmarshal(b, &p); err != nil {
			return fmt.Errorf("cannot decode Nexus webhook payload: %s", err)
		}

		logger.Info(fmt.Sprintf("handling Nexus event from %s", p.RepositoryName))
		return nil
	case apiv1.ACRReceiver:
		type target struct {
			Repository string `json:"repository"`
			Tag        string `json:"tag"`
		}

		type payload struct {
			Action string `json:"action"`
			Target target `json:"target"`
		}

		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode ACR webhook payload: %s", err)
		}

		logger.Info(fmt.Sprintf("handling ACR event from %s for tag %s", p.Target.Repository, p.Target.Tag))
		return nil
	}

	return fmt.Errorf("recevier type '%s' not supported", receiver.Spec.Type)
}

func (s *ReceiverServer) token(ctx context.Context, receiver apiv1.Receiver) (string, error) {
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

// requestReconciliation requests reconciliation of all the resources matching the given CrossNamespaceObjectReference by annotating them accordingly.
func (s *ReceiverServer) requestReconciliation(ctx context.Context, logger logr.Logger, resource apiv1.CrossNamespaceObjectReference, defaultNamespace string) error {
	namespace := defaultNamespace
	if resource.Namespace != "" {
		namespace = resource.Namespace
	}

	apiVersion := resource.APIVersion
	if apiVersion == "" {
		if defaultFluxAPIVersions[resource.Kind] == "" {
			return fmt.Errorf("apiVersion must be specified for kind '%s'", resource.Kind)
		}
		apiVersion = defaultFluxAPIVersions[resource.Kind]
	}

	group, version := getGroupVersion(apiVersion)

	if resource.Name == "*" {
		if resource.MatchLabels == nil {
			return fmt.Errorf("matchLabels field not set when using wildcard '*' as name")
		}

		logger.V(1).Info(fmt.Sprintf("annotate resources by matchLabel for kind '%s' in '%s'",
			resource.Kind, namespace), "matchLabels", resource.MatchLabels)

		var resources metav1.PartialObjectMetadataList
		resources.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Kind:    resource.Kind,
			Version: version,
		})

		if err := s.kubeClient.List(ctx, &resources,
			client.InNamespace(namespace),
			client.MatchingLabels(resource.MatchLabels),
		); err != nil {
			return fmt.Errorf("failed listing resources in namespace %q by matching labels %q: %w", namespace, resource.MatchLabels, err)
		}

		if len(resources.Items) == 0 {
			noObjectsFoundErr := fmt.Errorf("no '%s' resources found with matching labels '%s' in '%s' namespace", resource.Kind, resource.MatchLabels, namespace)
			logger.Error(noObjectsFoundErr, "error annotating resources")
			return nil
		}

		for i, resource := range resources.Items {
			if err := s.annotate(ctx, &resources.Items[i]); err != nil {
				return fmt.Errorf("failed to annotate resource: '%s/%s.%s': %w", resource.Kind, resource.Name, namespace, err)
			} else {
				logger.Info(fmt.Sprintf("resource '%s/%s.%s' annotated",
					resource.Kind, resource.Name, namespace))
			}
		}

		return nil
	}

	u := &metav1.PartialObjectMetadata{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    resource.Kind,
		Version: version,
	})

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      resource.Name,
	}

	if err := s.kubeClient.Get(ctx, objectKey, u); err != nil {
		return fmt.Errorf("unable to read %s '%s' error: %w", resource.Kind, objectKey, err)
	}

	err := s.annotate(ctx, u)
	if err != nil {
		return fmt.Errorf("failed to annotate resource: '%s/%s.%s': %w", resource.Kind, resource.Name, namespace, err)
	} else {
		logger.Info(fmt.Sprintf("resource '%s/%s.%s' annotated",
			resource.Kind, resource.Name, namespace))
	}

	return nil
}

func (s *ReceiverServer) annotate(ctx context.Context, resource *metav1.PartialObjectMetadata) error {
	patch := client.MergeFrom(resource.DeepCopy())
	sourceAnnotations := resource.GetAnnotations()

	if sourceAnnotations == nil {
		sourceAnnotations = make(map[string]string)
	}

	sourceAnnotations[meta.ReconcileRequestAnnotation] = metav1.Now().String()
	resource.SetAnnotations(sourceAnnotations)

	if err := s.kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s '%s' error: %w", resource.Kind, client.ObjectKey{
			Namespace: resource.Namespace,
			Name:      resource.Name,
		}, err)
	}

	return nil
}

func authenticateGCRRequest(c *http.Client, bearer string, tokenIndex int) (err error) {
	type auth struct {
		Aud string `json:"aud"`
	}

	if len(bearer) < tokenIndex {
		return fmt.Errorf("Authorization header is missing or malformed: %v", bearer)
	}

	token := bearer[tokenIndex:]
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", token)

	resp, err := c.Get(url)
	if err != nil {
		return fmt.Errorf("cannot verify authenticity of payload: %w", err)
	}

	var p auth
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return fmt.Errorf("cannot decode auth payload: %w", err)
	}

	return nil
}

func verifyHmacSignature(key []byte, signature string, payload []byte) bool {
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func getGroupVersion(s string) (string, string) {
	slice := strings.Split(s, "/")
	if len(slice) == 0 {
		return "", ""
	}
	if len(slice) == 1 {
		return "", slice[0]
	}

	return slice[0], slice[1]
}
