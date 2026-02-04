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

package v1beta3

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProviderKind                     string = "Provider"
	GenericProvider                  string = "generic"
	GenericHMACProvider              string = "generic-hmac"
	SlackProvider                    string = "slack"
	GrafanaProvider                  string = "grafana"
	DiscordProvider                  string = "discord"
	MSTeamsProvider                  string = "msteams"
	RocketProvider                   string = "rocket"
	GitHubDispatchProvider           string = "githubdispatch"
	GitHubProvider                   string = "github"
	GitHubPullRequestCommentProvider string = "githubpullrequestcomment"
	GitLabProvider                   string = "gitlab"
	GiteaProvider                    string = "gitea"
	BitbucketServerProvider          string = "bitbucketserver"
	BitbucketProvider                string = "bitbucket"
	AzureDevOpsProvider              string = "azuredevops"
	GoogleChatProvider               string = "googlechat"
	GooglePubSubProvider             string = "googlepubsub"
	WebexProvider                    string = "webex"
	SentryProvider                   string = "sentry"
	AzureEventHubProvider            string = "azureeventhub"
	TelegramProvider                 string = "telegram"
	LarkProvider                     string = "lark"
	Matrix                           string = "matrix"
	OpsgenieProvider                 string = "opsgenie"
	AlertManagerProvider             string = "alertmanager"
	PagerDutyProvider                string = "pagerduty"
	DataDogProvider                  string = "datadog"
	NATSProvider                     string = "nats"
	ZulipProvider                    string = "zulip"
	OTELProvider                     string = "otel"
)

// ProviderSpec defines the desired state of the Provider.
// +kubebuilder:validation:XValidation:rule="self.type == 'github' || self.type == 'gitlab' || self.type == 'gitea' || self.type == 'bitbucketserver' || self.type == 'bitbucket' || self.type == 'azuredevops' || !has(self.commitStatusExpr)", message="spec.commitStatusExpr is only supported for the 'github', 'gitlab', 'gitea', 'bitbucketserver', 'bitbucket', 'azuredevops' provider types"
type ProviderSpec struct {
	// Type specifies which Provider implementation to use.
	// +kubebuilder:validation:Enum=slack;discord;msteams;rocket;generic;generic-hmac;github;gitlab;gitea;bitbucketserver;bitbucket;azuredevops;googlechat;googlepubsub;webex;sentry;azureeventhub;telegram;lark;matrix;opsgenie;alertmanager;grafana;githubdispatch;githubpullrequestcomment;pagerduty;datadog;nats;zulip;otel
	// +required
	Type string `json:"type"`

	// Interval at which to reconcile the Provider with its Secret references.
	// Deprecated and not used in v1beta3.
	//
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	// +deprecated
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
	// Deprecated: Use ProxySecretRef instead. Will be removed in v1.
	// +kubebuilder:validation:Pattern="^(http|https)://.*$"
	// +kubebuilder:validation:MaxLength:=2048
	// +kubebuilder:validation:Optional
	// +optional
	Proxy string `json:"proxy,omitempty"`

	// ProxySecretRef specifies the Secret containing the proxy configuration
	// for this Provider. The Secret should contain an 'address' key with the
	// HTTP/S address of the proxy server. Optional 'username' and 'password'
	// keys can be provided for proxy authentication.
	// +optional
	ProxySecretRef *meta.LocalObjectReference `json:"proxySecretRef,omitempty"`

	// SecretRef specifies the Secret containing the authentication
	// credentials for this Provider.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// ServiceAccountName is the name of the Kubernetes ServiceAccount used to
	// authenticate with cloud provider services through workload identity.
	// This enables multi-tenant authentication without storing static credentials.
	//
	// Supported provider types: azureeventhub, azuredevops, googlepubsub
	//
	// When specified, the controller will:
	// 1. Create an OIDC token for the specified ServiceAccount
	// 2. Exchange it for cloud provider credentials via STS
	// 3. Use the obtained credentials for API authentication
	//
	// When unspecified, controller-level authentication is used (single-tenant).
	//
	// An error is thrown if static credentials are also defined in SecretRef.
	// This field requires the ObjectLevelWorkloadIdentity feature gate to be enabled.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// CertSecretRef specifies the Secret containing TLS certificates
	// for secure communication.
	//
	// Supported configurations:
	// - CA-only: Server authentication (provide ca.crt only)
	// - mTLS: Mutual authentication (provide ca.crt + tls.crt + tls.key)
	// - Client-only: Client authentication with system CA (provide tls.crt + tls.key only)
	//
	// Legacy keys "caFile", "certFile", "keyFile" are supported but deprecated. Use "ca.crt", "tls.crt", "tls.key" instead.
	//
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// Suspend tells the controller to suspend subsequent
	// events handling for this Provider.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// CommitStatusExpr is a CEL expression that evaluates to a string value
	// that can be used to generate a custom commit status message for use
	// with eligible Provider types (github, gitlab, gitea, bitbucketserver,
	// bitbucket, azuredevops). Supported variables are: event, provider,
	// and alert.
	// +optional
	CommitStatusExpr string `json:"commitStatusExpr,omitempty"`
}

// +genclient
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Provider is the Schema for the providers API
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ProviderList contains a list of Provider
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
