//go:build e2e

// Package validation contains E2E tests for confd error handling and validation.
// These tests verify that confd properly rejects invalid configurations and
// handles error conditions gracefully.
//
// Test categories:
//   - Invalid backend types
//   - Malformed TOML configurations
//   - Template syntax errors
//   - Missing template files
//   - Non-existent keys
//   - Invalid file mode formats
//   - Empty configuration directories
//
// All tests use the env backend which requires no external services.
package validation
