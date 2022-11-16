# Incoming Webhook Receivers

The `Receiver` API defines an incoming webhook receiver that triggers
the reconciliation for a group of Flux Custom Resources.

## Example

The following is an example of how to configure an incoming webhook for the GitHub repository where
Flux was bootstrapped with `flux bootstrap github`. After a Git push, GitHub will send a push event to
notification-controller, which in turn tells Flux to pull and apply the latest changes from upstream.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: github-receiver
  namespace: flux-system
spec:
  type: github
  events:
    - "ping"
    - "push"
  secretRef:
    name: receiver-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: flux-system
```

In the above example:

- A Receiver named `github-receiver` is created, indicated by the
  `.metadata.name` field.
- The notification-controller generates a unique webhook path using the Receiver
  name, namespace and the token from the referenced `.spec.secretRef.name` secret.
- The incoming webhook path is reported in the `.status.webhookPath` field.
- When a GitHub push event is received, the controller verifies the that the
  request is legitimate using HMAC and the `X-Hub-Signature` HTTP header.
- If the event type matches `.spec.events` and the payload is verified, then the controller
  triggers a reconciliation for the `flux-system` GitRepository which is listed under `.spec.resouces`.

You can run this example by saving the manifest into `github-receiver.yaml`.

1. Generate a random string and create a secret with a `token` field:

   ```sh
   TOKEN=$(head -c 12 /dev/urandom | shasum | cut -d ' ' -f1)

   kubectl -n flux-system create secret generic receiver-token \
     --from-literal=token=$TOKEN
   ```

2. Apply the resource on the cluster:

   ```sh
   kubectl -n flux-system apply -f github-receiver.yaml
   ```

3. Run `kubectl -n flux-system describe receiver github-receiver` to see its status:

   ```console
   ...
   Status:
     Conditions:
       Last Transition Time:  2022-11-16T23:43:38Z
       Message:               Receiver initialised for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b
       Observed Generation:   1
       Reason:                Succeeded
       Status:                True
       Type:                  Ready
     Observed Generation:     1
     Webhook Path:            /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b
   Events:
     Type    Reason    Age   From                     Message
     ----    ------    ----  ----                     -------
     Normal  Succeeded 82s   notification-controller  Reconciliation finished, next run in 10m
   ```

4. Run `kubectl -n flux-system get receivers` to see the generated webhook path:

   ```console
   NAME              READY   STATUS                                                                        
   github-receiver   True    Receiver initialised for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b
   ```

5. On GitHub, navigate to your repository and click on the "Add webhook" button under "Settings/Webhooks".
   Fill the form with:
   - **Payload URL**: compose the address using the receiver ingress hostname and the generated path `https://<hostname>/<webhookPath>`.
   - **Secret**: use the token string

## Writing a Receiver spec

As with all other Kubernetes config, a Receiver needs `apiVersion`,
`kind`, and `metadata` fields. The name of a Receiver object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).
A Receiver also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Type

`.spec.type` is a required field that specifies which SaaS API to use.

The supported receiver types are:

| Receiver                                            | Type           |
|-----------------------------------------------------|----------------|
| [Generic webhook](#generic-receiver)                | `generic`      |
| [Generic webhook with HMAC](#generic-hmac-receiver) | `generic-hmac` |
| [GitHub](#github-receiver)                          | `github`       |
| [Gitea](#github-receiver)                           | `github`       |
| [GitLab](#gitlab-receiver)                          | `gitlab`       |
| [Bitbucket server](#bitbucket-server-receiver)      | `bitbucket`    |
| [Harbor](#harbor-receiver)                          | `harbor`       |
| [DockerHub](#dockerhub-receiver)                    | `dockerhub`    |
| [Quay](#quay-receiver)                              | `quay`         |
| [Nexus](#nexus-receiver)                            | `nexus`        |
| [Azure Container Registry](#acr-receiver)           | `acr`          |
| [Google Container Registry](#gcr-receiver)          | `gcr`          |

### Events filtering

`.spec.events` in an optional field to specify a list of event types
that this Receiver should handle. If left empty, all events are handled.

### Resources

`.spec.resources` is a required field to specify which Flux Custom Resources
should be reconciled when an event is received.

A resource entry must contain the following fields:
- `apiVersion` is the Flux Custom Resource API group and version such as `source.toolkit.fluxcd.io/v1beta2`.
- `kind` is the Flux Custom Resource `.kind` such as GitRepository, OCIRepository, HelmRepository and Bucket.
- `name` is the Flux Custom Resource `.metadata.name`.
- `namespace` is the Flux Custom Resource `.metadata.namespace`.
  When not specified, the Receiver `.metadata.namespace` is used instead.

#### Disable cross-namespace selectors

**Note:** On multi-tenant clusters, platform admins can disable cross-namespace references with the
`--no-cross-namespace-refs=true` flag. When this flag is set, Receivers can only refer to resources
in the same namespace as the alert object, preventing tenants from triggering reconciliations
to another tenant's resources.

### Secret reference

`.spec.secretRef.name` is a required field to specify a name reference to a
Secret in the same namespace as the Receiver, containing the secret token.

### Interval

`.spec.interval` is a required field with a default of ten minutes that specifies
the time interval at which the controller reconciles the provider with its Secret
references.

### Suspend

`.spec.suspend` is an optional field to suspend the receiver.
When set to `true`, the controller will stop processing events for this receiver.
When the field is set to `false` or removed, it will resume.

## Public ingress considerations

Considerations should be made when exposing the controller's `webhook-receiver` Kubernetes Service
to the public internet. Each request to the receiver endpoint will result in request to the Kubernetes
API as the controller needs to fetch information about the receiver. The receiver endpoint may be
protected with a token, but it does not defend against a situation where a legitimate webhook source
starts sending large amounts of requests, or the token is somehow leaked.
This may result in unwanted consequences for the controller,
as it may get rate limited by the Kubernetes API, degrading its functionality.

It is therefore a good idea to set rate limits on the ingress resource which exposes the receiver.
If you are using ingress-nginx that can be done by
[adding annotations](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#rate-limiting).

## Working with Receivers

### Generic receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: generic-receiver
  namespace: default
spec:
  type: generic
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: webapp
      namespace: default
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: HelmRepository
      name: webapp
      namespace: default
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: Bucket
      name: webapp
      namespace: default
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
      kind: ImageRepository
      name: webapp
      namespace: default
```

When the receiver type is set to `generic`, the controller will not perform token validation nor event filtering.

### Generic HMAC receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: generic-hmac-receiver
  namespace: default
spec:
  type: generic-hmac
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
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
apiVersion: notification.toolkit.fluxcd.io/v1beta2
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
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: webapp
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: HelmRepository
      name: webapp
```

Note that you have to set the generated token as the GitHub webhook secret value.
The controller uses the `X-Hub-Signature` HTTP header to verify that the request is legitimate.

### Gitea receiver

The Gitea webhook works with the [GitHub receiver](#github-receiver). You can use the same example
given for the Github receiver.

### GitLab receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
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
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: webapp-frontend
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: webapp-backend
```

Note that you have to configure the GitLab webhook with the generated token.
The controller uses the `X-Gitlab-Token` HTTP header to verify that the request is legitimate.

### Bitbucket server receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
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
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
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
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: harbor-receiver
  namespace: default
spec:
  type: harbor
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: HelmRepository
      name: webapp
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
      kind: ImageRepository
      name: webapp
```

Note that you have to set the generated token as the Harbor webhook authentication header.
The controller uses the `Authentication` HTTP header to verify that the request is legitimate.

### DockerHub receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: dockerhub-receiver
  namespace: default
spec:
  type: dockerhub
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
      kind: ImageRepository
      name: webapp
```

### Quay receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: quay-receiver
  namespace: default
spec:
  type: quay
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
      kind: ImageRepository
      name: webapp
```

### Nexus receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: nexus-receiver
  namespace: default
spec:
  type: nexus
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
      kind: ImageRepository
      name: webapp
```

Note that you have to fill in the generated token as the secret key when creating the Nexus Webhook Capability.
See [Nexus Webhook Capability](https://help.sonatype.com/repomanager3/webhooks/enabling-a-repository-webhook-capability)
The controller uses the `X-Nexus-Webhook-Signature` HTTP header to verify that the request is legitimate.

### GCR receiver

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Receiver
metadata:
  name: gcr-receiver
  namespace: default
spec:
  type: gcr
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta1
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
apiVersion: notification.toolkit.fluxcd.io/v1beta2
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

Note that the controller doesn't verify the authenticity of the request as Azure does not provide any mechanism for verification.
You can take a look at the [Azure Container webhook reference](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-webhook-reference).

## Receiver Status

### Conditions

An Receiver enters various states during its lifecycle, reflected as
[Kubernetes Conditions][typical-status-properties].
It can be [ready](#ready-receiver), or it can [fail during
reconciliation](#failed-receiver).

The Receiver API is compatible with the [kstatus specification][kstatus-spec],
and reports the `Reconciling` condition where applicable.

#### Ready Receiver

The notification-controller marks a Receiver as _ready_ when it has the following
characteristics:

- The Receiver's Secret referenced in `.spec.secretRef.name` is found on the cluster.
- The Receiver's Secret contains a `token` key.

When the Receiver is "ready", the controller sets a Condition with the following
attributes in the Alert's `.status.conditions`:

- `type: Ready`
- `status: "True"`
- `reason: Succeeded`

#### Failed Receiver

The notification-controller may get stuck trying to reconcile a Receiver if its secret token
can't be found.

When this happens, the controller sets the `Ready` Condition status to `False`,
and adds a Condition with the following attributes:

- `type: Reconciling`
- `status: "True"`
- `reason: ProgressingWithRetry`

### Observed Generation

The notification-controller reports an
[observed generation][typical-status-properties]
in the Receiver's `.status.observedGeneration`. The observed generation is the
latest `.metadata.generation` which resulted in a [ready state](#ready-receiver).

### Last Handled Reconcile At

The notification-controller reports the last `reconcile.fluxcd.io/requestedAt`
annotation value it acted on in the `.status.lastHandledReconcileAt` field.

### Webhook Path

When a Receiver becomes [ready](#ready-receiver), the controller reports the generated
incoming webhook path under `.status.webhookPath`.
The path format is `/hook/sha256sum(token+name+namespace)`.

[typical-status-properties]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
[kstatus-spec]: https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
