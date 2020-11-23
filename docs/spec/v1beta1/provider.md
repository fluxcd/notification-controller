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

	// HTTP/S webhook address of this provider
	// +kubebuilder:validation:Pattern="^(http|https)://"
	// +optional
	Address string `json:"address,omitempty"`

	// HTTP/S address of the proxy
	// +kubebuilder:validation:Pattern="^(http|https)://"
	// +optional
	Proxy string `json:"proxy,omitempty"`

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
* Bitbucket
* Azure DevOps

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
  # HTTP(S) proxy (optional)
  proxy: https://proxy.corp:8080
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

The GitHub, GitLab, Bitbucket, and Azure DevOps provider is a special kind of notification
provider that based on the state of a Kustomization resource,
will update the commit status for the reconciled commit id.

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
    name: api-token
```

GitHub. GitLab, and Azure DevOps use personal access tokens to authenticate with their API.
* [GitHub personal access token](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token)
* [GitLab personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
* [Azure DevOps personal access token](https://docs.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=preview-page)
Both provider types require a secret in the same format, with the personal access token as the value for the token key.
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <personal-access-tokens>
```

Bitbucket authenticates using an [app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/). It requires
both the username and the password when authenticating. There for the token needs to be passed with the format `<username>:<app-password>`.
A token that is not in this format will cause the provider to fail.
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <username>:<app-password>
```
