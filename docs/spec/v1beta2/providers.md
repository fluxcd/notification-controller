# Providers

The `Provider` API defines how events are encoded and where to send them.

## Example

The following is an example of how to send alerts to Slack when Flux fails to install or upgrade Flagger.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: slack-bot
  namespace: flagger-system
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
  namespace: flagger-system
spec:
  summary: "Flagger impacted in us-east-2"
  providerRef:
    name: slack-bot
  eventSeverity: error
  eventSources:
    - kind: HelmRepository
      name: '*'
    - kind: HelmRelease
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
  all HelmRepositories and HelmReleases in the `flagger-system` namespace.
- When an event with severity `error` is received, the controller posts
  a message on Slack containing the `summary` text and the Helm install or upgrade error.
- The controller uses the Slack Bot token from the secret indicated by the
  `Provider.spec.secretRef.name` to authenticate with the Slack API.

You can run this example by saving the manifests into `slack-alerts.yaml`.

1. First create a secret with the Slack bot token:

   ```sh
   kubectl -n flagger-system create secret generic slack-bot-token --from-literal=token=xoxb-YOUR-TOKEN
   ```

2. Apply the resources on the cluster:

   ```sh
   kubectl -n flagger-system apply --server-side -f slack-alerts.yaml
   ```

3. Run `kubectl -n flagger-system describe provider slack-bot` to see its status:

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
     Normal  Succeeded 82s   notification-controller  Reconciliation finished, next run in 10m
   ```

## Writing a provider spec

As with all other Kubernetes config, a Provider needs `apiVersion`,
`kind`, and `metadata` fields. The name of an Alert object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).
A Provider also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Type

`.spec.type` is a required field that specifies which SaaS API to use.

The supported providers are:

| Provider                                                | Type             |
|---------------------------------------------------------|------------------|
| [Prometheus Alertmanager](#prometheus-alertmanager)     | `alertmanager`   |
| [Azure Event Hub](#azure-event-hub)                     | `azureeventhub`  |
| [Discord](#discord)                                     | `discord`        |
| [Generic webhook](#generic-webhook)                     | `generic`        |
| [Generic webhook with HMAC](#generic-webhook-with-hmac) | `generic-hmac`   |
| [GitHub dispatch](#github-dispatch)                     | `githubdispatch` |
| [Google Chat](#google-chat)                             | `googlechat`     |
| [Grafana](#grafana)                                     | `grafana`        |
| [Lark](#lark)                                           | `lark`           |
| [Matrix](#matrix)                                       | `matrix`         |
| [Microsoft Teams](#microsoft-teams)                     | `msteams`        |
| [Opsgenie](#opsgenie)                                   | `opsgenie`       |
| [Rocket](#rocket)                                       | `rocket`         |
| [Sentry](#sentry)                                       | `sentry`         |
| [Slack](#slack)                                         | `slack`          |
| [Telegram](#telegram)                                   | `telegram`       |
| [WebEx](#webex)                                         | `webex`          |

### Address

`.spec.address` is an optional field that specifies the URL where the events are posted.

If the address contains sensitive information such as tokens or passwords, it is 
recommended to store the address in the Kubernetes secret referenced by `.spec.secretRef.name`.
When the referenced Secret contains an `address` key, the `.spec.address` value is ignored.

### Channel

`.spec.channel` is an optional field that specifies the channel where the events are posted.

### Secret reference

`.spec.secretRef.name` is an optional field to specify a name reference to a
Secret in the same namespace as the Provider, containing the authentication
credentials for the provider API.

The Kubernetes secret can have any of the following keys:

- `address` - overrides `.spec.address`
- `proxy` - overrides `.spec.proxy`
- `token` - used for authentication
- `headers` - HTTP headers values included in the POST request

#### Address example

For providers which embed tokens or other sensitive information in the URL,
the incoming webhooks address can be stored in the secret using the `address` key:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-provider-url
  namespace: default
stringData:
  address: "https://webhook.example.com/token"
```

#### Token example

For providers which require token based authentication, the API token
can be stored in the secret using the `token` key:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-provider-auth
  namespace: default
stringData:
  token: "my-api-token"
```

#### HTTP headers example

For providers which require specific HTTP headers to be attached to the POST request,
the headers can be set in the secret using the `headers` key:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-provider-headers
  namespace: default
stringData:
  headers: |
     Authorization: my-api-token
     X-Forwarded-Proto: https
```

#### Proxy auth example

Some networks need to use an authenticated proxy to access external services.
Therefore, the proxy address can be stored as a secret to hide parameters like the username and password:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-provider-proxy
  namespace: default
stringData:
  proxy: "http://username:password@proxy_url:proxy_port"
```

### TLS certificates

`.spec.certSecretRef` is an optional field to specify a name reference to a
Secret in the same namespace as the Provider, containing the TLS CA certificate.

#### Example

To enable notification-controller to communicate with a provider API over HTTPS
using a self-signed TLS certificate, set the `caFile` like so:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: my-webhook
  namespace: flagger-system
spec:
  type: generic
  address: https://my-webhook.internal
  certSecretRef:
    name: my-ca-crt
---
apiVersion: v1
kind: Secret
metadata:
  name: my-ca-crt
  namespace: default
stringData:
  caFile: |
    <--- CA Key --->
```

### HTTP/S proxy

`.spec.proxy` is an optional field to specify an HTTP/S proxy address.

If the proxy address contains sensitive information such as basic auth credentials, it is
recommended to store the proxy in the Kubernetes secret referenced by `.spec.secretRef.name`.
When the referenced Secret contains a `proxy` key, the `.spec.proxy` value is ignored.

### Interval

`.spec.interval` is a required field with a default of ten minutes that specifies
the time interval at which the controller reconciles the provider with its Secret
references.

### Suspend

`.spec.suspend` is an optional field to suspend the provider.
When set to `true`, the controller will stop sending events to this provider.
When the field is set to `false` or removed, it will resume.

## Working with Providers

### Generic webhook

The `generic` webhook triggers an HTTP POST request to the provided endpoint.

The `Gotk-Component` header identifies which component this event is coming
from, e.g. `source-controller`, `kustomize-controller`.

```
POST / HTTP/1.1
Host: example.com
Accept-Encoding: gzip
Content-Length: 452
Content-Type: application/json
Gotk-Component: source-controller
User-Agent: Go-http-client/1.1
```

The body of the request looks like this:

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
  "reportingInstance":"kustomize-controller-7c7b47f5f-8bhrp",
  "timestamp":"2022-10-28T07:26:19Z"
}
```

The `involvedObject` key contains the object that triggered the event.

You can add additional headers to the POST request by providing a `headers` field to the secret
referenced by the provider. An example is given below:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: generic
  namespace: default
spec:
  type: generic
  address: https://api.github.com/repos/owner/repo/dispatches
  secretRef:
    name: generic-secret
---
apiVersion: v1
kind: Secret
metadata:
  name: generic-secret
  namespace: default
stringData:
  headers: |
     Authorization: token
     X-Forwarded-Proto: https
```

### Generic webhook with HMAC

If you set the `.spec.type` of a `Provider` resource to `generic-hmac` then the HTTP request
sent to the webhook will include the `X-Signature` HTTP header carrying the HMAC of the request body.
This allows the webhook server to authenticate the request.
The key used for the HMAC must be supplied in the `token` field of the Secret resource referenced in `.spec.secretRef`.
The HTTP header value has the following format:

```
X-Signature: HASH_FUNC=HASH
```

`HASH_FUNC` denotes the Hash function used to generate the HMAC and currently defaults
to `sha256` but may change in the future. You must make sure to take this value into
account when verifying the HMAC. `HASH` is the hex-encoded HMAC value.
The following Go code illustrates how the header is parsed and verified:

```go
func verifySignature(sig string, payload, key []byte) error {
	sigHdr := strings.Split(sig, "=")
	if len(shgHdr) != 2 {
		return fmt.Errorf("invalid signature value")
	}
	var newF func() hash.Hash
	switch sigHdr[0] {
	case "sha224":
		newF = sha256.New224
	case "sha256":
		newF = sha256.New
	case "sha384":
		newF = sha512.New384
	case "sha512":
		newF = sha512.New
	default:
		return fmt.Errorf("unsupported signature algorithm %q", sigHdr[0])
	}
	mac := hmac.New(newF, key)
	if _, err := mac.Write(payload); err != nil {
		return fmt.Errorf("error MAC'ing payload: %w", err)
	}
	sum := fmt.Sprintf("%x", mac.Sum(nil))
	if sum != sigHdr[1] {
		return fmt.Errorf("HMACs don't match: %#v != %#v", sum, sigHdr[1])
	}
	return nil
}
[...]
key := []byte("b1fad212fb1b87a56c79e5da48018650b85ab7cf")
if len(r.Header["X-Signature"]) > 0 {
	if err := verifySignature(r.Header["X-Signature"][0], body, key); err != nil {
		// handle signature verification failure here
	}
}
```

### Slack

To send alerts to Slack, we recommend using a Slack Bot App token.
To obtain a token, please follow [Slack's guide on bot users](https://api.slack.com/bot-users).

Once you have a Slack bot token (starts with `xoxb-`), create a secret for it with:

```shell
kubectl create secret generic slack-token --from-literal=token=BOT-TOKEN
```

Create a provider of type `slack`, with the address set to `https://slack.com/api/chat.postMessage`
and reference the `slack-token` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: slack
  namespace: default
spec:
  type: slack
  channel: general
  address: https://slack.com/api/chat.postMessage
  secretRef:
    name: slack-token
```

Slack legacy webhooks are also supported, the webhook URL can be set in the `address` field or in the secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: slack
  namespace: default
spec:
  type: slack
  secretRef:
    name: slack-webhook
---
apiVersion: v1
kind: Secret
metadata:
  name: slack-webhook
  namespace: default
stringData:
  address: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
```

### Microsoft Teams

To send alerts to Teams, first create an
[incoming webhook](https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook)
on the Microsoft Teams UI:

1. Open the settings of the channel you want the notifications to be sent to.
2. Click on `Connectors`.
3. Click on the `Add` button for `Incoming Webhook`.
4. Click on `Configure` and copy the webhook URL given.

Once you have the webhook URL, create a secret for it with:

```shell
kubectl create secret generic teams-webhook \
--from-literal=address=<YOUR-TEAMS-WEBHOOK>
```

Create a provider of type `msteam` and reference the `teams-webhook` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: msteams
  namespace: default
spec:
  type: msteams
  secretRef:
    name: slack-webhook
```

### Discord

To send events to Discord, first [create a webhook](https://discord.com/developers/docs/resources/webhook#create-webhook).

Once you have the webhook URL, create a secret for it with:

```shell
kubectl create secret generic discord-webhook \
--from-literal=address=<YOUR-WEBHOOK>
```

Create a provider of type `discord` and reference the `discord-webhook` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: discord
  namespace: default
spec:
  type: discord
  secretRef:
    name: discord-webhook
```

### Sentry

To send events to Sentry, create a provider of type `sentry` and a secret with the Sentry URL:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: sentry
  namespace: default
spec:
  type: sentry
  channel: staging-env
  secretRef:
    name: sentry-webhook
---
apiVersion: v1
kind: Secret
metadata:
  name: sentry-webhook
  namespace: default
stringData:
  address: "https://....@sentry.io/12341234"
```

Note that the `.spec.channel` field can be used to specify which environment the messages are sent for.

The sentry provider also sends traces for events with the severity `info`.
This can be disabled by setting, the `Alert.spec.eventSeverity` field to `error`.

### Telegram

For telegram, You can get the token from [the botfather](https://core.telegram.org/bots#6-botfather)
and use `https://api.telegram.org/` as the address.

Once you have a Telegram token, create a secret for it with:

```shell
kubectl create secret generic telegram-token \
--from-literal=token=BOT-TOKEN
```

Create a provider of type `telegram` and reference the `telegram-token` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: telegram
  namespace: default
spec:
  type: telegram
  address: https://api.telegram.org
  channel: "@fluxtest" # or "-1557265138" (channel id)
  secretRef:
    name: telegram-token
```

Note that `.spec.channel` can be a unique identifier for the target chat
or the username of the target channel (in the format `@channelusername`).

### Matrix

For Matrix, the address is the homeserver URL and the token is the access token
returned by a call to `/login` or `/register`.

Once you have a Matrix token, create a secret for it with:

```shell
kubectl create secret generic matrix-token \
--from-literal=token=MY-TOKEN
```

Create a provider of type `matrix` and reference the `matrix-token` secret:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: matrix
  namespace: default
spec:
  type: matrix
  address: https://matrix.org
  channel: "!jezptmDwEeLapMLjOc:matrix.org"
  secretRef:
    name: matrix-token
```

Note that `.spec.channel` holds the room ID.

### Lark

For sending notifications to Lark, you will have to
[add a bot to the group](https://www.larksuite.com/hc/en-US/articles/360048487736-Bot-Use-bots-in-groups#III.%20How%20to%20configure%20custom%20bots%20in%20a%20group%C2%A0)
and set up a webhook for that bot account. This serves as the address field in the secret:

```shell
kubectl create secret generic lark-webhook \
--from-literal=address=<lark-webhook-url>
```

Create a provider of type `lark` and reference the `lark-webhook` secret:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: lark
  namespace: default
spec:
  type: lark
  secretRef:
    name: lark-webhook
```

### Rocket

To send events to Rocket chat, first [create an incoming webhook](https://docs.rocket.chat/guides/administration/admin-panel/integrations#create-a-new-incoming-webhook).

Once you have the webhook URL, create a secret for it with:

```shell
kubectl create secret generic rocket-webhook \
--from-literal=address=<YOUR-WEBHOOK>
```

Create a provider of type `rocket` and reference the `rocket-webhook` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: rocket
  namespace: default
spec:
  type: rocket
  secretRef:
    name: rocket-webhook
```

### Google Chat

To send notifications to Google chat, first [create an incoming webhook](https://developers.google.com/chat/how-tos/webhooks#create_a_webhook).

Once you have the webhook URL, create a secret for it with:

```shell
kubectl create secret generic google-webhook \
--from-literal=address=<YOUR-WEBHOOK>
```

Create a provider of type `googlechat` and reference the `google-webhook` secret:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: google
  namespace: default
spec:
  type: googlechat
  secretRef:
    name: google-webhook
```

### Opsgenie

To send notifications to Opsgenie, first
[add a REST API integration](https://support.atlassian.com/opsgenie/docs/create-a-default-api-integration/).

Once you have a Opsgenie API key, create a secret for it with:

```shell
kubectl create secret generic opsgenie-token \
--from-literal=token=<opsgenie-api-key>
```

Create a provider of type `opsgenie` and reference the `opsgenie-token` secret:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: opsgenie
  namespace: default
spec:
  type: opsgenie
  address: https://api.opsgenie.com/v2/alerts
  secretRef:
    name: opsgenie-token
```

### Prometheus Alertmanager

To send events to the Prometheus [Alertmanager v2 API](https://github.com/prometheus/alertmanager/blob/main/api/v2/openapi.yaml),
create a provider of type `alertmanager` and set the `address` field to the `api/v2/alerts` endpoint:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: alertmanager
  namespace: default
spec:
  type: alertmanager
  # webhook address (ignored if secretRef is specified)
  address: https://....@<alertmanager-url>/api/v2/alerts/"
```

If Alertmanager has basic authentication configured, it is recommended to use
`.spec.secretRef` and include the `username:password` in the address string inside the secret.

When an event is received, the controller will send a single alert with at least one annotation
which is the `message` found for the event.
If an  `Alert.spec.summary` is provided, an additional "summary" annotation will be added.

The provider will send the following labels for the event:


| Label     | Description                                                                                          |
|-----------|------------------------------------------------------------------------------------------------------|
| alertname | The string Flux followed by the Kind and the reason for the event e.g `FluxKustomizationProgressing` |
| severity  | The severity of the event (`error` or `info`)                                                        |
| timestamp | The timestamp of the event                                                                           |
| reason    | The machine readable reason for the objects transition into the current status                       |
| kind      | The kind of the involved object associated with the event                                            |
| name      | The name of the involved object associated with the event                                            |
| namespace | The namespace of the involved object associated with the event                                       |

### Webex

General steps on how to send notifications to a Webex space:

From the Webex App UI:

- create a Webex space where you want notifications to be sent
- after creating a Webex bot (described in next section), add the bot email address to the Webex space ("People | Add people")

Register to https://developer.webex.com/, after signing in:

- Create a bot for forwarding Flux notifications to a Webex Space
  (User profile icon | MyWebexApps | Create a New App | Create a Bot).
- Make a note of the bot email address, this email needs to be added to the Webex space from the Webex App.
- Generate a bot access token, this is the ID to use in the kubernetes Secret "token" field.
- Find the room ID associated to the webex space using https://developer.webex.com/docs/api/v1/rooms/list-rooms
  (select GET, click on "Try It" and search the GET results for the matching Webex space entry),
  this is the ID to use in the webex Provider manifest "channel" field.

Example:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: webex
  namespace: default
spec:
  type: webex
  address: https://webexapis.com/v1/messages
  channel: <webexSpaceRoomID>
  secretRef:
    name: webex-token
---
apiVersion: v1
kind: Secret
metadata:
  name: webex-token
  namespace: default
stringData:
  token: <bot-token>
```

Notes:

- `.spec.address` should always be set to the same global Webex API gateway `https://webexapis.com/v1/messages`
- `.spec.channel` should contain the Webex space room ID as obtained from `https://developer.webex.com/` (long alphanumeric string copied as is).

If you do not see any notifications in the targeted Webex space, check that you have added the bot
email address to the Webex space, if the bot email address is not added to the space,
the notification-controller will log a 404 room not found error every time a notification is sent out.

### Grafana

To send notifications to [Grafana annotations API](https://grafana.com/docs/grafana/latest/http_api/annotations/),
enable the annotations on a Dashboard like so:

- Annotations > Query > Enable Match any
- Annotations > Query > Tags (Add Tag: `flux`)

If Grafana has authentication configured, create a Kubernetes Secret with the API token:

```shell
kubectl create secret generic grafana-token \
--from-literal=token=<grafana-api-key> \
```

Grafana can also use basic authorization to authenticate the requests, if both the token and
the username/password are set in the secret, then token takes precedence over`basic auth:

```shell
kubectl create secret generic grafana-token \
--from-literal=username=<your-grafana-username> \
--from-literal=password=<your-grafana-password>
```

Create a provider of type `grafana` and reference the `grafana-token` secret:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: grafana
  namespace: default
spec:
  type: grafana
  address: https://<grafana-url>/api/annotations
  secretRef:
    name: grafana-token
```

### GitHub dispatch

The `githubdispatch` provider generates GitHub events of type
[`repository_dispatch`](https://docs.github.com/en/rest/reference/repos#create-a-repository-dispatch-event)
for the selected repository. The `repository_dispatch` events can be used to trigger GitHub Actions workflow.

The request includes the `event_type` and `client_payload` fields:

- `event_type` is generated from the involved object in the format `{Kind}/{Name}.{Namespace}`.
- `client_payload` contains the [Flux event](events.md).

### Setting up the GitHub dispatch provider

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: github-dispatch
  namespace: flux-system
spec:
  type: githubdispatch
  address: https://github.com/stefanprodan/podinfo
  secretRef:
    name: api-token
```

The `address` is the address of your repository where you want to send webhooks to trigger GitHub workflows.

GitHub uses personal access tokens for authentication with its API:

* [GitHub personal access token](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token)

The provider requires a secret in the same format, with the personal access token as the value for the token key:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <personal-access-tokens>
```

#### Setting up a GitHub workflow

To trigger a GitHub Actions workflow when a Flux Kustomization finishes reconciling,
you need to set the event type for the repository_dispatch trigger to match the Flux object ID:

```yaml
name: test-github-dispatch-provider
on:
  repository_dispatch:
    types: [Kustomization/podinfo.flux-system]
```

Assuming that we deploy all Flux kustomization resources in the same namespace,
it will be useful to have a unique kustomization resource name for each application.
This will allow you to use only `event_type` to trigger tests for the exact application.

Let's say we have following folder structure for applications kustomization manifests:

```bash
apps/
├── app1
│   └── overlays
│       ├── production
│       └── staging
└── app2
    └── overlays
        ├── production
        └── staging
```

You can then create a flux kustomization resource for the app to have unique `event_type` per app.
The kustomization manifest for app1/staging:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: app1
  namespace: flux-system
spec:
  path: "./app1/staging"
```

You would also like to know from the notification which cluster is being used for deployment.
You can add the `spec.summary` field to the Flux alert configuration to mention the relevant cluster:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: github-dispatch
  namespace: flux-system
spec:
  summary: "staging (us-west-2)"
  providerRef:
    name: github-dispatch
  eventSeverity: info
  eventSources:
    - kind: Kustomization
      name: 'podinfo'
```

Now you can the trigger tests in the GitHub workflow for app1 in a staging cluster when
the app1 resources defined in `./app1/staging/` are reconciled by Flux:

```yaml
name: test-github-dispatch-provider
on:
  repository_dispatch:
    types: [Kustomization/podinfo.flux-system]
jobs:
  run-tests-staging:
    if: github.event.client_payload.metadata.summary == 'staging (us-west-2)'
    runs-on: ubuntu-18.04
    steps:
    - name: Run tests
      run: echo "running tests.."
```

### Azure Event Hub

The Azure Event Hub supports two authentication methods, [JWT](https://docs.microsoft.com/en-us/azure/event-hubs/authenticate-application)
and [SAS](https://docs.microsoft.com/en-us/azure/event-hubs/authorize-access-shared-access-signature) based.

#### JWT based auth

In JWT we use 3 input values. Channel, token and address.
We perform the following translation to match we the data we need to communicate with Azure Event Hub.

- channel = Azure Event Hub namespace
- address = Azure Event Hub name 
- token   = JWT

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: azure
  namespace: default
spec:
  type: azureeventhub
  address: <event-hub-name>
  channel: <event-hub-namespace>
  secretRef:
    name: azure-token
---
apiVersion: v1
kind: Secret
metadata:
  name: azure-token
  namespace: default
stringData:
  token: <event-hub-token>
```

The controller doesn't take any responsibility for the JWT token to be updated.
You need to use a secondary tool to make sure that the token in the secret is renewed.

If you want to make a easy test assuming that you have setup a Azure Enterprise application and you called it
event-hub you can follow most of the bellow commands. You will need to provide the `client_secret` that you got
when generating the Azure Enterprise Application.

```shell
export AZURE_CLIENT=$(az ad app list --filter "startswith(displayName,'event-hub')" --query '[].appId' |jq -r '.[0]')
export AZURE_SECRET='secret-client-secret-generated-at-creation'
export AZURE_TENANT=$(az account show -o tsv --query tenantId)

curl -X GET --data 'grant_type=client_credentials' --data "client_id=$AZURE_CLIENT" --data "client_secret=$AZURE_SECRET" --data 'resource=https://eventhubs.azure.net' -H 'Content-Type: application/x-www-form-urlencoded' https://login.microsoftonline.com/$AZURE_TENANT/oauth2/token |jq .access_token
```

Use the output you got from `curl` and add it to your secret like bellow:

```shell
kubectl create secret generic azure-token \
--from-literal=token='A-valid-JWT-token'
```

#### SAS based auth

When using SAS auth, we only use the `address` field in the secret.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: azure
  namespace: default
spec:
  type: azureeventhub
  secretRef:
    name: azure-webhook
---
apiVersion: v1
kind: Secret
metadata:
  name: azure-webhook
  namespace: default
stringData:
  address: <SAS-URL>
```

Assuming that you have created Azure event hub and namespace you should be able to use a similar command to get your
connection string. This will give you the default Root SAS, it's NOT supposed to be used in production.

```shell
az eventhubs namespace authorization-rule keys list --resource-group <rg-name> --namespace-name <namespace-name> --name RootManageSharedAccessKey -o tsv --query primaryConnectionString
# The output should look something like this:
Endpoint=sb://fluxv2.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=yoursaskeygeneatedbyazure
```

To create the needed secret:

```shell
kubectl create secret generic azure-webhook \
--from-literal=address="Endpoint=sb://fluxv2.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=yoursaskeygeneatedbyazure"
```
