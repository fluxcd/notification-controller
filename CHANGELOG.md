# Changelog

All notable changes to this project are documented in this file.

## 0.2.0 (2020-10-29)

This is the second MINOR prerelease, it comes with breaking changes:
* the histogram metric `gotk_reconcile_duration` was renamed to `gotk_reconcile_duration_seconds`
* the annotation `fluxcd.io/reconcileAt` was renamed to `reconcile.fluxcd.io/requestedAt`

## 0.1.2 (2020-10-19)

This prerelease adds support for HTTP/S proxies when sending alerts.
An optional field called `Proxy` was added to the Provider API.

Features:
* Add support for http(s) proxy when sending alerts
    [#62](https://github.com/fluxcd/notification-controller/pull/62)

## 0.1.1 (2020-10-13)

This prerelease comes with Prometheus instrumentation for the controller's resources.

For each kind, the controller exposes a gauge metric to track the `Ready` condition status,
and a histogram with the reconciliation duration in seconds:

* `gotk_reconcile_condition{kind, name, namespace, status, type="Ready"}`
* `gotk_reconcile_duration{kind, name, namespace}`

## 0.1.0 (2020-09-30)

This is the first MINOR prerelease, it promotes the
`notification.toolkit.fluxcd.io` API to `v1beta1`
and removes support for `v1alpha1`.

Going forward, changes to the API will be accompanied by a conversion
mechanism. With this release the API becomes more stable, but while in
beta phase there are no guarantees about backwards compatibility
between beta releases.

## 0.0.11 (2020-09-22)

This prerelease comes with support for publishing events
to GitLab commit status API.
The alerts and receivers were extended to support
S3 Bucket sources.
Container images for ARMv7 and ARMv8 are published to
`ghcr.io/fluxcd/notification-controller-arm64`.

## 0.0.10 (2020-09-12)

This prerelease comes with the option to watch for resources
in the runtime namespace of the controller or at cluster level.

## 0.0.9 (2020-09-11)

This prerelease makes the `api` package available as
a dedicated versioned module.

## 0.0.8 (2020-09-02)

This prerelease comes with support for publishing events
to GitHub commit status API.

## 0.0.7 (2020-08-05)

This prerelease comes with a fix to the Prometheus scraping endpoint.

## 0.0.6 (2020-07-31)

This prerelease comes with a breaking change, the CRDs group
has been renamed to `notification.toolkit.fluxcd.io`.
The dependency on `source-controller` has been updated to `v0.0.7` to
be able to work with `source.toolkit.fluxcd.io` resources.

## 0.0.5 (2020-07-20)

This prerelease drops support for Kubernetes <1.16.
The CRDs have been updated to `apiextensions.k8s.io/v1`.

## 0.0.4 (2020-07-16)

This prerelease comes with improvements to logging and
fixes a bug preventing alerts to be dispatched for resources
outside of the controller's namespace.

## 0.0.3 (2020-07-14)

This prerelease allows alert rules to be reconciled
outside of the controller's namespace.

## 0.0.2 (2020-07-13)

This prerelease comes with improvements to logging.
The default logging format is JSON and the timestamp format is ISO8601.

## 0.0.1 (2020-07-07)

This prerelease comes with webhook receivers support.
With the [Receiver API](https://github.com/fluxcd/notification-controller/blob/v0.0.1/docs/spec/v1alpha1/receiver.md)
you can define a webhook receiver (GitHub, GitLab, Bitbucket, Harbour, generic)
that triggers reconciliation for a group of resources.

## 0.0.1-beta.1 (2020-07-03)

This beta release comes with wildcard support for defining alerts
that target all resources of a particular kind in a namespace.

## 0.0.1-alpha.2 (2020-07-02)

This alpha release comes with improvements to alerts delivering.
The alert delivery method is **at-most once** with a timeout of 15 seconds.
The controller performs automatic retries for connection errors and 500-range response code.
If the webhook receiver returns an error, the controller will retry sending an alert for
four times with an exponential backoff of maximum 30 seconds.

## 0.0.1-alpha.1 (2020-07-01)

This is the first alpha release of notifications controller.
