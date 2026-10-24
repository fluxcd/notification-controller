# Notification Controller

## Flags

| Name                                  | Type          | Description                                                                                                                        |
|---------------------------------------|---------------|------------------------------------------------------------------------------------------------------------------------------------|
| `--authorized-notifier-spiffe-id`     | strings       | SPIFFE ID of a notifier authorized to send events to the event server. Can be repeated. When set, the event server uses SPIFFE mTLS. |
| `--authorized-receiver-spiffe-id`     | strings       | SPIFFE ID of a receiver authorized to send requests to the receiver server. Can be repeated. When set, the receiver server uses SPIFFE mTLS. |
| `--concurrent`                        | int           | The number of concurrent notification reconciles. (default 4)                                                                      |
| `--default-service-account`           | string        | Default service account to use for workload identity when not specified in resources.                                             |
| `--enable-leader-election`            | boolean       | Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.              |
| `--events-addr`                       | string        | The address of the events receiver.                                                                                                |
| `--health-addr`                       | string        | The address the health endpoint binds to. (default ":9440")                                                                        |
| `--leader-election-lease-duration`    | duration      | Interval at which non-leader candidates will wait to force acquire leadership (duration string). (default 35s)                     |
| `--leader-election-release-on-cancel` | boolean       | Defines if the leader should step down voluntarily on controller manager shutdown. (default true)                                  |
| `--leader-election-renew-deadline`    | duration      | Duration that the leading controller manager will retry refreshing leadership before giving up (duration string). (default 30s)    |
| `--leader-election-retry-period`      | duration      | Duration the LeaderElector clients should wait between tries of actions (duration string). (default 5s)                            |
| `--log-encoding`                      | string        | Log encoding format. Can be 'json' or 'console'. (default "json")                                                                  |
| `--log-level`                         | string        | Log verbosity level. Can be one of 'trace', 'debug', 'info', 'error'. (default "info")                                             |
| `--metrics-addr`                      | string        | The address the metric endpoint binds to. (default ":8080")                                                                        |
| `--no-cross-namespace-refs`           | boolean       | When set to true, references between custom resources are allowed only if the reference and the referee are in the same namespace. |
| `--rate-limit-interval`               | duration      | Interval in which rate limit has effect. (default 5m0s)                                                                            |
| `--receiverAddr`                      | string        | The address the webhook receiver endpoint binds to. (default ":9292")                                                              |
| `--token-cache-max-size`              | int           | The maximum amount of entries in the LRU cache used for tokens. (default 100, enabled)                                             |
| `--spiffe-broker-endpoint`            | string        | The gRPC URL of the SPIFFE Broker endpoint.                                                                                                                                              |
| `--spiffe-broker-id`                  | strings       | SPIFFE ID of the SPIFFE Broker endpoint. Can be repeated.                                                                                                                                |
| `--token-cache-max-duration`          | duration      | The maximum duration for which a token would be considered unexpired. This is capped at 1h. (default 1h)                           |
| `--watch-all-namespaces`              | boolean       | Watch for custom resources in all namespaces, if set to false it will only watch the runtime namespace. (default true)             |
| `--feature-gates`                     | mapStringBool | A comma separated list of key=value pairs defining the state of experimental features.                                             |

### SPIFFE mTLS

When at least one of the repeatable flags `--authorized-notifier-spiffe-id` or
`--authorized-receiver-spiffe-id` is set, notification-controller acquires an
X.509-SVID from the SPIFFE Workload API. The `SPIFFE_ENDPOINT_SOCKET` environment
variable must point to the Workload API socket.

- `--authorized-notifier-spiffe-id` authorizes the given SPIFFE IDs of
  notifiers to send events to the event server. When set, the event server uses
  mTLS.
- `--authorized-receiver-spiffe-id` authorizes the given SPIFFE IDs of
  receivers to send requests to the receiver server. When set, the receiver
  server uses mTLS.

Each traffic type is protected independently.

Each flag value is interpreted as follows:

- `spiffe://<trust domain>/<path>` authorizes exactly that SPIFFE ID.
- `spiffe://<trust domain>/<path>/` (with a trailing slash) authorizes any
  SPIFFE ID with that path prefix.
- `spiffe://<trust domain>` (without a path) authorizes any SPIFFE ID in that
  trust domain.

Whenever `<trust domain>` equals the special value `self`,
notification-controller understands it as the trust domain of the controller's
own X509-SVID.

Invalid values cause notification-controller to fail at startup.

### SPIFFE Broker API

When `--spiffe-broker-endpoint` and at least one `--spiffe-broker-id` are set,
notification-controller acquires an X.509-SVID from the SPIFFE Workload API and uses it to
authenticate to the SPIFFE Broker API. The `SPIFFE_ENDPOINT_SOCKET` environment
variable must point to the Workload API socket.

The Broker is used to acquire JWT-SVIDs for object identity when a
ServiceAccount sets `credential.auth.fluxcd.io/provider` to `spiffe`. The
Broker endpoint is a TCP gRPC target, e.g.
`dns:///spire-server.spire.svc.cluster.local:8443`.

Each `--spiffe-broker-id` value is interpreted like the SPIFFE IDs used by the
mTLS flags. Invalid values cause notification-controller to fail at startup.

### Feature Gates

| Name                          | Default Value | Description                                                                                                                                                                                                               |
|-------------------------------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `CacheSecretsAndConfigMaps`   | `false`       | Configures the caching of Secrets and ConfigMaps by the controller-runtime client. When enabled, it will cache both object types, resulting in increased memory usage.                                                    |
| `DisableConfigWatchers`       | `false`       | Disables the watchers for ConfigMaps and Secrets.                                                                                                                                                                         |
