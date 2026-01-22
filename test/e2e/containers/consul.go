//go:build e2e

package containers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	consulImage = "hashicorp/consul:1.15"
	consulPort  = "8500/tcp"
)

// ConsulContainer manages a Consul container for E2E tests.
type ConsulContainer struct {
	container testcontainers.Container
	client    *api.Client
	endpoint  string
}

// NewConsulContainer creates a new ConsulContainer instance.
func NewConsulContainer() *ConsulContainer {
	return &ConsulContainer{}
}

// Start starts the Consul container and initializes the client.
func (c *ConsulContainer) Start(ctx context.Context) error {
	req := testcontainers.ContainerRequest{
		Image:        consulImage,
		ExposedPorts: []string{consulPort},
		Cmd:          []string{"agent", "-dev", "-client=0.0.0.0"},
		WaitingFor:   wait.ForHTTP("/v1/status/leader").WithPort("8500/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start consul container: %w", err)
	}
	c.container = container

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "8500")
	if err != nil {
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host: %w", err)
	}

	c.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Create Consul client
	config := api.DefaultConfig()
	config.Address = c.endpoint
	c.client, err = api.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create consul client: %w", err)
	}

	// Verify connection
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opts := &api.QueryOptions{}
	opts = opts.WithContext(healthCtx)
	_, _, err = c.client.KV().List("", opts)
	if err != nil {
		return fmt.Errorf("failed to verify consul connection: %w", err)
	}

	return nil
}

// Stop stops the Consul container.
func (c *ConsulContainer) Stop(ctx context.Context) error {
	// Consul API client doesn't have a Close method
	c.client = nil

	if c.container != nil {
		if err := c.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate consul container: %w", err)
		}
		c.container = nil
	}

	return nil
}

// Endpoint returns the Consul connection endpoint.
func (c *ConsulContainer) Endpoint(ctx context.Context) (string, error) {
	if c.endpoint == "" {
		return "", fmt.Errorf("consul container not started")
	}
	return c.endpoint, nil
}

// SetValue sets a key-value pair in Consul.
// Keys should be provided with leading slash (e.g., "/test/value")
// which will be converted to Consul format (e.g., "test/value").
func (c *ConsulContainer) SetValue(ctx context.Context, key, value string) error {
	if c.client == nil {
		return fmt.Errorf("consul client not initialized")
	}

	// Remove leading slash for Consul
	consulKey := strings.TrimPrefix(key, "/")

	opts := &api.WriteOptions{}
	opts = opts.WithContext(ctx)

	_, err := c.client.KV().Put(&api.KVPair{
		Key:   consulKey,
		Value: []byte(value),
	}, opts)
	if err != nil {
		return fmt.Errorf("failed to set value for key %s: %w", key, err)
	}

	return nil
}

// DeleteValue deletes a key from Consul.
func (c *ConsulContainer) DeleteValue(ctx context.Context, key string) error {
	if c.client == nil {
		return fmt.Errorf("consul client not initialized")
	}

	// Remove leading slash for Consul
	consulKey := strings.TrimPrefix(key, "/")

	opts := &api.WriteOptions{}
	opts = opts.WithContext(ctx)

	_, err := c.client.KV().Delete(consulKey, opts)
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// BackendName returns "consul".
func (c *ConsulContainer) BackendName() string {
	return "consul"
}
