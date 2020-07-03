# Receiver

The `Receiver` API defines a webhook receiver that triggers
reconciliation for a group of resources.

## Specification

```go
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=github;gitlab
	// +required
	Type string `json:"type"`

	// A list of events to handle
	// e.g. 'push' for GitHub or 'Push Hook' for GitLab.
	// +required
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
```

Webhook sender type:

```go
const (
	GitHubWebhook string = "github"
	GitLabWebhook string = "gitlab"
)
```

## Status

```go
type ReceiverStatus struct {
	// Generated webhook URL in the format
	// of '/hook/sha256sum(token+name+namespace)'.
	// +required
	URL string `json:"url"`
}
```

## Implementation

The controller handles the webhook requests on a dedicated port. This port can be used to create
a Kubernetes LoadBalancer Service or Ingress to expose the receiver endpoint outside the cluster.

When a `Receiver` is created, the controller sets the `Receiver`
status to Ready and generates the URL in the format `/hook/sha256sum(token+name+namespace)`.
The `ReceiverReconciler` creates an indexer for the SHA265 digest
so that it can be used as a field selector.

When the controller receives a POST request:
* extract the SHA265 digest from the URL
* loads the `Receiver` using the digest field selector
* extracts the signature from HTTP headers based on `spec.type`
* validates the signature using `status.Token` based on `spec.type`
* extract the event type from the payload 
* triggers a reconciliation for `spec.resources` if the event type matches one of the `spec.events` items
