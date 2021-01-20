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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
)

// ReceiverSpec defines the desired state of Receiver
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=generic;generic-hmac;github;gitlab;bitbucket;harbor;dockerhub;quay;gcr;nexus
	// +required
	Type string `json:"type"`

	// A list of events to handle,
	// e.g. 'push' for GitHub or 'Push Hook' for GitLab.
	// +optional
	Events []string `json:"events"`

	// A list of resources to be notified about changes.
	// +required
	Resources []CrossNamespaceObjectReference `json:"resources"`

	// Secret reference containing the token used
	// to validate the payload authenticity
	// +required
	SecretRef corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// This flag tells the controller to suspend subsequent events handling.
	// Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReceiverStatus defines the observed state of Receiver
type ReceiverStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Generated webhook URL in the format
	// of '/hook/sha256sum(token+name+namespace)'.
	// +optional
	URL string `json:"url,omitempty"`
}

const (
	GenericReceiver     string = "generic"
	GenericHMACReceiver string = "generic-hmac"
	GitHubReceiver      string = "github"
	GitLabReceiver      string = "gitlab"
	BitbucketReceiver   string = "bitbucket"
	HarborReceiver      string = "harbor"
	DockerHubReceiver   string = "dockerhub"
	QuayReceiver        string = "quay"
	GCRReceiver         string = "gcr"
	NexusReceiver       string = "nexus"
)

func ReceiverReady(receiver Receiver, reason, message, url string) Receiver {
	meta.SetResourceCondition(&receiver, meta.ReadyCondition, metav1.ConditionTrue, reason, message)
	receiver.Status.URL = url
	return receiver
}

func ReceiverNotReady(receiver Receiver, reason, message string) Receiver {
	meta.SetResourceCondition(&receiver, meta.ReadyCondition, metav1.ConditionFalse, reason, message)
	return receiver
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
func (in *Receiver) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// +genclient
// +genclient:Namespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Receiver is the Schema for the receivers API
type Receiver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReceiverSpec   `json:"spec,omitempty"`
	Status ReceiverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReceiverList contains a list of Receiver
type ReceiverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Receiver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Receiver{}, &ReceiverList{})
}
