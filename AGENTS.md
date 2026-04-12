# AGENTS.md

Guidance for AI coding assistants working in `fluxcd/notification-controller`. Read this file before making changes.

## Contribution workflow for AI agents

These rules come from [`fluxcd/flux2/CONTRIBUTING.md`](https://github.com/fluxcd/flux2/blob/main/CONTRIBUTING.md) and apply to every Flux repository.

- **Do not add `Signed-off-by` or `Co-authored-by` trailers with your agent name.** Only a human can legally certify the DCO.
- **Disclose AI assistance** with an `Assisted-by` trailer naming your agent and model:
  ```sh
  git commit -s -m "Add support for X" --trailer "Assisted-by: <agent-name>/<model-id>"
  ```
  The `-s` flag adds the human's `Signed-off-by` from their git config — do not remove it.
- **Commit message format:** Subject in imperative mood ("Add feature X" instead of "Adding feature X"), capitalized, no trailing period, ≤50 characters. Body wrapped at 72 columns, explaining what and why. No `@mentions` or `#123` issue references in the commit — put those in the PR description.
- **Trim verbiage:** in PR descriptions, commit messages, and code comments. No marketing prose, no restating the diff, no emojis.
- **Rebase, don't merge:** Never merge `main` into the feature branch; rebase onto the latest `main` and push with `--force-with-lease`. Squash before merge when asked.
- **Pre-PR gate:** `make tidy fmt vet && make test` must pass and the working tree must be clean after codegen. Commit regenerated files in the same PR.
- **Flux is GA:** Backward compatibility is mandatory. Breaking changes to CRD fields, status, CLI flags, metrics, or observable behavior will be rejected. Design additive changes and keep older API versions round-tripping.
- **Copyright:** All new `.go` files must begin with the boilerplate from `hack/boilerplate.go.txt` (Apache 2.0). Update the year to the current year when copying.
- **Spec docs:** New features and API changes must be documented in `docs/spec/` under the current version — `v1/receivers.md` for Receiver, `v1beta3/alerts.md` and `v1beta3/providers.md` for Alert and Provider. Update the relevant file in the same PR that introduces the change.
- **Tests:** New features, improvements and fixes must have test coverage. Add unit tests in `internal/controller/*_test.go` and other `internal/*` packages as appropriate. Follow the existing patterns for test organization, fixtures, and assertions. Run tests locally before pushing.

## Code quality

Before submitting code, review your changes for the following:

- **No secrets in logs or events.** Anything derived from a Secret (tokens, passwords, webhook URLs with embedded secrets) must be scrubbed via `fluxcd/pkg/masktoken` before logging or surfacing in conditions. Never log the receiver webhook path at info level — it is effectively a secret.
- **No unchecked I/O.** Close HTTP response bodies and file handles in `defer` statements. Check and propagate errors from I/O operations.
- **Bounded request bodies.** Both HTTP servers enforce `maxRequestSizeBytes` (3 MiB). Always read through `io.LimitReader` when extending handlers. Do not introduce new readers without bounds.
- **Single HTTP client for notifiers.** `internal/notifier/client.go` is the only sanctioned way to make outbound HTTP calls from a notifier. It handles proxies, TLS, retries, and response validation. Do not add your own HTTP client or retry loops.
- **No hardcoded defaults for security settings.** TLS verification must remain enabled by default; proxy settings come from user-provided secrets. Receiver signature verification must never be short-circuited.
- **Error handling.** Wrap errors with `%w` for chain inspection. Do not swallow errors silently. Return actionable error messages that help users diagnose the issue without leaking internal state.
- **No duplicate rate limiting.** The event server already applies a token-bucket rate limiter keyed by event fingerprint. Do not add a second layer of deduplication.
- **Concurrency safety.** Do not introduce shared mutable state without synchronization. Both HTTP servers and the reconcilers run concurrently.
- **No panics.** Never use `panic` in runtime code paths. Return errors and let the reconciler or handler handle them gracefully.
- **Minimal surface.** Keep new exported APIs, flags, and environment variables to the minimum needed. Every export is a backward-compatibility commitment.

## Project overview

notification-controller is the eventing edge of the [Flux GitOps Toolkit](https://fluxcd.io/flux/components/). It reconciles three custom resources under `notification.toolkit.fluxcd.io` — `Provider`, `Alert`, and `Receiver` — and runs two HTTP servers alongside the controller manager:

- An inbound **event sink** (default `:9090`) that ingests `eventv1.Event` payloads from the other Flux controllers and dispatches them to external notifier backends (Slack, MS Teams, PagerDuty, Git commit status, …) according to matching `Alert` resources.
- An inbound **webhook receiver** (default `:9292`) that translates third-party webhooks (GitHub, GitLab, Harbor, image registries, CDEvents, …) into reconcile requests on Flux objects.

It does not reconcile source or workload state itself.

## Repository layout

- `main.go` — manager entrypoint. Registers the three reconcilers, starts `server.NewEventServer` and `server.NewReceiverServer` as goroutines, wires feature gates, token cache, and workload identity.
- `api/` — separate Go module (`replace`d from root `go.mod`). CRD Go types for `v1`, `v1beta1`, `v1beta2`, `v1beta3`. Storage versions: `v1` for `Receiver`, `v1beta3` for `Alert` and `Provider`. `zz_generated.deepcopy.go` is generated.
- `internal/controller/` — reconcilers: `provider_controller.go`, `alert_controller.go`, `receiver_controller.go`, predicates, and the envtest `suite_test.go`.
- `internal/server/` — the two HTTP servers. `event_server.go` + `event_handlers.go` implement event ingestion, alert matching, filtering, rate limiting (`sethvargo/go-limiter`) and token masking (`fluxcd/pkg/masktoken`). `receiver_server.go` + `receiver_handlers.go` implement webhook verification and dispatch. `provider_commit_status.go` and `provider_change_request.go` handle Git commit-status/MR commenting.
- `internal/notifier/` — one file per backend plus `notifier.go` (the `Interface` with a single `Post(ctx, event)` method), `factory.go` (provider name → constructor map), `client.go` (shared retryable HTTP client), `util.go`, `markdown.go`. Each backend has a sibling `_test.go`.
- `internal/features/` — feature-gate registration.
- `config/` — Kustomize overlays. `config/crd/bases/` holds generated CRDs; `config/manager/`, `config/default/`, `config/rbac/`, `config/samples/`, `config/testdata/`.
- `hack/` — `boilerplate.go.txt` and `api-docs/` templates for `gen-crd-api-reference-docs`.
- `docs/spec/` — human-readable specs per API version. `docs/api/` — generated reference docs.
- `tests/` — `listener/` and `proxy/` helpers for integration tests.

### Notifiers implemented in `internal/notifier/`

Mapped in `factory.go`:

- generic, generic-hmac (the built-in `Forwarder`)
- slack, discord, rocket, msteams, googlechat, googlepubsub, webex, telegram, lark, matrix, zulip
- sentry, pagerduty, opsgenie, datadog, grafana, alertmanager
- github, githubdispatch, githubpullrequestcomment
- gitlab, gitlabmergerequestcomment
- gitea, giteapullrequestcomment
- bitbucket, bitbucketserver, azuredevops
- azureeventhub, nats, otel

## APIs and CRDs

- Group: `notification.toolkit.fluxcd.io`. Kinds: `Provider`, `Alert`, `Receiver`.
- Types under `api/<version>/{provider,alert,receiver}_types.go` with constants for every provider/receiver type name (e.g. `SlackProvider`, `GitHubReceiver`). The notifier factory and the receiver validator switch on these constants — keep them in sync when adding a type.
- `v1beta3` is the current `Alert`/`Provider` storage version; `Receiver` graduated to `v1`. Conversion logic is code-generated.
- CRD manifests under `config/crd/bases/` and reference docs under `docs/api/<version>/notification.md` are generated by `make manifests` and `make api-docs`. Do not hand-edit.
- `config/crd/bases/gitrepositories.yaml` is pulled at build time from source-controller via `make download-crd-deps` (for envtest); do not commit changes to it.

## Build, test, lint

All targets in the root `Makefile`. Tool versions pinned via `CONTROLLER_GEN_VERSION`, `GEN_API_REF_DOCS_VERSION`, `SOURCE_VER`. Go version tracks `go.mod`.

- `make tidy` — tidy both the root and `api/` modules.
- `make fmt` / `make vet` — run in both modules.
- `make generate` — `controller-gen object` in `api/` to refresh `zz_generated.deepcopy.go`.
- `make manifests` — regenerate CRDs and RBAC under `config/crd/bases` and `config/rbac`.
- `make api-docs` — regenerate `docs/api/v1`, `docs/api/v1beta2`, `docs/api/v1beta3` markdown.
- `make test` — full pre-PR gate. Depends on `tidy generate fmt vet manifests api-docs download-crd-deps install-envtest`, then runs `go test ./...` with envtest assets plus `go test ./...` in `api/`. `GO_TEST_ARGS` for extra flags.
- `make manager` — builds `bin/manager`.
- `make run` — runs the controller against `~/.kube/config`.
- `make install` / `make uninstall` / `make deploy` / `make dev-deploy` / `make dev-cleanup` — cluster workflows (`IMG` variable).
- `make docker-build` / `make docker-push` — buildx image build (`linux/amd64` default via `BUILD_PLATFORMS`).

## Codegen and generated files

Check `go.mod` and the `Makefile` for current dependency and tool versions. After changing API types, kubebuilder markers, or RBAC comments, regenerate and commit the results:

```sh
make generate manifests api-docs
```

Generated files (never hand-edit):

- `api/*/zz_generated.deepcopy.go`
- `config/crd/bases/notification.toolkit.fluxcd.io_*.yaml`
- `config/crd/bases/gitrepositories.yaml` (downloaded from source-controller)
- `config/rbac/role.yaml`
- `docs/api/**/*.md`

No load-bearing `replace` directives beyond the standard `api/` local replace.

Bump `fluxcd/pkg/*` modules as a set — version skew breaks `go.sum`. Run `make tidy` after any bump.

## Conventions

- `go fmt`, `go vet`. All exported identifiers get doc comments, including provider-type-name constants. Non-trivial unexported helpers should also be documented.
- **Adding a notifier.** Add a constant in `api/v1beta3/provider_types.go`, extend the `+kubebuilder:validation:Enum` list on `ProviderSpec.Type`, implement the backend in `internal/notifier/<name>.go` returning the `notifier.Interface`, and add its entry to the `notifiers` map plus a `xxxNotifierFunc` in `internal/notifier/factory.go`. Then regenerate CRDs and docs.
- **Receiver verification.** `internal/server/receiver_handlers.go::validate` is the single entry point. Each `Receiver.Spec.Type` has its own signature scheme — GitHub HMAC-SHA1/256 over the body with `X-Hub-Signature`, GitLab compares `X-Gitlab-Token`, Bitbucket/Harbor/DockerHub/Quay/Nexus/GCR/ACR/CDEvents each have their own rules. Do not short-circuit these checks. Payloads are capped at `maxRequestSizeBytes = 3 * 1024 * 1024` (3 MiB); reuse the constant.
- **Webhook path.** Receivers are addressed under `apiv1.ReceiverWebhookPath` (`/hook/`) with a per-resource digest. The path is indexed on `Receiver.Status.WebhookPath` via `WebhookPathIndexKey` — do not bypass the index when looking up receivers.
- **Event dispatch.** `EventServer.handleEvent` matches each event against all `Alert` resources, applies inclusion/exclusion filters and CEL `eventMetadata`, rate-limits duplicates through the memory store, then calls each notifier's `Post(ctx, event)` with a 15s context timeout. HTTP-based notifiers inherit retry behavior from `internal/notifier/client.go` via `go-retryablehttp`.
- **Proxy and TLS.** Every HTTP notifier takes an optional `ProxyURL` and `*tls.Config` from `notifierOptions`. Respect both. TLS material comes from `ProviderSpec.CertSecretRef` via `pkg/runtime/secrets`; never disable verification by default.
- **Service account / workload identity.** Cloud notifiers (Azure Event Hub, Azure DevOps, Google Pub/Sub, GitHub App) use `fluxcd/pkg/auth` with the shared token cache passed through `WithTokenCache` / `WithTokenClient`.
- **Cross-namespace refs.** Gated by `--no-cross-namespace-refs` (ACL option); both servers honor it when resolving `Alert.Spec.ProviderRef` and `Receiver.Spec.Resources`.

## Testing

- `internal/controller/suite_test.go` bootstraps `envtest` with controller-runtime. `make install-envtest` downloads the binaries into `build/testbin` via `setup-envtest`; `make test` exports `KUBEBUILDER_ASSETS`.
- Reconciler tests (`alert_controller_test.go`, `provider_controller_test.go`, `receiver_controller_test.go`) drive the envtest API server. Use `testdata/` fixtures when possible.
- Server tests (`event_server_test.go`, `event_handlers_test.go`, `receiver_handler_test.go`, `receiver_resource_filter_test.go`, `provider_commit_status_test.go`) exercise the HTTP handlers directly with `httptest`.
- Notifier tests: one `<name>_test.go` per backend standing up an `httptest.Server` (or a fake SDK client for platforms that don't accept a custom URL) and asserting on the captured request. Match this pattern when adding a new notifier.- Run a single test: `make test GO_TEST_ARGS='-run TestSlack'`.
- The `api/` submodule has its own tests; `make test` runs them in a separate `go test` pass.

## Gotchas and non-obvious rules

- Two Go modules. The root module imports `github.com/fluxcd/notification-controller/api` via a local `replace`. Run `tidy`, `fmt`, `vet` in both when you touch `api/` — the Makefile targets already do this.
- **CRD enum drift.** `ProviderSpec.Type` and `ReceiverSpec.Type` have explicit `+kubebuilder:validation:Enum` lists. Adding a new constant in Go is not enough — extend the enum marker and rerun `make manifests`, or the new type will be rejected by the API server.
- `v1beta3` is the storage version for `Alert`/`Provider` and introduced breaking-ish shape changes from `v1beta2` (e.g. `interval` deprecated on `Provider`). New fields go on `v1beta3` (and `v1` for `Receiver`); do not add fields to older versions.
- The `Alert` reconciler is intentionally minimal — it exists to migrate `v1beta3` alerts and manage finalizers. Actual dispatch happens in `internal/server/event_handlers.go`. Do not move business logic into the reconciler.
- The receiver webhook path is derived from a SHA-256 digest of the resource and a token from the referenced Secret; it is written to `Receiver.Status.WebhookPath` and indexed with `WebhookPathIndexKey`.
- The event server applies a token-bucket rate limiter keyed by event fingerprint (`eventKeyFunc`) using `sethvargo/go-limiter` — duplicate events within `--rate-limit-interval` (default 5m) are dropped before notifier dispatch.
- `--events-addr :9090` must match what source-controller, kustomize-controller, and helm-controller post to via `EVENT_ENDPOINT`; changing the default breaks the GitOps Toolkit wiring.
- Prefer existing `fluxcd/pkg` helpers (runtime, secrets, auth, cache, masktoken) before adding new top-level dependencies.
