# Contributing to confd

Thank you for your interest in contributing to confd! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)
- [Adding New Features](#adding-new-features)

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all experience levels.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally
3. Set up your development environment (see [Development Guide](docs/development.md))
4. Create a branch for your changes
5. Make your changes and test them
6. Submit a pull request

## Development Setup

For detailed instructions on setting up your development environment, installing prerequisites, and running tests, see the [Development Guide](docs/development.md).

**Quick start:**

```bash
# Clone and build
git clone https://github.com/YOUR_USERNAME/confd.git
cd confd
make build

# Run tests
make test

# Run linter
make lint
```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-xyz-backend` - New features
- `fix/issue-123-description` - Bug fixes
- `docs/update-readme` - Documentation changes
- `refactor/cleanup-template-code` - Code refactoring

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>: <description>

[optional body]

[optional footer]
```

**Types:**
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring (no functional changes)
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks (dependencies, CI, etc.)

**Examples:**
```
feat: add Redis TLS support

fix: resolve template cache invalidation on SIGHUP

docs: update installation guide for v0.40.0

refactor: extract command execution into separate module

test: add integration tests for Vault KV v2
```

### What Makes a Good Contribution

- **Focused**: One logical change per PR
- **Tested**: Include tests for new functionality
- **Documented**: Update docs if behavior changes
- **Backward compatible**: Avoid breaking changes when possible

## Pull Request Process

1. **Before submitting:**
   - Run `make test` and ensure all tests pass
   - Run `make lint` and fix any issues
   - Update documentation if needed
   - Add tests for new functionality

2. **PR description should include:**
   - What the change does
   - Why it's needed
   - How it was tested
   - Any breaking changes

3. **Review process:**
   - PRs require review before merging
   - Address review feedback promptly
   - Keep PRs updated with main branch

4. **After approval:**
   - Squash commits if requested
   - Maintainer will merge the PR

**Important:** Always target `abtreece/confd` repository, not the upstream `kelseyhightower/confd`.

## Code Style

### Go Conventions

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (enforced by CI)
- Run `golangci-lint` before submitting

### Project-Specific Guidelines

- **Error handling**: Return errors, don't panic
- **Logging**: Use the `pkg/log` package for structured logging
- **Context**: Pass `context.Context` for cancellation and timeouts
- **Testing**: Table-driven tests preferred

### File Organization

```
cmd/confd/          # CLI entry point
pkg/backends/       # Backend implementations
pkg/template/       # Template processing
pkg/metrics/        # Prometheus metrics
pkg/service/        # Service management (shutdown, reload)
pkg/log/            # Logging
pkg/util/           # Utilities
```

## Testing

### Running Tests

```bash
# Unit tests with linting
make test

# Unit tests only
go test ./...

# Specific package
go test ./pkg/template/...

# With coverage
go test ./... -coverprofile=coverage.out

# Integration tests (requires backend services)
make integration
```

### Writing Tests

- Place tests in `*_test.go` files alongside the code
- Use table-driven tests for multiple cases
- Mock external dependencies
- Integration tests go in `test/integration/`

See the [Development Guide](docs/development.md#testing) for detailed testing instructions.

## Adding New Features

### Adding a New Backend

1. Create package under `pkg/backends/<name>/`
2. Implement the `StoreClient` interface
3. Add to factory in `pkg/backends/client.go`
4. Add CLI command in `cmd/confd/cli.go`
5. Create `pkg/backends/<name>/README.md`
6. Add integration tests in `test/integration/backends/<name>/`
7. Update documentation

See [Architecture: Extension Points](docs/architecture.md#extension-points) for details.

### Adding Template Functions

1. Add function to `pkg/template/template_funcs.go`
2. Add tests in `pkg/template/template_funcs_test.go`
3. Document in `docs/templates.md`

### Adding CLI Flags

1. Add field to appropriate struct in `cmd/confd/cli.go`
2. Update `docs/command-line-flags.md`
3. Update `docs/configuration-guide.md` if config file option

## Questions?

- Check existing [issues](https://github.com/abtreece/confd/issues)
- Open a new issue for questions or discussion
- Review the [Architecture Guide](docs/architecture.md) for design context

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
