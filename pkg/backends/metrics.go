package backends

import (
	"context"
	"time"

	"github.com/abtreece/confd/pkg/metrics"
)

// metricsStoreClient wraps a StoreClient with metrics instrumentation
type metricsStoreClient struct {
	backend string
	client  StoreClient
}

// NewWithMetrics wraps a StoreClient with metrics collection
func NewWithMetrics(backend string, client StoreClient) StoreClient {
	// Set backend connected status
	metrics.SetBackendConnected(backend, true)
	
	return &metricsStoreClient{
		backend: backend,
		client:  client,
	}
}

func (m *metricsStoreClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	start := time.Now()
	result, err := m.client.GetValues(ctx, keys)
	duration := time.Since(start).Seconds()
	
	success := err == nil
	metrics.RecordBackendRequest(m.backend, "GetValues", success, duration)
	
	if !success {
		metrics.SetBackendConnected(m.backend, false)
	}
	
	return result, err
}

func (m *metricsStoreClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	start := time.Now()
	index, err := m.client.WatchPrefix(ctx, prefix, keys, waitIndex, stopChan)
	duration := time.Since(start).Seconds()
	
	success := err == nil
	metrics.RecordBackendRequest(m.backend, "WatchPrefix", success, duration)
	
	if !success {
		metrics.SetBackendConnected(m.backend, false)
	} else {
		metrics.SetBackendConnected(m.backend, true)
	}
	
	return index, err
}

func (m *metricsStoreClient) HealthCheck(ctx context.Context) error {
	start := time.Now()
	err := m.client.HealthCheck(ctx)
	duration := time.Since(start).Seconds()
	
	success := err == nil
	metrics.RecordBackendRequest(m.backend, "HealthCheck", success, duration)
	metrics.SetBackendConnected(m.backend, success)
	
	return err
}
