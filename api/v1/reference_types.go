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

// CrossNamespaceObjectReference contains enough information to let you locate the
// typed referenced object at cluster level
type CrossNamespaceObjectReference struct {
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent
	// +kubebuilder:validation:Enum=Bucket;GitRepository;Kustomization;HelmRelease;HelmChart;HelmRepository;ImageRepository;ImagePolicy;ImageUpdateAutomation;OCIRepository
	// +required
	Kind string `json:"kind"`

	// Name of the referent
	// If multiple resources are targeted `*` may be set.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=53
	// +required
	Name string `json:"name"`

	// Namespace of the referent
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=53
	// +kubebuilder:validation:Optional
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// MatchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
	// map is equivalent to an element of matchExpressions, whose key field is "key", the
	// operator is "In", and the values array contains only "value". The requirements are ANDed.
	// MatchLabels requires the name to be set to `*`.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
