# Alerts

<!-- menuweight:10 -->

The `Alert` API defines how events are filtered by severity and involved object, and what provider to use for dispatching.

## Example

The following is an example of how to send alerts to Slack when Flux fails to reconcile the `flux-system` namespace.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: slack-bot
  namespace: flux-system
spec:
  type: slack
  channel: general
  address: https://slack.com/api/chat.postMessage
  secretRef:
    name: slack-bot-token
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: slack
  namespace: flux-system
spec:
  summary: "Cluster addons impacted in us-east-2"
  providerRef:
    name: slack-bot
  eventSeverity: error
  eventSources:
    - kind: GitRepository
      name: '*'
    - kind: Kustomization
      name: '*'
```

In the above example:

- A Provider named `slack-bot` is created, indicated by the
  `Provider.metadata.name` field.
- An Alert named `slack` is created, indicated by the
  `Alert.metadata.name` field.
- The Alert references the `slack-bot` provider, indicated by the
  `Alert.spec.providerRef` field.
- The notification-controller starts listening for events sent for
  all GitRepositories and Kustomizations in the `flux-system` namespace.
- When an event with severity `error` is received, the controller posts
  a message on Slack channel from `.spec.channel`,
  containing the `summary` text and the reconciliation error.

You can run this example by saving the manifests into `slack-alerts.yaml`.

1. First create a secret with the Slack bot token:

   ```sh
   kubectl -n flux-system create secret generic slack-bot-token --from-literal=token=xoxb-YOUR-TOKEN
   ```

2. Apply the resources on the cluster:

   ```sh
   kubectl -n flux-system apply --server-side -f slack-alerts.yaml
   ```

3. Run `kubectl -n flux-system describe alert slack` to see its status:

   ```console
   ...
   Status:
     Conditions:
       Last Transition Time:  2022-11-16T23:43:38Z
       Message:               Initialized
       Observed Generation:   1
       Reason:                Succeeded
       Status:                True
       Type:                  Ready
     Observed Generation:     1
   Events:
     Type    Reason    Age   From                     Message
     ----    ------    ----  ----                     -------
     Normal  Succeeded 82s   notification-controller  Initialized
   ```

## Writing an Alert spec

As with all other Kubernetes config, an Alert needs `apiVersion`,
`kind`, and `metadata` fields. The name of an Alert object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).

An Alert also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Summary

`.spec.summary` is an optional field to specify a short description of the
impact and affected cluster.

The summary max length can't be greater than 255 characters. 

### Provider reference

`.spec.providerRef.name` is a required field to specify a name reference to a
[Provider](providers.md) in the same namespace as the Alert.

### Event sources

`.spec.eventSources` is a required field to specify a list of references to
Flux objects for which events are forwarded to the alert provider API.

To select events issued by Flux objects, each entry in the `.spec.eventSources` list
must contain the following fields:

- `kind` is the Flux Custom Resource Kind such as GitRepository, HelmRelease, Kustomization, etc.
- `name` is the Flux Custom Resource `.metadata.name`, or it can be set to the `*` wildcard.
- `namespace` is the Flux Custom Resource `.metadata.namespace`.
  When not specified, the Alert `.metadata.namespace` is used instead.

#### Select objects by name

To select events issued by a single Flux object, set the `kind`, `name` and `namespace`:

```yaml
eventSources:
  - kind: GitRepository
    name: webapp
    namespace: apps
```

#### Select all objects in a namespace

The `*` wildcard can be used to select events issued by all Flux objects of a particular `kind` in a `namespace`:

```yaml
eventSources:
  - kind: HelmRelease
    name: '*'
    namespace: apps
```

#### Select objects by label

To select events issued by all Flux objects of a particular `kind` with specific `labels`:

```yaml
eventSources:
  - kind: HelmRelease
    name: '*'
    namespace: apps
    matchLabels:
      team: app-dev
```

#### Disable cross-namespace selectors

**Note:** On multi-tenant clusters, platform admins can disable cross-namespace references by
starting the controller with the `--no-cross-namespace-refs=true` flag.
When this flag is set, alerts can only refer to event sources in the same namespace as the alert object,
preventing tenants from subscribing to another tenant's events.

### Event metadata

`.spec.eventMetadata` is an optional field for adding metadata to events dispatched by
the controller. This can be used for enhancing the context of the event. If a field
would override one already present on the original event as generated by the emitter,
then the override doesn't happen, i.e. the original value is preserved, and an info
log is printed.

#### Example

Add metadata fields to successful `HelmRelease` events:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: <name>
spec:
  eventSources:
    - kind: HelmRelease
      name: '*'
  inclusionList:
    - ".*succeeded.*"
  eventMetadata:
    app.kubernetes.io/env: "production"
    app.kubernetes.io/cluster: "my-cluster"
    app.kubernetes.io/region: "us-east-1"
```

### Event severity

`.spec.eventSeverity` is an optional field to filter events based on severity. When not specified, or
when the value is set to `info`, all events are forwarded to the alert provider API, including errors.
To receive alerts only on errors, set the field value to `error`.

### Event exclusion

`.spec.exclusionList` is an optional field to specify a list of regex expressions to filter
events based on message content. The event will be excluded if the message matches at least
one of the expressions in the list.

#### Example

Skip alerting if the message matches a [Go regex](https://golang.org/pkg/regexp/syntax)
from the exclusion list:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: <name>
spec:
  eventSources:
    - kind: GitRepository
      name: '*'
  exclusionList:
    - "waiting.*socket"
```

The above definition will not send alerts for transient Git clone errors like:

```text
unable to clone 'ssh://git@ssh.dev.azure.com/v3/...', error: SSH could not read data: Error waiting on socket
```

### Event inclusion

`.spec.inclusionList` is an optional field to specify a list of regex expressions to filter
events based on message content. The event will be sent if the message matches at least one
of the expressions in the list, and discarded otherwise. If the message matches one of the
expressions in the inclusion list but also matches one of the expressions in the exclusion
list, then the event is still discarded (exclusion is stronger than inclusion).

#### Example

Alert if the message matches a [Go regex](https://golang.org/pkg/regexp/syntax)
from the inclusion list:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: <name>
spec:
  eventSources:
    - kind: HelmRelease
      name: '*'
  inclusionList:
    - ".*succeeded.*"
  exclusionList:
    - ".*uninstall.*"
    - ".*test.*"
```

The above definition will send alerts for successful Helm installs, upgrades and rollbacks,
but not uninstalls and tests.

### Suspend

`.spec.suspend` is an optional field to suspend the altering.
When set to `true`, the controller will stop processing events.
When the field is set to `false` or removed, it will resume.

## Alert Status

### Conditions

An Alert enters various states during its lifecycle, reflected as
[Kubernetes Conditions][typical-status-properties].
It can be [ready](#ready-alert), or it can [fail during
reconciliation](#failed-alert).

The Alert API is compatible with the [kstatus specification][kstatus-spec],
and reports the `Reconciling` condition where applicable.

#### Ready Alert

The notification-controller marks an Alert as _ready_ when it has the following
characteristics:

- The Alert's Provider referenced in `.spec.providerRef.name` is found on the cluster.
- The Alert's Provider `Ready` status condition is set to `True`.

When the Alert is "ready", the controller sets a Condition with the following
attributes in the Alert's `.status.conditions`:

- `type: Ready`
- `status: "True"`
- `reason: Succeeded`

#### Failed Alert

The notification-controller may get stuck trying to reconcile an Alert if its Provider
can't be found or if the Provider is not ready.

When this happens, the controller sets the `Ready` Condition status to `False`,
and adds a Condition with the following attributes:

- `type: Reconciling`
- `status: "True"`
- `reason: ProgressingWithRetry`

### Observed Generation

The notification-controller reports an
[observed generation][typical-status-properties]
in the Alert's `.status.observedGeneration`. The observed generation is the
latest `.metadata.generation` which resulted in a [ready state](#ready-alert).

### Last Handled Reconcile At

The notification-controller reports the last `reconcile.fluxcd.io/requestedAt`
annotation value it acted on in the `.status.lastHandledReconcileAt` field.

[typical-status-properties]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
[kstatus-spec]: https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
