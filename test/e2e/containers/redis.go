//go:build e2e

package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	redisImage = "redis:7-alpine"
	redisPort  = "6379/tcp"
)

// RedisContainer manages a Redis container for E2E tests.
type RedisContainer struct {
	container testcontainers.Container
	client    *redis.Client
	endpoint  string
}

// NewRedisContainer creates a new RedisContainer instance.
func NewRedisContainer() *RedisContainer {
	return &RedisContainer{}
}

// Start starts the Redis container and initializes the client.
func (r *RedisContainer) Start(ctx context.Context) error {
	// Start Redis with keyspace notifications enabled for watch mode
	// KEA = Keyspace events for all commands (K=keyspace, E=keyevent, A=all)
	req := testcontainers.ContainerRequest{
		Image:        redisImage,
		ExposedPorts: []string{redisPort},
		Cmd:          []string{"redis-server", "--notify-keyspace-events", "KEA"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start redis container: %w", err)
	}
	r.container = container

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host: %w", err)
	}

	r.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Create Redis client
	r.client = redis.NewClient(&redis.Options{
		Addr:         r.endpoint,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	// Verify connection
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Ping(healthCtx).Err(); err != nil {
		return fmt.Errorf("failed to verify redis connection: %w", err)
	}

	return nil
}

// Stop stops the Redis container and closes the client.
func (r *RedisContainer) Stop(ctx context.Context) error {
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return fmt.Errorf("failed to close redis client: %w", err)
		}
		r.client = nil
	}

	if r.container != nil {
		if err := r.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate redis container: %w", err)
		}
		r.container = nil
	}

	return nil
}

// Restart stops and restarts the Redis container, reinitializing the client.
// Note: Redis data is ephemeral and will be lost on restart.
func (r *RedisContainer) Restart(ctx context.Context) error {
	if r.container == nil {
		return fmt.Errorf("redis container not started")
	}

	// Close the existing client
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return fmt.Errorf("failed to close redis client: %w", err)
		}
		r.client = nil
	}

	// Stop the container (don't terminate)
	timeout := 10 * time.Second
	if err := r.container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("failed to stop redis container: %w", err)
	}

	// Start the container again
	if err := r.container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start redis container: %w", err)
	}

	// Re-fetch the endpoint in case port mapping changed
	mappedPort, err := r.container.MappedPort(ctx, "6379")
	if err != nil {
		return fmt.Errorf("failed to get mapped port after restart: %w", err)
	}
	host, err := r.container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container host after restart: %w", err)
	}
	r.endpoint = fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Wait for Redis to be ready
	time.Sleep(3 * time.Second)

	// Recreate Redis client with retries
	for i := 0; i < 15; i++ {
		r.client = redis.NewClient(&redis.Options{
			Addr:         r.endpoint,
			DialTimeout:  3 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		})

		// Verify connection
		healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = r.client.Ping(healthCtx).Err()
		cancel()
		if err == nil {
			return nil
		}

		// Close failed client and retry
		r.client.Close()
		r.client = nil
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("failed to verify redis connection after restart: %w", err)
}

// Endpoint returns the Redis connection endpoint.
func (r *RedisContainer) Endpoint(ctx context.Context) (string, error) {
	if r.endpoint == "" {
		return "", fmt.Errorf("redis container not started")
	}
	return r.endpoint, nil
}

// SetValue sets a key-value pair in Redis.
// Keys are stored as-is (e.g., "/test/value").
func (r *RedisContainer) SetValue(ctx context.Context, key, value string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	setCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Set(setCtx, key, value, 0).Err(); err != nil {
		return fmt.Errorf("failed to set value for key %s: %w", key, err)
	}

	return nil
}

// DeleteValue deletes a key from Redis.
func (r *RedisContainer) DeleteValue(ctx context.Context, key string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	delCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Del(delCtx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// BackendName returns "redis".
func (r *RedisContainer) BackendName() string {
	return "redis"
}
