/*
Copyright 2022 The Flux authors

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

package v1beta2

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProviderKind            string = "Provider"
	GenericProvider         string = "generic"
	GenericHMACProvider     string = "generic-hmac"
	SlackProvider           string = "slack"
	GrafanaProvider         string = "grafana"
	DiscordProvider         string = "discord"
	MSTeamsProvider         string = "msteams"
	RocketProvider          string = "rocket"
	GitHubDispatchProvider  string = "githubdispatch"
	GitHubProvider          string = "github"
	GitLabProvider          string = "gitlab"
	GiteaProvider           string = "gitea"
	BitbucketServerProvider string = "bitbucketserver"
	BitbucketProvider       string = "bitbucket"
	AzureDevOpsProvider     string = "azuredevops"
	GoogleChatProvider      string = "googlechat"
	GooglePubSubProvider    string = "googlepubsub"
	WebexProvider           string = "webex"
	SentryProvider          string = "sentry"
	AzureEventHubProvider   string = "azureeventhub"
	TelegramProvider        string = "telegram"
	LarkProvider            string = "lark"
	Matrix                  string = "matrix"
	OpsgenieProvider        string = "opsgenie"
	AlertManagerProvider    string = "alertmanager"
	PagerDutyProvider       string = "pagerduty"
	DataDogProvider         string = "datadog"
)

// ProviderSpec defines the desired state of the Provider.
type ProviderSpec struct {
	// Type specifies which Provider implementation to use.
	// +kubebuilder:validation:Enum=slack;discord;msteams;rocket;generic;generic-hmac;github;gitlab;gitea;bitbucketserver;bitbucket;azuredevops;googlechat;googlepubsub;webex;sentry;azureeventhub;telegram;lark;matrix;opsgenie;alertmanager;grafana;githubdispatch;pagerduty;datadog
	// +required
	Type string `json:"type"`

	// Interval at which to reconcile the Provider with its Secret references.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Channel specifies the destination channel where events should be posted.
	// +kubebuilder:validation:MaxLength:=2048
	// +optional
	Channel string `json:"channel,omitempty"`

	// Username specifies the name under which events are posted.
	// +kubebuilder:validation:MaxLength:=2048
	// +optional
	Username string `json:"username,omitempty"`

	// Address specifies the endpoint, in a generic sense, to where alerts are sent.
	// What kind of endpoint depends on the specific Provider type being used.
	// For the generic Provider, for example, this is an HTTP/S address.
	// For other Provider types this could be a project ID or a namespace.
	// +kubebuilder:validation:MaxLength:=2048
	// +kubebuilder:validation:Optional
	// +optional
	Address string `json:"address,omitempty"`

	// Timeout for sending alerts to the Provider.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m))+$"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Proxy the HTTP/S address of the proxy server.
	// +kubebuilder:validation:Pattern="^(http|https)://.*$"
	// +kubebuilder:validation:MaxLength:=2048
	// +kubebuilder:validation:Optional
	// +optional
	Proxy string `json:"proxy,omitempty"`

	// SecretRef specifies the Secret containing the authentication
	// credentials for this Provider.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// CertSecretRef specifies the Secret containing
	// a PEM-encoded CA certificate (in the `ca.crt` key).
	// +optional
	//
	// Note: Support for the `caFile` key has
	// been deprecated.
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// Suspend tells the controller to suspend subsequent
	// events handling for this Provider.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ProviderStatus defines the observed state of the Provider.
type ProviderStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions holds the conditions for the Provider.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +genclient:Namespaced
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// Provider is the Schema for the providers API.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status ProviderStatus `json:"status,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *Provider) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Provider) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Providers.
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

// GetTimeout returns the timeout value with a default of 15s for this Provider.
func (in *Provider) GetTimeout() time.Duration {
	duration := 15 * time.Second
	if in.Spec.Timeout != nil {
		duration = in.Spec.Timeout.Duration
	}

	return duration
}

// GetInterval returns the interval value with a default of 10m for this Provider.
func (in *Provider) GetInterval() time.Duration {
	duration := 10 * time.Minute
	if in.Spec.Interval != nil {
		duration = in.Spec.Interval.Duration
	}

	return duration
}

// GetRequeueAfter returns the duration after which the Provider must be
// reconciled again.
func (in *Provider) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}
