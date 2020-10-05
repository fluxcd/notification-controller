# Notification Controller

The Notification Controller is a Kubernetes operator, specialized in handling inbound and outbound events.

## Motivation

### Events dispatching 

The main goal is to provide a notification service that can
receive events via HTTP and dispatch them to external systems
based on event severity and involved objects.

When operating a cluster, different teams may wish to receive notification about the status
of their CD pipelines. For example, the on-call team would receive alerts about all
failures in the cluster, while the dev team may wish to be alerted when a new version 
of an app was deployed and if the deployment is healthy.

### Webhook receivers

GitOps controllers are by nature pull based, in order to notify the controllers about
changes in Git or Helm repositories, one may wish to setup webhooks and trigger 
a cluster reconciliation every time a source changes.

## Design

### Events dispatching

The controller exposes an HTTP endpoint for receiving events from other controllers.
An event must contain information about the involved object such as kind, name, namespace,
a human-readable description of the event and the severity type e.g. info or error.

The controller can be configured with Kubernetes custom resources that define how
events are processed and where to dispatch them.

Notification API:

* [Provider](v1beta1/provider.md)
* [Alert](v1beta1/alert.md)
* [Event](v1beta1/event.md)

The alert delivery method is **at-most once** with a timeout of 15 seconds.
The controller performs automatic retries for connection errors and 500-range response code.
If the webhook receiver returns an error, the controller will retry sending an alert for four times
with an exponential backoff of maximum 30 seconds.

### Webhook receivers

The notification controller handles webhook requests on a dedicated port.
This port can be used to create a Kubernetes LoadBalancer Service or
Ingress to expose the receiver endpoint outside the cluster
to be accessed by GitHub, GitLab, Bitbucket, Harbor, Jenkins, etc.

Receiver API:

* [Receiver](v1beta1/receiver.md)

When a `Receiver` is created, the controller sets the `Receiver`
status to Ready and generates a URL in the format `/hook/sha256sum(token+name+namespace)`.

When the controller receives a POST request:
* extract the SHA265 digest from the URL
* loads the `Receiver` using the digest field selector
* extracts the signature from HTTP headers based on `spec.type`
* validates the signature using the `token` secret
* extract the event type from the payload 
* triggers a reconciliation for `spec.resources` if the event type matches one of the `spec.events` items

## Example

After installing notification-controller, we can configure alerting for events issued
by source-controller and kustomize-controller.

Create a notification provider for Slack:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: slack
spec:
  type: slack
  channel: prod-alerts
  secretRef:
    name: slack-url
---
apiVersion: v1
kind: Secret
metadata:
  name: slack-url
data:
  address: <encoded-url>
```

Create an alert for a list of GitRepositories and Kustomizations:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Alert
metadata:
  name: on-call-webapp
spec:
  providerRef: 
    name: slack
  eventSeverity: info
  eventSources:
    - kind: GitRepository
      name: '*'
    - kind: Kustomization
      name: webapp-frontend
    - kind: Kustomization
      name: webapp-backend
```

Based on the above configuration, the controller will post messages on Slack every time there is an event
issued for the webapp Git repository and Kustomizations.

Kustomization apply event example:

```json
{
  "severity": "info",
  "ts": "2020-09-17T07:27:11.921Z",
  "reportingController": "kustomize-controller",
  "reason": "ApplySucceed",
  "message": "Kustomization applied in 1.4s, revision: master/a1afe267b54f38b46b487f6e938a6fd508278c07",
  "involvedObject": {
    "kind": "Kustomization",
    "name": "webapp-backend",
    "namespace": "default"
  },
  "metadata": {
    "service/backend": "created",
    "deployment.apps/backend": "created",
    "horizontalpodautoscaler.autoscaling/backend": "created"
  }
}
```
