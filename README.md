**_Note: This is a divergent fork of [kelseyhightower/confd](https://github.com/kelseyhightower/confd). Backward compatibility is not guaranteed. YMMV_**

# confd

[![Integration Tests](https://github.com/abtreece/confd/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/abtreece/confd/actions/workflows/integration-tests.yml)
[![CodeQL](https://github.com/abtreece/confd/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/abtreece/confd/actions/workflows/codeql-analysis.yml)
[![Codecov](https://codecov.io/gh/abtreece/confd/branch/main/graph/badge.svg?token=bNZ2ngzQO1)](https://codecov.io/gh/abtreece/confd)

`confd` is a lightweight configuration management tool focused on:

* keeping local configuration files up-to-date using data stored in [etcd](https://github.com/etcd-io/etcd),
  [consul](http://consul.io), [dynamodb](http://aws.amazon.com/dynamodb/), [redis](http://redis.io),
  [vault](https://vaultproject.io), [zookeeper](https://zookeeper.apache.org), [aws ssm parameter store](https://aws.amazon.com/ec2/systems-manager/), [aws secrets manager](https://aws.amazon.com/secrets-manager/), [aws acm](https://aws.amazon.com/certificate-manager/), or env vars and processing [template resources](docs/template-resources.md).
* reloading applications to pick up new config file changes

## Features

- **Multiple Backends**: etcd, Consul, Vault, DynamoDB, Redis, Zookeeper, AWS SSM/Secrets Manager/ACM, environment variables, and files
- **Template Processing**: Go text/template with custom functions for configuration generation
- **Watch Mode**: Real-time config updates for supported backends (Consul, etcd, Redis, Zookeeper, env, file)
- **Polling Mode**: Configurable interval-based polling for all backends
- **Validation**: Pre-flight checks, template validation, and configuration validation
- **Metrics**: Prometheus metrics for observability (backend operations, template processing, commands)
- **Health Checks**: HTTP endpoints for health and readiness checks
- **Structured Logging**: JSON and text formats with timing metrics
- **Resilience**: Configurable timeouts, retries, and failure modes (best-effort/fail-fast)
- **Performance**: Template caching and backend client pooling

## Building

Go 1.25+ is required to build confd.

```bash
git clone https://github.com/abtreece/confd.git
cd confd
make build
```

You should now have `confd` in your `bin/` directory:

```bash
ls bin/
confd
```

## Quick Start

### One-time run with etcd

```bash
# Start with etcd backend
confd etcd --node http://127.0.0.1:2379 --onetime

# With environment variables
confd env --onetime

# With file backend
confd file --file /path/to/config.yaml --onetime
```

### Watch mode for real-time updates

```bash
# Watch etcd for changes
confd etcd --node http://127.0.0.1:2379 --watch

# Watch with debouncing (wait 2s after changes settle)
confd etcd --watch --debounce 2s

# Batch processing (collect changes every 5s)
confd etcd --watch --batch-interval 5s
```

### Interval polling

```bash
# Poll every 60 seconds
confd vault --node http://127.0.0.1:8200 --interval 60 \
  --auth-type token --auth-token s.XXX
```

## Metrics and Observability

Enable Prometheus metrics and health checks:

```bash
confd etcd --metrics-addr :9100
```

Endpoints:
- `http://localhost:9100/metrics` - Prometheus metrics
- `http://localhost:9100/health` - Health check
- `http://localhost:9100/ready` - Readiness check
- `http://localhost:9100/ready/detailed` - Detailed readiness

Metrics include:
- Backend request durations and error rates
- Template processing performance
- Command execution times
- Cache hit/miss rates
- File sync operations

## Configuration

confd can be configured via:
1. Configuration file (`/etc/confd/confd.toml`)
2. Environment variables (prefix: `CONFD_`)
3. Command-line flags

Example `confd.toml`:

```toml
backend = "etcd"
log-level = "info"
log-format = "json"
interval = 600
nodes = ["http://127.0.0.1:2379"]
prefix = "/production"

# Timeouts
backend-timeout = "30s"
check-cmd-timeout = "30s"
reload-cmd-timeout = "60s"

# Retries
retry-max-attempts = 3
retry-base-delay = "100ms"
retry-max-delay = "5s"

# Metrics
metrics_addr = ":9100"
```

## Validation and Testing

### Validate configuration

```bash
# Check template resource files
confd --check-config etcd

# Validate specific resource
confd --check-config --resource nginx.toml etcd
```

### Preflight checks

```bash
# Test backend connectivity and authentication
confd --preflight etcd --node http://127.0.0.1:2379
```

### Template validation

```bash
# Syntax check
confd --validate etcd

# With mock data
confd --validate --mock-data test-data.json etcd
```

### Dry run with diff

```bash
# Show pending changes without applying
confd --noop --diff --color etcd
```

## Documentation

- [Quick Start Guide](docs/quick-start-guide.md)
- [Installation](docs/installation.md)
- [Command Line Flags](docs/command-line-flags.md)
- [Configuration Guide](docs/configuration-guide.md)
- [Template Resources](docs/template-resources.md)
- [Template Functions](docs/templates.md)
- [Logging](docs/logging.md)

## Supported Backends

| Backend | Watch Mode | Polling | Authentication |
|---------|------------|---------|----------------|
| etcd | ✅ | ✅ | Basic, TLS, Token |
| Consul | ✅ | ✅ | Basic, TLS, Token |
| Redis | ✅ | ✅ | Password |
| Zookeeper | ✅ | ✅ | None |
| Env | ✅ | ✅ | None |
| File | ✅ | ✅ | None |
| Vault | ❌ | ✅ | Token, AppRole, App-ID, Kubernetes |
| DynamoDB | ❌ | ✅ | AWS SDK |
| SSM | ❌ | ✅ | AWS SDK |
| Secrets Manager | ❌ | ✅ | AWS SDK |
| ACM | ❌ | ✅ | AWS SDK |

## Development

### Running tests

```bash
# Unit tests
make test

# With coverage
go test ./... -race -coverprofile=coverage.out -covermode=atomic

# Integration tests (requires backend services)
make integration
```

### Building releases

```bash
# Snapshot build
make snapshot

# Release build
make release
```

## License

See [LICENSE](LICENSE) file.
