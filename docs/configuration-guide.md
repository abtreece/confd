# Configuration Guide

The confd configuration file is written in [TOML](https://github.com/mojombo/toml)
and loaded from `/etc/confd/confd.toml` by default. You can specify the config file via the `--config-file` command line flag.

> Note: You can use confd without a configuration file. See [Command Line Flags](command-line-flags.md).

## Configuration Hierarchy

Configuration is loaded from multiple sources in order of precedence (highest to lowest):

1. Command-line flags
2. Environment variables (prefix: `CONFD_`)
3. Configuration file (`/etc/confd/confd.toml`)
4. Built-in defaults

## Core Options

* `backend` (string) - The backend to use. Default: `"etcd"`
* `confdir` (string) - The path to confd configs. Default: `"/etc/confd"`
* `interval` (int) - The backend polling interval in seconds. Default: `600`
* `noop` (bool) - Enable noop mode. Process all template resources; skip target update. Default: `false`
* `prefix` (string) - The string to prefix to keys. This prefix is concatenated with any prefix set in template resource files (e.g., global `production` + resource `myapp` = `/production/myapp`).
* `sync_only` (bool) - Sync files without running check_cmd and reload_cmd. Default: `false`
* `watch` (bool) - Enable watch support for backends that support it. Default: `false`
* `keep_stage_file` (bool) - Keep staged files after processing (useful for debugging). Default: `false`

## Logging Options

* `log_level` (string) - Log level: `debug`, `info`, `warn`, `error`. Default: `"info"`
* `log_format` (string) - Log format: `text` or `json`. Default: `"text"`

## Backend Connection Options

* `nodes` (array of strings) - List of backend node addresses.
* `scheme` (string) - The backend URI scheme: `http` or `https`. Default: `"http"`
* `srv_domain` (string) - DNS SRV domain for service discovery.
* `srv_record` (string) - The SRV record to search for backend nodes.

## TLS/Authentication Options

* `client_cert` (string) - Path to client certificate file.
* `client_key` (string) - Path to client key file.
* `client_cakeys` (string) - Path to CA certificate file.
* `client_insecure` (bool) - Skip TLS certificate verification. Default: `false`
* `basic_auth` (bool) - Use Basic Auth to authenticate (consul and etcd backends only). Default: `false`
* `username` (string) - Username for authentication.
* `password` (string) - Password for authentication.
* `auth_token` (string) - Auth bearer token to use.

## Timeout Options

* `backend_timeout` (duration) - Overall timeout for backend operations. Default: `"30s"`
* `check_cmd_timeout` (duration) - Default timeout for check commands. Default: `"30s"`
* `reload_cmd_timeout` (duration) - Default timeout for reload commands. Default: `"60s"`
* `dial_timeout` (duration) - Connection timeout for backends. Default: `"5s"`
* `read_timeout` (duration) - Read timeout for backend operations. Default: `"1s"`
* `write_timeout` (duration) - Write timeout for backend operations. Default: `"1s"`
* `preflight_timeout` (duration) - Timeout for preflight checks. Default: `"10s"`
* `watch_error_backoff` (duration) - Backoff duration after watch errors. Default: `"2s"`
* `shutdown_timeout` (duration) - Graceful shutdown timeout. Default: `"30s"`

## Retry Options

* `retry_max_attempts` (int) - Maximum number of retry attempts. Default: `3`
* `retry_base_delay` (duration) - Initial backoff delay. Default: `"100ms"`
* `retry_max_delay` (duration) - Maximum backoff delay. Default: `"5s"`

## Performance Options

* `template_cache` (bool) - Enable template compilation caching. Default: `true`
* `stat_cache_ttl` (duration) - TTL for template file stat cache. Default: `"1s"`

## Error Handling Options

* `failure_mode` (string) - Error handling mode: `best-effort` or `fail-fast`. Default: `"best-effort"`
  - `best-effort`: Continue processing remaining templates when one fails
  - `fail-fast`: Stop all processing on first template error

## Watch Mode Options

* `debounce` (duration) - Global debounce duration for watch mode. Default: none
* `batch_interval` (duration) - Batch processing interval for watch mode. Default: none

## Metrics and Observability

* `metrics_addr` (string) - Address for metrics endpoint (e.g., `:9100`). Disabled if empty.

## Systemd Integration

* `systemd_notify` (bool) - Enable systemd sd_notify support. Default: `false`
* `watchdog_interval` (duration) - Systemd watchdog ping interval (0=disabled). Default: `"0"`

## Backend-Specific Options

### Vault

* `auth_type` (string) - Vault auth backend type: `token`, `app-id`, `app-role`, `kubernetes`, `userpass`.
* `app_id` (string) - Vault app-id for app-id auth.
* `user_id` (string) - Vault user-id for app-id auth.
* `role_id` (string) - Vault role-id for app-role/kubernetes auth.
* `secret_id` (string) - Vault secret-id for app-role auth.
* `path` (string) - Vault mount path of the auth method.

### DynamoDB

* `table` (string) - The name of the DynamoDB table.

### Redis

* `separator` (string) - The separator to replace `/` with when looking up keys.

### File

* `file` (array of strings) - The YAML/JSON files to watch for changes.
* `filter` (string) - Glob pattern to filter files. Default: `"*"`

### IMDS

* `imds_cache_ttl` (duration) - Cache TTL for IMDS metadata. Default: `"60s"`

### Secrets Manager

* `secretsmanager_version_stage` (string) - Version stage: `AWSCURRENT`, `AWSPREVIOUS`, or custom. Default: `"AWSCURRENT"`
* `secretsmanager_no_flatten` (bool) - Disable JSON flattening. Default: `false`

### ACM

* `acm_export_private_key` (bool) - Enable private key export. Default: `false`

## Example Configuration

### Basic etcd Configuration

```toml
backend = "etcd"
confdir = "/etc/confd"
log_level = "info"
interval = 600
nodes = [
  "http://127.0.0.1:2379",
]
prefix = "/production"
```

### Production Configuration with Timeouts and Retries

```toml
backend = "etcd"
confdir = "/etc/confd"
log_level = "info"
log_format = "json"
watch = true
nodes = [
  "https://etcd1.example.com:2379",
  "https://etcd2.example.com:2379",
  "https://etcd3.example.com:2379",
]
scheme = "https"
prefix = "/production"

# TLS
client_cert = "/etc/confd/ssl/client.crt"
client_key = "/etc/confd/ssl/client.key"
client_cakeys = "/etc/confd/ssl/ca.crt"

# Timeouts
backend_timeout = "30s"
check_cmd_timeout = "30s"
reload_cmd_timeout = "60s"
shutdown_timeout = "30s"

# Retries
retry_max_attempts = 5
retry_base_delay = "200ms"
retry_max_delay = "10s"

# Error handling
failure_mode = "best-effort"

# Performance
template_cache = true
stat_cache_ttl = "5s"

# Metrics
metrics_addr = ":9100"
```

### Vault Configuration with AppRole Auth

```toml
backend = "vault"
confdir = "/etc/confd"
log_level = "info"
interval = 60
nodes = [
  "https://vault.example.com:8200",
]

# Vault auth
auth_type = "app-role"
role_id = "my-role-id"
secret_id = "my-secret-id"
path = "approle"

# TLS
client_cakeys = "/etc/confd/ssl/ca.crt"
```

### Consul Configuration with Watch Mode

```toml
backend = "consul"
confdir = "/etc/confd"
log_level = "info"
watch = true
nodes = [
  "127.0.0.1:8500",
]
prefix = "/myapp"

# Watch mode tuning
debounce = "2s"

# Systemd integration
systemd_notify = true
watchdog_interval = "30s"
```
