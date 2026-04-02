# Documentation

## Getting Started

- [Installation](installation.md) — binary downloads, building from source, package managers
- [Quick Start Guide](quick-start-guide.md) — first run with env and file backends
- [Docker](docker.md) — Docker images, Docker Compose, Kubernetes

## Configuration

- [Configuration Guide](configuration-guide.md) — config file format, hierarchy, all options
- [Command-Line Flags](command-line-flags.md) — global flags, backend-specific flags, environment variables
- [Template Resources](template-resources.md) — template resource TOML files (`conf.d/*.toml`)
- [Template Functions](templates.md) — Go template functions available in templates

## Backends

Each backend has its own README with authentication, options, and examples:

| Backend | Guide | Watch Support |
|---------|-------|---------------|
| etcd | [README](../pkg/backends/etcd/README.md) | Yes |
| Consul | [README](../pkg/backends/consul/README.md) | Yes |
| Vault | [README](../pkg/backends/vault/README.md) | No |
| Redis | [README](../pkg/backends/redis/README.md) | Yes |
| Zookeeper | [README](../pkg/backends/zookeeper/README.md) | Yes |
| File | [README](../pkg/backends/file/README.md) | Yes |
| Environment Variables | [README](../pkg/backends/env/README.md) | No |
| DynamoDB | [README](../pkg/backends/dynamodb/README.md) | No |
| SSM Parameter Store | [README](../pkg/backends/ssm/README.md) | No |
| Secrets Manager | [README](../pkg/backends/secretsmanager/README.md) | No |
| ACM | [README](../pkg/backends/acm/README.md) | No |
| EC2 IMDS | [README](../pkg/backends/imds/README.md) | No |

- [Multi-Backend Architectures](multi-backend.md) — using multiple backends simultaneously (e.g., Consul for config + Vault for secrets)
- [DNS SRV Records](dns-srv-records.md) — backend node discovery via DNS SRV

## Operating

- [Service Deployment](service-deployment.md) — systemd, Docker, Kubernetes deployment patterns
- [Logging](logging.md) — log levels, JSON format, structured logging
- [Noop Mode](noop-mode.md) — dry runs, diff output, safe testing

## Architecture

- [Architecture](architecture.md) — package structure, execution flow, design patterns

## Development

- [Development Guide](development.md) — building, testing, debugging, adding backends
- [Release Checklist](release-checklist.md) — versioning, tagging, goreleaser workflow
- [Contributing](../CONTRIBUTING.md) — code style, commit conventions, PR process
