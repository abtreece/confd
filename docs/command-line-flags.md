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
      --dial-timeout=5s              connection timeout for backends
      --read-timeout=1s              read timeout for backend operations
      --write-timeout=1s             write timeout for backend operations
      --preflight-timeout=10s        preflight check timeout
      --watch-error-backoff=2s       backoff after watch errors
      --retry-max-attempts=3         max retry attempts for connections
      --retry-base-delay=100ms       initial backoff delay
      --retry-max-delay=5s           maximum backoff delay
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
| `--basic-auth` | Use basic authentication | `false` |
| `--username` | Authentication username | - |
| `--password` | Authentication password | - |
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
| `--basic-auth` | Use basic authentication | `false` |
| `--username` | Authentication username | - |
| `--password` | Authentication password | - |
| `--auth-token` | Bearer token for authentication | - |
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
| `--basic-auth` | Use basic authentication | `false` |
| `--username` | Authentication username | - |
| `--password` | Redis password | - |
| `--separator` | Key separator (replaces `/`) | `/` |
| `--client-cert` | Client certificate file | - |
| `--client-key` | Client key file | - |
| `--client-ca-keys` | CA certificate file | - |

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

Validates template syntax without processing. Can be combined with `--mock-data` to test template rendering:

```bash
# Syntax check only
confd --validate consul

# With mock data for full validation
confd --validate --mock-data /path/to/mock.json consul
```

### --resource

Validates a specific resource file instead of all resources:

```bash
confd --check-config --resource nginx.toml consul
confd --validate --resource nginx.toml consul
```

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

### --debounce

Sets a global debounce duration for watch mode. After detecting a change, confd waits this duration before processing. Additional changes reset the timer:

```bash
confd --watch --debounce 2s consul
```

### --batch-interval

Enables batch processing mode. Changes from all templates are collected and processed together after the interval:

```bash
confd --watch --batch-interval 5s consul
```

**Debounce vs Batch Processing:**
- `--debounce`: Per-template, waits for individual template changes to settle
- `--batch-interval`: Global, collects all changes and processes them together

## Timeout Flags

Configure timeouts for various operations:

| Flag | Description | Default |
|------|-------------|---------|
| `--backend-timeout` | Overall timeout for backend operations | `30s` |
| `--check-cmd-timeout` | Default timeout for check commands | `30s` |
| `--reload-cmd-timeout` | Default timeout for reload commands | `60s` |
| `--dial-timeout` | Connection timeout for backends | `5s` |
| `--read-timeout` | Read timeout for backend operations | `1s` |
| `--write-timeout` | Write timeout for backend operations | `1s` |
| `--preflight-timeout` | Timeout for preflight checks | `10s` |
| `--watch-error-backoff` | Backoff duration after watch errors | `2s` |
| `--shutdown-timeout` | Graceful shutdown timeout | `30s` |

### Example: Production timeout configuration

```bash
confd etcd --watch \
  --backend-timeout 30s \
  --check-cmd-timeout 30s \
  --reload-cmd-timeout 60s \
  --shutdown-timeout 30s
```

## Retry Flags

Configure retry behavior for backend connections:

| Flag | Description | Default |
|------|-------------|---------|
| `--retry-max-attempts` | Maximum number of retry attempts | `3` |
| `--retry-base-delay` | Initial backoff delay | `100ms` |
| `--retry-max-delay` | Maximum backoff delay | `5s` |

Retries use exponential backoff with jitter. The delay doubles after each attempt up to `--retry-max-delay`.

### Example: Resilient retry configuration

```bash
confd etcd --watch \
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

Global configuration can also be set via environment variables with the `CONFD_` prefix:

| Variable | Description |
|----------|-------------|
| `CONFD_CONFDIR` | Configuration directory |
| `CONFD_INTERVAL` | Polling interval |
| `CONFD_LOG_LEVEL` | Log level |
| `CONFD_PREFIX` | Key prefix |
| `CONFD_METRICS_ADDR` | Metrics endpoint address |
| `CONFD_SYSTEMD_NOTIFY` | Enable systemd notify |
| `CONFD_CLIENT_CERT` | Client certificate file |
| `CONFD_CLIENT_KEY` | Client key file |
| `CONFD_CLIENT_CAKEYS` | CA certificate file |

Backend-specific environment variables:

| Variable | Backend | Description |
|----------|---------|-------------|
| `VAULT_TOKEN` | Vault | Vault authentication token |
| `ACM_EXPORT_PRIVATE_KEY` | ACM | Enable private key export |
| `IMDS_ENDPOINT` | IMDS | Custom IMDS endpoint (testing only) |

Backend-specific environment variables are also documented in each backend's README.
