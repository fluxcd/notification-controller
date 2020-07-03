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

## Example

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Alert
metadata:
  name: webapp
  namespace: gitops-system
spec:
  providerRef: 
    name: on-call-slack
  eventSeverity: info
  eventSources:
    - kind: GitRepository
      name: webapp
    - kind: Kustomization
      name: webapp-backend
    - kind: Kustomization
      name: webapp-frontend
```

The event severity can be set to `info` or `error`. 

To target all resources of a particular kind in a namespace, you can use the `*` wildcard:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Alert
metadata:
  name: all-kustomizations
  namespace: gitops-system
spec:
  providerRef: 
    name: dev-msteams
  eventSeverity: error
  eventSources:
    - kind: Kustomization
      namespace: gitops-system
      name: '*'
  suspend: false
```

If you don't specify an event source namespace, the alert namespace will be used.
