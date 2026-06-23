/*
Copyright 2023 The Flux authors

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

package v1

import (
	"crypto/sha256"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
)

const (
	ReceiverKind        string = "Receiver"
	ReceiverWebhookPath string = "/hook/"
	GenericReceiver     string = "generic"
	GenericHMACReceiver string = "generic-hmac"
	GenericOIDCReceiver string = "generic-oidc"
	GitHubReceiver      string = "github"
	GitLabReceiver      string = "gitlab"
	BitbucketReceiver   string = "bitbucket"
	HarborReceiver      string = "harbor"
	DockerHubReceiver   string = "dockerhub"
	QuayReceiver        string = "quay"
	GCRReceiver         string = "gcr"
	NexusReceiver       string = "nexus"
	ACRReceiver         string = "acr"
	CDEventsReceiver    string = "cdevents"
)

// DefaultOIDCAudience is the default expected audience ('aud' claim) for tokens
// issued to a 'generic-oidc' Receiver when no audience is configured.
const DefaultOIDCAudience string = "notification-controller"

// ReceiverSpec defines the desired state of the Receiver.
// +kubebuilder:validation:XValidation:rule="self.type != 'generic-oidc' || (has(self.oidcProviders) && size(self.oidcProviders) > 0)",message="generic-oidc receivers must define at least one oidcProvider"
// +kubebuilder:validation:XValidation:rule="self.type == 'generic-oidc' || !has(self.oidcProviders) || size(self.oidcProviders) == 0",message="oidcProviders can only be set when type is generic-oidc"
// +kubebuilder:validation:XValidation:rule="self.type != 'generic-oidc' || !has(self.secretRef)",message="secretRef cannot be set when type is generic-oidc"
// +kubebuilder:validation:XValidation:rule="self.type == 'generic-oidc' || has(self.secretRef)",message="secretRef is required when type is not generic-oidc"
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=generic;generic-hmac;generic-oidc;github;gitlab;bitbucket;harbor;dockerhub;quay;gcr;nexus;acr;cdevents
	// +required
	Type string `json:"type"`

	// Interval at which to reconcile the Receiver with its Secret references.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default:="10m"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Events specifies the list of event types to handle,
	// e.g. 'push' for GitHub or 'Push Hook' for GitLab.
	// +optional
	Events []string `json:"events,omitempty"`

	// A list of resources to be notified about changes.
	// +required
	Resources []ReceiverResource `json:"resources"`

	// ResourceFilter is a CEL expression expected to return a boolean that is
	// evaluated for each resource referenced in the Resources field when a
	// webhook is received. If the expression returns false then the controller
	// will not request a reconciliation for the resource.
	// The expression can read the resource metadata via 'res' and the webhook
	// request body via 'req'. For generic-oidc receivers, the verified OIDC
	// token claims are also available via 'claims'.
	// When the expression is specified the controller will parse it and mark
	// the object as terminally failed if the expression is invalid or does not
	// return a boolean.
	// +optional
	ResourceFilter string `json:"resourceFilter,omitempty"`

	// SecretRef specifies the Secret containing the token used
	// to validate the payload authenticity. The Secret must contain a 'token'
	// key. For GCR receivers, the Secret must also contain an 'email' key
	// with the IAM service account email configured on the Pub/Sub push
	// subscription, and an 'audience' key with the expected OIDC token audience.
	//
	// Required for all receiver types except 'generic-oidc', which authenticates
	// requests using the OIDC token instead and must not set this field.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// OIDCProviders specifies the OIDC providers used to authenticate incoming
	// requests when Type is 'generic-oidc'. The provider whose IssuerURL matches
	// the token's 'iss' claim is used to verify the token signature, expiration
	// and audience, and to evaluate the configured CEL validations against the
	// token claims.
	// +listType=map
	// +listMapKey=issuerURL
	// +optional
	OIDCProviders []OIDCProvider `json:"oidcProviders,omitempty"`

	// Suspend tells the controller to suspend subsequent
	// events handling for this receiver.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReceiverResource references a resource to be notified about changes, with an
// optional per-resource CEL filter.
type ReceiverResource struct {
	CrossNamespaceObjectReference `json:",inline"`

	// Filter is a CEL expression expected to return a boolean that is evaluated
	// for each resource matched by this reference when a webhook is received,
	// in addition to the top-level resourceFilter. A reconciliation is requested
	// only when both expressions (when set) return true.
	// The expression can read the resource metadata via 'res' and the webhook
	// request body via 'req'. For generic-oidc receivers, the verified OIDC
	// token claims are also available via 'claims'.
	// When the expression is specified the controller will parse it and mark
	// the object as terminally failed if the expression is invalid or does not
	// return a boolean.
	// +optional
	Filter string `json:"filter,omitempty"`
}

// OIDCProvider configures an OIDC issuer used to authenticate requests for a
// 'generic-oidc' Receiver.
type OIDCProvider struct {
	// IssuerURL is the OIDC issuer URL used for provider discovery. It must
	// match the 'iss' claim of tokens issued by this provider.
	// +kubebuilder:validation:Pattern="^https?://"
	// +required
	IssuerURL string `json:"issuerURL"`

	// Audience is the expected audience ('aud' claim) for tokens issued by
	// this provider. Defaults to 'notification-controller'.
	// +optional
	Audience string `json:"audience,omitempty"`

	// Variables is an optional list of named CEL expressions, evaluated in order
	// and exposed as 'vars.<name>'. Each expression can read the token claims
	// via 'claims' and any variable defined before it. Use it to share
	// sub-expressions across validations.
	// +optional
	Variables []OIDCVariable `json:"variables,omitempty"`

	// Validations is the list of CEL boolean expressions evaluated against the
	// token claims and the variables. The request is accepted only if all of
	// them evaluate to true; the message of each failing expression is returned
	// to the caller.
	//
	// At least one validation is required. A valid signature alone does not
	// authorize a request: public issuers issue tokens to any caller on the
	// platform, so the validations must constrain the caller's identity claims
	// (e.g. 'repository_owner' for GitHub Actions).
	// +kubebuilder:validation:MinItems=1
	// +required
	Validations []OIDCValidation `json:"validations"`
}

// GetAudience returns the expected audience ('aud' claim) for tokens issued by
// this provider, defaulting to 'notification-controller'.
func (in *OIDCProvider) GetAudience() string {
	if in.Audience != "" {
		return in.Audience
	}

	return DefaultOIDCAudience
}

// OIDCVariable is a named CEL expression evaluated against the OIDC token
// claims of a 'generic-oidc' Receiver.
type OIDCVariable struct {
	// Name is the variable name; it must be a valid CEL identifier.
	// +required
	Name string `json:"name"`

	// Expression is the CEL expression that defines the variable value.
	// +required
	Expression string `json:"expression"`
}

// OIDCValidation is a CEL boolean expression evaluated against the OIDC token
// claims and variables of a 'generic-oidc' Receiver.
type OIDCValidation struct {
	// Expression is the CEL boolean expression to evaluate.
	// +required
	Expression string `json:"expression"`

	// Message is returned to the caller when the expression evaluates to false.
	// +required
	Message string `json:"message"`
}

// ReceiverStatus defines the observed state of the Receiver.
type ReceiverStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions holds the conditions for the Receiver.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// WebhookPath is the generated incoming webhook address in the format
	// of '/hook/sha256sum(token+name+namespace)'.
	// +optional
	WebhookPath string `json:"webhookPath,omitempty"`

	// ObservedGeneration is the last observed generation of the Receiver object.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *Receiver) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Receiver) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetWebhookPath returns the incoming webhook path for the given token.
func (in *Receiver) GetWebhookPath(token string) string {
	digest := sha256.Sum256([]byte(token + in.GetName() + in.GetNamespace()))
	return fmt.Sprintf("%s%x", ReceiverWebhookPath, digest)
}

// GetInterval returns the interval value with a default of 10m for this Receiver.
func (in *Receiver) GetInterval() time.Duration {
	duration := 10 * time.Minute
	if in.Spec.Interval != nil {
		duration = in.Spec.Interval.Duration
	}

	return duration
}

// +genclient
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;fluxcd;fluxcd-notifications
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// Receiver is the Schema for the receivers API.
type Receiver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ReceiverSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status ReceiverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReceiverList contains a list of Receivers.
type ReceiverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Receiver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Receiver{}, &ReceiverList{})
}
