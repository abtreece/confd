# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

confd is a lightweight configuration management tool that keeps local configuration files up-to-date using data stored in backends (etcd, Consul, Vault, DynamoDB, Redis, Zookeeper, AWS SSM, AWS ACM, AWS Secrets Manager, AWS EC2 IMDS, environment variables, or files). It processes Go text/templates and can reload applications when config changes.

This is a divergent fork of kelseyhightower/confd. The upstream repository is effectively abandoned.

## Git Workflow

**IMPORTANT**: Never create PRs against the upstream repository (kelseyhightower/confd). Always target `abtreece/confd`:

```bash
gh pr create --repo abtreece/confd ...
```

## Git Commits

All commits should be authored by the repository owner. Do not include Co-Authored-By lines or AI attribution in commit messages.

## Build Commands

```bash
make build          # Build to bin/confd (includes Git SHA via ldflags)
make test           # Run unit tests
make integration    # Run integration tests (requires backend services)
make mod            # Run go mod tidy
make clean          # Remove build artifacts
make snapshot       # Build snapshot release via goreleaser
make release        # Build release via goreleaser
```

## Dependency Management

**CRITICAL**: CI builds use `-mod=vendor` which requires all dependencies to be vendored.

After adding new dependencies to `go.mod`, always run:

```bash
go mod vendor
git add vendor/
```

This ensures CI integration tests can build successfully. Without vendoring, CI will fail with:
```
cannot find module providing package X: import lookup disabled by -mod=vendor
```

## Running Tests

```bash
# Unit tests
go test ./...

# Unit tests with coverage
go test ./... -race -coverprofile=coverage.out -covermode=atomic

# Run a single test
go test -run TestFunctionName ./pkg/template/

# Integration tests require running backend services (see .github/workflows/integration-tests.yml)
```

## Architecture

### Package Structure

- **cmd/confd/** - Entry point using Kong for CLI parsing, config handling, main loop
  - `cli.go` - CLI argument definitions and parsing with alecthomas/kong
  - `config.go` - Configuration loading from file, env vars, and flags
  - `main.go` - Entry point
- **pkg/backends/** - Backend abstraction layer with `StoreClient` interface
  - Each backend implements `GetValues()`, `WatchPrefix()`, and `HealthCheck()`
  - `client.go` contains factory function `New()` that creates appropriate backend
  - Backends: acm/, consul/, dynamodb/, env/, etcd/, file/, imds/, redis/, secretsmanager/, ssm/, vault/, zookeeper/
- **pkg/template/** - Template processing (follows Single Responsibility Principle)
  - `processor.go` - `Processor` interface with `IntervalProcessor` (polling) and `WatchProcessor` (continuous)
  - `resource.go` - `TemplateResource` core processing: TOML parsing, template rendering, check/reload commands
  - `template_funcs.go` - Custom template functions (string, data, network, encoding, math)
  - `template_cache.go` - Compiled template caching with mtime-based invalidation
  - `client_cache.go` - Backend client caching to prevent duplicate connections
  - `include.go` - Template include function with cycle detection and max depth enforcement
  - `validate.go` - Configuration and template validation (syntax, required fields, backend types)
  - `preflight.go` - Pre-flight checks: backend connectivity, template loading, key accessibility
  - `error_aggregator.go` - Error aggregation and failure mode handling (best-effort vs fail-fast)
- **pkg/memkv/** - In-memory key-value store used by template processing
- **pkg/log/** - Logging wrapper using slog (Go's standard library structured logging)
- **pkg/metrics/** - Prometheus metrics instrumentation for observability
- **pkg/util/** - File operations, MD5 comparison utilities

### Execution Flow

1. Main parses CLI args with Kong and loads config (TOML file + env vars + flags)
2. Creates `StoreClient` via `backends.New()`
3. Selects processor: `WatchProcessor` for `--watch`, `IntervalProcessor` for `--interval`
4. Processor loads template resources from conf.d/*.toml
5. Each resource fetches keys from backend, renders template, compares with dest file
6. If changed: write file, run check_cmd, run reload_cmd

### Configuration Hierarchy

1. Defaults
2. Config file (/etc/confd/confd.toml)
3. Environment variables (CONFD_* prefix)
4. Command-line flags

### Default Paths

- Config file: /etc/confd/confd.toml
- Template resources: /etc/confd/conf.d/*.toml
- Templates: /etc/confd/templates/

## Key Patterns

- **Factory Pattern**: `backends.New()` creates appropriate `StoreClient` based on backend type
- **Strategy Pattern**: `Processor` interface allows swapping between watch/interval modes
- **Template Functions**: Custom Go text/template functions in `template_funcs.go`
- **Caching**: Template cache (`template_cache.go`) and client cache (`client_cache.go`) for performance
- **Single Responsibility**: Template package split into focused modules (resource, validation, caching, includes)

## Supported Backends

acm, consul, dynamodb, env, etcd, file, imds, redis, secretsmanager, ssm, vault, zookeeper

**Watch mode supported**: consul, env, etcd, file, redis, zookeeper

**Polling only (no watch)**: acm, dynamodb, imds, secretsmanager, ssm, vault

### IMDS Backend

Retrieves metadata from AWS EC2 Instance Metadata Service version 2 (IMDSv2).

**Usage**:
```bash
confd imds --interval 300 --imds-cache-ttl 60s
```

**Metadata Categories**:
- `/meta-data/instance-id`, `/meta-data/instance-type`, `/meta-data/ami-id`
- `/meta-data/tags/instance/Name`, `/meta-data/tags/instance/Environment`
- `/meta-data/placement/availability-zone`, `/meta-data/placement/region`
- `/meta-data/local-ipv4`, `/meta-data/public-ipv4`, `/meta-data/mac`
- `/meta-data/network/interfaces/macs/{mac}/...`
- `/meta-data/iam/security-credentials/{role-name}`
- `/dynamic/instance-identity/document`
- `/user-data`

**Example template resource**:
```toml
[template]
src = "instance.tmpl"
dest = "/etc/app/instance.conf"
keys = [
    "/meta-data/instance-id",
    "/meta-data/tags/instance/Name",
    "/meta-data/placement/availability-zone"
]
```

**Configuration**:
- `--imds-cache-ttl`: Cache duration (default: 60s)
- Environment: `IMDS_ENDPOINT` for custom endpoint (testing only)

**Features**:
- Automatic IMDSv2 token management via AWS SDK
- In-memory caching reduces API calls
- No AWS credentials required (network-based security)
- Only available on EC2 instances

**Limitations**:
- Polling only (no watch mode)
- Requires EC2 instance with IMDSv2 enabled

## Template Functions

Available functions in templates (defined in `pkg/template/template_funcs.go`):

**String functions**: `base`, `dir`, `split`, `join`, `toUpper`, `toLower`, `contains`, `replace`, `trimSuffix`

**Data functions**: `json`, `jsonArray`, `map`, `getenv`, `hostname`, `datetime`

**Network functions**: `lookupIP`, `lookupIPV4`, `lookupIPV6`, `lookupSRV`, `lookupIfaceIPV4`, `lookupIfaceIPV6`

**Encoding functions**: `base64Encode`, `base64Decode`

**Utility functions**: `fileExists`, `parseBool`, `atoi`, `reverse`, `sortByLength`, `sortKVByLength`, `seq`

**Math functions**: `add`, `sub`, `div`, `mod`, `mul`

**Include function**: `include` - Include other templates with cycle detection (max depth: 10)

## Metrics and Observability

confd provides comprehensive Prometheus metrics when enabled via `--metrics-addr`. The metrics endpoint also exposes health and readiness checks.

### Enabling Metrics

```bash
confd etcd --metrics-addr :9100
```

Or via environment variable:
```bash
export CONFD_METRICS_ADDR=:9100
confd etcd
```

### Endpoints

- `/metrics` - Prometheus metrics
- `/health` - Basic health check (calls backend HealthCheck)
- `/ready` - Readiness check (backend connectivity)
- `/ready/detailed` - Detailed readiness with full diagnostics

### Available Metrics

**Backend Metrics**:
- `confd_backend_request_duration_seconds` - Histogram of backend request durations
- `confd_backend_request_total` - Counter of backend requests by operation
- `confd_backend_errors_total` - Counter of backend errors
- `confd_backend_healthy` - Gauge indicating backend health (1=healthy, 0=unhealthy)

**Template Metrics**:
- `confd_template_process_duration_seconds` - Histogram of template processing durations
- `confd_template_process_total` - Counter of template processing attempts
- `confd_template_cache_hits_total` - Counter of template cache hits
- `confd_template_cache_misses_total` - Counter of template cache misses
- `confd_templates_loaded` - Gauge of currently loaded templates
- `confd_watched_keys` - Gauge of keys being watched

**Batch Processing Metrics**:
- `confd_batch_process_total` - Counter of batch processing runs
- `confd_batch_process_failed` - Counter of failed batch runs
- `confd_batch_process_templates_succeeded` - Counter of templates succeeded in batch
- `confd_batch_process_templates_failed` - Counter of templates failed in batch

**Command Metrics**:
- `confd_command_duration_seconds` - Histogram of check/reload command durations
- `confd_command_total` - Counter of commands executed
- `confd_command_exit_codes` - Counter of command exit codes

**File Sync Metrics**:
- `confd_file_sync_total` - Counter of file sync operations by status
- `confd_file_changed_total` - Counter of files changed

### Logging

confd uses Go's `log/slog` for structured logging with support for both text and JSON formats:

```bash
# Text format (default)
confd etcd --log-level info

# JSON format for log indexing
confd etcd --log-format json --log-level debug
```

Structured logging provides timing metrics and contextual information for debugging and analysis.

## Configuration Timeouts and Retries

confd provides configurable timeouts and retry behavior for production resilience:

### Timeout Flags

- `--backend-timeout` - Overall timeout for backend operations (default: 30s)
- `--check-cmd-timeout` - Timeout for check commands (default: 30s)
- `--reload-cmd-timeout` - Timeout for reload commands (default: 60s)
- `--dial-timeout` - Connection timeout for backends (default: 5s)
- `--read-timeout` - Read timeout for backend operations (default: 1s)
- `--write-timeout` - Write timeout for backend operations (default: 1s)
- `--preflight-timeout` - Timeout for preflight checks (default: 10s)
- `--watch-error-backoff` - Backoff after watch errors (default: 2s)

### Retry Configuration

- `--retry-max-attempts` - Maximum retry attempts (default: 3)
- `--retry-base-delay` - Initial backoff delay (default: 100ms)
- `--retry-max-delay` - Maximum backoff delay (default: 5s)

### Failure Modes

confd supports two error handling modes via `--failure-mode`:

- `best-effort` (default) - Continue processing remaining templates when one fails
- `fail-fast` - Stop all processing on first template error

## Watch Mode Features

### Debouncing

Global debounce waits for changes to settle before processing:

```bash
confd --watch --debounce 2s etcd
```

### Batch Processing

Collect changes across all templates and process together:

```bash
confd --watch --batch-interval 5s etcd
```

**Difference**:
- `--debounce`: Per-template, resets timer on each change
- `--batch-interval`: Global, fixed interval for all templates
