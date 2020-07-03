package v1alpha1

// CrossNamespaceObjectReference contains enough information to let you locate the
// typed referenced object at cluster level
type CrossNamespaceObjectReference struct {
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent
	// +kubebuilder:validation:Enum=GitRepository;Kustomization;HelmRelease;HelmChart;HelmRepository
	// +required
	Kind string `json:"kind,omitempty"`

	// Name of the referent
	// +required
	Name string `json:"name"`

	// Namespace of the referent
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
