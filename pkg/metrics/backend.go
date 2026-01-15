package metrics

import (
	"context"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/backends/types"
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

// HealthCheckDetailed provides detailed health information if the backend supports it.
func (c *InstrumentedClient) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	// Check if the client implements DetailedHealthChecker
	if detailedChecker, ok := c.client.(types.DetailedHealthChecker); ok {
		start := time.Now()
		result, err := detailedChecker.HealthCheckDetailed(ctx)
		duration := time.Since(start).Seconds()

		BackendRequestDuration.WithLabelValues(c.backend, "health_check_detailed").Observe(duration)
		BackendRequestTotal.WithLabelValues(c.backend, "health_check_detailed").Inc()
		if err != nil {
			BackendErrorsTotal.WithLabelValues(c.backend, "health_check_detailed").Inc()
		}
		return result, err
	}

	// Fallback: backend doesn't support detailed health checks
	// Perform a basic health check and convert to HealthResult
	start := time.Now()
	err := c.client.HealthCheck(ctx)
	duration := time.Since(start)

	result := &types.HealthResult{
		Healthy:   err == nil,
		Message:   "Backend does not support detailed health checks",
		Duration:  types.DurationMillis(duration),
		CheckedAt: time.Now(),
		Details:   map[string]string{},
	}

	if err != nil {
		result.Message = err.Error()
		result.Details["error"] = err.Error()
	}

	return result, err
}
