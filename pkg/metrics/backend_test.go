package metrics

import (
	"context"
	"errors"
	"testing"
)

// mockStoreClient is a mock implementation of backends.StoreClient
type mockStoreClient struct {
	getValuesResult map[string]string
	getValuesError  error
	watchResult     uint64
	watchError      error
	healthError     error
}

func (m *mockStoreClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	return m.getValuesResult, m.getValuesError
}

func (m *mockStoreClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	return m.watchResult, m.watchError
}

func (m *mockStoreClient) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func (m *mockStoreClient) Close() error {
	return nil
}

func TestWrapStoreClient_NoOpWhenMetricsDisabled(t *testing.T) {
	// Reset state - metrics disabled
	Registry = nil

	mock := &mockStoreClient{
		getValuesResult: map[string]string{"key": "value"},
	}

	wrapped := WrapStoreClient(mock, "test")

	// When metrics are disabled, should return the original client
	if wrapped != mock {
		t.Error("WrapStoreClient should return original client when metrics disabled")
	}
}

func TestWrapStoreClient_WrapsWhenMetricsEnabled(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		getValuesResult: map[string]string{"key": "value"},
	}

	wrapped := WrapStoreClient(mock, "test")

	// When metrics are enabled, should return a wrapped client
	if wrapped == mock {
		t.Error("WrapStoreClient should return instrumented client when metrics enabled")
	}

	// Verify it's an InstrumentedClient
	_, ok := wrapped.(*InstrumentedClient)
	if !ok {
		t.Error("Wrapped client should be an InstrumentedClient")
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_GetValues_Success(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		getValuesResult: map[string]string{"key": "value"},
	}

	wrapped := WrapStoreClient(mock, "vault").(*InstrumentedClient)

	result, err := wrapped.GetValues(context.Background(), []string{"key"})

	if err != nil {
		t.Errorf("GetValues should not return error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("GetValues should return expected result, got: %v", result)
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_GetValues_Error(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		getValuesError: errors.New("connection failed"),
	}

	wrapped := WrapStoreClient(mock, "vault").(*InstrumentedClient)

	_, err := wrapped.GetValues(context.Background(), []string{"key"})

	if err == nil {
		t.Error("GetValues should return error from underlying client")
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_WatchPrefix_Success(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		watchResult: 123,
	}

	wrapped := WrapStoreClient(mock, "etcd").(*InstrumentedClient)

	result, err := wrapped.WatchPrefix(context.Background(), "/prefix", []string{"key"}, 0, nil)

	if err != nil {
		t.Errorf("WatchPrefix should not return error: %v", err)
	}
	if result != 123 {
		t.Errorf("WatchPrefix should return expected result, got: %v", result)
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_HealthCheck_Success(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{}

	wrapped := WrapStoreClient(mock, "consul").(*InstrumentedClient)

	err := wrapped.HealthCheck(context.Background())

	if err != nil {
		t.Errorf("HealthCheck should not return error: %v", err)
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_HealthCheck_Error(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		healthError: errors.New("backend unavailable"),
	}

	wrapped := WrapStoreClient(mock, "redis").(*InstrumentedClient)

	err := wrapped.HealthCheck(context.Background())

	if err == nil {
		t.Error("HealthCheck should return error from underlying client")
	}

	// Cleanup
	Registry = nil
}

func TestInstrumentedClient_RecordsMetrics(t *testing.T) {
	// Reset state and enable metrics
	Registry = nil
	Initialize()

	mock := &mockStoreClient{
		getValuesResult: map[string]string{"key": "value"},
	}

	wrapped := WrapStoreClient(mock, "vault").(*InstrumentedClient)

	// Make some calls
	wrapped.GetValues(context.Background(), []string{"key"})
	wrapped.GetValues(context.Background(), []string{"key2"})
	wrapped.HealthCheck(context.Background())

	// Verify metrics were recorded by gathering them
	metricFamilies, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Check that backend metrics are present
	found := make(map[string]bool)
	for _, mf := range metricFamilies {
		found[mf.GetName()] = true
	}

	if !found["confd_backend_request_total"] {
		t.Error("Expected confd_backend_request_total metric")
	}
	if !found["confd_backend_request_duration_seconds"] {
		t.Error("Expected confd_backend_request_duration_seconds metric")
	}

	// Cleanup
	Registry = nil
}
