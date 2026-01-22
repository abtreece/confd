//go:build e2e

// Package e2e provides end-to-end tests for confd using real backend containers.
//
// These tests use testcontainers-go to spin up actual backend services (etcd, etc.)
// and verify that confd's watch mode correctly detects and responds to backend
// data changes.
//
// To run E2E tests:
//
//	go test -v -tags=e2e -timeout 15m ./test/e2e/...
//
// E2E tests require Docker to be running on the host machine.
package e2e
