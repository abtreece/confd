//go:build e2e

package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	etcdImage      = "quay.io/coreos/etcd:v3.5.13"
	etcdClientPort = "2379/tcp"
)

// EtcdContainer manages an etcd container for E2E tests.
type EtcdContainer struct {
	container testcontainers.Container
	client    *clientv3.Client
	endpoint  string
}

// NewEtcdContainer creates a new EtcdContainer instance.
func NewEtcdContainer() *EtcdContainer {
	return &EtcdContainer{}
}

// Start starts the etcd container and initializes the client.
func (e *EtcdContainer) Start(ctx context.Context) error {
	req := testcontainers.ContainerRequest{
		Image:        etcdImage,
		ExposedPorts: []string{etcdClientPort},
		Env: map[string]string{
			"ETCD_NAME":                        "etcd0",
			"ETCD_ADVERTISE_CLIENT_URLS":       "http://0.0.0.0:2379",
			"ETCD_LISTEN_CLIENT_URLS":          "http://0.0.0.0:2379",
			"ETCD_INITIAL_ADVERTISE_PEER_URLS": "http://0.0.0.0:2380",
			"ETCD_LISTEN_PEER_URLS":            "http://0.0.0.0:2380",
			"ETCD_INITIAL_CLUSTER":             "etcd0=http://0.0.0.0:2380",
			"ETCD_INITIAL_CLUSTER_STATE":       "new",
			"ETCD_INITIAL_CLUSTER_TOKEN":       "etcd-cluster-e2e",
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("2379/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start etcd container: %w", err)
	}
	e.container = container

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "2379")
	if err != nil {
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host: %w", err)
	}

	e.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Create etcd client
	e.client, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{e.endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Verify connection
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = e.client.Status(healthCtx, e.endpoint)
	if err != nil {
		return fmt.Errorf("failed to verify etcd connection: %w", err)
	}

	return nil
}

// Stop stops the etcd container and closes the client.
func (e *EtcdContainer) Stop(ctx context.Context) error {
	if e.client != nil {
		if err := e.client.Close(); err != nil {
			return fmt.Errorf("failed to close etcd client: %w", err)
		}
		e.client = nil
	}

	if e.container != nil {
		if err := e.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate etcd container: %w", err)
		}
		e.container = nil
	}

	return nil
}

// Restart stops and restarts the etcd container, reinitializing the client.
// Note: etcd data is ephemeral and will be lost on restart.
func (e *EtcdContainer) Restart(ctx context.Context) error {
	if e.container == nil {
		return fmt.Errorf("etcd container not started")
	}

	// Close the existing client
	if e.client != nil {
		if err := e.client.Close(); err != nil {
			return fmt.Errorf("failed to close etcd client: %w", err)
		}
		e.client = nil
	}

	// Stop the container (don't terminate)
	timeout := 10 * time.Second
	if err := e.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop etcd container: %w", err)
	}

	// Start the container again
	if err := e.container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start etcd container: %w", err)
	}

	// Re-fetch the endpoint in case port mapping changed
	mappedPort, err := e.container.MappedPort(ctx, "2379")
	if err != nil {
		return fmt.Errorf("failed to get mapped port after restart: %w", err)
	}
	host, err := e.container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host after restart: %w", err)
	}
	e.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Wait for etcd to be ready - the container wait strategy should handle this,
	// but we add extra time for port mapping to stabilize
	time.Sleep(3 * time.Second)

	// Recreate etcd client with fresh connection
	for i := 0; i < 15; i++ {
		e.client, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{e.endpoint},
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Verify connection
		healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err = e.client.Status(healthCtx, e.endpoint)
		cancel()
		if err == nil {
			return nil
		}

		// Close failed client and retry
		e.client.Close()
		e.client = nil
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("failed to verify etcd connection after restart: %w", err)
}

// Endpoint returns the etcd connection endpoint.
func (e *EtcdContainer) Endpoint(ctx context.Context) (string, error) {
	if e.endpoint == "" {
		return "", fmt.Errorf("etcd container not started")
	}
	return e.endpoint, nil
}

// SetValue sets a key-value pair in etcd.
func (e *EtcdContainer) SetValue(ctx context.Context, key, value string) error {
	if e.client == nil {
		return fmt.Errorf("etcd client not initialized")
	}

	putCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := e.client.Put(putCtx, key, value)
	if err != nil {
		return fmt.Errorf("failed to set value for key %s: %w", key, err)
	}

	return nil
}

// DeleteValue deletes a key from etcd.
func (e *EtcdContainer) DeleteValue(ctx context.Context, key string) error {
	if e.client == nil {
		return fmt.Errorf("etcd client not initialized")
	}

	delCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := e.client.Delete(delCtx, key)
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// BackendName returns "etcd".
func (e *EtcdContainer) BackendName() string {
	return "etcd"
}
