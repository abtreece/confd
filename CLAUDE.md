# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

confd is a lightweight configuration management tool that keeps local configuration files up-to-date using data stored in backends (etcd, Consul, Vault, DynamoDB, Redis, Zookeeper, AWS SSM, environment variables, or files). It processes Go text/templates and can reload applications when config changes.

This is a divergent fork of kelseyhightower/confd. The upstream repository is effectively abandoned.

## Git Workflow

**IMPORTANT**: Never create PRs against the upstream repository (kelseyhightower/confd). Always target `abtreece/confd`:

```bash
gh pr create --repo abtreece/confd ...
```

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

- **cmd/confd/** - Entry point, config parsing, main loop with signal handling
- **pkg/backends/** - Backend abstraction layer with `StoreClient` interface
  - Each backend (consul/, etcd/, vault/, redis/, zookeeper/, dynamodb/, ssm/, env/, file/) implements `GetValues()` and `WatchPrefix()`
  - `client.go` contains factory function `New()` that creates appropriate backend
- **pkg/template/** - Template processing
  - `processor.go` - `Processor` interface with `IntervalProcessor` (polling) and `WatchProcessor` (continuous)
  - `resource.go` - `TemplateResource` parses TOML configs, renders templates, handles check/reload commands
  - `template_funcs.go` - Custom template functions (get, getAll, base, etc.)
- **pkg/log/** - Logging wrapper using logrus
- **pkg/util/** - File operations, MD5 comparison utilities

### Execution Flow

1. Main parses flags and config (TOML file + env vars + flags)
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

## Supported Backends

consul, etcd, vault, dynamodb, redis, zookeeper, ssm, env, file

Watch mode is not supported for DynamoDB and SSM (polling only).
