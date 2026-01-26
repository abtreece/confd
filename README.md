**_Note: This is a divergent fork of [kelseyhightower/confd](https://github.com/kelseyhightower/confd). Backward compatibility is not guaranteed. YMMV_**

# confd

[![Integration Tests](https://github.com/abtreece/confd/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/abtreece/confd/actions/workflows/integration-tests.yml)
[![CodeQL](https://github.com/abtreece/confd/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/abtreece/confd/actions/workflows/codeql-analysis.yml)
[![Codecov](https://codecov.io/gh/abtreece/confd/branch/main/graph/badge.svg?token=bNZ2ngzQO1)](https://codecov.io/gh/abtreece/confd)
[![Docker](https://img.shields.io/docker/v/abtreece/confd?label=docker&sort=semver)](https://hub.docker.com/r/abtreece/confd)

`confd` is a lightweight configuration management tool focused on:

* keeping local configuration files up-to-date using data stored in [etcd](https://github.com/etcd-io/etcd),
  [consul](http://consul.io), [dynamodb](http://aws.amazon.com/dynamodb/), [redis](http://redis.io),
  [vault](https://vaultproject.io), [zookeeper](https://zookeeper.apache.org), [aws ssm parameter store](https://aws.amazon.com/ec2/systems-manager/), [aws secrets manager](https://aws.amazon.com/secrets-manager/), [aws acm](https://aws.amazon.com/certificate-manager/), [aws ec2 imds](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html), or env vars and processing [template resources](docs/template-resources.md).
* reloading applications to pick up new config file changes

## Features

- **Multiple Backends**: etcd, Consul, Vault, DynamoDB, Redis, Zookeeper, AWS SSM/Secrets Manager/ACM/IMDS, environment variables, and files
- **Template Processing**: Go text/template with custom functions for configuration generation
- **Watch Mode**: Real-time config updates for supported backends (Consul, etcd, Redis, Zookeeper, file)
- **Polling Mode**: Configurable interval-based polling for all backends
- **Validation**: Pre-flight checks, template validation, and configuration validation
- **Metrics**: Prometheus metrics for observability (backend operations, template processing, commands)
- **Health Checks**: HTTP endpoints for health and readiness checks
- **Structured Logging**: JSON and text formats with timing metrics
- **Resilience**: Configurable timeouts, retries, and failure modes (best-effort/fail-fast)
- **Performance**: Template caching and backend client pooling

## Installation

### Docker

```bash
# Pull from Docker Hub
docker pull abtreece/confd:latest

# Or from GitHub Container Registry
docker pull ghcr.io/abtreece/confd:latest

# Run with env backend
docker run --rm \
  -e DATABASE_HOST=db.example.com \
  -v $(pwd)/conf.d:/etc/confd/conf.d:ro \
  -v $(pwd)/templates:/etc/confd/templates:ro \
  -v $(pwd)/output:/output \
  abtreece/confd:latest env --onetime
```

See [Docker documentation](docs/docker.md) for complete usage including Docker Compose and Kubernetes.

### Building from Source

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

See [Installation](docs/installation.md) for more options including binary downloads.

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
# Poll Vault every 60 seconds
confd vault --node http://127.0.0.1:8200 --interval 60 \
  --auth-type token --auth-token s.XXX

# Poll EC2 IMDS for instance metadata (on EC2 instances)
confd imds --interval 300
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

## Service Deployment

confd is production-ready with support for systemd, Docker, and Kubernetes deployments.

### Systemd Integration

Run confd as a systemd service with `Type=notify` support:

```bash
# Install service
sudo cp examples/systemd/confd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable confd
sudo systemctl start confd

# Reload configuration without restarting
sudo systemctl reload confd

# Check status
sudo systemctl status confd
```

Key features:
- **Graceful shutdown** - Wait for in-flight operations before exit
- **SIGHUP reload** - Reload templates and configuration without downtime
- **Watchdog support** - Automatic restart if service becomes unresponsive
- **Clean exits** - Proper backend connection cleanup

See [Service Deployment Guide](docs/service-deployment.md) for complete documentation including:
- systemd service configuration
- Docker deployment with signal forwarding
- Kubernetes manifests with health probes
- Monitoring and troubleshooting

### Command-Line Flags

```bash
# Graceful shutdown timeout (default: 30s)
confd --shutdown-timeout=30s etcd --watch

# Systemd integration (Linux only)
confd --systemd-notify --watchdog-interval=30s etcd --watch

# Reload configuration
kill -HUP $(pidof confd)
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
- [Docker](docs/docker.md)
- [Command Line Flags](docs/command-line-flags.md)
- [Configuration Guide](docs/configuration-guide.md)
- [Template Resources](docs/template-resources.md)
- [Template Functions](docs/templates.md)
- [Service Deployment](docs/service-deployment.md)
- [Logging](docs/logging.md)
- [Architecture](docs/architecture.md)

## Supported Backends

| Backend | Watch Mode | Polling | Authentication |
|---------|------------|---------|----------------|
| [etcd](pkg/backends/etcd/README.md) | ✅ | ✅ | Basic, TLS, Token |
| [Consul](pkg/backends/consul/README.md) | ✅ | ✅ | Basic, TLS, Token |
| [Redis](pkg/backends/redis/README.md) | ✅ | ✅ | Password |
| [Zookeeper](pkg/backends/zookeeper/README.md) | ✅ | ✅ | None |
| [Env](pkg/backends/env/README.md) | ❌ | ✅ | None |
| [File](pkg/backends/file/README.md) | ✅ | ✅ | None |
| [Vault](pkg/backends/vault/README.md) | ❌ | ✅ | Token, AppRole, App-ID, Kubernetes |
| [DynamoDB](pkg/backends/dynamodb/README.md) | ❌ | ✅ | AWS SDK |
| [SSM](pkg/backends/ssm/README.md) | ❌ | ✅ | AWS SDK |
| [Secrets Manager](pkg/backends/secretsmanager/README.md) | ❌ | ✅ | AWS SDK |
| [ACM](pkg/backends/acm/README.md) | ❌ | ✅ | AWS SDK |
| [IMDS](pkg/backends/imds/README.md) | ❌ | ✅ | AWS SDK (IMDSv2) |

## Development

See the [Development Guide](docs/development.md) for detailed instructions on setting up your environment, running tests, and adding new features.

### Quick Start

```bash
# Build
make build

# Run tests
make test

# Run linter
make lint

# Integration tests (requires backend services)
make integration
```

### Building Releases

```bash
# Snapshot build
make snapshot

# Release build
make release
```

See [Release Checklist](docs/release-checklist.md) for the full release process.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Code style and commit conventions
- Pull request process
- Adding new backends or template functions

## License

See [LICENSE](LICENSE) file.
