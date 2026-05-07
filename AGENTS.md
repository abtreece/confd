# Repository Guidelines

## Project Structure & Module Organization

`confd` is a Go configuration management tool. The CLI entry point lives in `cmd/confd/`. Core packages are under `pkg/`: backend clients in `pkg/backends/`, template rendering in `pkg/template/`, logging in `pkg/log/`, metrics in `pkg/metrics/`, service helpers in `pkg/service/`, and utilities in `pkg/util/`. Unit tests sit beside source files as `*_test.go`. Integration tests live in `test/integration/`, with backend scripts under `test/integration/backends/` and shared fixtures under `test/integration/shared/`. User-facing docs are in `docs/`, examples in `examples/`, Docker assets in `docker/`, and package files in `packaging/`.

## Build, Test, and Development Commands

- `make build`: builds `bin/confd` with version and Git SHA ldflags.
- `make test`: runs Go unit tests for all non-vendor packages.
- `make lint`: runs `golangci-lint run ./...`.
- `make integration`: runs all `test/integration/**/test.sh` scripts; backend services must be available.
- `make mod`: runs `go mod tidy`.
- `make clean`: removes build artifacts from `bin/`.
- `make snapshot`: creates a local GoReleaser snapshot.

After adding dependencies, run `go mod tidy` and `go mod vendor`; CI uses vendored modules.

## Coding Style & Naming Conventions

Use standard Go formatting (`gofmt`) and idioms. Package names should be short, lowercase, and descriptive. Exported identifiers require useful comments when they are part of package API. Prefer table-driven tests for multi-case behavior. Return errors instead of panicking, pass `context.Context` where cancellation or timeouts matter, and use `pkg/log` for structured logging. The configured linters are `errorlint` and `wrapcheck`, so wrap returned errors with context.

## Testing Guidelines

Use Go's standard `testing` package. Name tests `TestXxx`, benchmarks `BenchmarkXxx`, and keep unit tests next to the package under test. Run focused tests with `go test -run TestFunctionName ./pkg/template/`. Use `go test ./... -race -coverprofile=coverage.out -covermode=atomic` for broader validation. Integration tests should use the existing categories and shared expected-output checks.

## Commit & Pull Request Guidelines

Use Conventional Commits, matching project history: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, or `chore:`. Keep each PR focused on one logical change. Before opening a PR, run `make test` and `make lint`; run integration tests when backend behavior changes. PR descriptions should explain what changed, why it changed, how it was tested, linked issues, and any breaking changes. Always target `abtreece/confd`, not the abandoned upstream `kelseyhightower/confd`.

## Agent-Specific Instructions

Follow `CLAUDE.md` for repository-specific guidance. Do not add AI attribution or `Co-Authored-By` lines to commits. Preserve user changes in the worktree and avoid unrelated refactors.
