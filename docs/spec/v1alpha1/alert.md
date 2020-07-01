# Alert

The `Alert` API defines how events are filtered by severity and involved object, and what provider to use for dispatching.

## Specification

Spec:

```go
type AlertSpec struct {
	// Send events using this provider
	// +required
	ProviderRef corev1.ObjectReference `json:"providerRef"`

	// Filter events based on severity, defaults to ('info').
	// +kubebuilder:validation:Enum=info;error
	// +optional
	EventSeverity string `json:"eventSeverity,omitempty"`

	// Filter events based on the involved objects
	// +required
	EventSources []CrossNamespaceObjectReference `json:"eventSources"`

	// This flag tells the controller to suspend subsequent events dispatching.
	// Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}
```

Status:

```go
// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}
```

Status condition types:

```go
const (
	// ReadyCondition represents the fact that a given object has passed
	// validation and was acknowledge by the controller.
	ReadyCondition string = "Ready"
)
```
