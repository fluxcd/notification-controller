# Alerts

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
  a message on Slack containing the `summary` text and the reconciliation error.

You can run this example by saving the manifests into `slack-alerts.yaml`.

1. First create a secret with the Slack bot token:

   ```sh
   kubectl -n flux-system create secret generic slack-bot-token --from-literal=token=xoxb-YOUR-TOKEN
   ```

2. Apply the resources on the cluster:

   ```sh
   kubectl -n flux-system apply --server-side -f slack-alerts.yaml
   ```

## Writing an Alert spec

As with all other Kubernetes config, an Alert needs `apiVersion`,
`kind`, and `metadata` fields. The name of an Alert object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).

An Alert also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Provider reference

`.spec.providerRef.name` is a required field to specify a name reference to a
Provider in the same namespace as the Alert.

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

**Note:** On multi-tenant clusters, platform admins can disable cross-namespace references with the
`--no-cross-namespace-refs=true` flag. When this flag is set, alerts can only refer to event sources
in the same namespace as the alert object, preventing tenants from subscribing to another tenant's events.

### Event severity

`.spec.eventSeverity` is an optional field to filter events based on severity. When not specified, or
when the value is set to `info`, all events are forwarded to the alert provider API, including errors.
To receive alerts only on errors, set the field value to `error`.

### Event exclusion

`.spec.exclusionList` is an optional field to specify a list of regex expressions to filter
events based on message content.

### Example

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
