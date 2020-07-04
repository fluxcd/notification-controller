# Receiver

The `Receiver` API defines a webhook receiver that triggers
reconciliation for a group of resources.

## Specification

```go
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=generic;github;gitlab
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
```

Receiver types:

```go
const (
	GenericReceiver string = "generic"
	GitHubReceiver  string = "github"
	GitLabReceiver  string = "gitlab"
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

## Example

Generate a random string and create a secret with a `token` field:

```sh
TOKEN=$(head -c 12 /dev/urandom | shasum | cut -d ' ' -f1)
echo $TOKEN

kubectl -n gitops-system create secret generic webhook-token \	
--from-literal=token=$TOKEN
```

GitHub receiver:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Receiver
metadata:
  name: github-receiver
  namespace: gitops-system
spec:
  type: github
  events:
    - "ping"
    - "push"
  secretRef:
    name: webhook-token
  resources:
    - kind: GitRepository
      name: webapp
    - kind: HelmRepository
      name: webapp
```

GitLab receiver:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Receiver
metadata:
  name: gitlab-receiver
  namespace: gitops-system
spec:
  type: gitlab
  events:
    - "Push Hook"
    - "Tag Push Hook"
  secretRef:
    name: webhook-token
  resources:
    - kind: GitRepository
      name: webapp-frontend
    - kind: GitRepository
      name: webapp-backend
```

Generic receiver:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Receiver
metadata:
  name: ci-receiver
  namespace: gitops-system
spec:
  type: generic
  secretRef:
    name: webhook-token
  resources:
    - kind: GitRepository
      name: webapp
    - kind: HelmRepository
      name: webapp
```

When the receiver type is set to `generic`, the controller will not perform token validation nor event filtering.
