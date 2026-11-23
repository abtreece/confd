//go:build e2e

// Package e2e provides end-to-end tests for confd, one subpackage per area.
//
// The watch subpackage uses testcontainers-go to spin up actual backend services
// (consul, etcd, redis, zookeeper) and verifies that confd's watch mode correctly
// detects and responds to backend data changes. The other subpackages (features,
// operations, validation, resilience, concurrency) start no containers.
//
// To run all E2E tests:
//
//	go test -v -tags=e2e -timeout 15m ./test/e2e/...
//
// Only the watch subpackage requires Docker to be running on the host machine.
// To run the subpackages that do not:
//
//	go test -v -tags=e2e ./test/e2e/features/ ./test/e2e/operations/ ./test/e2e/validation/ ./test/e2e/resilience/ ./test/e2e/concurrency/
package e2e
