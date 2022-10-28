# Events

The `Event` API defines the structure of the events issued by Flux controllers.

Flux controllers use the [fluxcd/pkg/runtime/events](https://github.com/fluxcd/pkg/tree/main/runtime/events)
package to push events to the notification-controller API.

## Example

The following is an example of an event sent by kustomize-controller to report a reconciliation error.

```json
{
  "involvedObject": {
    "apiVersion": "kustomize.toolkit.fluxcd.io/v1beta2",
    "kind": "Kustomization",
    "name": "webapp",
    "namespace": "apps",
    "uid": "7d0cdc51-ddcf-4743-b223-83ca5c699632"
  },
  "metadata": {
    "kustomize.toolkit.fluxcd.io/revision": "main/731f7eaddfb6af01cb2173e18f0f75b0ba780ef1"
  },
  "severity":"error",
  "reason": "ValidationFailed",
  "message":"service/apps/webapp validation error: spec.type: Unsupported value: Ingress",
  "reportingController":"kustomize-controller",
  "timestamp":"2022-10-28T07:26:19Z"
}
```

In the above example:

- An event is issued by kustomize-controller for a specific object, indicated in the
  `involvedObject` field.
- The notification-controller receives the event and finds the [alerts](alerts.md)
  that match the `involvedObject` and `severity` values.
- For all matching alerts, the controller posts the `message` and the source revision
  extracted from `metadata` to the alert provider API.

## Event structure

The Go type that defines the event structure can be found in the
[fluxcd/pkg/runtime/events](https://github.com/fluxcd/pkg/blob/main/runtime/events/event.go)
package.

## Rate limiting

Events received by notification-controller are subject to rate limiting to reduce the
amount of duplicate alerts sent to external systems like Slack, Sentry, etc.

Events are rate limited based on `involvedObject.name`, `involvedObject.namespace`,
`involvedObject.kind`, `message`, and `metadata`.
The interval of the rate limit is set by default to `5m` but can be configured
with the `--rate-limit-interval` controller flag.

The event server exposes HTTP request metrics to track the amount of rate limited events.
The following promql will get the rate at which requests are rate limited:

```
rate(gotk_event_http_request_duration_seconds_count{code="429"}[30s])
```
