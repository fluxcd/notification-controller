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
	"bytes"
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
	cdevents04 "github.com/cdevents/sdk-go/pkg/api/v04"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v64/github"
	"google.golang.org/api/idtoken"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

const (
	WebhookPathIndexKey string = ".metadata.webhookPath"

	// maxRequestSizeBytes is the maximum size of a request to the API server
	maxRequestSizeBytes = 3 * 1024 * 1024
)

// defaultFluxAPIVersions is a map of Flux API kinds to their API versions.
var defaultFluxAPIVersions = map[string]string{
	"Bucket":            "source.toolkit.fluxcd.io/v1",
	"HelmChart":         "source.toolkit.fluxcd.io/v1",
	"HelmRepository":    "source.toolkit.fluxcd.io/v1",
	"GitRepository":     "source.toolkit.fluxcd.io/v1",
	"OCIRepository":     "source.toolkit.fluxcd.io/v1",
	"ImageRepository":   "image.toolkit.fluxcd.io/v1",
	"ArtifactGenerator": "source.extensions.fluxcd.io/v1beta1",
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

func (s *ReceiverServer) handlePayload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	resourceFilter := func(ctx context.Context, o client.Object) (*bool, error) {
		accept := true
		return &accept, nil
	}
	if receiver.Spec.ResourceFilter != "" {
		resourceFilter, err = newResourceFilter(receiver.Spec.ResourceFilter, r)
		if err != nil {
			logger.Error(err, "unable to create resource filter")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	var withErrors bool
	for _, resource := range receiver.Spec.Resources {
		if err := s.requestReconciliation(ctx, logger, resource, receiver.Namespace, resourceFilter); err != nil {
			logger.Error(err, "unable to request reconciliation", "resource", resource)
			withErrors = true
		}
	}

	if withErrors {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *ReceiverServer) notifySingleResource(ctx context.Context, logger logr.Logger, resource *metav1.PartialObjectMetadata, resourceFilter resourceFilter) error {
	objectKey := client.ObjectKeyFromObject(resource)
	if err := s.kubeClient.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s %q error: %w", resource.Kind, objectKey, err)
	}

	return s.notifyResource(ctx, logger, resource, resourceFilter)
}

func (s *ReceiverServer) notifyResource(ctx context.Context, logger logr.Logger, resource *metav1.PartialObjectMetadata, resourceFilter resourceFilter) error {
	accept, err := resourceFilter(ctx, resource)
	if err != nil {
		return err
	}
	if !*accept {
		logger.V(1).Info(fmt.Sprintf("resource '%s/%s.%s' NOT annotated because CEL expression returned false", resource.Kind, resource.Name, resource.Namespace))
		return nil
	}

	if err := s.annotate(ctx, resource); err != nil {
		return fmt.Errorf("failed to annotate resource: '%s/%s.%s': %w", resource.Kind, resource.Name, resource.Namespace, err)
	} else {
		logger.Info(fmt.Sprintf("resource '%s/%s.%s' annotated",
			resource.Kind, resource.Name, resource.Namespace))
	}

	return nil
}

func (s *ReceiverServer) notifyDynamicResources(ctx context.Context, logger logr.Logger, resource apiv1.CrossNamespaceObjectReference, namespace, group, version string, resourceFilter resourceFilter) error {
	if resource.MatchLabels == nil {
		return fmt.Errorf("matchLabels field not set when using wildcard '*' as name")
	}

	logger.V(1).Info(fmt.Sprintf("annotate resources by matchLabel for kind %q in %q",
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
		noObjectsFoundErr := fmt.Errorf("no %q resources found with matching labels %q' in %q namespace", resource.Kind, resource.MatchLabels, namespace)
		logger.Error(noObjectsFoundErr, "error annotating resources")
		return nil
	}

	for i := range resources.Items {
		if err := s.notifyResource(ctx, logger, &resources.Items[i], resourceFilter); err != nil {
			return err
		}
	}

	return nil
}

func (s *ReceiverServer) validate(ctx context.Context, receiver apiv1.Receiver, r *http.Request) error {
	// Validate payload size before doing anything else in case we are being DDoSed.
	b, err := io.ReadAll(io.LimitReader(r.Body, maxRequestSizeBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	if len(b) > maxRequestSizeBytes {
		return fmt.Errorf("request body exceeds the maximum size of %d bytes", maxRequestSizeBytes)
	}
	r.Body = io.NopCloser(bytes.NewReader(b))

	// Fetch the secret.
	secret, err := s.secret(ctx, receiver)
	if err != nil {
		return fmt.Errorf("unable to read secret, error: %w", err)
	}

	// Extract the token from the secret.
	secretName := types.NamespacedName{
		Namespace: receiver.GetNamespace(),
		Name:      receiver.Spec.SecretRef.Name,
	}
	tokenBytes, ok := secret.Data["token"]
	if !ok {
		return fmt.Errorf("invalid %q secret data: required field 'token'", secretName)
	}
	token := string(tokenBytes)

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
		r.Body = io.NopCloser(bytes.NewReader(b))
		return nil
	case apiv1.GitHubReceiver:
		b, err := github.ValidatePayload(r, []byte(token))
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
				return fmt.Errorf("the GitHub event %q is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling GitHub event: %s", event))
		r.Body = io.NopCloser(bytes.NewReader(b))
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
				return fmt.Errorf("the GitLab event %q is not authorised", event)
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

		cdevent, err := cdevents04.NewFromJsonBytes(b)
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
				return fmt.Errorf("the CDEvent %q is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling CDEvent: %s", event))
		r.Body = io.NopCloser(bytes.NewReader(b))
		return nil
	case apiv1.BitbucketReceiver:
		b, err := github.ValidatePayload(r, []byte(token))
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
				return fmt.Errorf("the Bitbucket server event %q is not authorised", event)
			}
		}

		logger.Info(fmt.Sprintf("handling Bitbucket server event: %s", event))
		r.Body = io.NopCloser(bytes.NewReader(b))
		return nil
	case apiv1.QuayReceiver:
		type payload struct {
			DockerUrl   string   `json:"docker_url"`
			UpdatedTags []string `json:"updated_tags"`
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("unable to read request body: %s", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode Quay webhook payload: %w", err)
		}

		logger.Info(fmt.Sprintf("handling Quay event from %s", p.DockerUrl))
		r.Body = io.NopCloser(bytes.NewReader(b))
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
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("unable to read request body: %s", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode DockerHub webhook payload")
		}

		logger.Info(fmt.Sprintf("handling DockerHub event from %s for tag %s", p.Repository.URL, p.PushData.Tag))
		r.Body = io.NopCloser(bytes.NewReader(b))
		return nil
	case apiv1.GCRReceiver:
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

		expectedEmail := string(secret.Data["email"])
		// TODO: in Flux 2.9, require the email. this will be a breaking change.
		// if expectedEmail == "" {
		// 	return fmt.Errorf("invalid secret data: required field 'email' for GCR receiver")
		// }

		expectedAudience := string(secret.Data["audience"])
		// TODO: in Flux 2.9, require the audience. this will be a breaking change.
		// if expectedAudience == "" {
		// 	return fmt.Errorf("invalid secret data: required field 'audience' for GCR receiver")
		// }

		authenticate := authenticateGCRRequest
		if s.gcrTokenValidator != nil {
			authenticate = s.gcrTokenValidator
		}
		if err := authenticate(ctx, r.Header.Get("Authorization"), expectedEmail, expectedAudience); err != nil {
			return fmt.Errorf("cannot authenticate GCR request: %w", err)
		}

		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode GCR webhook payload: %w", err)
		}
		// The GCR payload is a Google PubSub event with the GCR event wrapped
		// inside (in base64 JSON).
		raw, _ := base64.StdEncoding.DecodeString(p.Message.Data)

		var d data
		err = json.Unmarshal(raw, &d)
		if err != nil {
			return fmt.Errorf("cannot decode GCR webhook body: %w", err)
		}

		logger.Info(fmt.Sprintf("handling GCR event from %s for tag %s", d.Digest, d.Tag))
		encodedPayload, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("cannot decode GCR webhook body: %w", err)
		}
		// This only puts the unwrapped event into the payload.
		r.Body = io.NopCloser(bytes.NewReader(encodedPayload))
		return nil
	case apiv1.NexusReceiver:
		signature := r.Header.Get("X-Nexus-Webhook-Signature")
		if len(signature) == 0 {
			return fmt.Errorf("the Nexus signature is missing from header")
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("cannot read Nexus payload. error: %w", err)
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
			return fmt.Errorf("cannot decode Nexus webhook payload: %w", err)
		}

		logger.Info(fmt.Sprintf("handling Nexus event from %s", p.RepositoryName))
		r.Body = io.NopCloser(bytes.NewReader(b))
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

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("unable to read request body: %s", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(b))

		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return fmt.Errorf("cannot decode ACR webhook payload: %s", err)
		}

		logger.Info(fmt.Sprintf("handling ACR event from %s for tag %s", p.Target.Repository, p.Target.Tag))
		r.Body = io.NopCloser(bytes.NewReader(b))
		return nil
	}

	return fmt.Errorf("recevier type %q not supported", receiver.Spec.Type)
}

func (s *ReceiverServer) secret(ctx context.Context, receiver apiv1.Receiver) (*corev1.Secret, error) {
	secretName := types.NamespacedName{
		Namespace: receiver.GetNamespace(),
		Name:      receiver.Spec.SecretRef.Name,
	}

	var secret corev1.Secret
	if err := s.kubeClient.Get(ctx, secretName, &secret); err != nil {
		return nil, fmt.Errorf("unable to read secret %q: %w", secretName, err)
	}

	return &secret, nil
}

// requestReconciliation requests reconciliation of all the resources matching the given CrossNamespaceObjectReference by annotating them accordingly.
func (s *ReceiverServer) requestReconciliation(ctx context.Context, logger logr.Logger, resource apiv1.CrossNamespaceObjectReference, defaultNamespace string, resourceFilter resourceFilter) error {
	namespace := defaultNamespace
	if resource.Namespace != "" {
		if s.noCrossNamespaceRefs && resource.Namespace != defaultNamespace {
			return fmt.Errorf("cross-namespace references are not allowed")
		}
		namespace = resource.Namespace
	}

	apiVersion := resource.APIVersion
	if apiVersion == "" {
		if defaultFluxAPIVersions[resource.Kind] == "" {
			return fmt.Errorf("apiVersion must be specified for kind %q", resource.Kind)
		}
		apiVersion = defaultFluxAPIVersions[resource.Kind]
	}

	group, version := getGroupVersion(apiVersion)

	if resource.Name == "*" {
		return s.notifyDynamicResources(ctx, logger, resource, namespace, group, version, resourceFilter)
	}

	u := &metav1.PartialObjectMetadata{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    resource.Kind,
		Version: version,
	})
	u.SetNamespace(namespace)
	u.SetName(resource.Name)

	return s.notifySingleResource(ctx, logger, u, resourceFilter)
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
		return fmt.Errorf("unable to annotate %s %q error: %w", resource.Kind, client.ObjectKey{
			Namespace: resource.Namespace,
			Name:      resource.Name,
		}, err)
	}

	return nil
}

// authenticateGCRRequest validates the OIDC ID token according to
// https://docs.cloud.google.com/pubsub/docs/authenticate-push-subscriptions#go.
func authenticateGCRRequest(ctx context.Context, bearer string, expectedEmail string, expectedAudience string) error {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(bearer, bearerPrefix) {
		return fmt.Errorf("the Authorization header is missing or malformed")
	}

	token := bearer[len(bearerPrefix):]

	// Validate the OIDC ID token signature and claims using Google's public keys.
	v, err := idtoken.NewValidator(ctx)
	if err != nil {
		return fmt.Errorf("cannot create ID token validator: %w", err)
	}
	payload, err := v.Validate(ctx, token, expectedAudience)
	if err != nil {
		// Extract the actual audience from the token for logging.
		gotAudience := "<unknown>"
		if parts := strings.Split(token, "."); len(parts) == 3 {
			if claimsJSON, decErr := base64.RawURLEncoding.DecodeString(parts[1]); decErr == nil {
				var claims struct {
					Aud string `json:"aud"`
				}
				if json.Unmarshal(claimsJSON, &claims) == nil && claims.Aud != "" {
					gotAudience = claims.Aud
				}
			}
		}
		return fmt.Errorf("invalid ID token: audience is '%s', want '%s': %w", gotAudience, expectedAudience, err)
	}

	// Verify the token issuer.
	issuer, _ := payload.Claims["iss"].(string)
	if issuer != "accounts.google.com" && issuer != "https://accounts.google.com" {
		return fmt.Errorf("token issuer is '%s', want 'accounts.google.com' or 'https://accounts.google.com'", issuer)
	}

	// Verify the token was issued for the expected service account.
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	// TODO: in Flux 2.9, require the email (remove `expectedEmail != "" &&`). this will be a breaking change.
	if expectedEmail != "" && email != expectedEmail {
		return fmt.Errorf("token email is '%s', want '%s'", email, expectedEmail)
	}
	if !emailVerified {
		return fmt.Errorf("token email '%s' is not verified", email)
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
