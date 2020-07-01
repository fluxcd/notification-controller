# Provider

The `Provider` API defines how events are encoded and the webhook address where they are dispatched.

## Specification

Spec:

```go
type ProviderSpec struct {
	// Type of provider
	// +kubebuilder:validation:Enum=slack;discord;msteams;rocket;webhook
	// +required
	Type string `json:"type"`

	// Alert channel for this provider
	// +optional
	Channel string `json:"channel,omitempty"`

	// Bot username for this provider
	// +optional
	Username string `json:"username,omitempty"`

	// HTTP(S) webhook address of this provider
	// +optional
	Address string `json:"address,omitempty"`

	// Secret reference containing the provider webhook URL
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}
```

Notification providers:

* Slack
* Discord
* Microsoft Teams
* Rocket
* Generic webhook

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
