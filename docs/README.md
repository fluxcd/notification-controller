# Notification Controller

## Flags

| Name                                  | Type          | Description                                                                                                                        |
|---------------------------------------|---------------|------------------------------------------------------------------------------------------------------------------------------------|
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
| `--token-cache-max-duration`          | duration      | The maximum duration for which a token would be considered unexpired. This is capped at 1h. (default 1h)                           |
| `--watch-all-namespaces`              | boolean       | Watch for custom resources in all namespaces, if set to false it will only watch the runtime namespace. (default true)             |
| `--feature-gates`                     | mapStringBool | A comma separated list of key=value pairs defining the state of experimental features.                                             |


### Feature Gates

| Name                          | Default Value | Description                                                                                                                                                                                                               |
|-------------------------------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `CacheSecretsAndConfigMaps`   | `false`       | Configures the caching of Secrets and ConfigMaps by the controller-runtime client. When enabled, it will cache both object types, resulting in increased memory usage.                                                    |
| `DisableConfigWatchers`       | `false`       | Disables the watchers for ConfigMaps and Secrets.                                                                                                                                                                         |
| `ObjectLevelWorkloadIdentity` | `false`       | Enables the use of object-level workload identity for the controller.                                                                                                                                                     |
