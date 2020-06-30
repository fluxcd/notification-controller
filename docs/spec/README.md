# Notification Controller

The Notification Controller is a Kubernetes operator, specialized in 
dispatching events to external notification systems.

## Motivation

The main goal is to provide a notification service that can
receive events via HTTP and dispatch them to external webhooks
based on event severity and involved objects.

When operating a cluster, different teams may wish to receive notification about the status
of their CD pipelines. For example, the on-call team would receive alerts about all
failures in the cluster, while the dev team may wish to be alerted when a new version 
of an app was deployed and if the deployment is healthy.

## Design

The controller exposes an HTTP endpoint for receiving events from other controllers.
An event must contain information about the involved object such as kind, name, namespace,
a human-readable description of the event and the severity type e.g. info or error.

The controller can be configured with Kubernetes custom resources that define how
events are processed and where to dispatch them.

Notification API:

* [Provider](v1alpha1/provider.md)
* [Alert](v1alpha1/alert.md)
* [Event](v1alpha1/event.md)

## Example

After installing notification-controller, we can configure alerting for events issued
by source-controller and kustomize-controller.

Create a notification provider for Slack:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Provider
metadata:
  name: slack
  namespace: gitops-system
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
  namespace: gitops-system
data:
  address: <encoded-url>
```

Create an alert for a list of GitRepositories and Kustomizations:

```yaml
apiVersion: notification.fluxcd.io/v1alpha1
kind: Alert
metadata:
  name: on-call-webapp
  namespace: gitops-system
spec:
  providerRef: 
    name: slack
  eventSeverity: info
  eventSources:
    - kind: GitRepository
      name: webapp
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
  "timestamp": 1587195448.071468,
  "reportingController": "kustomize-controller",
  "reason": "ApplySucceed",
  "message": "Kustomization applied in 1.4s, revision: master/a1afe267b54f38b46b487f6e938a6fd508278c07",
  "involvedObject": {
    "kind": "Kustomization",
    "name": "webapp-backend",
    "namespace": "gitops-system"
  },
  "metadata": {
    "service/backend": "created",
    "deployment.apps/backend": "created",
    "horizontalpodautoscaler.autoscaling/backend": "created"
  }
}
```

Slack message example:

![info alert](https://raw.githubusercontent.com/fluxcd/kustomize-controller/master/docs/diagrams/slack-info-alert.png)