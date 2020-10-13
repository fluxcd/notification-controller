# Provider

The `Provider` API defines how events are encoded and the webhook address where they are dispatched.

## Specification

Spec:

```go
type ProviderSpec struct {
	// Type of provider
	// +kubebuilder:validation:Enum=slack;discord;msteams;rocket;generic;github;gitlab
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

Git commit status providers:

* GitHub
* GitLab

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

### Notifications

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: slack
  namespace: default
spec:
  type: slack
  channel: general
  # webhook address (ignored if secretRef is specified)
  address: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
  # secret containing the webhook address (optional)
  secretRef:
    name: webhook-url
```

Webhook URL secret:

```sh
kubectl create secret generic webhook-url \
--from-literal=address=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
```

Note that the secret must contain an `address` field.

The provider type can be: `slack`, `msteams`, `rocket`, `discord`, `github` or `generic`.

When type `generic` is specified, the notification controller will post the
incoming [event](event.md) in JSON format to the webhook address.

### Git commit status

The GitHub/GitLab provider is a special kind of notification provider
that based on the state of a Kustomization resource,
will update the commit status for the currently reconciled commit id.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: podinfo
  namespace: default
spec:
  # provider type can be github or gitlab
  type: github
  address: https://github.com/stefanprodan/podinfo
  secretRef:
    name: git-api-token
```

The secret referenced in the provider is expected to contain a
personal access token to authenticate with the GitHub or GitLab API.

```sh
kubectl create secret generic git-api-token \
--from-literal=token=YOUR-TOKEN
```
