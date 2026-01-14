package metrics

import (
	"context"
	"time"

	"github.com/abtreece/confd/pkg/backends"
)

// InstrumentedClient wraps a StoreClient with metrics instrumentation.
type InstrumentedClient struct {
	client  backends.StoreClient
	backend string
}

// WrapStoreClient wraps a StoreClient with metrics instrumentation.
// If metrics are not enabled (Registry is nil), returns the original client.
func WrapStoreClient(client backends.StoreClient, backendType string) backends.StoreClient {
	if !Enabled() {
		return client
	}
	return &InstrumentedClient{client: client, backend: backendType}
}

// GetValues retrieves values from the backend and records metrics.
func (c *InstrumentedClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	start := time.Now()
	result, err := c.client.GetValues(ctx, keys)
	duration := time.Since(start).Seconds()

	BackendRequestDuration.WithLabelValues(c.backend, "get_values").Observe(duration)
	BackendRequestTotal.WithLabelValues(c.backend, "get_values").Inc()
	if err != nil {
		BackendErrorsTotal.WithLabelValues(c.backend, "get_values").Inc()
	}
	return result, err
}

// WatchPrefix watches for changes on a prefix and records metrics.
func (c *InstrumentedClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	start := time.Now()
	result, err := c.client.WatchPrefix(ctx, prefix, keys, waitIndex, stopChan)
	duration := time.Since(start).Seconds()

	BackendRequestDuration.WithLabelValues(c.backend, "watch_prefix").Observe(duration)
	BackendRequestTotal.WithLabelValues(c.backend, "watch_prefix").Inc()
	if err != nil {
		BackendErrorsTotal.WithLabelValues(c.backend, "watch_prefix").Inc()
	}
	return result, err
}

// HealthCheck verifies backend health and records metrics.
func (c *InstrumentedClient) HealthCheck(ctx context.Context) error {
	start := time.Now()
	err := c.client.HealthCheck(ctx)
	duration := time.Since(start).Seconds()

	BackendRequestDuration.WithLabelValues(c.backend, "health_check").Observe(duration)
	BackendRequestTotal.WithLabelValues(c.backend, "health_check").Inc()
	if err != nil {
		BackendErrorsTotal.WithLabelValues(c.backend, "health_check").Inc()
		BackendHealthy.WithLabelValues(c.backend).Set(0)
	} else {
		BackendHealthy.WithLabelValues(c.backend).Set(1)
	}
	return err
}
