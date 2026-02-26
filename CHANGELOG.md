# Changelog

All notable changes to this project are documented in this file.

## 1.8.1

**Release date:** 2026-02-27

This patch release fixes a regression introduced in 1.8.0 where commit
status events were dropped.

Fixes:
- Fix commit status providers requiring only commit event key
  [#1247](https://github.com/fluxcd/notification-controller/pull/1247)

Improvements:
- Remove no longer needed workaround for Flux 2.8
  [#1244](https://github.com/fluxcd/notification-controller/pull/1244)
- Update fluxcd/pkg dependencies
  [#1250](https://github.com/fluxcd/notification-controller/pull/1250)

## 1.8.0

**Release date:** 2026-02-17

This minor release comes with new notification providers for pull/merge request
comments, expanded NATS authentication support, and various improvements.

### Provider

New notification providers have been introduced for posting comments on
pull/merge requests: `githubpullrequestcomment`, `gitlabmergerequestcomment`,
and `giteapullrequestcomment`.

The NATS provider has been enhanced to support multiple authentication methods
including JWT, NKey, and Username/Password, in addition to the existing
token-based authentication.

The GitHub provider now supports looking up the GitHub App installation ID
automatically, removing the need to configure it manually.

Commit status reporting has been extended to support any Flux APIs that include
commit metadata.

### Alert

The `ArtifactGenerator` and `ExternalArtifact` kinds can now be used as Alert
event sources.

### General updates

In addition, the Kubernetes dependencies have been updated to v1.35.0 and
the controller is now built with Go 1.26.

Fixes:
- Fix notification-controller memory leak in the `gitea` provider
  [#1228](https://github.com/fluxcd/notification-controller/pull/1228)

Improvements:
- Introduce `githubpullrequestcomment` notification provider
  [#1230](https://github.com/fluxcd/notification-controller/pull/1230)
- Introduce `gitlabmergerequestcomment` notification provider
  [#1231](https://github.com/fluxcd/notification-controller/pull/1231)
- Introduce `giteapullrequestcomment` notification provider
  [#1234](https://github.com/fluxcd/notification-controller/pull/1234)
- Improve zulip Alert Provider comment
  [#1232](https://github.com/fluxcd/notification-controller/pull/1232)
- Support commit status for any Flux APIs
  [#1233](https://github.com/fluxcd/notification-controller/pull/1233)
- Enhance NATS provider to support multiple authentication methods
  [#1222](https://github.com/fluxcd/notification-controller/pull/1222)
- Introduce support for looking up GH app installation ID
  [#1223](https://github.com/fluxcd/notification-controller/pull/1223)
- Add `ArtifactGenerator` and `ExternalArtifact` kinds to Alert source
  [#1237](https://github.com/fluxcd/notification-controller/pull/1237)
- Various dependency updates
  [#1225](https://github.com/fluxcd/notification-controller/pull/1225)
  [#1235](https://github.com/fluxcd/notification-controller/pull/1235)
  [#1236](https://github.com/fluxcd/notification-controller/pull/1236)
  [#1239](https://github.com/fluxcd/notification-controller/pull/1239)
  [#1240](https://github.com/fluxcd/notification-controller/pull/1240)

## 1.7.5

**Release date:** 2025-11-19

This patch release fixes Azure Workload Identity in Azure China Cloud
and introduces a feature gate to disable the ConfigMap and Secret watchers,
`DisableConfigWatchers`.

Improvements:
- Add feature gate for disabling config watchers
  [#1212](https://github.com/fluxcd/notification-controller/pull/1212)
- Upgrade k8s to 1.34.2 and c-r to 0.22.4
  [#1209](https://github.com/fluxcd/notification-controller/pull/1209)

## 1.7.4

**Release date:** 2025-10-28

This patch release fixes support for SOCKS5 proxy in the controller APIs
and support for `message_thread_id` in the `telegram` provider.

Fixes:
- Fix support for telegram message_thread_id
  [#1199](https://github.com/fluxcd/notification-controller/pull/1199)
- Restore SOCKS5 proxy support
  [#1196](https://github.com/fluxcd/notification-controller/pull/1196)

## 1.7.3

**Release date:** 2025-10-08

This patch release comes with various dependency updates.

The controller is now built with Go 1.25.2 which includes
fixes for vulnerabilities in the Go stdlib:
[CVE-2025-58183](https://github.com/golang/go/issues/75677),
[CVE-2025-58188](https://github.com/golang/go/issues/75675)
and many others. The full list of security fixes can be found
[here](https://groups.google.com/g/golang-announce/c/4Emdl2iQ_bI/m/qZN5nc-mBgAJ).

Improvements:
- Update dependencies to Kubernetes v1.34.1 and Go 1.25.2
  [#1191](https://github.com/fluxcd/notification-controller/pull/1191)

## 1.7.2

**Release date:** 2025-10-06

This patch release fixes the default Flux API versions in the Receiver handler.

Fixes:
- Update default API versions to GA
  [#1186](https://github.com/fluxcd/notification-controller/pull/1186)

## 1.7.1

**Release date:** 2025-09-24

This patch release fixes the release workflow.

Fixes:
- Fix release workflow
  [#1179](https://github.com/fluxcd/notification-controller/pull/1179)

## 1.7.0

**Release date:** 2025-09-24

This minor release comes with various bug fixes and improvements.

⚠️ The `v1beta1` APIs were removed. Before upgrading the CRDs, Flux users
must run [`flux migrate`](https://github.com/fluxcd/flux2/pull/5473) to
migrate the cluster storage off `v1beta1`.

### Provider

The field `.spec.proxySecretRef` has been added to the Provider API.
The field `.spec.proxy` and the field `proxy` inside the Secret
referenced by `.spec.secretRef` are now deprecated and will be removed
in the Provider API v1 GA.

The `JWT based auth` authentication method for the `azureeventhub`
provider has been deprecated and will be removed in the Provider
API v1 GA.

The `otel` provider has been introduced to send alerts as traces to an
[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/).

The `azuredevops` and `googlepubsub` providers now support workload
identity both at the controller and object levels. For object level,
the `.spec.serviceAccountName` field can be set to the name of a
service account in the same namespace that was configured with
a cloud identity. For this feature to work, the controller feature gate
`ObjectLevelWorkloadIdentity` must be enabled. See a complete guide
[here](https://fluxcd.io/flux/integrations/).

Support for mutual TLS (mTLS) has been added for GitHub App transport,
git-based notifiers, postMessage-based notifiers, DataDog and Sentry,
and TLS ServerName pinning has been removed for improved flexibility.

### Receiver

Users can now define a label selector for watching Secrets referenced
in Receivers through the controller flag `--watch-configs-label-selector`.
When an event on a Secret matching the label selector occurs, all
Receivers referencing the Secret will be reconciled. The default is
`--watch-configs-label-selector=reconcile.fluxcd.io/watch=Enabled`.

### General updates

In addition, the Kubernetes dependencies have been updated to v1.34 and
various other controller dependencies have been updated to their latest
version. The controller is now built with Go 1.25.

Fixes:
- Fix GitHub dispatch example documentation
  [#1168](https://github.com/fluxcd/notification-controller/pull/1168)

Improvements:
- Add ProxySecretRef field to Provider API
  [#1133](https://github.com/fluxcd/notification-controller/pull/1133)
- [RFC-0010] Add object-level workload identity support for Azure DevOps provider
  [#1145](https://github.com/fluxcd/notification-controller/pull/1145)
- [RFC-0010] Add object-level workload identity support for Google Pub/Sub provider
  [#1154](https://github.com/fluxcd/notification-controller/pull/1154)
- [RFC-0010] Add default-service-account flag for lockdown
  [#1161](https://github.com/fluxcd/notification-controller/pull/1161)
- [RFC-0011] Add OpenTelemetry (OTEL) provider type
  [#1149](https://github.com/fluxcd/notification-controller/pull/1149)
- Add Zulip alert provider
  [#1169](https://github.com/fluxcd/notification-controller/pull/1169)
- Add mTLS support for postMessage-based notifiers
  [#1137](https://github.com/fluxcd/notification-controller/pull/1137)
- Add mTLS support for git-based notifiers
  [#1146](https://github.com/fluxcd/notification-controller/pull/1146)
- Add mTLS support for DataDog and Sentry notifiers
  [#1148](https://github.com/fluxcd/notification-controller/pull/1148)
- Add support for mTLS to GitHub App transport
  [#1160](https://github.com/fluxcd/notification-controller/pull/1160)
- Add proxy support to Telegram notifier
  [#1140](https://github.com/fluxcd/notification-controller/pull/1140)
- Add proper basic auth support for Alertmanager Provider
  [#1152](https://github.com/fluxcd/notification-controller/pull/1152)
- Add label selector for watching Secrets referenced in Receivers
  [#1151](https://github.com/fluxcd/notification-controller/pull/1151)
- Make address field optional for providers that generate URLs internally
  [#1141](https://github.com/fluxcd/notification-controller/pull/1141)
- Remove TLS ServerName pinning in TLS config creation
  [#1158](https://github.com/fluxcd/notification-controller/pull/1158)
- Remove deprecated APIs in group `notification.toolkit.fluxcd.io/v1beta1`
  [#1157](https://github.com/fluxcd/notification-controller/pull/1157)
- Migrate Azure Event Hubs to new ProducerClient (azeventhubs) SDK
  [#1145](https://github.com/fluxcd/notification-controller/pull/1145)
- Unify BasicAuth processing using pkg/runtime/secrets
  [#1139](https://github.com/fluxcd/notification-controller/pull/1139)
  [#1142](https://github.com/fluxcd/notification-controller/pull/1142)
  [#1147](https://github.com/fluxcd/notification-controller/pull/1147)
- Refactor CI with `fluxcd/gha-workflows`
  [#1174](https://github.com/fluxcd/notification-controller/pull/1174)
- Various dependency updates
  [#1173](https://github.com/fluxcd/notification-controller/pull/1173)
  [#1166](https://github.com/fluxcd/notification-controller/pull/1166)
  [#1177](https://github.com/fluxcd/notification-controller/pull/1177)

## 1.6.0

**Release date:** 2025-05-27

This minor release comes with various bug fixes and improvements.

### Provider

The `azureeventhub` provider now supports workload identity both
at the controller and object levels. For object level, the
`.spec.serviceAccountName` field can be set to the name of a
service account in the same namespace that was configured with
a Managed Identity.
For object level to work, the controller feature gate
`ObjectLevelWorkloadIdentity` must be enabled. See a complete guide
[here](https://fluxcd.io/flux/integrations/azure/).

The `github` and `githubdispatch` providers now support authenticating
with a GitHub App. See docs
[here](https://fluxcd.io/flux/components/notification/providers/#github)
and
[here](https://fluxcd.io/flux/components/notification/providers/#github-dispatch).

For commit status providers it is now possible to define a custom
status string by defining a CEL expression in the `.spec.commitStatusExpr`
field. The variables `event`, `alert` and `provider` are available
for the CEL expression. See
[docs](https://fluxcd.io/flux/components/notification/providers/#custom-commit-status-messages).

### General updates

In addition, the Kubernetes dependencies have been updated to v1.33 and
various other controller dependencies have been updated to their latest
version. The controller is now built with Go 1.24.

Fixes:
- Fix Slack chat.postMessage error handling
  [#1086](https://github.com/fluxcd/notification-controller/pull/1086)
- Fix pass 'certPool' to Gitea client on creation
  [#1084](https://github.com/fluxcd/notification-controller/pull/1084)
- CrossNamespaceObjectReference: Fix MaxLength validation to kubernetes max size of 253
  [#1108](https://github.com/fluxcd/notification-controller/pull/1108)
- Sanitize proxy error logging
  [#1093](https://github.com/fluxcd/notification-controller/pull/1093)

Improvements:
- [RFC-0010] Workload Identity support for `azureeventhub` provider
  [#1106](https://github.com/fluxcd/notification-controller/pull/1106)
  [#1116](https://github.com/fluxcd/notification-controller/pull/1116)
  [#1120](https://github.com/fluxcd/notification-controller/pull/1120)
  [#1109](https://github.com/fluxcd/notification-controller/pull/1109)
  [#1112](https://github.com/fluxcd/notification-controller/pull/1112)
- GitHub App authentication support for `github` and `githubdispatch`
  [#1058](https://github.com/fluxcd/notification-controller/pull/1058)
- Support CEL expressions to construct commit statuses
  [#1068](https://github.com/fluxcd/notification-controller/pull/1068)
- Add proxy support to `gitea` provider
  [#1087](https://github.com/fluxcd/notification-controller/pull/1087)
- Various dependency updates
  [#1101](https://github.com/fluxcd/notification-controller/pull/1101)
  [#1119](https://github.com/fluxcd/notification-controller/pull/1119)
  [#1118](https://github.com/fluxcd/notification-controller/pull/1118)
  [#1113](https://github.com/fluxcd/notification-controller/pull/1113)
  [#1104](https://github.com/fluxcd/notification-controller/pull/1104)

## 1.5.0

**Release date:** 2025-02-13

This minor release comes with various bug fixes and improvements.

### Alert

Now notification-controller also sends event metadata specified in Flux objects through
annotations. See [docs](https://fluxcd.io/flux/components/notification/alerts/#event-metadata-from-object-annotations).

Now notification-controller is also capable of updating Git commit statuses
from events about Kustomizations that consume OCIRepositories. See
[docs](https://fluxcd.io/flux/cheatsheets/oci-artifacts/#git-commit-status-updates).

### Receiver

The Receiver API now supports filtering the declared resources that
match a given Common Expression Language (CEL) expression. See
[docs](https://fluxcd.io/flux/components/notification/receivers/#filtering-reconciled-objects-with-cel).

In addition, the Kubernetes dependencies have been updated to v1.32.1 and
various other controller dependencies have been updated to their latest
version.

Fixes:
- Remove deprecated object metrics from controllers
  [#997](https://github.com/fluxcd/notification-controller/pull/997)
- msteams notifier: adaptive cards full width
  [#1017](https://github.com/fluxcd/notification-controller/pull/1017)
- fix: adding of duplicate commit statuses in gitlab
  [#1010](https://github.com/fluxcd/notification-controller/pull/1010)
- Fix add missing return statement and a few style issues
  [#1039](https://github.com/fluxcd/notification-controller/pull/1039)

Improvements:
- [RFC-0008] Custom Event Metadata from Annotations
  [#1014](https://github.com/fluxcd/notification-controller/pull/1014)
- Add support for MetaOriginRevisionKey from the Event API
  [#1018](https://github.com/fluxcd/notification-controller/pull/1018)
- Add subsection for Git providers supporting commit status updates
  [#1019](https://github.com/fluxcd/notification-controller/pull/1019)
- Add support for Bearer Token authentication to Provider alertmanager
  [#1021](https://github.com/fluxcd/notification-controller/pull/1021)
- Enforce namespace check on receiver
  [#1022](https://github.com/fluxcd/notification-controller/pull/1022)
- Implement Receiver resource filtering with CEL
  [#948](https://github.com/fluxcd/notification-controller/pull/948)
- Clarify gitlab provider usage
  [#953](https://github.com/fluxcd/notification-controller/pull/953)
- Add involved object reference as annotations for the grafana provider
  [#1040](https://github.com/fluxcd/notification-controller/pull/1040)
- Improvements after CEL resource filtering
  [#1041](https://github.com/fluxcd/notification-controller/pull/1041)
- Various dependency updates
  [#1002](https://github.com/fluxcd/notification-controller/pull/1002)
  [#1016](https://github.com/fluxcd/notification-controller/pull/1016)
  [#1023](https://github.com/fluxcd/notification-controller/pull/1023)
  [#1025](https://github.com/fluxcd/notification-controller/pull/1025)
  [#1027](https://github.com/fluxcd/notification-controller/pull/1027)
  [#1032](https://github.com/fluxcd/notification-controller/pull/1032)
  [#1036](https://github.com/fluxcd/notification-controller/pull/1036)
  [#1037](https://github.com/fluxcd/notification-controller/pull/1037)
  [#1042](https://github.com/fluxcd/notification-controller/pull/1042)

## 1.4.0

**Release date:** 2024-09-27

This minor release comes with various bug fixes and improvements.

MS Teams Provider has been updated to support MS Adaptive Card payloads.
This allows users to migrate from the deprecated
[Office 365 Connector for Incoming Webhooks](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/)
to the new [Microsoft Teams Incoming Webhooks with Workflows](https://support.microsoft.com/en-us/office/create-incoming-webhooks-with-workflows-for-microsoft-teams-8ae491c7-0394-4861-ba59-055e33f75498).
See the [Provider API documentation](https://fluxcd.io/flux/components/notification/providers/#microsoft-teams)
for more information. After getting the URL for the new Incoming Webhook Workflow,
update the secret used by the `msteams` Provider object with the new URL.

In addition, the Kubernetes dependencies have been updated to v1.31.1 and
various other controller dependencies have been updated to their latest
version. The controller is now built with Go 1.23.

Fixes:
- telegram notifier should escape with metadata key
  [#829](https://github.com/fluxcd/notification-controller/pull/829)
- docs: use stringData for secret of GitHub PAT
  [#873](https://github.com/fluxcd/notification-controller/pull/873)
- Fix incorrect use of format strings with the conditions package.
  [#879](https://github.com/fluxcd/notification-controller/pull/879)

Improvements:
- New flag to disable detailed metrics for path
  [#841](https://github.com/fluxcd/notification-controller/pull/841)
- Fix telegram test flake
  [#894](https://github.com/fluxcd/notification-controller/pull/894)
- Build with Go 1.23
  [#907](https://github.com/fluxcd/notification-controller/pull/907)
- Add MS Adaptive Card payload to msteams Provider
  [#920](https://github.com/fluxcd/notification-controller/pull/920)
- Various dependency updates
  [#845](https://github.com/fluxcd/notification-controller/pull/845)
  [#855](https://github.com/fluxcd/notification-controller/pull/855)
  [#854](https://github.com/fluxcd/notification-controller/pull/854)
  [#857](https://github.com/fluxcd/notification-controller/pull/857)
  [#865](https://github.com/fluxcd/notification-controller/pull/865)
  [#866](https://github.com/fluxcd/notification-controller/pull/866)
  [#905](https://github.com/fluxcd/notification-controller/pull/905)
  [#903](https://github.com/fluxcd/notification-controller/pull/903)
  [#912](https://github.com/fluxcd/notification-controller/pull/912)
  [#925](https://github.com/fluxcd/notification-controller/pull/925)
  [#931](https://github.com/fluxcd/notification-controller/pull/931)
  [#932](https://github.com/fluxcd/notification-controller/pull/932)
  [#933](https://github.com/fluxcd/notification-controller/pull/933)
  [#934](https://github.com/fluxcd/notification-controller/pull/934)

## 1.3.0

**Release date:** 2024-05-06

This minor release comes with new features, improvements and bug fixes.

The `Receiver` API has been extended to support CDEvents,
for more information, please see the
[CDEvents Receiver API documentation](https://github.com/fluxcd/notification-controller/blob/release/v1.3.x/docs/spec/v1/receivers.md#cdevents).

Starting with this version, the controller allows grouping alerts for Alertmanager
by setting the `startsAt` label instead of `timestamp`. When sending alerts to
OpsGenie, the controller now sets the `severity` field to the alert's details.

In addition, the controller dependencies have been updated to Kubernetes v1.30
and controller-runtime v0.18. Various other dependencies have also been updated to
their latest version to patch upstream CVEs.

Lastly, the controller is now built with Go 1.22.

Improvements:
- Add CDEvent Receiver Support
  [#772](https://github.com/fluxcd/notification-controller/pull/772)
- Add severity to opsgenie alerts
  [#796](https://github.com/fluxcd/notification-controller/pull/796)
- Alertmanager: Change timestamp label to .StartsAt
  [#795](https://github.com/fluxcd/notification-controller/pull/795)
- Use `password` as fallback for the Git provider `token` auth
  [#790](https://github.com/fluxcd/notification-controller/pull/790)
- Add support for Bitbucket Context path
  [#747](https://github.com/fluxcd/notification-controller/pull/747)
- Various dependency updates
  [#816](https://github.com/fluxcd/notification-controller/pull/816)
  [#814](https://github.com/fluxcd/notification-controller/pull/814)
  [#813](https://github.com/fluxcd/notification-controller/pull/813)
  [#810](https://github.com/fluxcd/notification-controller/pull/810)
  [#809](https://github.com/fluxcd/notification-controller/pull/809)
  [#787](https://github.com/fluxcd/notification-controller/pull/787)
  [#783](https://github.com/fluxcd/notification-controller/pull/783)
  [#763](https://github.com/fluxcd/notification-controller/pull/763)

Fixes:
- Sanitize provider data loaded from secret
  [#789](https://github.com/fluxcd/notification-controller/pull/789)
- Fix timeout propagation for alerts
  [#757](https://github.com/fluxcd/notification-controller/pull/757)
- Fix Telegram MarkdownV2 escaping
  [#776](https://github.com/fluxcd/notification-controller/pull/776)
- Remove `genclient:Namespaced` tag
  [#749](https://github.com/fluxcd/notification-controller/pull/749)

## 1.2.4

**Release date:** 2024-02-01

This patch release fixes various issues, updates the Kubernetes dependencies
to v1.28.6 and various other dependencies to their latest version to patch
upstream CVEs.

Improvements:
- Various dependency updates
  [#727](https://github.com/fluxcd/notification-controller/pull/727)
  [#726](https://github.com/fluxcd/notification-controller/pull/726)
  [#721](https://github.com/fluxcd/notification-controller/pull/721)
  [#718](https://github.com/fluxcd/notification-controller/pull/718)
  [#707](https://github.com/fluxcd/notification-controller/pull/707)
  [#695](https://github.com/fluxcd/notification-controller/pull/695)

Fixes:
- Fix BitBucket status update panic
  [#722](https://github.com/fluxcd/notification-controller/pull/722)
- fix typo in docs/spec/v1beta3/providers.md
  [#699](https://github.com/fluxcd/notification-controller/pull/699)
- fix(grafana-provider): replace ":" character in eventMetadata
  [#703](https://github.com/fluxcd/notification-controller/pull/703)
- Remove old/incorrect API version usage
  [#693](https://github.com/fluxcd/notification-controller/pull/693)

## 1.2.3

**Release date:** 2023-12-14

This patch release fixes various issues, most notably, the Provider v1beta3 API
backwards compatibility issue when `.spec.interval` was explicitly set in a
v1beta2 version of Provider.

Fixes:
- Exclude eventv1.MetaTokenKey from event metadata
  [#686](https://github.com/fluxcd/notification-controller/pull/686)
- Add .spec.interval in v1beta3 Provider
  [#683](https://github.com/fluxcd/notification-controller/pull/683)
- Remove URL syntax validation for provider address entirely
  [#682](https://github.com/fluxcd/notification-controller/pull/682)

## 1.2.2

**Release date:** 2023-12-11

This patch releases updates a variety of dependencies, including an update of
the container base image to Alpine v3.19.

Improvements:
- build: update Alpine to 3.19
  [#675](https://github.com/fluxcd/notification-controller/pull/675)
- Update dependencies
  [#677](https://github.com/fluxcd/notification-controller/pull/677)

## 1.2.1

**Release date:** 2023-12-08

This patch release updates the Go version the controller is built with to
`1.21.x`, while mitigating recently published security vulnerabilities in the
`net/http` package.

In addition, it ensures static analyzers no longer detect a vulnerability in the
`whilp/git-urls` module by using `chainguard-dev/git-urls`. For which the
(potential) issue itself got already addressed internally in the [previous
v1.2.0 release](#120).

Lastly, a small number of dependencies got updated to their latest versions.

Improvements:
- Update Go to 1.21.x
  [#666](https://github.com/fluxcd/notification-controller/pull/666)
- Replace whilp/git-urls module by chainguard-dev/git-urls
  [#667](https://github.com/fluxcd/notification-controller/pull/667)
- Update dependencies
  [#669](https://github.com/fluxcd/notification-controller/pull/669)

## 1.2.0

**Release date:** 2023-12-05

This minor release graduates the notification `Alert` and `Provider` APIs to
`v1beta3`. In addition, this version comes with alert Provider support for
[BitBucket
Server](https://github.com/fluxcd/notification-controller/blob/api/v1.2.0/docs/spec/v1beta3/providers.md#bitbucket-serverdata-center)
and
[NATS](https://github.com/fluxcd/notification-controller/blob/api/v1.2.0/docs/spec/v1beta3/providers.md#nats).

### `notification.toolkit.fluxcd.io/v1beta3`

After upgrading the controller to v1.2.0, please update the notification Custom
Resources for `Alert` and `Provider` in Git by replacing
`notification.toolkit.fluxcd.io/v1beta2` with
`notification.toolkit.fluxcd.io/v1beta3` in all the YAML manifests.

#### Static Alerts and Providers

The notification Alert and Provider API resources will become static objects
with configurations that will be used by the event handlers for processing the
respective incoming events. They will no longer be reconciled by a reconciler
and will not advertise any status. Once `Alerts` and `Providers` are created,
they can be considered ready. Users of
[kstatus](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md)
shouldn't see any difference. Existing `Alerts` and `Providers` objects in
`v1beta2` API will undergo a one-time automatic migration to be converted into
static objects without any status.

#### Enhanced Alert events

The event handler will emit Kubernetes native events on the respective Alert
object for any relevant information, including failures due to any
misconfiguration.

Improvements:
- Add Provider for NATS Subject
  [#651](https://github.com/fluxcd/notification-controller/pull/651)
- Cap provider address at 2048 bytes
  [#654](https://github.com/fluxcd/notification-controller/pull/654)
- Refactor events and introduce v1beta3 API for Alert and Provider
  [#540](https://github.com/fluxcd/notification-controller/pull/540)
- Add Bitbucket server/Bitbucket Data Center provider for git commit status
  [#639](https://github.com/fluxcd/notification-controller/pull/639)
- Address miscellaneous issues throughout code base
  [#627](https://github.com/fluxcd/notification-controller/pull/627)
- Update dependencies
  [#609](https://github.com/fluxcd/notification-controller/pull/609)
  [#612](https://github.com/fluxcd/notification-controller/pull/612)
  [#613](https://github.com/fluxcd/notification-controller/pull/613)
  [#617](https://github.com/fluxcd/notification-controller/pull/617)
  [#621](https://github.com/fluxcd/notification-controller/pull/621)
  [#623](https://github.com/fluxcd/notification-controller/pull/623)
  [#628](https://github.com/fluxcd/notification-controller/pull/628)
  [#629](https://github.com/fluxcd/notification-controller/pull/629)
  [#632](https://github.com/fluxcd/notification-controller/pull/632)
  [#635](https://github.com/fluxcd/notification-controller/pull/635)
  [#637](https://github.com/fluxcd/notification-controller/pull/637)
  [#641](https://github.com/fluxcd/notification-controller/pull/641)
  [#643](https://github.com/fluxcd/notification-controller/pull/643)
  [#646](https://github.com/fluxcd/notification-controller/pull/646)
  [#648](https://github.com/fluxcd/notification-controller/pull/648)
  [#652](https://github.com/fluxcd/notification-controller/pull/652)
  [#656](https://github.com/fluxcd/notification-controller/pull/656)
  [#657](https://github.com/fluxcd/notification-controller/pull/657)

Fixes:
- Fix README.md links to notification APIs
  [#619](https://github.com/fluxcd/notification-controller/pull/619)

## 1.1.0

**Release date:** 2023-08-23

This minor release comes with support for sending alerts
to [PagerDuty](https://github.com/fluxcd/notification-controller/blob/v1.1.0/docs/spec/v1beta2/providers.md#datadog).

In addition, this version deprecates the usage of the `caFile` key in favor of `ca.crt`
for the `.spec.certSecretRef` secret in the Provider v1beta2 API.

Starting with this version, the controller now stops exporting an object's
metrics as soon as the object has been deleted.

Improvements:

- Add support for Datadog
  [#592](https://github.com/fluxcd/notification-controller/pull/592)
- Adopt Kubernetes style TLS Secret
  [#597](https://github.com/fluxcd/notification-controller/pull/597)
- Remove checks for empty user and channel parameters in Rocket notifier
  [#603](https://github.com/fluxcd/notification-controller/pull/603)
- Clarify permission requirements for Gitea provider token
  [#583](https://github.com/fluxcd/notification-controller/pull/583)
- Align docs structure with other controllers
  [#582](https://github.com/fluxcd/notification-controller/pull/582)
- Update dependencies
  [#600](https://github.com/fluxcd/notification-controller/pull/600)
  [#606](https://github.com/fluxcd/notification-controller/pull/606)

Fixes:

- Use TrimPrefix instead of TrimLeft
  [#590](https://github.com/fluxcd/notification-controller/pull/590)
- Handle delete before adding finalizer
  [#584](https://github.com/fluxcd/notification-controller/pull/584)
- Delete stale metrics on object delete
  [#599](https://github.com/fluxcd/notification-controller/pull/599)
- docs: change key type to `[]byte` in provider spec
  [#585](https://github.com/fluxcd/notification-controller/pull/585)

## 1.0.0

**Release date:** 2023-07-04

This is the first stable release of the controller. From now on, this controller
follows the [Flux 2 release cadence and support pledge](https://fluxcd.io/flux/releases/).

Starting with this version, the build, release and provenance portions of the
Flux project supply chain [provisionally meet SLSA Build Level 3](https://fluxcd.io/flux/security/slsa-assessment/).

This release comes with support for sending alerts
to [PagerDuty](https://github.com/fluxcd/notification-controller/blob/v1.0.0/docs/spec/v1beta2/providers.md#pagerduty)
and [Google Pub/Sub](https://github.com/fluxcd/notification-controller/blob/v1.0.0/docs/spec/v1beta2/providers.md#google-pubsub).

In addition, dependencies have been updated
to their latest version, including an update of Kubernetes to v1.27.3.

For a comprehensive list of changes since `v0.33.x`, please refer to the
changelog for [v1.0.0-rc.1](#100-rc1), [v1.0.0-rc.2](#100-rc2),
[v1.0.0-rc.3](#100-rc3) and [`v1.0.0-rc.4](#100-rc4).

Improvements:

- Add support for PagerDuty
  [#527](https://github.com/fluxcd/notification-controller/pull/527)
- Add support for Google Pub/Sub
  [#543](https://github.com/fluxcd/notification-controller/pull/543)
- Lift HTTP/S validation from Provider spec.address
  [#565](https://github.com/fluxcd/notification-controller/pull/565)
- Improve error messages in Gitea notifier
  [#556](https://github.com/fluxcd/notification-controller/pull/556)
- Make Gitea tests independent of 3rd-party service
  [#558](https://github.com/fluxcd/notification-controller/pull/558)
- Align go.mod version with Kubernetes (Go 1.20)
  [#558](https://github.com/fluxcd/notification-controller/pull/558)
- Update dependencies
  [#563](https://github.com/fluxcd/notification-controller/pull/563)
- Update GCP dependencies
  [#569](https://github.com/fluxcd/notification-controller/pull/569)

Fixes:

- Fix Alert `.spec.eventMetadata` documentation
  [#541](https://github.com/fluxcd/notification-controller/pull/541)
- Fix `TestProviderReconciler_Reconcile/finalizes_suspended_object` to use patch instead of update
  [#550](https://github.com/fluxcd/notification-controller/pull/550)

## 1.0.0-rc.4

**Release date:** 2023-05-26

This release candidate comes with support for Kubernetes v1.27.

The `Event` API has been modified to have a dedicated key for `metadata` called
`token`. The value of the `token` key is meant to be defined on a per event
emitter basis for uniquely identifying the contents of the event payload.
This key if present, is included in calculating the unique key used for rate
limiting events.
Furthermore, the event attributes are prefixed with an identifier to avoid
collisions between different event attributes.

In addition, a bug in the event rate limiting key calculation logic which led
to the inconsideration of the revision specified in `.metadata` of the event has
been fixed.

Lastly, the behavior of `.spec.eventMetadata` has been modified such that if a
key present in the map already exists in the original event's `metadata`, then
the key in the latter takes precedence and an error log is printed for visibility.

Improvements:

- Include eventv1.MetaTokenKey on event rate limiting key calculation
  [#530](https://github.com/fluxcd/notification-controller/pull/530)
- Update dependencies and Kubernetes to 1.27.2
  [#532](https://github.com/fluxcd/notification-controller/pull/532)
- Remove the tini supervisor
  [#533](https://github.com/fluxcd/notification-controller/pull/533)
- Prefix event key attributes with identifier
  [#534](https://github.com/fluxcd/notification-controller/pull/534)
- Update workflows and enable dependabot
  [#535](https://github.com/fluxcd/notification-controller/pull/535)
- build(deps): bump github/codeql-action from 2.3.3 to 2.3.4
  [#536](https://github.com/fluxcd/notification-controller/pull/536)

Fixes:

- Fix revision discarded on event rate limiting key calculation
  [#517](https://github.com/fluxcd/notification-controller/pull/517)
- Fix Alert .spec.eventMetadata behavior
  [#529](https://github.com/fluxcd/notification-controller/pull/529)

## 1.0.0-rc.3

**Release date:** 2023-05-12

This release candidate comes with support for
adding [custom metadata](https://github.com/fluxcd/notification-controller/blob/v1.0.0-rc.3/docs/spec/v1beta2/alerts.md#event-metadata)
to Flux events. A new field was added to the Alert v1beta2 API named
`.spec.eventMetadata` that allows users to enrich the alerts with
information about the cluster name, region, environment, etc.

In addition, the controller dependencies have been updated to patch
CVE-2023-1732 and the base image has been updated to Alpine 3.18.

Improvements:
- Add event metadata field to Alert spec
  [#519](https://github.com/fluxcd/notification-controller/pull/506)
- Update Alpine to 3.18
  [#524](https://github.com/fluxcd/notification-controller/pull/524)
- build(deps): bump github.com/cloudflare/circl from 1.3.2 to 1.3.3
  [#525](https://github.com/fluxcd/notification-controller/pull/525)

## 1.0.0-rc.2

**Release date:** 2023-05-09

This release candidate comes with performance improvements for Receivers
and removes the deprecated `.status.url` field from the Receiver v1 API.

A new field was added to the Alert v1beta2 API named `.spec.inclusionList` for
better control over events filtering.

In addition, the controller dependencies have been updated to their latest
versions.

Improvements:
- Index receivers using webhook path as key
  [#506](https://github.com/fluxcd/notification-controller/pull/506)
- Append the Alert summary to Azure DevOps genre field
  [#514](https://github.com/fluxcd/notification-controller/pull/514)
- Add InclusionList to Alert CRD
  [#515](https://github.com/fluxcd/notification-controller/pull/515)
- Update dependencies
  [#520](https://github.com/fluxcd/notification-controller/pull/520)
- Improve event handler tests
  [#521](https://github.com/fluxcd/notification-controller/pull/521)
- receiver/v1: Remove deprecated `.status.url` field
  [#482](https://github.com/fluxcd/notification-controller/pull/482)

## 1.0.0-rc.1

**Release date:** 2023-03-30

This release candidate promotes the Receiver API from v1beta2 to v1. The Receiver v1 API now supports triggering the reconciliation of multiple 
resources using match labels.

### Highlights

#### API changes

The `Receiver` kind was promoted from v1beta2 to v1 (GA). All other kinds of the notification.toolkit.fluxcd.io group stay at version v1beta2.

The receivers.notification.toolkit.fluxcd.io CRD contains the following versions:

- v1 (storage version)
- v1beta2 (deprecated)
- v1beta1 (deprecated)

#### Upgrade Procedure

The `Receiver` v1 API is backwards compatible with v1beta2.

To upgrade from v1beta2, after deploying the new CRD and controller, set `apiVersion: notification.toolkit.fluxcd.io/v1` in the YAML files that 
contain `Receiver` definitions. Bumping the API version in manifests can be done gradually. It is advised to not delay this procedure as the beta 
versions will be removed after 6 months.

### Full Changelog

Improvements:
- GA: Promote Receiver API to notification.toolkit.fluxcd.io/v1
  [#498](https://github.com/fluxcd/notification-controller/pull/498)
- support multiple resources in Receivers by using match labels
  [#482](https://github.com/fluxcd/notification-controller/pull/482)
- docs: fixes to the Receiver documentation
  [#495](https://github.com/fluxcd/notification-controller/pull/495)

## 0.33.0

**Release date:** 2023-03-08

This release updates to Go version the controller is build with to `1.20`,
and updates the dependencies to their latest versions.

In addition, `klog` is now configured to log using the same logger as the rest
of the controller (providing a consistent log format).

Improvements:
* Update Go to 1.20
  [#483](https://github.com/fluxcd/notification-controller/pull/483)
* Update dependencies
  [#485](https://github.com/fluxcd/notification-controller/pull/485)
* Use `logger.SetLogger` to also configure `klog`
  [#486](https://github.com/fluxcd/notification-controller/pull/486)

## 0.32.1

**Release date:** 2023-02-28

This prerelease comes with a fix to the version of the ImageRepository API
when it is not specified in the Receiver spec, now defaulting to
`image.toolkit.fluxcd.io/v1beta2`.

In addition, the controller dependencies have been updated to their latest
versions.

Fixes:
* receiver: update default ImageRepository version
  [#479](https://github.com/fluxcd/notification-controller/pull/479)

Improvements:
* Update dependencies
  [#478](https://github.com/fluxcd/notification-controller/pull/478)
  [#480](https://github.com/fluxcd/notification-controller/pull/480)

## 0.32.0

**Release date:** 2023-02-16

This prerelease adds support for parsing
[RFC-0005](https://github.com/fluxcd/flux2/tree/main/rfcs/0005-artifact-revision-and-digest)
revision format. Similar to artifact `Checksum`, the new `Digest` metadata is
also removed from Alerts.

In addition, the controller dependencies have been updated to their latest
versions.

Improvements:
* Support RFC-0005 revision format
  [#472](https://github.com/fluxcd/notification-controller/pull/472)
* Update dependencies
  [#474](https://github.com/fluxcd/notification-controller/pull/474)

## 0.31.0

**Release date:** 2023-02-01

This prerelease disables caching of Secrets and ConfigMaps to improve memory
usage. To opt-out from this behavior, start the controller with:
`--feature-gates=CacheSecretsAndConfigMaps=true`.

In addition, the controller dependencies have been updated to Kubernetes
v1.26.1 and controller-runtime v0.14.2. The controller base image has been
updated to Alpine 3.17.

Improvements:
* docs: fix up typos in providers document and changelog
  [#459](https://github.com/fluxcd/notification-controller/pull/459)
* Remove erroneous mention of wildcard in Receivers
  [#462](https://github.com/fluxcd/notification-controller/pull/462)
* docs: fix secret name in example
  [#463](https://github.com/fluxcd/notification-controller/pull/463)
* Set rate limiter option in test reconcilers
  [#465](https://github.com/fluxcd/notification-controller/pull/465)
* Update dependencies
  [#466](https://github.com/fluxcd/notification-controller/pull/466)
* build: Enable SBOM and SLSA Provenance
  [#467](https://github.com/fluxcd/notification-controller/pull/467)
* Disable caching of Secrets and ConfigMaps
  [#468](https://github.com/fluxcd/notification-controller/pull/468)

## 0.30.2

**Release date:** 2022-12-22

This prerelease comes with a fix for the Provider and Receiver
custom resources upgrade to `v1beta2`.

Fixes:
* Remove interval default value from CRDs
  [#457](https://github.com/fluxcd/notification-controller/pull/457)

## 0.30.1

**Release date:** 2022-12-21

This prerelease comes with a fix to prevent the controller from panicking
when the Kubernetes conversion webhook upgrades the Provider and Receiver
custom resources from `v1beta1`to `v1beta2` without setting the
default value for `spec.interval`.

Fixes:
* Fix panic when upgrading to v1beta2
  [#455](https://github.com/fluxcd/notification-controller/pull/455)

## 0.30.0

**Release date:** 2022-12-20

This prerelease graduates the notification APIs to `v1beta2`.
In addition, this version comes with support for
[Gitea commit status updates](https://github.com/fluxcd/notification-controller/blob/api/v0.30.0/docs/spec/v1beta2/providers.md#gitea).

### `notification.toolkit.fluxcd.io/v1beta2`

After upgrading the controller to v0.30.0, you need to update the notification
**Custom Resources** in Git
by replacing `notification.toolkit.fluxcd.io/v1beta1` with
`notification.toolkit.fluxcd.io/v1beta2` in all YAML manifests.

#### Breaking changes

- The `Alert.spec.summary` has a max length of 255 characters.
- The `Provider.spec.address` and `Provider.spec.proxy` have a max length of 2048 characters.
- The `Receiver.status.url` was deprecated in favour of `Receiver.status.webhookPath`.

#### API specifications in a user-friendly format

[The new specifications for the `v1beta2` API](https://github.com/fluxcd/notification-controller/tree/v0.30.0/docs/spec/v1beta2)
have been written in a new format with the aim to be more valuable to a user.
Featuring separate sections with examples, and information on how to write
and work with them.

#### Enhanced Kubernetes Conditions

Notification API resources will now advertise more explicit Condition types,
provide `Reconciling` and `Stalled` Conditions where applicable for
[better integration with `kstatus`](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md#conditions),
and record the Observed Generation on the Condition.

#### Enhanced Git commit status updates

Starting with this version, the controller uses the `Provider` cluster assigned `UID`
to compose a unique Git commit status ID to avoid name collisions
when multiple clusters write to the same repository.

Improvements:
* Refactor reconcilers and introduce v1beta2 API
  [#435](https://github.com/fluxcd/notification-controller/pull/435)
- feat: add gitea notifier
  [#451](https://github.com/fluxcd/notification-controller/pull/451)

## 0.29.1

**Release date:** 2022-12-01

This prerelease comes with a minor improvement in the receiver to use its own
ServeMux instead of using the default global one.

Fixes:
* receiver: Use new ServeMux
  [#448](https://github.com/fluxcd/notification-controller/pull/448)

Improvements:
* build: Fix cifuzz and improve fuzz tests' reliability
  [#446](https://github.com/fluxcd/notification-controller/pull/446)

## 0.29.0

**Release date:** 2022-11-22

This prerelease comes with a change to the Event API, which is now declared
in the [`github.com/fluxcd/pkg/apis/event/v1beta1`](https://pkg.go.dev/github.com/fluxcd/pkg/apis/event/v1beta1)
package. For more information, refer to the updated [Event API
documentation](https://github.com/fluxcd/notification-controller/blob/main/docs/spec/v1beta1/event.md).

In addition, dependencies have been updated.

Fixes:
* Remove `nsswitch.conf` creation
  [#439](https://github.com/fluxcd/notification-controller/pull/439)

Improvements:
* Refactor notifiers to use Flux Event v1beta1 API
  [#442](https://github.com/fluxcd/notification-controller/pull/442)
* Update dependencies
  [#442](https://github.com/fluxcd/notification-controller/pull/442)
* docs: update spec to reflect v1beta1 Event API
  [#443](https://github.com/fluxcd/notification-controller/pull/443)

## 0.28.0

**Release date:** 2022-10-20

This prerelease comes with a new Alert Provider type named `generic-hmac`
for authenticating the webhook requests coming from notification-controller.

In addition, the controller dependencies have been updated to Kubernetes v1.25.3.
The `golang.org/x/text` package was updated to v0.4.0 (fix for CVE-2022-32149).

Features:
* Add `generic-hmac` Provider
  [#426](https://github.com/fluxcd/notification-controller/pull/426)

Improvements:
* Update dependencies
  [#430](https://github.com/fluxcd/notification-controller/pull/430)

## 0.27.0

**Release date:** 2022-09-27

This prerelease comes with strict validation rules for API fields which define a
(time) duration. Effectively, this means values without a time unit (e.g. `ms`,
`s`, `m`, `h`) will now be rejected by the API server. To stimulate sane
configurations, the units `ns`, `us` and `µs` can no longer be configured, nor
can `h` be set for fields defining a timeout value.

In addition, the controller dependencies have been updated
to Kubernetes controller-runtime v0.13.

:warning: **Breaking changes:**
- `Provider.spec.timeout` new validation pattern is `"^([0-9]+(\\.[0-9]+)?(ms|s|m))+$"`

Improvements:
* api: add custom validation for v1.Duration types
  [#420](https://github.com/fluxcd/notification-controller/pull/420)
* Update dependencies
  [#423](https://github.com/fluxcd/notification-controller/pull/423)
* Dockerfile: Build with Go 1.19
  [#424](https://github.com/fluxcd/notification-controller/pull/424)
* docs: Fix table with git commit status providers
  [#421](https://github.com/fluxcd/notification-controller/pull/421)

## 0.26.0

**Release date:** 2022-09-12

This prerelease comes with with finalizers to properly record the reconciliation metrics
for deleted resources. In addition, the controller dependencies have been updated
to Kubernetes controller-runtime v0.12.

:warning: **Breaking change:** The controller logs have been aligned
with the Kubernetes structured logging. For more details on the new logging
structure please see: [fluxcd/flux2#3051](https://github.com/fluxcd/flux2/issues/3051).

Improvements:
* Align controller logs to Kubernetes structured logging
  [#412](https://github.com/fluxcd/notification-controller/pull/412)
* Add finalizers to the custom resources
  [#416](https://github.com/fluxcd/notification-controller/pull/416)
* Add `.spec.timeout` to the Provider API
  [#410](https://github.com/fluxcd/notification-controller/pull/410)
* Refactor Fuzzers based on Go native fuzzing
  [#414](https://github.com/fluxcd/notification-controller/pull/414)
* Fuzz optimisations
  [#413](https://github.com/fluxcd/notification-controller/pull/413)

## 0.25.2

**Release date:** 2022-08-29

This prerelease comes with panic recovery, to protect the controller
from crashing when reconciliations lead to a crash.

In addition, the controller dependencies have been updated to Kubernetes v1.25.0.

Fixes:
* Fix context cancel defer for commit status updates
  [#408](https://github.com/fluxcd/notification-controller/pull/408)

Improvements:
* Enables RecoverPanic option on reconcilers
  [#403](https://github.com/fluxcd/notification-controller/pull/403)
* Update Kubernetes packages to v1.25.0
  [#407](https://github.com/fluxcd/notification-controller/pull/407)

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
