# Changelog

All notable changes to this project are documented in this file.

## 0.25.1

**Release date:** 2022-08-11

This prerelease comes with a fix to GitHub Container Registry webhook receivers.

Starting with this version, Flux `Receivers` can be used to trigger `OCIRepositories`
reconciliation when pushing OCI artifacts to GHCR from GH Action.

Fixes:
* Remove code for parsing github payload
  [#401](https://github.com/fluxcd/notification-controller/pull/401)

Improvements:
* Add flags to configure exponential back-off retry
  [#399](https://github.com/fluxcd/notification-controller/pull/399)

## 0.25.0

**Release date:** 2022-08-08

This prerelease comes with support for sending alerts based on `OCIRepository` events.

In addition, various dependencies have been updated to their latest versions.

Improvements:
* Add OCIRepository event source
  [#396](https://github.com/fluxcd/notification-controller/pull/396)
* Update dependencies
  [#397](https://github.com/fluxcd/notification-controller/pull/397)
* Add missing language to fenced code block
  [#394](https://github.com/fluxcd/notification-controller/pull/394)

## 0.24.1

**Release date:** 2022-07-13

This prerelease comes with some minor improvements and updates dependencies
to patch upstream CVEs.

Improvements:
* Force github.com/emicklei/go-restful to v2.16.0 [#390](https://github.com/fluxcd/notification-controller/pull/390)
* Update flux types api versions [#381](https://github.com/fluxcd/notification-controller/pull/381)
* Update Azure DevOps Go API [#384](https://github.com/fluxcd/notification-controller/pull/384)
* Update dependencies [#387](https://github.com/fluxcd/notification-controller/pull/387)
* Use masktoken pkg for redacting token [#388](https://github.com/fluxcd/notification-controller/pull/388)
* build: Upgrade to Go 1.18 [#389](https://github.com/fluxcd/notification-controller/pull/389)

## 0.24.0

**Release date:** 2022-05-27

This prerelease comes with support for triggering GitHub Actions workflows using
the repository dispatch provider. For more information on how to configure
this integration see the
[alerting provider docs](https://github.com/fluxcd/notification-controller/blob/api/v0.24.0/docs/spec/v1beta1/provider.md#github-repository-dispatch).

Features:
* Add GitHub dispatch provider
  [#369](https://github.com/fluxcd/notification-controller/pull/369)

Improvements:
* Better error messages for alert providers
  [#375](https://github.com/fluxcd/notification-controller/pull/375)
* Add docs for Microsoft Teams
  [#370](https://github.com/fluxcd/notification-controller/pull/370)
* Update dependencies
  [#371](https://github.com/fluxcd/notification-controller/pull/371)
  [#373](https://github.com/fluxcd/notification-controller/pull/373)
  [#379](https://github.com/fluxcd/notification-controller/pull/379)

## 0.23.5

**Release date:** 2022-05-03

This prerelease comes with dependencies updates, and improvements to the BitBucket
commit status notifications.

Improvements:
- Check for duplicate commit status in Bitbucket
  [#366](https://github.com/fluxcd/notification-controller/pull/366)
- Update dependencies
  [#371](https://github.com/fluxcd/notification-controller/pull/371)

## 0.23.4

**Release date:** 2022-04-21

This prerelease updates the Go `golang.org/x/crypto` dependency to latest to
please static security analysers (CVE-2022-27191).

Fixes:
- Update golang.org/x/crypto
  [#367](https://github.com/fluxcd/notification-controller/pull/367)

## 0.23.3

**Release date:** 2022-04-19

This prerelease solves an issue with invalid UTF-8 characters while redacting
tokens. Furthermore, dependencies have been updated to their latest versions.

Improvements:
- Update dependencies
  [#364](https://github.com/fluxcd/notification-controller/pull/364)

Fixes:
- Return err on invalid UTF-8 character in token
  [#361](https://github.com/fluxcd/notification-controller/pull/361)

## 0.23.2

**Release date:** 2022-03-30

This prerelease comes with updates to the Webex notification provider and its
integration docs.

In addition, various dependencies have been updated to their latest versions.

Improvements:
- Update the webex notification provider and markdown
  [#352](https://github.com/fluxcd/notification-controller/pull/352)
- Align version of dependencies when Fuzzing
  [#354](https://github.com/fluxcd/notification-controller/pull/354)
- Update fluxcd/pkg/runtime to v0.13.4
  [#355](https://github.com/fluxcd/notification-controller/pull/355)

## 0.23.1

**Release date:** 2022-03-23

This prerelease comes with strict filtering of events metadata.
Starting with this version, the metadata keys considered for
alerting must be prefixed with the involved object API group.

Improvements:
- Filter event metadata based on the object group
  [#350](https://github.com/fluxcd/notification-controller/pull/350)

## 0.23.0

**Release date:** 2022-03-21

This prerelease updates various dependencies to their latest versions.
The code base was refactored to align with `fluxcd/pkg/runtime` v0.13 release.

Improvements:
- Update `pkg/runtime` and `apis/meta`
  [#345](https://github.com/fluxcd/notification-controller/pull/345)
- Update dependencies
  [#346](https://github.com/fluxcd/notification-controller/pull/346)
- Cleanup metadata fields before alerting
  [#347](https://github.com/fluxcd/notification-controller/pull/347)

## 0.22.3

**Release date:** 2022-03-15

This prerelease patches the Deployment manifest to set the
`.spec.securityContext.fsGroup`, which may be required for some EKS setups as
reported in https://github.com/fluxcd/flux2/issues/2537.

In addition, it also updates `nhooyr.io/websocket` to `v1.8.7` and
`github.com/gin-gonic/gin` to `v1.7.7`, to please static security analysers and
fix any warnings.

Improvements:
- Update dependencies
  [#338](https://github.com/fluxcd/notification-controller/pull/338)
- add fsgroup for securityContext
  [#342](https://github.com/fluxcd/notification-controller/pull/342)

## 0.22.2

**Release date:** 2022-02-23

This prerelease patches the container image tag in the Deployment manifest that was previously missed in 0.22.1.

## 0.22.1

**Release date:** 2022-02-22

This prerelease comes with support for using basic auth when sending alerts to Grafana annotations API.

Improvements:
- Add basic auth support to Grafana provider
  [#334](https://github.com/fluxcd/notification-controller/pull/334)
- Allow the proxy address to specified in the Kubernetes Secret from Alert `spec.secretRef`
  [#331](https://github.com/fluxcd/notification-controller/pull/331)
- Switch to controller-runtime metadata client
  [#330](https://github.com/fluxcd/notification-controller/pull/330)
- Update dependencies
  [#333](https://github.com/fluxcd/notification-controller/pull/333)

## 0.22.0

**Release date:** 2022-02-16

This prerelease comes with support for sending alerts to Grafana annotations API.

In addition, the Alert API comes with an optional field `spec.eventSources[].matchLabels` that allows selecting event sources based on labels.

Features:
- Implement label selectors for event sources in alerts [#325](https://github.com/fluxcd/notification-controller/pull/325)
- Add Grafana alerting provider [#322](https://github.com/fluxcd/notification-controller/pull/322)

Improvements:
- Update documentation for alert provider type [#321](https://github.com/fluxcd/notification-controller/pull/321)
- Make username and channel field optional for Discord provider [#324](https://github.com/fluxcd/notification-controller/pull/324)

## 0.21.0

**Release date:** 2022-01-28

This prerelease comes with security improvements for multi-tenant clusters.

Platform admins can disable cross-namespace references with the
`--no-cross-namespace-refs=true` flag.
When this flag is set, alerts can only refer to event sources in the same namespace
as the alert object, preventing tenants from subscribing to another tenant's events.

Starting with this version, the controller deployment conforms to the
Kubernetes [restricted pod security standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted):
- all Linux capabilities were dropped
- the root filesystem was set to read-only
- the seccomp profile was set to the runtime default
- run as non-root was enabled
- the user and group ID was set to 65534

**Breaking changes**:
- The use of new seccomp API requires Kubernetes 1.19.
- The controller container is now executed under 65534:65534 (userid:groupid).
  This change may break deployments that hard-coded the user ID of 'controller' in their PodSecurityPolicy.

Features:
* Pass headers to generic provider through secretRef
  [#317](https://github.com/fluxcd/notification-controller/pull/317)

Improvements:
* Allow disabling cross-namespace event sources
  [#319](https://github.com/fluxcd/notification-controller/pull/319)
* Drop capabilities, enable seccomp and enforce runAsNonRoot
  [#313](https://github.com/fluxcd/notification-controller/pull/313)
* Publish SBOM and sign release artifacts
  [#314](https://github.com/fluxcd/notification-controller/pull/314)
* Add fuzz testing for notifiers
  [#306](https://github.com/fluxcd/notification-controller/pull/306)
* Add documentation for gitea
  [#308](https://github.com/fluxcd/notification-controller/pull/308)
* Update development documentation
  [#309](https://github.com/fluxcd/notification-controller/pull/309)

Fixes:
* Fix(Provider/Matrix): Load CA from CertSecretRef
  [#318](https://github.com/fluxcd/notification-controller/pull/318)
* Fix the missing protocol for the first port in manager config
  [#315](https://github.com/fluxcd/notification-controller/pull/315)

## 0.20.1

**Release date:** 2022-01-11

This prerelease comes with a fix for the Prometheus Alertmanager provider and
downgrades the `fluxcd/pkg/apis/meta` package from `v0.11.0-rc.3` to `v0.10.2` 
which is required by flux2.

Fixes:
* Downgrade fluxcd/pkg/apis/meta to v0.10.2
  [#303](https://github.com/fluxcd/notification-controller/pull/303)
* Add timestamp as label for Prometheus Alertmanager
  [#298](https://github.com/fluxcd/notification-controller/pull/298)

## 0.20.0

**Release date:** 2022-01-11

This prerelease comes with an update to the Kubernetes and controller-runtime dependencies
to align them with the Kubernetes 1.23 release.

In addition, the controller is now built with Go 1.17 and Alpine 3.15.

Improvements:
* Update Go to v1.17 and controller-runtime to v0.11
  [#279](https://github.com/fluxcd/notification-controller/pull/279)
* Update dependencies (fix CVE-2021-43565)
  [#301](https://github.com/fluxcd/notification-controller/pull/301)
* Update Dockerfile xx base and bump alpine to 3.15
  [#297](https://github.com/fluxcd/notification-controller/pull/297)
* Update docs for telegram
  [#300](https://github.com/fluxcd/notification-controller/pull/300)

## 0.19.0

**Release date:** 2021-11-23

This prerelease introduces the `Reconciling` and `Stalled` Condition Types
to indicate if a resource is under reconciliation, or has failed to reach
a `Ready==True` state. This is most beneficial to consumers making use of
[`kstatus`](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md),
which uses the information to return early instead of timing out.

It introduces support for Slack apps bots. See the
[Provider spec](https://github.com/fluxcd/notification-controller/blob/v0.19.0/docs/spec/v1beta1/provider.md#slack-app)
for more information on how to get started.

Lastly, `controller-runtime` has been updated to `v0.10.2`, solving an
issue with `rest_client_request_latency_seconds_.*` high cardinality
metrics.

Improvements:
* Add support for Slack app
  [#245](https://github.com/fluxcd/notification-controller/pull/245)
* chore: Drop deprecated io/ioutil
  [#277](https://github.com/fluxcd/notification-controller/pull/277)
* Use condition helpers in reconciler (kstatus compat)
  [#282](https://github.com/fluxcd/notification-controller/pull/282)
* Update Alpine to v3.14
  [#285](https://github.com/fluxcd/notification-controller/pull/285)
* Update controller-runtime v0.10.2
  [#289](https://github.com/fluxcd/notification-controller/pull/289)

Fixes:
* Use x509 certificate for Webex
  [#280](https://github.com/fluxcd/notification-controller/pull/280)
* Fix nil dereference err in AlertManager
  [#287](https://github.com/fluxcd/notification-controller/pull/287)

## 0.18.1

**Release date:** 2021-10-22

This prerelease comes with support for self-signed certificates when forwarding events to a TLS endpoint.

Fixes:
* Fixed missing setter for Forwarder CertPool
  [#262](https://github.com/fluxcd/notification-controller/pull/262)
* Fix MSTeams certificates
  [#257](https://github.com/fluxcd/notification-controller/pull/257)
* Use regex to find and replace token
  [#271](https://github.com/fluxcd/notification-controller/pull/271)

## 0.18.0

**Release date:** 2021-10-19

This prerelease comes with support for sending alerts to Prometheus Alertmanager.

Features:
* Add alertmanager provider
  [#258](https://github.com/fluxcd/notification-controller/pull/258)

## 0.17.1

**Release date:** 2021-10-13

This prerelease comes with a fix to the readiness status reporting of the notification custom resources.

Fixes:
* Set observed generation when recording status
  [#261](https://github.com/fluxcd/notification-controller/pull/261)

## 0.17.0

**Release date:** 2021-10-08

This prerelease comes with support for sending alerts to Opsgenie.

Features:
* Add opsgenie provider
  [#252](https://github.com/fluxcd/notification-controller/pull/252)

Fixes:
* Escape metadata string for Telegram notification
  [#249](https://github.com/fluxcd/notification-controller/pull/249)

## 0.16.0

**Release date:** 2021-08-26

This prerelease comes with support for sending alerts to Telegram, Lark and Matrix.

Features:
* Add Telegram alerting provider
  [#232](https://github.com/fluxcd/notification-controller/pull/232)
* Add Matrix alerting provider
  [#233](https://github.com/fluxcd/notification-controller/pull/233)
* Add Lark alerting provider
  [#236](https://github.com/fluxcd/notification-controller/pull/236)

## 0.15.1

**Release date:** 2021-08-05

This prerelease comes with extended support for Sentry such as:
using channel configuration for Sentry environment to re-use the
same DSN for multiple clusters,
and sending info event as Sentry traces.

Improvements:
* providers/sentry: send traces
  [#224](https://github.com/fluxcd/notification-controller/pull/224)
* providers/sentry: add environment support
  [#223](https://github.com/fluxcd/notification-controller/pull/223)
* Request reconcile using patch instead of update
  [#217](https://github.com/fluxcd/notification-controller/pull/217) 
* Update dependencies
  [#226](https://github.com/fluxcd/notification-controller/pull/226)

Fixes:
* providers/sentry: fix default HTTP Transport causing panic
  [#221](https://github.com/fluxcd/notification-controller/pull/221)

## 0.15.0

**Release date:** 2021-06-08

This prerelease comes with an update to the Kubernetes and controller-runtime
dependencies to align them with the Kubernetes 1.21 release.

Improvements:
* Update Kubernetes dependencies
  [#210](https://github.com/fluxcd/notification-controller/pull/210)
* Add cert pool to Slack provider requests
  [#207](https://github.com/fluxcd/notification-controller/pull/207)
* Make Slack channel optional
  [#208](https://github.com/fluxcd/notification-controller/pull/208)

## 0.14.1

**Release date:** 2021-05-26

This prerelease comes with a bug fix to the parsing of revisions to
make it accept branches with slashes.

Fixes:
* Fix revision parsing when branch contains slash
  [#201](https://github.com/fluxcd/notification-controller/pull/201)

## 0.14.0

**Release date:** 2021-05-11

This prerelease comes with support for sending events to Azure Event Hub.

Features:
* Add support for Azure EventHub provider
  [#191](https://github.com/fluxcd/notification-controller/pull/191)

Improvements:
* Redact token from error log
  [#196](https://github.com/fluxcd/notification-controller/pull/196)
* Add note about exposing receiver to the internet
  [#193](https://github.com/fluxcd/notification-controller/pull/193)

## 0.13.0

**Release date:** 2021-04-21

This prerelease comes with support for sending alerts to HTTPS servers with self-signed TLS certs.

Features:
* Add self-signed cert to provider
  [#184](https://github.com/fluxcd/notification-controller/pull/184)

## 0.12.0

**Release date:** 2021-03-26

This prerelease comes with support for sending alerts to Sentry.

Starting with this version, events are subject to rate limiting to
reduce the amount of duplicate alerts sent by notification-controller.
The interval of the rate limit is set by default to `5m`
but can be configured with the `--rate-limit-interval` command arg.

The event server exposes HTTP request metrics to track the amount of rate limited events.
The following promql will get the rate at which requests are rate limited:
```
rate(gotk_event_http_request_duration_seconds_count{code="429"}[30s])
```

Features:
* Add support for Sentry provider
  [#176](https://github.com/fluxcd/notification-controller/pull/176)

Improvements:
* Add rate limiter to event http servers
  [#167](https://github.com/fluxcd/notification-controller/pull/167)

## 0.11.0

**Release date:** 2021-03-26

This is the eleventh MINOR prerelease.

This prerelease comes with support for sending alerts to Webex and
for posting commit status updates to GitHub enterprise.

This prerelease comes with a breaking change: the leader election ID
was renamed from `4ae6d3b3.fluxcd.io` to
`notification-controller-leader-election`.
This change should however not have a direct impact.

The suspended status of resources is now recorded to a
`gotk_suspend_status` Prometheus gauge metric.

Features:
* Add support for Webex as an alert provider
  [#168](https://github.com/fluxcd/notification-controller/pull/168)
* Add support for GitHub enterprise commit status
  [#162](https://github.com/fluxcd/notification-controller/pull/162)

Improvements:
* Set leader election deadline to 30s
  [#170](https://github.com/fluxcd/notification-controller/pull/170)
* Record suspension metrics
  [#164](https://github.com/fluxcd/notification-controller/pull/164)

Fixes:
* Fix Google Chart alert filters
  [#169](https://github.com/fluxcd/notification-controller/pull/169)
* Fix BitBucket key length
  [#174](https://github.com/fluxcd/notification-controller/pull/174)
* Fix alerts mix up summary
  [#166](https://github.com/fluxcd/notification-controller/pull/166)
  
## 0.10.0

**Release date:** 2021-03-16

This is the tenth MINOR prerelease.

This prerelease comes with support for sending alerts to Google Chat
and for triggering container image updates to Git using Azure Container Registry. 

Features:
* Provide the ability to send events to Google Chat
  [#149](https://github.com/fluxcd/notification-controller/pull/149)
* Add ACR webhook receiver
  [#153](https://github.com/fluxcd/notification-controller/pull/153)

Improvements:
* Use controller-runtime structured logging
  [#156](https://github.com/fluxcd/notification-controller/pull/156)
* Use unstructured client to annotate receiver targets
  [#151](https://github.com/fluxcd/notification-controller/pull/151)
* Update runtime dependencies
  [#157](https://github.com/fluxcd/notification-controller/pull/157)

Fixes:
* Fix Azure Devops notifier issues
  [#154](https://github.com/fluxcd/notification-controller/pull/154)
* Add missing provider types to docs
  [#155](https://github.com/fluxcd/notification-controller/pull/155)

## 0.9.0

**Release date:** 2021-02-24

This is the ninth MINOR prerelease.

This prerelease comes with a fix to the alerting exclusion list.

The Kubernetes custom resource definitions are packaged as
a multi-doc YAML asset and published on the GitHub release page.

Improvements:
* Refactor release workflow
  [#146](https://github.com/fluxcd/notification-controller/pull/146)
* Unit tests for event forwarding
  [#145](https://github.com/fluxcd/notification-controller/pull/145)

Fixes:
* Fix alerts regex filtering
  [#144](https://github.com/fluxcd/notification-controller/pull/144)
  
## 0.8.0

**Release date:** 2021-02-12

This is the eight MINOR prerelease.

This prerelease comes with support for excluding messages
form alerting using regular expressions.

Golang `pprof` endpoints have been enabled on the metrics server,
making it easier to collect runtime information to debug performance issues.

Features:
* Implement regex exclusions for alerts
  [#138](https://github.com/fluxcd/notification-controller/pull/138)

Improvements:
* Enable pprof endpoints on metrics server
  [#136](https://github.com/fluxcd/notification-controller/pull/136)

## 0.7.1

**Release date:** 2021-01-26

This prerelease adds `*Kind` string constants for the kind objects
exposed by the API to further normalise it to GitOps Toolkit standards.

Improvements
* Add kinds to API types
  [#131](https://github.com/fluxcd/notification-controller/pull/131)

## 0.7.0

**Release date:** 2021-01-22

This is the seventh MINOR prerelease.

The `Receiver` API gains a new webhook type called `generic-hmac`,
that validates the caller legitimacy using HMAC signatures.

The `Alert` API comes with support for image update notifications
and is now possible to trigger container image updates to Git
using Sonatype Nexus webhooks.

Two new argument flags are introduced to support configuring the QPS
(`--kube-api-qps`) and burst (`--kube-api-burst`) while communicating
with the Kubernetes API server.

The `LocalObjectReference` from the Kubernetes core has been replaced
with our own, making `Name` a required field. The impact of this should
be limited to direct API consumers only, as the field was already
required by controller logic.

Features:
* Add generic webhook receiver for HMAC signing
  [#127](https://github.com/fluxcd/notification-controller/pull/127)
* Add Nexus webhook receiver
  [#126](https://github.com/fluxcd/notification-controller/pull/126)

Improvements:
* Add the object kind to notification messages
  [#124](https://github.com/fluxcd/notification-controller/pull/124)
* Allow ImageUpdateAutomations in object refs
  [#128](https://github.com/fluxcd/notification-controller/pull/128)
* Update fluxcd/pkg/runtime to v0.8.0
  [#129](https://github.com/fluxcd/notification-controller/pull/129)
  
## 0.6.2

**Release date:** 2021-01-19

This prerelease comes with support for triggering
container image updates to Git using Quay and GCR webhooks.

The Kubernetes packages were updated to v1.20.2 and controller-runtime to v0.8.0.

Features:
* Add GCR webhook receiver
  [#121](https://github.com/fluxcd/notification-controller/pull/121)
* Add Quay webhook receiver
  [#118](https://github.com/fluxcd/notification-controller/pull/118)

Improvements:
* Update Kubernetes packages to v1.20.2
  [#119](https://github.com/fluxcd/notification-controller/pull/119)

## 0.6.1

**Release date:** 2021-01-14

This prerelease comes with support for triggering
container image updates to Git using webhook receiver and
fixes a regression bug introduced in `v0.6.0` that caused
reconciliation request annotations to be ignored in certain scenarios.

Features:
* Trigger ImageRepository reconciliation with webhook receivers
  [#110](https://github.com/fluxcd/notification-controller/pull/110)
* Implement DockerHub webhook receiver
  [#112](https://github.com/fluxcd/notification-controller/pull/112)

Improvements:
* Upgrade runtime package to v0.6.2
  [#111](https://github.com/fluxcd/notification-controller/pull/111)

## 0.6.0

**Release date:** 2021-01-12

This is the sixth MINOR prerelease, upgrading the `controller-runtime`
dependencies to `v0.7.0`.

The container image for ARMv7 and ARM64 that used to be published
separately as `notification-controller:*-arm64` has been merged with
the AMD64 image.

## 0.5.0

**Release date:** 2020-12-10

This is the fifth MINOR prerelease. It comes with support for
customising the alert message with `spec.summary`.

Improvements:
* Add alert summary to notification metadata
    [#97](https://github.com/fluxcd/notification-controller/pull/97)
* Add example generic webhook request
    [#98](https://github.com/fluxcd/notification-controller/pull/98)

Fixes:
* Lookup ready receivers in all namespaces
    [#96](https://github.com/fluxcd/notification-controller/pull/96)
* Add check for duplicate status to avoid spamming the same status
    [#93](https://github.com/fluxcd/notification-controller/pull/93)

## 0.4.0

**Release date:** 2020-11-26

This is the fourth MINOR prerelease. It comes with 
support for Azure DevOps commit status updates. 

Improvements:
* Add Azure DevOps provider
    [#86](https://github.com/fluxcd/notification-controller/pull/86)
* Add readiness/liveness probes
    [#89](https://github.com/fluxcd/notification-controller/pull/89)

## 0.3.0

**Release date:** 2020-11-20

This is the third MINOR prerelease. It introduces a breaking change to
the API package; the status condition type has changed to the type
introduced in Kubernetes API machinery `v1.19.0`.

Improvements:
* Add support for sending a `Notification-Controller` HTTP header from
  the forward notifier
    [#84](https://github.com/fluxcd/notification-controller/pull/84)
* Verify repository ID in Git notifiers
    [#82](https://github.com/fluxcd/notification-controller/pull/82)
* Use subgroup in GitLab
    [#80](https://github.com/fluxcd/notification-controller/pull/80)

## 0.2.1

**Release date:** 2020-11-09

This prerelease comes with support for Bitbucket commit status updates.

Improvements:
* Add validation for providers and alerts
    [#74](https://github.com/fluxcd/notification-controller/pull/74)
* Add bitbucket notifier
    [#73](https://github.com/fluxcd/notification-controller/pull/73)

## 0.2.0

**Release date:** 2020-10-29

This is the second MINOR prerelease, it comes with breaking changes:
* the histogram metric `gotk_reconcile_duration` was renamed to `gotk_reconcile_duration_seconds`
* the annotation `fluxcd.io/reconcileAt` was renamed to `reconcile.fluxcd.io/requestedAt`

## 0.1.2

**Release date:** 2020-10-19

This prerelease adds support for HTTP/S proxies when sending alerts.
An optional field called `Proxy` was added to the Provider API.

Features:
* Add support for http(s) proxy when sending alerts
    [#62](https://github.com/fluxcd/notification-controller/pull/62)

## 0.1.1

**Release date:** 2020-10-13

This prerelease comes with Prometheus instrumentation for the controller's resources.

For each kind, the controller exposes a gauge metric to track the `Ready` condition status,
and a histogram with the reconciliation duration in seconds:

* `gotk_reconcile_condition{kind, name, namespace, status, type="Ready"}`
* `gotk_reconcile_duration{kind, name, namespace}`

## 0.1.0

**Release date:** 2020-09-30

This is the first MINOR prerelease, it promotes the
`notification.toolkit.fluxcd.io` API to `v1beta1`
and removes support for `v1alpha1`.

Going forward, changes to the API will be accompanied by a conversion
mechanism. With this release the API becomes more stable, but while in
beta phase there are no guarantees about backwards compatibility
between beta releases.

## 0.0.11

**Release date:** 2020-09-22

This prerelease comes with support for publishing events
to GitLab commit status API.
The alerts and receivers were extended to support
S3 Bucket sources.
Container images for ARMv7 and ARMv8 are published to
`ghcr.io/fluxcd/notification-controller-arm64`.

## 0.0.10

**Release date:** 2020-09-12

This prerelease comes with the option to watch for resources
in the runtime namespace of the controller or at cluster level.

## 0.0.9

**Release date:** 2020-09-11

This prerelease makes the `api` package available as
a dedicated versioned module.

## 0.0.8

**Release date:** 2020-09-02

This prerelease comes with support for publishing events
to GitHub commit status API.

## 0.0.7

**Release date:** 2020-08-05

This prerelease comes with a fix to the Prometheus scraping endpoint.

## 0.0.6

**Release date:** 2020-07-31

This prerelease comes with a breaking change, the CRDs group
has been renamed to `notification.toolkit.fluxcd.io`.
The dependency on `source-controller` has been updated to `v0.0.7` to
be able to work with `source.toolkit.fluxcd.io` resources.

## 0.0.5

**Release date:** 2020-07-20

This prerelease drops support for Kubernetes <1.16.
The CRDs have been updated to `apiextensions.k8s.io/v1`.

## 0.0.4

**Release date:** 2020-07-16

This prerelease comes with improvements to logging and
fixes a bug preventing alerts to be dispatched for resources
outside of the controller's namespace.

## 0.0.3

**Release date:** 2020-07-14

This prerelease allows alert rules to be reconciled
outside of the controller's namespace.

## 0.0.2

**Release date:** 2020-07-13

This prerelease comes with improvements to logging.
The default logging format is JSON and the timestamp format is ISO8601.

## 0.0.1

**Release date:** 2020-07-07

This prerelease comes with webhook receivers support.
With the [Receiver API](https://github.com/fluxcd/notification-controller/blob/v0.0.1/docs/spec/v1alpha1/receiver.md)
you can define a webhook receiver (GitHub, GitLab, Bitbucket, Harbour, generic)
that triggers reconciliation for a group of resources.

## 0.0.1-beta.1

**Release date:** 2020-07-03

This beta release comes with wildcard support for defining alerts
that target all resources of a particular kind in a namespace.

## 0.0.1-alpha.2

**Release date:** 2020-07-02

This alpha release comes with improvements to alerts delivering.
The alert delivery method is **at-most once** with a timeout of 15 seconds.
The controller performs automatic retries for connection errors and 500-range response code.
If the webhook receiver returns an error, the controller will retry sending an alert for
four times with an exponential backoff of maximum 30 seconds.

## 0.0.1-alpha.1

**Release date:** 2020-07-01

This is the first alpha release of notifications controller.
