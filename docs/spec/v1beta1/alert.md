# Alert

The `Alert` API defines how events are filtered by severity and involved object, and what provider to use for dispatching.

## Specification

Spec:

```go
type AlertSpec struct {
	// Send events using this provider.
	// +required
	ProviderRef meta.LocalObjectReference `json:"providerRef"`

	// Filter events based on severity, defaults to ('info').
	// +kubebuilder:validation:Enum=info;error
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
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Alert
metadata:
  name: webapp
  namespace: default
spec:
  providerRef: 
    name: on-call-slack
  eventSeverity: info
  eventSources:
    - kind: GitRepository
      name: webapp
    - kind: Bucket
      name: secrets
    - kind: Kustomization
      name: webapp-backend
    - kind: Kustomization
      name: webapp-frontend
```

The event severity can be set to `info` or `error`. 

To target all resources of a particular kind in a namespace, you can use the `*` wildcard:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Alert
metadata:
  name: all-kustomizations
  namespace: default
spec:
  providerRef: 
    name: dev-msteams
  eventSeverity: error
  eventSources:
    - kind: Kustomization
      namespace: default
      name: '*'
  suspend: false
```

If you don't specify an event source namespace, the alert namespace will be used.

> **Note** that on multi-tenant clusters, platform admins can disable cross-namespace references
> with the `--no-cross-namespace-refs=true` flag. When this flag is set, alerts can only refer to
> event sources in the same namespace as the alert object,
> preventing tenants from subscribing to another tenant's events.

You can add a summary to describe the impact of an event:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Alert
metadata:
  name: ingress
  namespace: nginx
spec:
  summary: "Ingress traffic affected in production (us-west-2)"
  providerRef: 
    name: on-call-slack
  eventSeverity: error
  eventSources:
    - kind: HelmRelease
      name: nginx-ingress
```

Skip alerting if the message matches a [Go regex](https://golang.org/pkg/regexp/syntax)
from the exclusion list:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Alert
metadata:
  name: flux-system
  namespace: flux-system
spec:
  providerRef: 
    name: on-call-slack
  eventSeverity: error
  eventSources:
    - kind: GitRepository
      name: flux-system
  exclusionList:
    - "waiting.*socket"
```

The above definition will not send alerts for transient Git clone errors like:

```
unable to clone 'ssh://git@ssh.dev.azure.com/v3/...', error: SSH could not read data: Error waiting on socket
```
