//go:build e2e

// Package features provides end-to-end tests for confd feature-specific functionality.
//
// These tests verify confd's template processing features by executing the actual
// confd binary using exec.Command. This black-box testing approach validates:
//   - check_cmd and reload_cmd execution
//   - File permission (mode) handling
//   - Template functions
//   - Template includes
//   - Failure mode behavior
//
// The tests use the env backend which requires no external services, making them
// suitable for running in any environment.
//
// To run feature tests:
//
//	go test -v -tags=e2e ./test/e2e/features/...
//
// To run all E2E tests:
//
//	go test -v -tags=e2e -timeout 15m ./test/e2e/...
package features
