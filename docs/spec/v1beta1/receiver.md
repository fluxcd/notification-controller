# Receiver

The `Receiver` API defines a webhook receiver that triggers
reconciliation for a group of resources.

## Specification

```go
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=generic;github;gitlab;harbor
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
	GenericReceiver   string = "generic"
	GitHubReceiver    string = "github"
	GitLabReceiver    string = "gitlab"
	BitbucketReceiver string = "bitbucket"
	HarborReceiver    string = "harbor"
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

kubectl create secret generic webhook-token \
  --from-literal=token=$TOKEN
```

GitHub receiver:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: github-receiver
  namespace: default
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

Note that you have to set the generated token as the GitHub webhook secret value.
The controller uses the `X-Hub-Signature` HTTP header to verify that the request is legitimate.

GitLab receiver:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: gitlab-receiver
  namespace: default
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

Note that you have to configure the GitLab webhook with the generated token.
The controller uses the `X-Gitlab-Token` HTTP header to verify that the request is legitimate.

Bitbucket server receiver:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: bitbucket-receiver
  namespace: default
spec:
  type: bitbucket
  events:
    - "repo:refs_changed"
  secretRef:
    name: webhook-token
  resources:
    - kind: GitRepository
      name: webapp
```

Note that you have to set the generated token as the Bitbucket server webhook secret value.
The controller uses the `X-Hub-Signature` HTTP header to verify that the request is legitimate.

Harbor receiver:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: harbor-receiver
  namespace: default
spec:
  type: harbor
  secretRef:
    name: webhook-token
  resources:
    - kind: HelmRepository
      name: webapp
```

Note that you have to set the generated token as the Harbor webhook authentication header.
The controller uses the `Authentication` HTTP header to verify that the request is legitimate.

Generic receiver:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: generic-receiver
  namespace: default
spec:
  type: generic
  secretRef:
    name: webhook-token
  resources:
    - kind: GitRepository
      name: webapp
    - kind: HelmRepository
      name: webapp
    - kind: Bucket
      name: secrets
```

When the receiver type is set to `generic`, the controller will not perform token validation nor event filtering.
