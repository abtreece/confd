# Command Line Flags

Command line flags override the confd [configuration file](configuration-guide.md).

confd uses a subcommand-based CLI where you specify the backend as a subcommand:

```bash
confd <backend> [flags]
```

## Global Flags

These flags are available for all backends:

```
confd --help
```

```text
Usage: confd <command> [flags]

Manage local config files using templates and data from backends

Flags:
  -h, --help                         Show context-sensitive help.
      --confdir="/etc/confd"         confd conf directory
      --config-file="/etc/confd/confd.toml"
                                     confd config file
      --interval=600                 backend polling interval
      --log-level=""                 log level (debug, info, warn, error)
      --log-format=""                log format (text, json)
      --noop                         only show pending changes
      --onetime                      run once and exit
      --prefix=STRING                key path prefix
      --sync-only                    sync without check_cmd and reload_cmd
      --watch                        enable watch support
      --failure-mode="best-effort"   error handling mode (best-effort, fail-fast)
      --keep-stage-file              keep staged files
      --srv-domain=STRING            DNS SRV domain
      --srv-record=STRING            SRV record for backend node discovery
      --check-config                 validate configuration files and exit
      --preflight                    run connectivity checks and exit
      --validate                     validate templates without processing
      --mock-data=STRING             JSON file with mock data for validation
      --resource=STRING              specific resource file to validate
      --diff                         show diff output in noop mode
      --diff-context=3               lines of context for diff
      --color                        colorize diff output
      --debounce=STRING              debounce duration for watch mode
      --batch-interval=STRING        batch processing interval for watch mode
      --template-cache               enable template compilation caching (default: true)
      --no-template-cache            disable template compilation caching
      --stat-cache-ttl=1s            TTL for template file stat cache
      --backend-timeout=30s          timeout for backend operations
      --check-cmd-timeout=30s        default timeout for check commands
      --reload-cmd-timeout=60s       default timeout for reload commands
      --dial-timeout=5s              connection timeout (not used by consul, env, file or vault)
      --read-timeout=1s              redis only: read timeout
      --write-timeout=1s             redis only: write timeout
      --preflight-timeout=10s        preflight check timeout
      --watch-error-backoff=2s       backoff after watch errors
      --retry-max-attempts=3         redis only: max connection retry attempts
      --retry-base-delay=100ms       redis only: initial retry backoff delay
      --retry-max-delay=5s           redis only: maximum retry backoff delay
      --shutdown-timeout=30s         graceful shutdown timeout
      --systemd-notify               enable systemd sd_notify support
      --watchdog-interval=0          systemd watchdog ping interval (0=disabled)
      --metrics-addr=STRING          address for metrics endpoint (e.g., :9100)
      --version                      print version and exit

Commands:
  consul        Use Consul backend
  etcd          Use etcd backend
  vault         Use Vault backend
  redis         Use Redis backend
  zookeeper     Use Zookeeper backend
  dynamodb      Use DynamoDB backend
  ssm           Use AWS SSM Parameter Store backend
  acm           Use AWS ACM backend
  secretsmanager Use AWS Secrets Manager backend
  imds          Use AWS EC2 IMDS backend
  env           Use environment variables backend
  file          Use file backend

Run "confd <command> --help" for more information on a command.
```

## Backend-Specific Flags

Each backend has its own set of flags. Use `confd <backend> --help` to see all available options.

### consul

```bash
confd consul --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | Consul agent address | `127.0.0.1:8500` |
| `--scheme` | URI scheme (http or https) | `http` |
| `--basic-auth` | Send `--username` and `--password` as HTTP basic auth, in place of `CONSUL_HTTP_AUTH`. Without it the two flags are ignored (confd logs a warning) and `CONSUL_HTTP_AUTH` applies if set | `false` |
| `--username` | Basic auth username; used only with `--basic-auth` | - |
| `--password` | Basic auth password; used only with `--basic-auth` | - |
| `--auth-token` | ACL token; wins over `CONSUL_HTTP_TOKEN` and `CONSUL_HTTP_TOKEN_FILE` | - |
| `--client-cert` | Client certificate file | - |
| `--client-key` | Client key file | - |
| `--client-ca-keys` | CA certificate file | - |

### etcd

```bash
confd etcd --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | etcd node addresses (repeatable) | - |
| `--scheme` | URI scheme (http or https) | `http` |
| `--basic-auth` | Authenticate with `--username` and `--password`; nothing is sent unless both are non-empty. Without it the two flags are ignored and confd logs a warning | `false` |
| `--username` | Username; used only with `--basic-auth` | - |
| `--password` | Password; used only with `--basic-auth` | - |
| `--auth-token` | JWT used in place of a username and password; the etcd client refuses to start when it is combined with `--basic-auth` and a non-empty `--username` or `--password` | - |
| `--client-cert` | Client certificate file | - |
| `--client-key` | Client key file | - |
| `--client-ca-keys` | CA certificate file | - |
| `--client-insecure` | Skip TLS certificate verification | `false` |

### vault

```bash
confd vault --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | Vault server address | - |
| `--auth-type` | Auth method (token, app-id, app-role, kubernetes) | - |
| `--auth-token` | Vault auth token ($VAULT_TOKEN) | - |
| `--app-id` | App ID for app-id auth | - |
| `--user-id` | User ID for app-id auth | - |
| `--role-id` | Role ID for app-role/kubernetes auth | - |
| `--secret-id` | Secret ID for app-role auth | - |
| `--path` | Auth mount path | - |
| `--username` | Username for userpass auth | - |
| `--password` | Password for userpass auth | - |
| `--client-cert` | Client certificate file | - |
| `--client-key` | Client key file | - |
| `--client-ca-keys` | CA certificate file | - |

### redis

```bash
confd redis --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | Redis server address | `127.0.0.1:6379` |
| `--username` | Redis ACL username (Redis 6+), sent with the password | - |
| `--password` | Redis password. Also read from `REDIS_PASSWORD`; the flag wins when both are set. `--client-key` is the legacy way to pass the password and is used only when no password is set | - |
| `--separator` | Key separator (replaces `/`) | `/` |
| `--client-key` | Legacy way to pass the password; used only when no password is set | - |
| `--basic-auth`, `--auth-token`, `--client-cert`, `--client-ca-keys` | Accepted but not used by the Redis client; confd logs a warning at startup when one is set | - |

### zookeeper

```bash
confd zookeeper --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `-n, --node` | ZooKeeper server addresses (repeatable) | - |

### dynamodb

```bash
confd dynamodb --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `--table` | DynamoDB table name | Required |

### ssm

```bash
confd ssm --help
```

No backend-specific flags. Uses AWS SDK credential chain.

### secretsmanager

```bash
confd secretsmanager --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `--secretsmanager-version-stage` | Version stage (AWSCURRENT, AWSPREVIOUS) | `AWSCURRENT` |
| `--secretsmanager-no-flatten` | Disable JSON flattening | `false` |

### acm

```bash
confd acm --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `--acm-export-private-key` | Enable private key export | `false` |

### imds

```bash
confd imds --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `--imds-cache-ttl` | Cache TTL for metadata (e.g., 60s, 5m) | `60s` |

**Note**: IMDS backend only works on EC2 instances with IMDSv2 enabled.

### env

```bash
confd env --help
```

No backend-specific flags.

### file

```bash
confd file --help
```

| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to YAML/JSON file or directory (repeatable) | Required |
| `--filter` | Glob pattern to filter files | `*` |

## Examples

### One-time run with etcd

```bash
confd etcd --node http://127.0.0.1:2379 --onetime
```

### Watch mode with Consul

```bash
confd consul --node 127.0.0.1:8500 --watch
```

### Interval polling with Vault

```bash
confd vault --node http://127.0.0.1:8200 \
  --auth-type token --auth-token s.XXX \
  --interval 60
```

### Environment variables backend

```bash
confd env --onetime
```

### File backend with watch

```bash
confd file --file /etc/myapp/config.yaml --watch
```

## Validation Flags

These flags help validate your configuration without making changes:

### --check-config

Validates all template resource TOML files and exits. Checks for:
- Valid TOML syntax
- Required fields (src, dest, keys)
- Valid file mode format
- Template file existence
- Destination directory existence
- Valid backend configuration (if specified)
- Valid duration formats for min_reload_interval and debounce

```bash
confd --check-config consul
```

### --preflight

Runs connectivity checks against the backend and exits. Verifies:
- Backend is reachable
- Authentication is valid
- Keys referenced in templates are accessible

```bash
confd --preflight consul --node 127.0.0.1:8500
```

### --validate

Validates template syntax without processing. No backend connection is required — confd uses stub functions that return empty values for `getv`, `gets`, `ls`, etc.

```bash
# Syntax check only
confd --validate consul

# Validate a specific resource file
confd --validate --resource nginx.toml consul
```

### --mock-data

Provides a JSON file to test template execution during `--validate`. The JSON object is passed as Go's template dot context (`.`), so templates access it via `{{ .key }}` syntax. This is useful for templates that receive data via `include` or use the dot context directly.

Note: confd's store functions (`getv`, `gets`, etc.) still use stubs during validation. Mock data does not populate the store — it populates the template's dot context.

```bash
confd --validate --mock-data mock.json consul
```

**Mock data format** — a flat or nested JSON object:

```json
{
  "name": "myapp",
  "port": 8080,
  "features": ["auth", "logging"]
}
```

**Template using mock data:**

```
{
  "app_name": "{{ .name }}",
  "listen_port": {{ .port }}
}
```

When combined with `output_format = "json"` in the template resource, this also validates that the rendered output is valid JSON.

### --resource

Validates a specific resource file instead of all resources:

```bash
confd --check-config --resource nginx.toml consul
confd --validate --resource nginx.toml consul
```

## Operational Mode Flags

### --sync-only

Writes rendered templates to destination files but skips all `check_cmd` and `reload_cmd` execution. The template is still rendered, compared, and synced — only the commands are suppressed.

Use cases:
- **Initial provisioning**: Lay down config files before the application is installed or running
- **External orchestration**: When a separate tool (systemd, Kubernetes, etc.) handles service restarts
- **Bulk updates**: Update many config files without triggering cascading reloads

```bash
# Write all configs without restarting services
confd --sync-only etcd --onetime
```

### --keep-stage-file

Preserves the staged (rendered) template file after syncing to the destination. Stage files are created in the destination file's directory as hidden dotfiles (e.g., if `dest = "/etc/myapp/app.conf"`, the stage file is `/etc/myapp/.app.conf<random>`). Normally, confd removes the temp file after syncing. With this flag, the staged file is copied (not moved) so it remains for inspection.

```bash
confd --keep-stage-file etcd --onetime
# Stage files are hidden dotfiles next to the destination
ls -a /etc/myapp/.app.conf*
```

This is useful for debugging when:
- Template output doesn't match expectations
- You want to inspect the rendered result without affecting the destination
- Diagnosing file permission or ownership issues

## Dry-Run and Diff Flags

### --noop with --diff

Shows what changes would be made without applying them:

```bash
# Show pending changes with unified diff
confd --noop --diff consul

# With context lines (default: 3)
confd --noop --diff --diff-context 5 consul

# With colorized output
confd --noop --diff --color consul
```

## Watch Mode Flags

### --interval in watch mode

`--interval` also applies with `--watch`, to template resources whose `[backend]` section uses a backend that cannot watch (acm, dynamodb, env, imds, secretsmanager, ssm, vault). Those resources are processed once when resources load and then every `--interval` seconds; watch-capable resources are watched as usual. The default is 600 seconds, so without `--interval` they are refreshed every 10 minutes. At startup and after every `SIGHUP` reload, confd logs one INFO line per polled resource:

```text
polling <path> every <interval>: <backend> backend cannot watch
```

The global backend must still support watch. With `--batch-interval`, polled resources go through the batch like watched ones.

With `--watch`, and neither `--onetime` nor `--preflight`, an `--interval` below 1 is rejected at startup, whether or not any resource is polled:

```text
invalid --interval <n>: must be at least 1 second with --watch
```

```bash
confd --watch --interval 60 consul
```

### --debounce

Sets a global debounce duration for watch mode. After detecting a change, confd waits this duration and then processes the template once. Additional changes during the wait do not extend it, so a key that keeps changing is processed about once per debounce period. A resource's own `debounce` key overrides this flag, and neither is applied when `--batch-interval` is set. Debounce is not applied to polled resources either (see [--interval in watch mode](#--interval-in-watch-mode)); the interval already limits how often they are processed:

```bash
confd --watch --debounce 2s consul
```

### --batch-interval

Enables batch processing mode. Changes from all templates are collected and processed together after the interval:

```bash
confd --watch --batch-interval 5s consul
```

**Debounce vs Batch Processing:**
- `--debounce`: Per-template, waits a fixed delay after a change and then processes that template
- `--batch-interval`: Global, collects all changes and processes them together

## Timeout Flags

Configure timeouts for various operations:

| Flag | Description | Default |
|------|-------------|---------|
| `--backend-timeout` | Overall timeout for backend operations | `30s` |
| `--check-cmd-timeout` | Default timeout for check commands | `30s` |
| `--reload-cmd-timeout` | Default timeout for reload commands | `60s` |
| `--dial-timeout` | Connection timeout. Used by etcd, redis and zookeeper (connecting), imds (client setup), and acm, dynamodb, secretsmanager and ssm (only the region lookup against instance metadata, and only when `AWS_REGION` is not set). Not used by consul, vault, env or file | `5s` |
| `--read-timeout` | Read timeout. Used by redis only | `1s` |
| `--write-timeout` | Write timeout. Used by redis only | `1s` |
| `--preflight-timeout` | Timeout for preflight checks | `10s` |
| `--watch-error-backoff` | Backoff duration after watch errors | `2s` |
| `--shutdown-timeout` | Graceful shutdown timeout | `30s` |

### Which backends use the connection timeout, retry, auth and TLS flags

`--backend-timeout` applies to every backend. The connection timeout and [retry](#retry-flags) flags, and some of the auth and TLS settings, do not:

| Flag | Used by | Not used by |
|------|---------|-------------|
| `--dial-timeout` | etcd, redis and zookeeper (connecting); imds (client setup); acm, dynamodb, secretsmanager and ssm (only the region lookup against instance metadata, and only when `AWS_REGION` is not set) | consul, vault, env, file |
| `--read-timeout`, `--write-timeout` | redis | every other backend |
| `--retry-max-attempts`, `--retry-base-delay`, `--retry-max-delay` | redis | every other backend |
| `--auth-token` | consul (ACL token), etcd (JWT), vault | every other backend, including redis |
| `--basic-auth` | consul, etcd | every other backend, including redis |
| `--client-cert`, `--client-ca-keys` | consul, etcd, vault | every other backend, including redis |

Setting one of these for a backend that does not use it changes nothing. confd logs a warning for each such flag at startup and carries on, for example:

```
--retry-max-attempts has no effect with the env backend
```

The warning is logged whether the setting came from the command line, its `CONFD_*` environment variable or `confd.toml`, and also when it is set to its default value.

consul and etcd use `--username` and `--password` only when `--basic-auth` is set. When either is supplied (on the command line or as `username` / `password` in `confd.toml`) without basic auth, confd logs, for example:

```
--username has no effect with the consul backend unless --basic-auth is set
```

A per-resource `[backend]` section gets the same warnings for its own backend, prefixed with the resource file. confd also warns about any key in the section that it does not recognise, since such a key is ignored. That includes `client_ca_keys` and `node`, which `confd.toml` accepts but a `[backend]` section does not (use `client_cakeys` and `nodes`):

```
/etc/confd/conf.d/app.toml: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"
/etc/confd/conf.d/app.toml: --basic-auth has no effect with the redis backend
```

These are logged each time the resource is loaded (at startup and on every SIGHUP reload), and with `--check-config` and `--validate`.

### --watch-error-backoff

When running in `--watch` mode, confd maintains a persistent watch connection to the backend. If `WatchPrefix` returns an error — due to network issues, backend restarts, authentication expiry, or connection timeouts — confd sleeps for this duration before retrying. This prevents tight error loops from overwhelming the backend or consuming excessive CPU during outages.

The default of `2s` is appropriate for most deployments. Increase it if your backend has slow recovery (e.g., Vault unsealing) or if you want to reduce retry pressure during extended outages:

```bash
confd --watch --watch-error-backoff 5s consul
```

### Example: Production timeout configuration

```bash
confd etcd --watch \
  --backend-timeout 30s \
  --check-cmd-timeout 30s \
  --reload-cmd-timeout 60s \
  --shutdown-timeout 30s
```

## Retry Flags

Configure retry behavior for connections to the redis backend. No other backend uses these flags; setting one for another backend logs a warning at startup and has no other effect (see [Which backends use the connection timeout, retry, auth and TLS flags](#which-backends-use-the-connection-timeout-retry-auth-and-tls-flags)).

| Flag | Description | Default |
|------|-------------|---------|
| `--retry-max-attempts` | Maximum number of retry attempts. Used by redis only | `3` |
| `--retry-base-delay` | Initial backoff delay. Used by redis only | `100ms` |
| `--retry-max-delay` | Maximum backoff delay. Used by redis only | `5s` |

Retries use exponential backoff with jitter. The delay doubles after each attempt up to `--retry-max-delay`.

### Example: Resilient retry configuration

```bash
confd redis --watch \
  --retry-max-attempts 5 \
  --retry-base-delay 200ms \
  --retry-max-delay 10s
```

## Failure Mode

### --failure-mode

Controls how confd handles errors when processing multiple templates:

| Value | Behavior |
|-------|----------|
| `best-effort` | Continue processing remaining templates when one fails (default) |
| `fail-fast` | Stop all processing on first template error |

A template resource file that fails to load (invalid TOML, for example) is treated as follows:

| Mode | `best-effort` | `fail-fast` |
|------|---------------|-------------|
| `--onetime` | Process the resources that loaded, then exit non-zero with an error naming each file that failed to load | Process nothing and exit non-zero |
| Interval and watch | Log the load error and keep running with the resources that loaded | Same as `best-effort` |

In interval and watch mode, if resource files exist and none of them load, confd shuts down and exits non-zero under either failure mode, at startup and on a `SIGHUP` reload. An empty or missing `conf.d` is not a load failure.

```bash
# Continue processing other templates even if one fails
confd --failure-mode best-effort etcd --onetime

# Stop immediately on first error
confd --failure-mode fail-fast etcd --onetime
```

## Systemd Integration Flags

Enable systemd service integration for production deployments:

| Flag | Description | Default |
|------|-------------|---------|
| `--systemd-notify` | Enable systemd sd_notify support | `false` |
| `--watchdog-interval` | Systemd watchdog ping interval (0=disabled) | `0` |

When `--systemd-notify` is enabled, confd sends:
- `READY=1` when service is ready
- `RELOADING=1` when reloading configuration (SIGHUP)
- `STOPPING=1` when shutting down
- `WATCHDOG=1` heartbeat pings (if `--watchdog-interval` > 0)

### Example: Systemd service configuration

```bash
confd etcd --watch \
  --systemd-notify \
  --watchdog-interval 30s \
  --shutdown-timeout 30s
```

See [Service Deployment Guide](service-deployment.md) for complete systemd configuration.

## Metrics and Observability Flags

### --metrics-addr

Enable Prometheus metrics and health check endpoints:

```bash
confd etcd --watch --metrics-addr :9100
```

When enabled, the following endpoints are available:
- `/metrics` - Prometheus metrics
- `/health` - Basic health check
- `/ready` - Readiness check
- `/ready/detailed` - Detailed readiness with diagnostics

Can also be set via environment variable `CONFD_METRICS_ADDR`.

## Performance Flags

### --template-cache / --no-template-cache

Enable or disable template compilation caching (enabled by default):

```bash
# Disable caching (useful for debugging)
confd --no-template-cache etcd --onetime
```

### --stat-cache-ttl

TTL for caching template file stat results. Reduces filesystem calls when templates don't change frequently:

```bash
confd --stat-cache-ttl 5s etcd --watch
```

## Environment Variables

Global configuration can also be set via environment variables with the `CONFD_` prefix. Command-line flags take precedence over environment variables.

For a string setting, an environment variable set to an empty value (`CONFD_PREFIX=`) is treated as unset: the value from `confd.toml` applies, and without one the built-in default. `CONFD_DEBOUNCE` and `CONFD_BATCH_INTERVAL` hold durations but are string settings, so an empty value for them is treated as unset too. For a boolean, integer or any other duration setting an empty value is an error.

### Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_CONFDIR` | Configuration directory | `/etc/confd` |
| `CONFD_CONFIG_FILE` | Configuration file path | `/etc/confd/confd.toml` |
| `CONFD_INTERVAL` | Polling interval (seconds) | `600` |
| `CONFD_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `CONFD_LOG_FORMAT` | Log format (text, json) | `text` |
| `CONFD_PREFIX` | Key prefix | |
| `CONFD_WATCH` | Enable watch mode | `false` |
| `CONFD_ONETIME` | Run once and exit | `false` |
| `CONFD_NOOP` | Dry-run mode (no changes) | `false` |
| `CONFD_SYNC_ONLY` | Sync without check/reload commands | `false` |
| `CONFD_FAILURE_MODE` | Error handling (best-effort, fail-fast) | `best-effort` |
| `CONFD_KEEP_STAGE_FILE` | Keep staged files after processing | `false` |

### Timeouts and Performance

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_BACKEND_TIMEOUT` | Backend operation timeout | `30s` |
| `CONFD_CHECK_CMD_TIMEOUT` | Check command timeout | `30s` |
| `CONFD_RELOAD_CMD_TIMEOUT` | Reload command timeout | `60s` |
| `CONFD_DIAL_TIMEOUT` | Connection timeout (not used by consul, vault, env or file) | `5s` |
| `CONFD_READ_TIMEOUT` | Read timeout (redis only) | `1s` |
| `CONFD_WRITE_TIMEOUT` | Write timeout (redis only) | `1s` |
| `CONFD_PREFLIGHT_TIMEOUT` | Preflight check timeout | `10s` |
| `CONFD_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30s` |

### Retry Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_RETRY_MAX_ATTEMPTS` | Max retry attempts (redis only) | `3` |
| `CONFD_RETRY_BASE_DELAY` | Initial backoff delay (redis only) | `100ms` |
| `CONFD_RETRY_MAX_DELAY` | Maximum backoff delay (redis only) | `5s` |

### Watch Mode Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_DEBOUNCE` | Debounce duration for watch mode | |
| `CONFD_BATCH_INTERVAL` | Batch processing interval | |
| `CONFD_WATCH_ERROR_BACKOFF` | Backoff after watch errors | `2s` |

### Caching

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_TEMPLATE_CACHE` | Enable template caching | `true` |
| `CONFD_STAT_CACHE_TTL` | Template file stat cache TTL | `1s` |

### Service Discovery

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_SRV_DOMAIN` | DNS SRV domain for backend discovery | |
| `CONFD_SRV_RECORD` | SRV record for backend node discovery | |

### Observability

| Variable | Description | Default |
|----------|-------------|---------|
| `CONFD_METRICS_ADDR` | Metrics endpoint address (e.g., `:9100`) | |
| `CONFD_SYSTEMD_NOTIFY` | Enable systemd sd_notify | `false` |
| `CONFD_WATCHDOG_INTERVAL` | Systemd watchdog ping interval (0=disabled) | `0` |

### TLS/Authentication

| Variable | Description |
|----------|-------------|
| `CONFD_CLIENT_CERT` | Client certificate file |
| `CONFD_CLIENT_KEY` | Client key file |
| `CONFD_CLIENT_CAKEYS` | CA certificate file |

### Backend-Specific Variables

| Variable | Backend | Description |
|----------|---------|-------------|
| `VAULT_TOKEN` | Vault | Vault authentication token |
| `ACM_EXPORT_PRIVATE_KEY` | ACM | Private key export. Takes `true` or `false`; `false` disables export |
| `REDIS_PASSWORD` | Redis | Redis password |
| `IMDS_ENDPOINT` | IMDS | Custom IMDS endpoint (testing only) |

Backend-specific environment variables are also documented in each backend's README.
