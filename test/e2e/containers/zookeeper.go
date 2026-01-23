//go:build e2e

package containers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	zookeeperImage = "zookeeper:3.9"
	zookeeperPort  = "2181/tcp"
)

// ZookeeperContainer manages a Zookeeper container for E2E tests.
type ZookeeperContainer struct {
	container testcontainers.Container
	conn      *zk.Conn
	endpoint  string
}

// NewZookeeperContainer creates a new ZookeeperContainer instance.
func NewZookeeperContainer() *ZookeeperContainer {
	return &ZookeeperContainer{}
}

// Start starts the Zookeeper container and initializes the client.
func (z *ZookeeperContainer) Start(ctx context.Context) error {
	req := testcontainers.ContainerRequest{
		Image:        zookeeperImage,
		ExposedPorts: []string{zookeeperPort},
		Env: map[string]string{
			"ZOO_STANDALONE_ENABLED": "true",
		},
		WaitingFor: wait.ForLog("binding to port").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start zookeeper container: %w", err)
	}
	z.container = container

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "2181")
	if err != nil {
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host: %w", err)
	}

	z.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Create Zookeeper connection with retries
	var conn *zk.Conn
	for i := 0; i < 10; i++ {
		conn, _, err = zk.Connect([]string{z.endpoint}, 5*time.Second)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to zookeeper: %w", err)
	}
	z.conn = conn

	// Verify connection by checking root node
	exists, _, err := z.conn.Exists("/")
	if err != nil {
		z.conn.Close()
		return fmt.Errorf("failed to verify zookeeper connection: %w", err)
	}
	if !exists {
		z.conn.Close()
		return fmt.Errorf("zookeeper root node does not exist")
	}

	return nil
}

// Stop stops the Zookeeper container and closes the connection.
func (z *ZookeeperContainer) Stop(ctx context.Context) error {
	if z.conn != nil {
		z.conn.Close()
		z.conn = nil
	}

	if z.container != nil {
		if err := z.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate zookeeper container: %w", err)
		}
		z.container = nil
	}

	return nil
}

// Restart stops and restarts the Zookeeper container, reinitializing the connection.
// Note: Zookeeper data is ephemeral and will be lost on restart.
func (z *ZookeeperContainer) Restart(ctx context.Context) error {
	if z.container == nil {
		return fmt.Errorf("zookeeper container not started")
	}

	// Close existing connection
	if z.conn != nil {
		z.conn.Close()
		z.conn = nil
	}

	// Stop the container (don't terminate)
	timeout := 10 * time.Second
	if err := z.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop zookeeper container: %w", err)
	}

	// Start the container again
	if err := z.container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start zookeeper container: %w", err)
	}

	// Re-fetch the endpoint in case port mapping changed
	mappedPort, err := z.container.MappedPort(ctx, "2181")
	if err != nil {
		return fmt.Errorf("failed to get mapped port after restart: %w", err)
	}
	host, err := z.container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host after restart: %w", err)
	}
	z.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Recreate connection with retries (retry loop handles waiting)
	for i := 0; i < 15; i++ {
		z.conn, _, err = zk.Connect([]string{z.endpoint}, 5*time.Second)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Verify connection
		exists, _, err := z.conn.Exists("/")
		if err == nil && exists {
			return nil
		}
		if z.conn != nil {
			z.conn.Close()
			z.conn = nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("failed to verify zookeeper connection after restart: %w", err)
}

// Endpoint returns the Zookeeper connection endpoint.
func (z *ZookeeperContainer) Endpoint(ctx context.Context) (string, error) {
	if z.endpoint == "" {
		return "", fmt.Errorf("zookeeper container not started")
	}
	return z.endpoint, nil
}

// SetValue sets a key-value pair in Zookeeper.
// Keys should be provided with leading slash (e.g., "/test/value").
// Creates parent znodes if they don't exist.
func (z *ZookeeperContainer) SetValue(ctx context.Context, key, value string) error {
	if z.conn == nil {
		return fmt.Errorf("zookeeper connection not initialized")
	}

	// Ensure the key starts with /
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	// Create parent znodes if needed
	if err := z.createParentNodes(key); err != nil {
		return fmt.Errorf("failed to create parent nodes for %s: %w", key, err)
	}

	// Check if znode exists
	exists, stat, err := z.conn.Exists(key)
	if err != nil {
		return fmt.Errorf("failed to check if key exists %s: %w", key, err)
	}

	if exists {
		// Update existing znode
		_, err = z.conn.Set(key, []byte(value), stat.Version)
		if err != nil {
			return fmt.Errorf("failed to update value for key %s: %w", key, err)
		}
	} else {
		// Create new znode
		_, err = z.conn.Create(key, []byte(value), 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			return fmt.Errorf("failed to create key %s: %w", key, err)
		}
	}

	return nil
}

// createParentNodes creates all parent znodes for a given path.
func (z *ZookeeperContainer) createParentNodes(path string) error {
	parts := strings.Split(path, "/")
	current := ""

	// Skip empty string at start and the final node
	for i := 1; i < len(parts)-1; i++ {
		current = current + "/" + parts[i]
		exists, _, err := z.conn.Exists(current)
		if err != nil {
			return fmt.Errorf("failed to check parent node %s: %w", current, err)
		}
		if !exists {
			_, err = z.conn.Create(current, []byte{}, 0, zk.WorldACL(zk.PermAll))
			if err != nil && err != zk.ErrNodeExists {
				return fmt.Errorf("failed to create parent node %s: %w", current, err)
			}
		}
	}
	return nil
}

// DeleteValue deletes a key from Zookeeper.
func (z *ZookeeperContainer) DeleteValue(ctx context.Context, key string) error {
	if z.conn == nil {
		return fmt.Errorf("zookeeper connection not initialized")
	}

	// Ensure the key starts with /
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	// Get the current version
	exists, stat, err := z.conn.Exists(key)
	if err != nil {
		return fmt.Errorf("failed to check if key exists %s: %w", key, err)
	}

	if !exists {
		// Key doesn't exist, nothing to delete
		return nil
	}

	err = z.conn.Delete(key, stat.Version)
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// BackendName returns "zookeeper".
func (z *ZookeeperContainer) BackendName() string {
	return "zookeeper"
}
