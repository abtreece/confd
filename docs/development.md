# Development Guide

This guide covers setting up a development environment, building, testing, and debugging confd.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Building](#building)
- [Testing](#testing)
- [Integration Tests](#integration-tests)
- [Adding a New Backend](#adding-a-new-backend)
- [Adding Template Functions](#adding-template-functions)
- [Debugging](#debugging)
- [Release Process](#release-process)

## Prerequisites

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Build and test |
| golangci-lint | latest | Linting |
| make | any | Build automation |

### Optional Tools

| Tool | Version | Purpose |
|------|---------|---------|
| goreleaser | latest | Release builds |
| Docker | latest | Integration tests, Alpine builds |
| docker-compose | latest | Running backend services for integration tests |

### Installing Prerequisites

**macOS (Homebrew):**
```bash
brew install go golangci-lint goreleaser
```

**Linux:**
```bash
# Go (check https://go.dev/dl/ for latest)
wget https://go.dev/dl/go1.25.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# goreleaser
go install github.com/goreleaser/goreleaser@latest
```

## Getting Started

### Clone and Build

```bash
git clone https://github.com/abtreece/confd.git
cd confd
make build
```

This creates `bin/confd` with the Git SHA embedded via ldflags.

### Verify Build

```bash
./bin/confd --version
# Output: confd 0.40.0-rc.1 (Git SHA: abc1234, Go Version: go1.25)
```

### Project Structure

```
confd/
├── cmd/confd/           # CLI entry point
│   ├── main.go          # Main function
│   ├── cli.go           # CLI definitions (Kong)
│   ├── config.go        # Config file loading
│   └── version.go       # Version constant
├── pkg/
│   ├── backends/        # Backend implementations
│   ├── template/        # Template processing
│   ├── memkv/           # In-memory KV store
│   ├── metrics/         # Prometheus metrics
│   ├── service/         # Service management
│   ├── log/             # Logging
│   └── util/            # Utilities
├── test/integration/    # Integration tests
├── docs/                # Documentation
└── vendor/              # Vendored dependencies
```

## Building

### Make Targets

```bash
make build        # Build to bin/confd
make install      # Install to /usr/local/bin
make clean        # Remove build artifacts
make mod          # Run go mod tidy
make lint         # Run golangci-lint
make test         # Run linter + unit tests
make integration  # Run integration tests
make snapshot     # Build snapshot release (goreleaser)
make release      # Build release (goreleaser)
```

### Manual Build

```bash
# Basic build
go build -o bin/confd ./cmd/confd

# With version info
GIT_SHA=$(git rev-parse --short HEAD)
go build -ldflags "-X main.GitSHA=${GIT_SHA}" -o bin/confd ./cmd/confd
```

### Cross-Compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/confd-linux-amd64 ./cmd/confd

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o bin/confd-linux-arm64 ./cmd/confd

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/confd.exe ./cmd/confd
```

### Alpine Linux Build

```bash
docker build -t confd_builder -f Dockerfile.build.alpine .
docker run -ti --rm -v $(pwd):/app confd_builder make build
```

## Testing

### Unit Tests

```bash
# Run all unit tests with linting
make test

# Run unit tests only (skip lint)
go test ./...

# Run tests for specific package
go test ./pkg/template/...
go test ./pkg/backends/etcd/...

# Run specific test
go test -run TestTemplateFuncs ./pkg/template/

# With verbose output
go test -v ./pkg/template/...

# With coverage
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out  # View in browser

# With race detection
go test -race ./...
```

### Test Patterns

Tests use standard Go testing patterns:

```go
func TestMyFunction(t *testing.T) {
    // Table-driven tests preferred
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"basic case", "input", "output", false},
        {"error case", "bad", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

## Integration Tests

Integration tests verify confd works correctly with real backend services.

### Test Structure

```
test/integration/
├── backends/           # Backend-specific tests
│   ├── consul/
│   ├── etcd/
│   ├── vault/
│   │   ├── approle/
│   │   ├── kv-v1/
│   │   └── kv-v2/
│   ├── redis/
│   ├── zookeeper/
│   ├── dynamodb/
│   ├── ssm/
│   ├── secretsmanager/
│   ├── acm/
│   ├── imds/
│   ├── env/
│   └── file/
├── features/           # Feature tests
│   ├── commands/       # check_cmd, reload_cmd
│   ├── failuremode/    # best-effort, fail-fast
│   ├── functions/      # Template functions
│   ├── include/        # Template includes
│   ├── per-resource-backend/
│   └── permissions/    # File mode handling
├── operations/         # Operational tests
│   ├── healthcheck/
│   ├── metrics/
│   └── signals/
├── validation/         # Error handling tests
│   └── negative/
└── shared/             # Shared resources
    ├── confdir/
    ├── data/
    └── expect/
```

### Running Integration Tests

**All integration tests:**
```bash
make integration
```

**Standalone tests (no external services):**
```bash
# Environment variables backend
test/integration/backends/env/test.sh

# File backend
test/integration/backends/file/test_yaml.sh

# Template functions
test/integration/features/functions/test.sh
```

**Backend tests (require services):**

First, start the required services (example using Docker):

```bash
# etcd
docker run -d --name etcd -p 2379:2379 \
  quay.io/coreos/etcd:v3.5.0 \
  /usr/local/bin/etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379

# Consul
docker run -d --name consul -p 8500:8500 consul:latest

# Redis
docker run -d --name redis -p 6379:6379 redis:latest

# Vault
docker run -d --name vault -p 8200:8200 \
  -e VAULT_DEV_ROOT_TOKEN_ID=root \
  vault:latest
```

Then run the tests:
```bash
test/integration/backends/etcd/test.sh
test/integration/backends/consul/test.sh
test/integration/backends/redis/test.sh
test/integration/backends/vault/kv-v2/test.sh
```

### Writing Integration Tests

Each test directory contains:
- `test.sh` - Main test script
- `confdir/conf.d/*.toml` - Template resource configs
- `confdir/templates/*.tmpl` - Template files

Test script pattern:
```bash
#!/bin/bash
set -e

# Build confd if needed
cd "$(dirname "$0")/../../../.."
make build

# Load test data into backend
etcdctl put /myapp/key "value"

# Run confd
./bin/confd etcd --onetime --confdir test/integration/backends/etcd/confdir

# Verify output
diff /tmp/confd-test-output test/integration/shared/expect/expected.conf
```

## Adding a New Backend

### 1. Create Package Structure

```bash
mkdir -p pkg/backends/mybackend
```

### 2. Implement StoreClient Interface

Create `pkg/backends/mybackend/client.go`:

```go
package mybackend

import (
    "context"
    "github.com/abtreece/confd/pkg/backends"
)

type Client struct {
    // connection fields
}

func New(config backends.Config) (*Client, error) {
    // Initialize connection
    return &Client{}, nil
}

func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
    // Fetch values from backend
    return nil, nil
}

func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string,
    waitIndex uint64, stopChan chan bool) (uint64, error) {
    // Watch for changes (return backends.ErrWatchNotSupported if not supported)
    return 0, backends.ErrWatchNotSupported
}

func (c *Client) HealthCheck(ctx context.Context) error {
    // Check backend connectivity
    return nil
}

func (c *Client) Close() error {
    // Clean up connections
    return nil
}
```

### 3. Register in Factory

Edit `pkg/backends/client.go`:

```go
import "github.com/abtreece/confd/pkg/backends/mybackend"

func New(config Config) (StoreClient, error) {
    switch config.Backend {
    // ... existing cases
    case "mybackend":
        return mybackend.New(config)
    }
}
```

### 4. Add CLI Command

Edit `cmd/confd/cli.go`:

```go
type MyBackendCmd struct {
    NodeFlags
    // backend-specific flags
}

func (m *MyBackendCmd) Run(cli *CLI) error {
    cfg := backends.Config{
        Backend: "mybackend",
        // ...
    }
    return run(cli, cfg)
}
```

Add to CLI struct:
```go
type CLI struct {
    // ...
    MyBackend MyBackendCmd `cmd:"" name:"mybackend" help:"Use MyBackend backend"`
}
```

### 5. Add Documentation

Create `pkg/backends/mybackend/README.md` with:
- Configuration options
- Authentication methods
- Usage examples
- Limitations

### 6. Add Integration Tests

Create `test/integration/backends/mybackend/`:
- `test.sh`
- `confdir/conf.d/test.toml`
- `confdir/templates/test.tmpl`

### 7. Vendor Dependencies

```bash
go mod tidy
go mod vendor
git add vendor/
```

## Adding Template Functions

### 1. Add Function

Edit `pkg/template/template_funcs.go`:

```go
func newFuncMap() map[string]interface{} {
    m := make(map[string]interface{})
    // ... existing functions
    m["myFunc"] = MyFunc
    return m
}

// MyFunc does something useful
func MyFunc(input string) string {
    // Implementation
    return input
}
```

### 2. Add Tests

Edit `pkg/template/template_funcs_test.go`:

```go
func TestMyFunc(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"hello", "HELLO"},
    }
    for _, tt := range tests {
        result := MyFunc(tt.input)
        if result != tt.expected {
            t.Errorf("MyFunc(%q) = %q, want %q", tt.input, result, tt.expected)
        }
    }
}
```

### 3. Document

Update `docs/templates.md` with function documentation and examples.

## Debugging

### Log Levels

```bash
# Debug logging
./bin/confd --log-level debug etcd --onetime

# JSON format for parsing
./bin/confd --log-level debug --log-format json etcd --onetime
```

### Noop Mode

Test changes without applying:

```bash
# Show what would change
./bin/confd --noop --diff --color etcd --onetime
```

### Preflight Checks

Verify connectivity and configuration:

```bash
# Check config files
./bin/confd --check-config etcd

# Test backend connectivity
./bin/confd --preflight etcd --node http://localhost:2379

# Validate templates with mock data
./bin/confd --validate --mock-data test-data.json etcd
```

### Keep Stage Files

Inspect rendered templates before sync:

```bash
./bin/confd --keep-stage-file etcd --onetime
ls /tmp/confd-*  # Examine staged files
```

### Using Delve

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/confd -- etcd --onetime

# Set breakpoint
(dlv) break pkg/template/resource.go:150
(dlv) continue
```

### Common Issues

**"cannot find module providing package X: import lookup disabled by -mod=vendor"**
- Run `go mod vendor` after adding dependencies
- CI uses `-mod=vendor` flag

**Template not updating:**
- Check key paths match between template and backend
- Verify prefix concatenation
- Enable debug logging to see fetched values

**Watch mode not detecting changes:**
- Confirm backend supports watch (etcd, consul, redis, zookeeper, env, file)
- Check for connection errors in logs
- Verify key prefix matches watched paths

## Release Process

See [Release Checklist](release-checklist.md) for detailed instructions.

### Quick Reference

```bash
# 1. Ensure tests pass
make test
make build

# 2. Update version in cmd/confd/version.go
# For RC: "0.40.0-rc.1"
# For release: "0.40.0"

# 3. Update docs/installation.md with new version

# 4. Commit and tag
git add cmd/confd/version.go docs/installation.md
git commit -m "chore: bump version to 0.40.0"
git tag -a v0.40.0 -m "v0.40.0"
git push origin main v0.40.0

# 5. GitHub Actions runs goreleaser automatically
```

### Local Release Build

```bash
# Snapshot (no publish)
make snapshot

# Full release (no publish)
make release

# Artifacts in dist/
ls dist/
```

## IDE Setup

### VS Code

Recommended extensions:
- Go (official)
- Go Test Explorer
- EditorConfig

`.vscode/settings.json`:
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.testFlags": ["-v"],
  "editor.formatOnSave": true
}
```

### GoLand

- Enable "Go Modules" integration
- Set golangci-lint as external linter
- Configure test runner for table-driven tests

## Further Reading

- [Architecture Guide](architecture.md) - Internal design and data flow
- [Command Line Flags](command-line-flags.md) - Complete CLI reference
- [Template Resources](template-resources.md) - Resource configuration
- [Templates](templates.md) - Template function reference
