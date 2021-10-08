# Provider

The `Provider` API defines how events are encoded and the webhook address where they are dispatched.

## Specification

Spec:

```go
type ProviderSpec struct {
	// Type of provider
	// +kubebuilder:validation:Enum=slack;discord;msteams;rocket;generic;github;gitlab;bitbucket;azuredevops;googlechat;webex;sentry;azureeventhub;telegram;lark;matrix;opsgenie
	// +required
	Type string `json:"type"`

	// Alert channel for this provider
	// +optional
	Channel string `json:"channel,omitempty"`

	// Bot username for this provider
	// +optional
	Username string `json:"username,omitempty"`

	// HTTP/S webhook address of this provider
	// +kubebuilder:validation:Pattern="^(http|https)://"
	// +optional
	Address string `json:"address,omitempty"`

	// HTTP/S address of the proxy
	// +kubebuilder:validation:Pattern="^(http|https)://"
	// +optional
	Proxy string `json:"proxy,omitempty"`

	// Secret reference containing the provider webhook URL
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// CertSecretRef can be given the name of a secret containing
	// a PEM-encoded CA certificate (`caFile`)
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`
}
```

Notification providers:

* Slack
* Discord
* Microsoft Teams
* Rocket
* Google Chat
* Webex
* Sentry
* Telegram
* Lark
* Matrix
* Azure Event Hub
* Generic webhook
* Opsgenie

Git commit status providers:

* GitHub
* GitLab
* Bitbucket
* Azure DevOps

Status:

```go
// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}
```

Status condition types:

```go
const (
	// ReadyCondition represents the fact that a given object has passed
	// validation and was acknowledge by the controller.
	ReadyCondition string = "Ready"
)
```

## Example

### Notifications

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: slack
  namespace: default
spec:
  type: slack
  channel: general
  # webhook address (ignored if secretRef is specified)
  address: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
  # HTTP(S) proxy (optional)
  proxy: https://proxy.corp:8080
  # secret containing the webhook address (optional)
  secretRef:
    name: webhook-url
```

Webhook URL secret:

```sh
kubectl create secret generic webhook-url \
--from-literal=address=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
```

Note that the secret must contain an `address` field.

The provider type can be: `slack`, `msteams`, `rocket`, `discord`, `googlechat`, `webex`, `sentry`,
`telegram`, `lark`, `matrix`, `azureeventhub` or `generic`.

When type `generic` is specified, the notification controller will post the
incoming [event](event.md) in JSON format to the webhook address.

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
    "kind":"GitRepository",
    "namespace":"flux-system",
    "name":"flux-system",
    "uid":"cc4d0095-83f4-4f08-98f2-d2e9f3731fb9",
    "apiVersion":"source.toolkit.fluxcd.io/v1beta1",
    "resourceVersion":"56921",
  },
  "severity":"info",
  "timestamp":"2006-01-02T15:04:05Z",
  "message":"Fetched revision: main/731f7eaddfb6af01cb2173e18f0f75b0ba780ef1",
  "reason":"info",
  "reportingController":"source-controller",
  "reportingInstance":"source-controller-7c7b47f5f-8bhrp",
}
```

The `involvedObject` key contains the object that triggered the event.

### Self-signed certificates

The `certSecretRef` field names a secret with TLS certificate data. This is for the purpose
of enabling a provider to communicate with a server using a self-signed cert.

To use the field create a secret, containing a CA file, in the same namespace and reference
it from the provider.

```sh
kubectl create secret generic tls-certs \
  --from-file=caFile=ca.crt
```

### Sentry

The sentry provider uses the `channel` field to specify which environment the messages are sent for:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: sentry
  namespace: default
spec:
  type: sentry
  channel: my-cluster-name
  # webhook address (ignored if secretRef is specified)
  address: https://....@sentry.io/12341234
```

The sentry provider also sends traces for events with the severity Info. This can be disabled by setting
the `eventSeverity` field on the related `Alert` Rule to `error`.

### Telegram

For telegram, You can get the token from [the botfather](https://core.telegram.org/bots#6-botfather)
and use `https://api.telegram.org/` as the api url.

```sh
 k create secret generic telegram-token \
 --from-literal=token=<token> \
 --from-literal=address=https://api.telegram.org
```

Also note that the channel name should start with '@'.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: telegram
  namespace: flux-system
spec:
  type: telegram
  channel: "@fluxtest"
  secretRef:
    name: telegram-token
```

### Matrix

For Matrix, the address is the homeserver URL and the token is the access token
returned by a call to `/login` or `/register`.

Create a secret:
```
kubectl create secret generic matrix-token \
--from-literal=token=<access-token> \
--from-literal=address=https://matrix.org # replace with if using a different server
```

Then reference the secret in `spec.secretRef`:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: matrix
  namespace: default
spec:
  type: matrix
  channel: "!jezptmDwEeLapMLjOc:matrix.org"
  secretRef:
    name: matrix-token
```

Note that `spec.channel` holds the room id.

### Lark

For sending notifications to Lark, you will have to
[add a bot to the group](https://www.larksuite.com/hc/en-US/articles/360048487736-Bot-Use-bots-in-groups#III.%20How%20to%20configure%20custom%20bots%20in%20a%20group%C2%A0)
and set up a webhook for the bot. This serves as the address field in the secret:

```shell
kubectl create secret generic lark-token \
--from-literal=address=<lark-webhook-url>
```

Then reference the secret in `spec.secretRef`:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: lark
  namespace: default
spec:
  type: lark
  secretRef:
    name: lark-token
```


### Opsgenie

For sending notifications to Opsgenie, you will have to
[add a REST api integration](https://support.atlassian.com/opsgenie/docs/create-a-default-api-integration/)
and setup a api integration for notification provider.

A secret needs to be generated with the api key given by Opsgenie for the integration

```shell
kubectl create secret generic opsgenie-token \
--from-literal=token=<opsgenie-api-key>
```

Then reference the secret in `spec.secretRef`:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
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


### Git commit status

The GitHub, GitLab, Bitbucket, and Azure DevOps provider will write to the
commit status in the git repository from which the event originates from.

!!! hint "Limitations"
The git notification providers require that a commit hash present in the meta data
of the event. There for the the providers will only work with `Kustomization` as an
event source, as it is the only resource which includes this data.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: podinfo
  namespace: default
spec:
  # provider type can be github or gitlab
  type: github
  address: https://github.com/stefanprodan/podinfo
  secretRef:
    name: api-token
```

The provider type can be: `github`, `gitlab`, `bitbucket` or `azuredevops`.

For bitbucket, the token should contain the username and [app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/#Create-an-app-password) 
in the format `<username>:<password>`. The app password should have `Repositories (Read/Write)` permission.

You can create the secret using this command:
```shell
kubectl create secret generic api-token --from-literal=token=<username>:<app-password>
```

#### Authentication

GitHub. GitLab, and Azure DevOps use personal access tokens to authenticate with their API:

- [GitHub personal access token](https://docs.github.com/en/free-pro-team@latest/github/authenticating-to-github/creating-a-personal-access-token)
- [GitLab personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [Azure DevOps personal access token](https://docs.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=preview-page)

The providers require a secret in the same format, with the personal access token as the value for the token key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <personal-access-tokens>
```

Bitbucket authenticates using an [app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/).
It requires both the username and the password when authenticating.
There for the token needs to be passed with the format `<username>:<app-password>`.
A token that is not in this format will cause the provider to fail.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <username>:<app-password>
```


Opsgenie uses an api key to authenticate [api key](https://support.atlassian.com/opsgenie/docs/api-key-management/).
The providers require a secret in the same format, with the api key as the value for the token key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
data:
  token: <api-key>
```
### Azure Event Hub

The Azure Event Hub supports two authentication methods, [JWT](https://docs.microsoft.com/en-us/azure/event-hubs/authenticate-application)
and [SAS](https://docs.microsoft.com/en-us/azure/event-hubs/authorize-access-shared-access-signature) based.

#### JWT based auth

In JWT we use 3 input values. Channel, token and address.
We perform the following translation to match we the data we need to communicate with Azure Event Hub.

* channel = Azure Event Hub namespace
* address = Azure Event Hub name
* token   = JWT

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: azureeventhub
spec:
  type: azureeventhub
  channel: fluxv2
  secretRef:
    name: webhook-url
---
apiVersion: v1
data:
  address: Zmx1eHYy
  token: QS12YWxpZC1KV1QtdG9rZW4=
kind: Secret
metadata:
  name: webhook-url
  namespace: default
type: Opaque
```

Notification controller doesn't take any responsibility for the JWT token to be updated.
You need to use a secondary tool to make sure that the token in the secret is renewed.

If you want to make a easy test assuming that you have setup a Azure Enterprise application and you called it
event-hub you can follow most of the bellow commands. You will need to provide the client_secret that you got
when generating the Azure Enterprise Application.

```shell
export AZURE_CLIENT=$(az ad app list --filter "startswith(displayName,'event-hub')" --query '[].appId' |jq -r '.[0]')
export AZURE_SECRET='secret-client-secret-generated-at-creation'
export AZURE_TENANT=$(az account show -o tsv --query tenantId)

curl -X GET --data 'grant_type=client_credentials' --data "client_id=$AZURE_CLIENT" --data "client_secret=$AZURE_SECRET" --data 'resource=https://eventhubs.azure.net' -H 'Content-Type: application/x-www-form-urlencoded' https://login.microsoftonline.com/$AZURE_TENANT/oauth2/token |jq .access_token
```

Use the output you got from the curl and add it to your secret like bellow.

```shell
kubectl create secret generic webhook-url \
--from-literal=address="fluxv2" \
--from-literal=token='A-valid-JWT-token'
```

#### SAS based auth

In SAS we only use the `address`field in the secret.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta1
kind: Provider
metadata:
  name: azureeventhub
spec:
  type: azureeventhub
  secretRef:
    name: webhook-url
---
apiVersion: v1
data:
  address: RW5kcG9pbnQ9c2I6Ly9mbHV4djIuc2VydmljZWJ1cy53aW5kb3dzLm5ldC87U2hhcmVkQWNjZXNzS2V5TmFtZT1Sb290TWFuYWdlU2hhcmVkQWNjZXNzS2V5O1NoYXJlZEFjY2Vzc0tleT15b3Vyc2Fza2V5Z2VuZWF0ZWRieWF6dXJlCg==
kind: Secret
metadata:
  name: webhook-url
  namespace: default
type: Opaque
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
kubectl create secret generic webhook-url \
--from-literal=address="Endpoint=sb://fluxv2.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=yoursaskeygeneatedbyazure"
```
