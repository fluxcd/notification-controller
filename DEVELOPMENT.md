# Development

> **Note:** Please take a look at <https://fluxcd.io/contributing/flux/>
> to find out about how to contribute to Flux and how to interact with the
> Flux Development team.

## Installing required dependencies

There are a number of dependencies required to be able to run the controller and its test suite locally:

- [Install Go](https://golang.org/doc/install)
- [Install Kustomize](https://kubernetes-sigs.github.io/kustomize/installation/)
- [Install Docker](https://docs.docker.com/engine/install/)
- (Optional) [Install Kubebuilder](https://book.kubebuilder.io/quick-start.html#installation)

In addition to the above, the following dependencies are also used by some of the `make` targets:

- `controller-gen` (v0.19.0)
- `gen-crd-api-reference-docs` (v0.3.0)
- `setup-envtest` (latest)

If any of the above dependencies are not present on your system, the first invocation of a `make` target that requires them will install them.

## How to run the test suite

Prerequisites:
- Go >= 1.25

You can run the test suite by simply doing:

```sh
make test
```

## How to run the controller locally

Install the controller's CRDs on your test cluster:

```sh
make install
```

Run the controller locally:

```sh
make run
```

`make run` starts the binary against the cluster in your current kubeconfig and
binds metrics to `:9094` (so it does not collide with the default events
address). With the default flags the process also listens for:

| Endpoint | Default address | Purpose |
| --- | --- | --- |
| Events API | `:9090` (`--events-addr`) | Other Flux controllers POST [`Event`](docs/spec/v1beta3/events.md) payloads here |
| Webhook receiver | `:9292` (`--receiverAddr`) | External webhooks hit `/hook/<token>` |
| Health probes | `:9440` (`--health-addr`) | Liveness / readiness |
| Metrics / pprof | `:9094` via `make run` (`--metrics-addr`) | Prometheus metrics and pprof handlers |

## Debugging the controller locally

Use this section when you need to reproduce a reported issue or step through
notification dispatch / webhook handling against a real cluster.

### Avoid racing an in-cluster controller

If the cluster already runs `notification-controller`, scale it down before
starting your local process so only one instance reconciles objects and serves
events/webhooks:

```sh
kubectl -n flux-system scale deploy/notification-controller --replicas=0
```

Restore it when you are done:

```sh
kubectl -n flux-system scale deploy/notification-controller --replicas=1
```

### Suspend objects that are not part of the reproduction

Shared clusters often have many `Alert`, `Provider`, and `Receiver` objects.
Suspend everything you do not need so their reconciles and outbound
notifications do not interleave with the case you are debugging:

```sh
flux suspend alert --all
flux suspend alert-provider --all
flux suspend receiver --all
```

`--all` applies to the current kubeconfig namespace (commonly `flux-system`).
Resume specific objects (or use `flux resume <kind> --all`) when finished.

### Increase log verbosity

`make run` uses the default `info` level. For a console-friendly local trace:

```sh
go run ./main.go --metrics-addr=:9094 --log-level=debug --log-encoding=console
```

Supported `--log-level` values are `trace`, `debug`, `info`, and `error`.

### Exercise the events API

Flux source/kustomize/helm controllers normally POST events to the events
server. When the controller runs on your laptop those in-cluster clients cannot
reach `localhost`, so the practical approach is to POST a synthetic event
yourself after creating the `Provider` / `Alert` objects under test:

```sh
curl -sS -X POST http://localhost:9090/ \
  -H 'Content-Type: application/json' \
  -d '{
    "involvedObject": {
      "kind": "Kustomization",
      "namespace": "default",
      "name": "demo",
      "apiVersion": "kustomize.toolkit.fluxcd.io/v1"
    },
    "severity": "info",
    "timestamp": "2026-08-09T00:00:00Z",
    "message": "Reconciliation finished in 250ms",
    "reason": "ReconciliationSucceeded",
    "reportingController": "kustomize-controller"
  }'
```

Watch the local process logs for matching / filtering / provider dispatch, and
confirm the notification arrived at your provider (or a local requestbin /
webhook.site sink configured on the `Provider`).

### Exercise Receiver webhooks

Receivers expose an HTTP path recorded on the object status. Read it with:

```sh
kubectl get receiver <name> -o jsonpath='{.status.webhookPath}{"\n"}'
```

With the local default listen address, POST to that path on `:9292`:

```sh
curl -sS -X POST "http://localhost:9292$(kubectl get receiver <name> -o jsonpath='{.status.webhookPath}')" \
  -H 'Content-Type: application/json' \
  -d '{"ref":"refs/heads/main"}'
```

Use a `Receiver` whose type matches the payload you send (generic, github,
gitlab, …).

### Cross-controller interactions

`notification-controller` is driven by two inbound paths:

1. **Events API (`--events-addr`)** — used by other GitOps Toolkit controllers to
   forward reconciliation events that `Alert` objects may notify on.
2. **Webhook receiver (`--receiverAddr`)** — used by external systems
   (GitHub/GitLab/Harbor/… ) to trigger Flux objects referenced by a
   `Receiver`.

When debugging against a shared cluster, keep watching all namespaces (the
default `--watch-all-namespaces=true`). Narrowing the cache with
`--watch-all-namespaces=false` / `RUNTIME_NAMESPACE` hides cross-namespace
refs that Alerts and Receivers often exercise, so it is a poor default for
local reproductions. Prefer suspending unrelated objects instead.

If you specifically need an in-cluster controller to deliver events to your
laptop, expose the local events port with a reverse tunnel or similar and
point that controller's events address at it. For most reproductions, posting
synthetic events (above) is simpler and sufficient.

### Debugging with VS Code

Create a `.vscode/launch.json` file:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch notification-controller",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/main.go",
            "args": [
                "--metrics-addr=:9094",
                "--log-level=debug",
                "--log-encoding=console"
            ]
        }
    ]
}
```

Scale down the in-cluster Deployment first, then start debugging with
**Run → Start Debugging**.

## How to install the controller

### Building the container image

Set the name of the container image to be created from the source code. This will be used when building, pushing and referring to the image on YAML files:

```sh
export IMG=registry-path/notification-controller:latest
```

Build the container image, tagging it as `$(IMG)`:

```sh
make docker-build
```

Push the image into the repository:

```sh
make docker-push
```

**Note**: `make docker-build` will build an image for the `amd64` architecture.


### Deploying into a cluster

Deploy `notification-controller` into the cluster that is configured in the local kubeconfig file (i.e. `~/.kube/config`):

```sh
make deploy
```
