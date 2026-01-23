//go:build e2e

// Package operations provides end-to-end tests for confd operational features.
//
// These tests verify confd's runtime behavior by executing the actual confd binary
// using exec.Command. This black-box testing approach validates:
//   - Health check endpoints (/health, /ready, /ready/detailed)
//   - Prometheus metrics endpoint (/metrics)
//   - Signal handling (SIGHUP, SIGTERM)
//
// The tests use the env backend which requires no external services, making them
// suitable for running in any environment.
//
// To run operations tests:
//
//	go test -v -tags=e2e ./test/e2e/operations/...
//
// To run all E2E tests:
//
//	go test -v -tags=e2e -timeout 15m ./test/e2e/...
package operations
