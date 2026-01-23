//go:build e2e

package containers

import "context"

// BackendContainer defines the interface for backend containers used in E2E tests.
// Each backend implementation (etcd, Consul, Redis, etc.) should implement this
// interface to provide a consistent way to manage test containers.
type BackendContainer interface {
	// Start starts the container and waits for it to be ready.
	Start(ctx context.Context) error

	// Stop stops and removes the container.
	Stop(ctx context.Context) error

	// Restart stops and restarts the container, reinitializing the client connection.
	// This is used to test reconnection behavior of the watch processor.
	Restart(ctx context.Context) error

	// Endpoint returns the connection endpoint for the backend.
	// For example, "localhost:2379" for etcd.
	Endpoint(ctx context.Context) (string, error)

	// SetValue sets a key-value pair in the backend.
	SetValue(ctx context.Context, key, value string) error

	// DeleteValue deletes a key from the backend.
	DeleteValue(ctx context.Context, key string) error

	// BackendName returns the backend type name (e.g., "etcd", "consul").
	BackendName() string
}
