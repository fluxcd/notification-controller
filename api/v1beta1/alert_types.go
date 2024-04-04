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
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AlertKind string = "Alert"
)

// AlertSpec defines an alerting rule for events involving a list of objects
type AlertSpec struct {
	// Send events using this provider.
	// +required
	ProviderRef meta.LocalObjectReference `json:"providerRef"`

	// Filter events based on severity, defaults to ('info').
	// If set to 'info' no events will be filtered.
	// +kubebuilder:validation:Enum=info;error
	// +kubebuilder:default:=info
	// +optional
	EventSeverity string `json:"eventSeverity,omitempty"`

	// Filter events based on the involved objects.
	// +required
	EventSources []CrossNamespaceObjectReference `json:"eventSources"`

	// A list of Golang regular expressions to be used for excluding messages.
	// +optional
	ExclusionList []string `json:"exclusionList,omitempty"`

	// Short description of the impact and affected cluster.
	// +optional
	Summary string `json:"summary,omitempty"`

	// This flag tells the controller to suspend subsequent events dispatching.
	// Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// AlertStatus defines the observed state of Alert
type AlertStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="v1beta1 Alert is deprecated, upgrade to v1beta3"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// Alert is the Schema for the alerts API
type Alert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AlertSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status AlertStatus `json:"status,omitempty"`
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
// Deprecated: use GetConditions instead.
func (in *Alert) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// GetConditions returns the status conditions of the object.
func (in *Alert) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Alert) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// AlertList contains a list of Alert
type AlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Alert `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Alert{}, &AlertList{})
}
