# Receivers

<!-- menuweight:30 -->

The `Receiver` API defines an incoming webhook receiver that triggers the
reconciliation for a group of Flux Custom Resources.

## Example

The following is an example of how to configure an incoming webhook for the
GitHub repository where Flux was bootstrapped with `flux bootstrap github`.
After a Git push, GitHub will send a push event to notification-controller,
which in turn tells Flux to pull and apply the latest changes from upstream.

**Note:** The following assumes an Ingress exposes the controller's
`webhook-receiver` Kubernetes Service. How to configure the Ingress is out of
scope for this example.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
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
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: flux-system
```

In the above example:

- A Receiver named `github-receiver` is created, indicated by the
  `.metadata.name` field.
- The notification-controller generates a unique webhook path using the
  Receiver name, namespace and the token from the referenced
  `.spec.secretRef.name` secret.
- The incoming webhook path is reported in the `.status.webhookPath` field.
- When a GitHub push event is received, the controller verifies the payload's
  integrity and authenticity, using [HMAC][] and the `X-Hub-Signature` HTTP
  header.
- If the event type matches `.spec.events` and the payload is verified, then
  the controller triggers a reconciliation for the `flux-system` GitRepository
  which is listed under `.spec.resources`.

You can run this example by saving the manifest into `github-receiver.yaml`.

1. Generate a random string and create a Secret with a `token` field:

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

5. On GitHub, navigate to your repository and click on the "Add webhook" button
   under "Settings/Webhooks". Fill the form with:

   - **Payload URL**: The composed address, consisting of the Ingress' hostname
     exposing the controller's `webhook-receiver` Kubernetes Service, and the
     generated path for the Receiver. For this example:
     `https://<hostname>/hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b`
   - **Secret**: The `token` string generated in step 1.

## Writing a Receiver spec

As with all other Kubernetes config, a Receiver needs `apiVersion`,
`kind`, and `metadata` fields. The name of a Receiver object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).

A Receiver also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Type

`.spec.type` is a required field that specifies how the controller should
handle the incoming webhook request.

#### Supported Receiver types

| Receiver                                   | Type           | Supports filtering using [Events](#events) |
| ------------------------------------------ | -------------- | ------------------------------------------ |
| [Generic webhook](#generic)                | `generic`      | ❌                                          |
| [Generic webhook with HMAC](#generic-hmac) | `generic-hmac` | ❌                                          |
| [GitHub](#github)                          | `github`       | ✅                                          |
| [Gitea](#github)                           | `github`       | ✅                                          |
| [GitLab](#gitlab)                          | `gitlab`       | ✅                                          |
| [Bitbucket server](#bitbucket-server)      | `bitbucket`    | ✅                                          |
| [Harbor](#harbor)                          | `harbor`       | ❌                                          |
| [DockerHub](#dockerhub)                    | `dockerhub`    | ❌                                          |
| [Quay](#quay)                              | `quay`         | ❌                                          |
| [Nexus](#nexus)                            | `nexus`        | ❌                                          |
| [Azure Container Registry](#acr)           | `acr`          | ❌                                          |
| [Google Container Registry](#gcr)          | `gcr`          | ❌                                          |

#### Generic

When a Receiver's `.spec.type` is set to `generic`, the controller will respond
to any HTTP request to the generated [`.status.webhookPath` path](#webhook-path),
and request a reconciliation for all listed [Resources](#resources).

**Note:** This type of Receiver does not perform any validation on the incoming
request, and it does not support filtering using [Events](#events).

##### Generic example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: generic-receiver
  namespace: default
spec:
  type: generic
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp
      namespace: default
```

#### Generic HMAC

When a Receiver's `.spec.type` is set to `generic-hmac`, the controller will
respond to any HTTP request to the generated [`.status.webhookPath` path](#webhook-path),
while verifying the request's payload integrity and authenticity using [HMAC][].

The controller uses the `X-Signature` header to get the hash signature. This
signature should be prefixed with the hash function (`sha1`, `sha256` or
`sha512`) used to generate the signature, in the following format:
`<hash-function>=<hash>`.

To validate the HMAC signature, the controller will use the `token` string
from the [Secret reference](#secret-reference) to generate a hash signature
using the same hash function as the one specified in the `X-Signature` header.

If the generated hash signature matches the one specified in the `X-Signature`
header, the controller will request a reconciliation for all listed
[Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events).

##### Generic HMAC example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: generic-hmac-receiver
  namespace: default
spec:
  type: generic-hmac
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp
      namespace: default
```

##### HMAC signature generation example

1. Generate the HMAC hash for the request body using OpenSSL:

   ```sh
   printf '<request-body>' | openssl dgst -sha1 -r -hmac "<token>" | awk '{print $1}'
   ```

   You can replace the `-sha1` flag with `-sha256` or `-sha512` to use a
   different hash function.

2. Send an HTTP POST request with the body and the HMAC hash to the webhook URL:

   ```sh
   curl <webhook-url> -X POST -H "X-Signature: <hash-function>=<generated-hash>" -d '<request-body>'
   ```

#### GitHub

When a Receiver's `.spec.type` is set to `github`, the controller will respond
to an [HTTP webhook event payload](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads)
from GitHub to the generated [`.status.webhookPath` path](#webhook-path),
while verifying the payload using [HMAC][].

The controller uses the [`X-Hub-Signature` header](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#delivery-headers)
from the request made by GitHub to get the hash signature. To enable the
inclusion of this header, the `token` string from the [Secret reference](#secret-reference)
must be configured as the [secret token for the
webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/securing-your-webhooks#setting-your-secret-token).

The controller will calculate the HMAC hash signature for the received request
payload using the same `token` string, and compare it with the one specified in
the header. If the two signatures match, the controller will request a
reconciliation for all listed [Resources](#resources).

This type of Receiver offers the ability to filter incoming events by comparing
the `X-GitHub-Event` header to the list of [Events](#events).
For a list of available events, see the [GitHub
documentation](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads).

##### GitHub example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
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
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp
```

The above example makes use of the [`.spec.events` field](#events) to filter
incoming events from GitHub, instructing the controller to only respond to
[`ping`](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#ping)
and [`push`](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#push)
events.

#### Gitea

For Gitea, the `.spec.type` field can be set to `github` as it produces [GitHub
type](#github) compatible [webhook event payloads](https://docs.gitea.io/en-us/webhooks/).

**Note:** While the payloads are compatible with the GitHub type, the number of
available events may be limited and/or different from the ones available in
GitHub. Refer to the [Gitea source code](https://github.com/go-gitea/gitea/blob/main/models/webhook/hooktask.go#L28)
to see the list of available [events](#events).

#### GitLab

When a Receiver's `.spec.type` is set to `gitlab`, the controller will respond
to an [HTTP webhook event payload](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#events)
from GitLab to the generated [`.status.webhookPath` path](#webhook-path).

The controller validates the payload's authenticity by comparing the
[`X-Gitlab-Token` header](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#validate-payloads-by-using-a-secret-token)
from the request made by GitLab to the `token` string from the [Secret
reference](#secret-reference). To enable the inclusion of this header, the
`token` string must be configured as the "Secret token" while [configuring a
webhook in GitLab](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#configure-a-webhook-in-gitlab).

If the two tokens match, the controller will request a reconciliation for all
listed [Resources](#resources).

This type of Receiver offers the ability to filter incoming events by comparing
the `X-Gitlab-Event` header to the list of [Events](#events). For a list of
available webhook types, refer to the [GitLab
documentation](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html).

##### GitLab example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
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
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp-frontend
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp-backend
```

The above example makes use of the [`.spec.events` field](#events) to filter
incoming events from GitLab, instructing the controller to only respond to
[`Push Hook`](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#push-events)
and [`Tag Push Hook`](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#tag-events)
events.

#### Bitbucket Server

When a Receiver's `.spec.type` is set to `bitbucket`, the controller will
respond to an [HTTP webhook event payload](https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html)
from Bitbucket Server to the generated [`.status.webhookPath` path](#webhook-path),
while verifying the payload's integrity and authenticity using [HMAC][].

The controller uses the [`X-Hub-Signature` header](https://confluence.atlassian.com/bitbucketserver/manage-webhooks-938025878.html#Managewebhooks-webhooksecrets)
from the request made by BitBucket Server to get the hash signature. To enable
the inclusion of this header, the `token` string from the [Secret
reference](#secret-reference) must be configured as the "Secret" while creating
a webhook in Bitbucket Server.

The controller will calculate the HMAC hash signature for the received request
payload using the same `token` string, and compare it with the one specified in
the header. If the two signatures match, the controller will request a
reconciliation for all listed [Resources](#resources).

This type of Receiver offers the ability to filter incoming events by comparing
the `X-Event-Key` header to the list of [Events](#events). For a list of
available event keys, refer to the [Bitbucket Server
documentation](https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html#Eventpayload-Repositoryevents).

**Note:** Bitbucket Cloud does not support signing webhook requests
([BCLOUD-14683](https://jira.atlassian.com/browse/BCLOUD-14683),
[BCLOUD-12195](https://jira.atlassian.com/browse/BCLOUD-12195)). If your
repositories are on Bitbucket Cloud, you will need to use a [Generic
Receiver](#generic) instead.

##### Bitbucket Server example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
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
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      name: webapp
```

The above example makes use of the [`.spec.events` field](#events) to filter
incoming events from Bitbucket Server, instructing the controller to only
respond to [`repo:refs_changed` (Push)](https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html#Eventpayload-Push)
events.

#### Harbor

When a Receiver's `.spec.type` is set to `harbor`, the controller will respond
to an [HTTP webhook event payload](https://goharbor.io/docs/latest/working-with-projects/project-configuration/configure-webhooks/#payload-format)
from Harbor to the generated [`.status.webhookPath` path](#webhook-path).

The controller validates the payload's authenticity by comparing the
`Authorization` header from the request made by Harbor to the `token` string
from the [Secret reference](#secret-reference). To enable the inclusion of this
header, the `token` string must be configured as the "Auth Header" while
[configuring a webhook in
Harbor](https://goharbor.io/docs/latest/working-with-projects/project-configuration/configure-webhooks/#configure-webhooks).

If the two tokens match, the controller will request a reconciliation for all
listed [Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events). However, Harbor does support configuring event types for
which a webhook will be triggered.

##### Harbor example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: harbor-receiver
  namespace: default
spec:
  type: harbor
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta2
      kind: ImageRepository
      name: webapp
```

#### DockerHub

When a Receiver's `.spec.type` is set to `dockerhub`, the controller will
respond to an [HTTP webhook event payload](https://docs.docker.com/docker-hub/webhooks/)
from DockerHub to the generated [`.status.webhookPath` path](#webhook-path).

The controller performs minimal validation of the payload by attempting to
unmarshal the [JSON request body](https://docs.docker.com/docker-hub/webhooks/#example-webhook-payload).
If the unmarshalling is successful, the controller will request a reconciliation
for all listed [Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events).

##### DockerHub example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: dockerhub-receiver
  namespace: default
spec:
  type: dockerhub
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta2
      kind: ImageRepository
      name: webapp
```

#### Quay

When a Receiver's `.spec.type` is set to `quay`, the controller will respond to
an HTTP [Repository Push Notification payload](https://docs.quay.io/guides/notifications.html#repository-push)
from Quay to the generated [`.status.webhookPath` path](#webhook-path).

The controller performs minimal validation of the payload by attempting to
unmarshal the JSON request body to the expected format. If the unmarshalling is
successful, the controller will request a reconciliation for all listed
[Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events). In addition, it does not support any "Repository
Notification" other than "Repository Push".

##### Quay example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: quay-receiver
  namespace: default
spec:
  type: quay
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta2
      kind: ImageRepository
      name: webapp
```

#### Nexus

When a Receiver's `.spec.type` is set to `nexus`, the controller will respond
to an [HTTP webhook event payload](https://help.sonatype.com/repomanager3/integrations/webhooks/example-headers-and-payloads)
from Nexus Repository Manager 3 to the generated [`.status.webhookPath`
path](#webhook-path), while verifying the payload's integrity and
authenticity using [HMAC][].

The controller validates the payload by comparing the
[`X-Nexus-Webhook-Signature` header](https://help.sonatype.com/repomanager3/integrations/webhooks/working-with-hmac-payloads)
from the request made by Nexus to the `token` string from the [Secret
reference](#secret-reference). To enable the inclusion of this header, the
`token` string must be configured as the "Secret Key" while [enabling a
repository webhook capability](https://help.sonatype.com/repomanager3/integrations/webhooks/enabling-a-repository-webhook-capability).

The controller will calculate the HMAC hash signature for the received request
payload using the same `token` string, and compare it with the one specified in
the header. If the two signatures match, the controller will attempt to
unmarshal the request body to the expected format. If the unmarshalling is
successful, the controller will request a reconciliation for all listed
[Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events).

##### Nexus example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: nexus-receiver
  namespace: default
spec:
  type: nexus
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta2
      kind: ImageRepository
      name: webapp
```

#### GCR

When a Receiver's `.spec.type` is set to `gcr`, the controller will respond to
an [HTTP webhook event payload](https://cloud.google.com/container-registry/docs/configuring-notifications#notification_examples)
from Google Cloud Registry to the generated [`.status.webhookPath`](#webhook-path),
while verifying the payload is legitimate using [JWT](https://cloud.google.com/pubsub/docs/push#authentication).

The controller verifies the request originates from Google by validating the 
token from the [`Authorization` header](https://cloud.google.com/pubsub/docs/push#validate_tokens).
For this to work, authentication must be enabled for the Pub/Sub subscription,
refer to the [Google Cloud documentation](https://cloud.google.com/pubsub/docs/push#configure_for_push_authentication)
for more information.

When the verification succeeds, the request payload is unmarshalled to the
expected format. If this is successful, the controller will request a
reconciliation for all listed [Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events).

##### GCR example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: gcr-receiver
  namespace: default
spec:
  type: gcr
  secretRef:
    name: webhook-token
  resources:
    - apiVersion: image.toolkit.fluxcd.io/v1beta2
      kind: ImageRepository
      name: webapp
      namespace: default
```

#### ACR

When a Receiver's `.spec.type` is set to `acr`, the controller will respond to
an [HTTP webhook event payload](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-webhook-reference),
from Azure Container Registry to the generated [`.status.webhookPath`](#webhook-path).

The controller performs minimal validation of the payload by attempting to
unmarshal the JSON request body. If the unmarshalling is successful, the
controller will request a reconciliation for all listed [Resources](#resources).

**Note:** This type of Receiver does not support filtering using
[Events](#events). However, Azure Container Registry does [support configuring
webhooks to only send events for specific actions](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-webhook#create-webhook---azure-portal).

##### ACR example

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
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

### Events

`.spec.events` is an optional field to specify a list of webhook payload event
types this Receiver should act on. If left empty, no filtering is applied and
any (valid) payload is handled.

**Note:** Support for this field, and the entries in it, is dependent on the
Receiver type. See the [supported Receiver types](#supported-receiver-types)
section for more information.

### Resources

`.spec.resources` is a required field to specify which Flux Custom Resources
should be reconciled when the Receiver's [webhook path](#webhook-path) is
called.

A resource entry contains the following fields:

- `apiVersion` (Optional): The Flux Custom Resource API group and version, such as
  `source.toolkit.fluxcd.io/v1beta2`.
- `kind`: The Flux Custom Resource kind, supported values are `Bucket`,
  `GitRepository`, `Kustomization`, `HelmRelease`, `HelmChart`,
  `HelmRepository`, `ImageRepository`, `ImagePolicy`, `ImageUpdateAutomation`
  and `OCIRepository`.
- `name`: The Flux Custom Resource `.metadata.name` or `*` (if `matchLabels` is specified)
- `namespace` (Optional): The Flux Custom Resource `.metadata.namespace`.
  When not specified, the Receiver's `.metadata.namespace` is used instead.
- `matchLabels` (Optional): Annotate Flux Custom Resources with specific labels.
   The `name` field must be set to `*` when using `matchLabels`

#### Reconcile objects by name

To reconcile a single object, set the `kind`, `name` and `namespace`:

```yaml
resources:
  - apiVersion: image.toolkit.fluxcd.io/v1beta2
    kind: ImageRepository
    name: podinfo
```

#### Reconcile objects by label

To reconcile objects of a particular kind with specific labels:

```yaml
resources:
  - apiVersion: image.toolkit.fluxcd.io/v1beta2
    kind: ImageRepository
    name: "*"
    matchLabels:
      app: podinfo
```

**Note:** Cross-namespace references [can be disabled for security
reasons](#disabling-cross-namespace-selectors).

### Secret reference

`.spec.secretRef.name` is a required field to specify a name reference to a
Secret in the same namespace as the Receiver. The Secret must contain a `token`
key, whose value is a string containing a (random) secret token.

This token is used to salt the generated [webhook path](#webhook-path), and
depending on the Receiver [type](#supported-receiver-types), to verify the
authenticity of a request.

#### Secret example

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: webhook-token
  namespace: default
type: Opaque
stringData:
  token: <random token>
```

### Interval

`.spec.interval` is an optional field with a default of ten minutes that specifies
the time interval at which the controller reconciles the provider with its Secret
reference.

### Suspend

`.spec.suspend` is an optional field to suspend the Receiver.
When set to `true`, the controller will stop processing events for this Receiver.
When the field is set to `false` or removed, it will resume.

## Working with Receivers

### Disabling cross-namespace selectors

On multi-tenant clusters, platform admins can disable cross-namespace
references with the `--no-cross-namespace-refs=true` flag. When this flag is
set, Receivers can only refer to [Resources](#resources) in the same namespace
as the [Alert](alerts.md) object, preventing tenants from triggering
reconciliations to another tenant's resources.

### Public Ingress considerations

Considerations should be made when exposing the controller's `webhook-receiver`
Kubernetes Service to the public internet. Each request to a Receiver [webhook
path](#webhook-path) will result in request to the Kubernetes API, as the
controller needs to fetch information about the resource. This endpoint may be
protected with a token, but this does not defend against a situation where a
legitimate webhook caller starts sending large amounts of requests, or the
token is somehow leaked. This may result in the controller, as it may get rate
limited by the Kubernetes API, degrading its functionality.

It is therefore a good idea to set rate limits on the Ingress which exposes
the Kubernetes Service. If you are using ingress-nginx, this can be done by
[adding annotations](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#rate-limiting).

### Triggering a reconcile

To manually tell the notification-controller to reconcile a Receiver outside
of the [specified interval window](#interval), a Receiver can be annotated with
`reconcile.fluxcd.io/requestedAt: <arbitrary value>`. Annotating the resource
queues the Receiver for reconciliation if the `<arbitrary-value>` differs from
the last value the controller acted on, as reported in
[`.status.lastHandledReconcileAt`](#last-handled-reconcile-at).

Using `kubectl`:

```sh
kubectl annotate --field-manager=flux-client-side-apply --overwrite  receiver/<receiver-name> reconcile.fluxcd.io/requestedAt="$(date +%s)"
```

Using `flux`:

```sh
flux reconcile source receiver <receiver-name>
```

### Waiting for `Ready`

When a change is applied, it is possible to wait for the Receiver to reach a
[ready state](#ready-receiver) using `kubectl`:

```sh
kubectl wait receiver/<receiver-name> --for=condition=ready --timeout=1m
```

### Suspending and resuming

When you find yourself in a situation where you temporarily want to pause the
reconciliation of a Receiver and the handling of requests, you can suspend it
using the [`.spec.suspend` field](#suspend).

#### Suspend a Receiver

In your YAML declaration:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: <receiver-name>
spec:
  suspend: true
```

Using `kubectl`:

```sh
kubectl patch receiver <receiver-name> --field-manager=flux-client-side-apply -p '{\"spec\": {\"suspend\" : true }}'
```

Using `flux`:

```sh
flux suspend receiver <receiver-name>
```

#### Resume a Receiver

In your YAML declaration, comment out (or remove) the field:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: <receiver-name>
spec:
  # suspend: true
```

**Note:** Setting the field value to `false` has the same effect as removing
it, but does not allow for "hot patching" using e.g. `kubectl` while practicing
GitOps; as the manually applied patch would be overwritten by the declared
state in Git.

Using `kubectl`:

```sh
kubectl patch receiver <receiver-name> --field-manager=flux-client-side-apply -p '{\"spec\" : {\"suspend\" : false }}'
```

Using `flux`:

```sh
flux resume receiver <receiver-name>
```

### Debugging a Receiver

There are several ways to gather information about a Receiver for debugging
purposes.

#### Describe the Receiver

Describing a Receiver using `kubectl describe receiver <receiver-name>` displays
the latest recorded information for the resource in the Status and Events
sections:

```console
...
Status:
...
Status:
  Conditions:
    Last Transition Time:  2022-11-21T12:41:48Z
    Message:               Reconciliation in progress
    Observed Generation:   1
    Reason:                ProgressingWithRetry
    Status:                True
    Type:                  Reconciling
    Last Transition Time:  2022-11-21T12:41:48Z
    Message:               unable to read token from secret 'default/webhook-token' error: Secret "webhook-token" not found
    Observed Generation:   1
    Reason:                TokenNotFound
    Status:                False
    Type:                  Ready
  Observed Generation:     -1
Events:
  Type     Reason  Age               From                     Message
  ----     ------  ----              ----                     -------
  Warning  Failed  5s (x4 over 16s)  notification-controller  unable to read token from secret 'default/webhook-token' error: Secret "webhook-token" not found
```

#### Trace emitted Events

To view events for specific Receiver(s), `kubectl events` can be used in
combination with `--for` to list the Events for specific objects.
For example, running

```sh
kubectl events --for=Receiver/<receiver-name>
```

lists

```console
LAST SEEN   TYPE      REASON   OBJECT                     MESSAGE
3m44s       Warning   Failed   receiver/<receiver-name>   unable to read token from secret 'default/webhook-token' error: Secret "webhook-token" not found
```

## Receiver Status

### Conditions

A Receiver enters various states during its lifecycle, reflected as
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

The notification-controller may get stuck trying to reconcile a Receiver if its
secret token can not be found.

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

When a Receiver becomes [ready](#ready-receiver), the controller reports the
generated incoming webhook path under `.status.webhookPath`. The path format is
`/hook/sha256sum(token+name+namespace)`.

[typical-status-properties]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
[kstatus-spec]: https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
[HMAC]: https://en.wikipedia.org/wiki/HMAC
