# Changelog

All notable changes to this project are documented in this file.

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
