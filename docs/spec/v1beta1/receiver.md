# Receiver

The `Receiver` API defines a webhook receiver that triggers
reconciliation for a group of resources.

## Specification

```go
type ReceiverSpec struct {
	// Type of webhook sender, used to determine
	// the validation procedure and payload deserialization.
	// +kubebuilder:validation:Enum=generic;generic-hmac;github;gitlab;bitbucket;harbor;dockerhub;quay;gcr;nexus
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
	SecretRef meta.LocalObjectReference `json:"secretRef,omitempty"`

	// This flag tells the controller to suspend subsequent events handling.
	// Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}
```

Receiver types:

```go
const (
	GenericReceiver     string = "generic"
	GenericHMACReceiver string = "generic-hmac"
	GitHubReceiver      string = "github"
	GitLabReceiver      string = "gitlab"
	BitbucketReceiver   string = "bitbucket"
	HarborReceiver      string = "harbor"
	DockerHubReceiver   string = "dockerhub"
	QuayReceiver        string = "quay"
	GCRReceiver         string = "gcr"
	NexusReceiver       string = "nexus"
	ACRReceiver         string = "acr"
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

## Public ingress considerations

Considerations should be made when exposing the notification controller receiver
to the public internet. Each request to the receiver endpoint will result in request
to the Kubernetes API as the controller needs to fetch information about the receiver.
The receiver endpoint may be protected with a token but it does not defend against a
situation where a legitimate webhook source starts sending large amounts of requests,
or the token is somehow leaked. This may result in unwanted consequences like the controller
being rate limited by the Kubernetes API, degrading its functionality.

It is therefore a good idea to set rate limits on the ingress resource which exposes
the receiver. If you are using ingress-nginx that can be done by [adding annotations](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#rate-limiting).

## Example

Generate a random string and create a secret with a `token` field:

```sh
TOKEN=$(head -c 12 /dev/urandom | shasum | cut -d ' ' -f1)
echo $TOKEN

kubectl create secret generic webhook-token \
  --from-literal=token=$TOKEN
```

### Generic receiver

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
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp
      namespace: default
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: HelmRepository
      name: webapp
      namespace: default
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: Bucket
      name: webapp
      namespace: default
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
      namespace: default
```

When the receiver type is set to `generic`, the controller will not perform token validation nor event filtering.

### Generic HMAC receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: generic-hmac-receiver
  namespace: default
spec:
  type: generic-hmac
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp
      namespace: default
```

This generic receiver verifies that the request is legitimate using HMAC.
The controller uses the `X-Signature` header to get the hash signature.
The signature should be prefixed with the hash function(`sha1`, `sha256`, or `sha512`) like this:
`<hash-function>=<hash-signation>`.

1. Generate hash signature using OpenSSL:

```sh
printf '<request-body>' | openssl dgst -sha1 -r -hmac "<secret-key>" | awk '{print $1}'
```

You can use the flag `sha256` or `sha512` if you want a different hash function.

2. Send a HTTP POST request to the webhook URL:

```sh
curl <webhook-url> -X POST -H "X-Signature: sha1=<generated-hash>" -d '<request-body>'
```

Generate hash signature using Go:

```go
func sign(payload, key string) string {
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(payload))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// set headers
req.Header.Set("X-Signature", fmt.Sprintf("sha1=%s", sign(payload, key)))
```

### GitHub receiver

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
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: HelmRepository
      name: webapp
```

Note that you have to set the generated token as the GitHub webhook secret value.
The controller uses the `X-Hub-Signature` HTTP header to verify that the request is legitimate.


### Gitea receiver

The Gitea webhook works with the [Github receiver](#github-receiver). You can use the same example
given for the Github receiver.

### GitLab receiver

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
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp-frontend
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp-backend
```

Note that you have to configure the GitLab webhook with the generated token.
The controller uses the `X-Gitlab-Token` HTTP header to verify that the request is legitimate.

### Bitbucket server receiver

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
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: GitRepository
      name: webapp
```

Note that you have to set the generated token as the Bitbucket server webhook secret value.
The controller uses the `X-Hub-Signature` HTTP header to verify that the request is legitimate.

Also note, the *Bitbucket cloud* service does not yet provide any support for signing webhook requests.
([1](https://jira.atlassian.com/browse/BCLOUD-14683), [2](https://jira.atlassian.com/browse/BCLOUD-12195)).
If your repositories are on Bitbucket cloud, you will need to use the Generic receiver instead.

### Harbor receiver

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
    - apiVersion: source.toolkit.fluxcd.io/v1beta1
      kind: HelmRepository
      name: webapp
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
```

Note that you have to set the generated token as the Harbor webhook authentication header.
The controller uses the `Authentication` HTTP header to verify that the request is legitimate.

### DockerHub receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: dockerhub-receiver
  namespace: default
spec:
  type: dockerhub
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
```

### Quay receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: quay-receiver
  namespace: default
spec:
  type: quay
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
```

### Nexus receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: nexus-receiver
  namespace: default
spec:
  type: nexus
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
```

Note that you have to fill in the generated token as the secret key when creating the Nexus Webhook Capability.
See [Nexus Webhook Capability](https://help.sonatype.com/repomanager3/webhooks/enabling-a-repository-webhook-capability)
The controller uses the `X-Nexus-Webhook-Signature` HTTP header to verify that the request is legitimate.

### GCR receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: gcr-receiver
  namespace: default
spec:
  type: gcr
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1alpha1
      kind: ImageRepository
      name: webapp
      namespace: default
```

Note that the controller decodes the JWT from the authorization
header of the push request and verifies it against the GCP API.
For more information, take a look at this
[documentation](https://cloud.google.com/pubsub/docs/push?&_ga=2.123897930.-1945316571.1602156486#authentication_and_authorization).

### ACR receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Receiver
metadata:
  name: acr-receiver
  namespace: default
spec:
  type: acr
  secretRef:
    name: webhook-token
  resources:
    - kind: ImageRepository
      name: webapp
```

Note that the controller doesn't verify the authenticity of the request as Azure doesn't provide any mechanism for verification.
You can take a look at the [Azure Container webhook reference](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-webhook-reference).
