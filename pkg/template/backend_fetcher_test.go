package template

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/memkv"
)

// mockBackendStoreClient is a test implementation of backends.StoreClient for backend_fetcher_test
type mockBackendStoreClient struct {
	getValuesFunc   func(ctx context.Context, keys []string) (map[string]string, error)
	watchPrefixFunc func(ctx context.Context, prefix string, keys []string, lastIndex uint64, stopChan chan bool) (uint64, error)
	healthCheckFunc func(ctx context.Context) error
}

func (m *mockBackendStoreClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	if m.getValuesFunc != nil {
		return m.getValuesFunc(ctx, keys)
	}
	return nil, errors.New("getValuesFunc not implemented")
}

func (m *mockBackendStoreClient) WatchPrefix(ctx context.Context, prefix string, keys []string, lastIndex uint64, stopChan chan bool) (uint64, error) {
	if m.watchPrefixFunc != nil {
		return m.watchPrefixFunc(ctx, prefix, keys, lastIndex, stopChan)
	}
	return 0, errors.New("watchPrefixFunc not implemented")
}

func (m *mockBackendStoreClient) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil // Default to healthy
}

func (m *mockBackendStoreClient) Close() error {
	return nil
}

func TestNewBackendFetcher(t *testing.T) {
	store := memkv.New()
	client := &mockBackendStoreClient{}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient:    client,
		Store:          store,
		Prefix:         "/test",
		Ctx:            context.Background(),
		BackendTimeout: 5 * time.Second,
	})

	if fetcher == nil {
		t.Fatal("newBackendFetcher() returned nil")
	}
	if fetcher.storeClient != client {
		t.Error("newBackendFetcher() storeClient not set correctly")
	}
	if fetcher.store != store {
		t.Error("newBackendFetcher() store not set correctly")
	}
	if fetcher.prefix != "/test" {
		t.Errorf("newBackendFetcher() prefix = %v, want %v", fetcher.prefix, "/test")
	}
	if fetcher.backendTimeout != 5*time.Second {
		t.Errorf("newBackendFetcher() backendTimeout = %v, want %v", fetcher.backendTimeout, 5*time.Second)
	}
}

func TestFetchValues_Success(t *testing.T) {
	store := memkv.New()
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"/app/database/host": "localhost",
				"/app/database/port": "5432",
				"/app/api/key":       "secret",
			}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
		Ctx:         context.Background(),
	})

	err := fetcher.fetchValues([]string{"database/host", "database/port", "api/key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify values are stored correctly
	if val, err := store.Get("/database/host"); err != nil || val.Value != "localhost" {
		t.Errorf("Expected /database/host=localhost, got %v (err=%v)", val.Value, err)
	}
	if val, err := store.Get("/database/port"); err != nil || val.Value != "5432" {
		t.Errorf("Expected /database/port=5432, got %v (err=%v)", val.Value, err)
	}
	if val, err := store.Get("/api/key"); err != nil || val.Value != "secret" {
		t.Errorf("Expected /api/key=secret, got %v (err=%v)", val.Value, err)
	}
}

func TestFetchValues_BackendError(t *testing.T) {
	store := memkv.New()
	expectedErr := errors.New("backend connection failed")
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, expectedErr
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
	})

	err := fetcher.fetchValues([]string{"key1", "key2"})
	if err == nil {
		t.Error("fetchValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("fetchValues() error = %v, want %v", err, expectedErr)
	}
}

func TestFetchValues_PurgesExistingData(t *testing.T) {
	store := memkv.New()

	// Pre-populate store with old data
	store.Set("/old/key1", "old_value1")
	store.Set("/old/key2", "old_value2")

	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{
				"/app/new/key": "new_value",
			}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
	})

	err := fetcher.fetchValues([]string{"new/key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify old data is purged
	if _, err := store.Get("/old/key1"); err == nil {
		t.Error("fetchValues() should have purged old data")
	}
	if _, err := store.Get("/old/key2"); err == nil {
		t.Error("fetchValues() should have purged old data")
	}

	// Verify new data is present
	if val, err := store.Get("/new/key"); err != nil || val.Value != "new_value" {
		t.Errorf("Expected /new/key=new_value, got %v (err=%v)", val.Value, err)
	}
}

func TestFetchValues_NormalizesKeys(t *testing.T) {
	store := memkv.New()
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			// Backend returns keys with full prefix
			return map[string]string{
				"/production/app/database/host": "localhost",
				"/production/app/database/port": "5432",
			}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/production/app",
	})

	err := fetcher.fetchValues([]string{"database/host", "database/port"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify keys are normalized (prefix removed)
	if val, err := store.Get("/database/host"); err != nil || val.Value != "localhost" {
		t.Errorf("Expected /database/host=localhost, got %v (err=%v)", val.Value, err)
	}
	if val, err := store.Get("/database/port"); err != nil || val.Value != "5432" {
		t.Errorf("Expected /database/port=5432, got %v (err=%v)", val.Value, err)
	}
}

func TestFetchValues_WithTimeout(t *testing.T) {
	store := memkv.New()
	var receivedCtx context.Context
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			receivedCtx = ctx
			return map[string]string{}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient:    client,
		Store:          store,
		Prefix:         "/app",
		Ctx:            context.Background(),
		BackendTimeout: 5 * time.Second,
	})

	err := fetcher.fetchValues([]string{"key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify context has timeout
	if receivedCtx == nil {
		t.Fatal("fetchValues() did not pass context to GetValues")
	}
	deadline, ok := receivedCtx.Deadline()
	if !ok {
		t.Error("fetchValues() context should have deadline when backendTimeout is set")
	} else if time.Until(deadline) > 5*time.Second {
		t.Error("fetchValues() context deadline exceeds configured timeout")
	}
}

func TestFetchValues_WithoutTimeout(t *testing.T) {
	store := memkv.New()
	var receivedCtx context.Context
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			receivedCtx = ctx
			return map[string]string{}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient:    client,
		Store:          store,
		Prefix:         "/app",
		Ctx:            context.Background(),
		BackendTimeout: 0, // No timeout
	})

	err := fetcher.fetchValues([]string{"key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify context has no deadline when timeout is 0
	if receivedCtx == nil {
		t.Fatal("fetchValues() did not pass context to GetValues")
	}
	if _, ok := receivedCtx.Deadline(); ok {
		t.Error("fetchValues() context should not have deadline when backendTimeout is 0")
	}
}

func TestFetchValues_NilContext(t *testing.T) {
	store := memkv.New()
	var receivedCtx context.Context
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			receivedCtx = ctx
			return map[string]string{}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
		Ctx:         nil, // No context provided
	})

	err := fetcher.fetchValues([]string{"key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify a background context is used when ctx is nil
	if receivedCtx == nil {
		t.Fatal("fetchValues() did not pass context to GetValues")
	}
	// Should be context.Background() or derived from it
	if receivedCtx.Err() != nil {
		t.Error("fetchValues() should use valid context when Ctx is nil")
	}
}

func TestGetStore(t *testing.T) {
	store := memkv.New()
	client := &mockBackendStoreClient{}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
	})

	retrievedStore := fetcher.getStore()
	if retrievedStore != store {
		t.Error("getStore() did not return the same store instance")
	}
}

func TestFetchValues_EmptyKeys(t *testing.T) {
	store := memkv.New()
	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
	})

	err := fetcher.fetchValues([]string{})
	if err != nil {
		t.Errorf("fetchValues() with empty keys should not error: %v", err)
	}
}

func TestFetchValues_EmptyResult(t *testing.T) {
	store := memkv.New()

	// Pre-populate store
	store.Set("/old/key", "old_value")

	client := &mockBackendStoreClient{
		getValuesFunc: func(ctx context.Context, keys []string) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}

	fetcher := newBackendFetcher(backendFetcherConfig{
		StoreClient: client,
		Store:       store,
		Prefix:      "/app",
	})

	err := fetcher.fetchValues([]string{"key"})
	if err != nil {
		t.Errorf("fetchValues() unexpected error: %v", err)
	}

	// Verify store is purged even when result is empty
	if _, err := store.Get("/old/key"); err == nil {
		t.Error("fetchValues() should purge store even when result is empty")
	}
}
