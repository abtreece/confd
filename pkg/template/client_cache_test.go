package template

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// mockClosableClient is a StoreClient whose Close behaviour is controllable.
type mockClosableClient struct {
	closed    bool
	closeErr  error
	closeFunc func() error // optional; called instead of returning closeErr when set
}

func (m *mockClosableClient) GetValues(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}
func (m *mockClosableClient) WatchPrefix(_ context.Context, _ string, _ []string, _ uint64, _ chan bool) (uint64, error) {
	return 0, nil
}
func (m *mockClosableClient) HealthCheck(_ context.Context) error { return nil }
func (m *mockClosableClient) Close() error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return m.closeErr
}

func TestConfigHash(t *testing.T) {
	log.SetLevel("warn")

	tests := []struct {
		name        string
		cfg1        backends.Config
		cfg2        backends.Config
		shouldMatch bool
	}{
		{
			name: "identical configs produce same hash",
			cfg1: backends.Config{
				Backend:      "env",
				BackendNodes: []string{"node1"},
			},
			cfg2: backends.Config{
				Backend:      "env",
				BackendNodes: []string{"node1"},
			},
			shouldMatch: true,
		},
		{
			name: "different backends produce different hashes",
			cfg1: backends.Config{
				Backend: "env",
			},
			cfg2: backends.Config{
				Backend: "file",
			},
			shouldMatch: false,
		},
		{
			name: "different nodes produce different hashes",
			cfg1: backends.Config{
				Backend:      "consul",
				BackendNodes: []string{"node1:8500"},
			},
			cfg2: backends.Config{
				Backend:      "consul",
				BackendNodes: []string{"node2:8500"},
			},
			shouldMatch: false,
		},
		{
			name: "different auth produces different hashes",
			cfg1: backends.Config{
				Backend:  "vault",
				Username: "user1",
			},
			cfg2: backends.Config{
				Backend:  "vault",
				Username: "user2",
			},
			shouldMatch: false,
		},
		{
			name: "empty configs produce same hash",
			cfg1: backends.Config{},
			cfg2: backends.Config{},
			shouldMatch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash1 := configHash(tc.cfg1)
			hash2 := configHash(tc.cfg2)

			if tc.shouldMatch && hash1 != hash2 {
				t.Errorf("Expected hashes to match, got %s != %s", hash1, hash2)
			}
			if !tc.shouldMatch && hash1 == hash2 {
				t.Errorf("Expected hashes to differ, got %s == %s", hash1, hash2)
			}
		})
	}
}

func TestConfigHashConsistency(t *testing.T) {
	log.SetLevel("warn")

	cfg := backends.Config{
		Backend:      "consul",
		BackendNodes: []string{"node1:8500", "node2:8500"},
		Scheme:       "https",
		Username:     "admin",
		Password:     "secret",
	}

	// Hash should be consistent across multiple calls
	hash1 := configHash(cfg)
	hash2 := configHash(cfg)
	hash3 := configHash(cfg)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash not consistent: %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestGetOrCreateClient(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	cfg := backends.Config{
		Backend: "env",
	}

	// First call should create a new client
	client1, err := getOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}
	if client1 == nil {
		t.Fatal("Expected non-nil client")
	}

	// Second call with same config should return cached client
	client2, err := getOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("Failed to get cached client: %s", err)
	}
	if client1 != client2 {
		t.Error("Expected same client instance from cache")
	}
}

func TestGetOrCreateClientDifferentConfigs(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	cfg1 := backends.Config{
		Backend: "env",
	}

	cfg2 := backends.Config{
		Backend:  "file",
		YAMLFile: []string{"/dev/null"},
	}

	client1, err := getOrCreateClient(cfg1)
	if err != nil {
		t.Fatalf("Failed to create client1: %s", err)
	}

	client2, err := getOrCreateClient(cfg2)
	if err != nil {
		t.Fatalf("Failed to create client2: %s", err)
	}

	// Different configs should create different clients
	if client1 == client2 {
		t.Error("Expected different client instances for different configs")
	}
}

func TestGetOrCreateClientConcurrency(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	cfg := backends.Config{
		Backend: "env",
	}

	var wg sync.WaitGroup
	clients := make([]backends.StoreClient, 10)
	errors := make([]error, 10)

	// Create 10 goroutines all trying to get/create the same client
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client, err := getOrCreateClient(cfg)
			clients[idx] = client
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d got error: %s", i, err)
		}
	}

	// All clients should be the same instance
	firstClient := clients[0]
	for i, client := range clients {
		if client != firstClient {
			t.Errorf("Goroutine %d got different client instance", i)
		}
	}
}

func TestClearClientCache(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	cfg := backends.Config{
		Backend: "env",
	}

	// Create a client
	_, err := getOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %s", err)
	}

	// Verify cache has entry
	clientCacheMu.RLock()
	cacheSize := len(clientCache)
	clientCacheMu.RUnlock()

	if cacheSize != 1 {
		t.Errorf("Expected cache size 1, got %d", cacheSize)
	}

	// Clear the cache
	clearClientCache()

	// Verify cache is empty
	clientCacheMu.RLock()
	cacheSize = len(clientCache)
	clientCacheMu.RUnlock()

	if cacheSize != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cacheSize)
	}

	// Can still create clients after clear
	_, err = getOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client after clear: %s", err)
	}
}

func TestGetOrCreateClientInvalidBackend(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	cfg := backends.Config{
		Backend: "nonexistent",
	}

	_, err := getOrCreateClient(cfg)
	if err == nil {
		t.Error("Expected error for invalid backend")
	}
}

func TestConfigHashExcludesTimeouts(t *testing.T) {
	log.SetLevel("warn")

	cfg1 := backends.Config{
		Backend:      "consul",
		BackendNodes: []string{"node1:8500"},
		DialTimeout:  5 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	}
	cfg2 := backends.Config{
		Backend:      "consul",
		BackendNodes: []string{"node1:8500"},
		DialTimeout:  30 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if configHash(cfg1) != configHash(cfg2) {
		t.Error("Timeout differences should not affect hash")
	}
}

func TestConfigHashExcludesRetryConfig(t *testing.T) {
	log.SetLevel("warn")

	cfg1 := backends.Config{
		Backend:          "vault",
		BackendNodes:     []string{"vault:8200"},
		RetryMaxAttempts: 3,
		RetryBaseDelay:   100 * time.Millisecond,
		RetryMaxDelay:    5 * time.Second,
	}
	cfg2 := backends.Config{
		Backend:          "vault",
		BackendNodes:     []string{"vault:8200"},
		RetryMaxAttempts: 10,
		RetryBaseDelay:   1 * time.Second,
		RetryMaxDelay:    30 * time.Second,
	}
	if configHash(cfg1) != configHash(cfg2) {
		t.Error("Retry config differences should not affect hash")
	}
}

func TestConfigHashExcludesIMDSCacheTTL(t *testing.T) {
	log.SetLevel("warn")

	cfg1 := backends.Config{
		Backend:      "imds",
		IMDSCacheTTL: 30 * time.Second,
	}
	cfg2 := backends.Config{
		Backend:      "imds",
		IMDSCacheTTL: 5 * time.Minute,
	}
	if configHash(cfg1) != configHash(cfg2) {
		t.Error("IMDSCacheTTL differences should not affect hash")
	}
}

func TestCloseAllCachedClients_ClosesAndClears(t *testing.T) {
	log.SetLevel("warn")

	// Inject two mock clients directly into the cache.
	c1 := &mockClosableClient{}
	c2 := &mockClosableClient{}
	clientCacheMu.Lock()
	clientCache = map[string]backends.StoreClient{"key1": c1, "key2": c2}
	clientCacheMu.Unlock()

	if err := CloseAllCachedClients(); err != nil {
		t.Fatalf("CloseAllCachedClients returned error: %v", err)
	}

	if !c1.closed {
		t.Error("expected c1.Close() to be called")
	}
	if !c2.closed {
		t.Error("expected c2.Close() to be called")
	}

	// Cache should be empty after close.
	clientCacheMu.RLock()
	size := len(clientCache)
	clientCacheMu.RUnlock()
	if size != 0 {
		t.Errorf("expected empty cache after CloseAllCachedClients, got %d entries", size)
	}
}

func TestCloseAllCachedClients_EmptyCache(t *testing.T) {
	log.SetLevel("warn")
	clearClientCache()

	// Should not panic or error on an empty cache.
	if err := CloseAllCachedClients(); err != nil {
		t.Fatalf("CloseAllCachedClients on empty cache returned error: %v", err)
	}
}

func TestCloseAllCachedClients_ReturnsErrorOnCloseFailure(t *testing.T) {
	log.SetLevel("warn")

	closeErr := errors.New("connection reset")
	c := &mockClosableClient{closeErr: closeErr}
	clientCacheMu.Lock()
	clientCache = map[string]backends.StoreClient{"key1": c}
	clientCacheMu.Unlock()

	err := CloseAllCachedClients()
	if err == nil {
		t.Fatal("expected error from CloseAllCachedClients, got nil")
	}
	if !errors.Is(err, closeErr) {
		t.Errorf("error = %v, want to wrap %v", err, closeErr)
	}

	// Cache should still be cleared even when close fails.
	clientCacheMu.RLock()
	size := len(clientCache)
	clientCacheMu.RUnlock()
	if size != 0 {
		t.Errorf("expected empty cache after failed close, got %d entries", size)
	}
}

func TestCloseAllCachedClients_ContinuesOnPartialFailure(t *testing.T) {
	log.SetLevel("warn")

	closeErr := errors.New("partial failure")
	good := &mockClosableClient{}
	bad := &mockClosableClient{closeErr: closeErr}
	clientCacheMu.Lock()
	clientCache = map[string]backends.StoreClient{"good": good, "bad": bad}
	clientCacheMu.Unlock()

	err := CloseAllCachedClients()
	if err == nil {
		t.Fatal("expected error when one client fails to close")
	}
	// All individual errors must be inspectable via errors.Is.
	if !errors.Is(err, closeErr) {
		t.Errorf("errors.Is(err, closeErr) = false, want true; err = %v", err)
	}

	// Both clients should have had Close() called despite the error.
	if !good.closed {
		t.Error("expected good client to be closed")
	}
	if !bad.closed {
		t.Error("expected bad client to have Close() called")
	}
}

// TestCloseAllCachedClients_LockReleasedBeforeClose is a deadlock regression
// test. If client.Close() were called while holding clientCacheMu, any
// goroutine that tried to acquire the mutex during Close() (e.g. a template
// processor still running) would deadlock. The test proves the lock is
// released before Close() is invoked by acquiring it from inside closeFunc.
func TestCloseAllCachedClients_LockReleasedBeforeClose(t *testing.T) {
	log.SetLevel("warn")

	lockAcquired := make(chan struct{})
	c := &mockClosableClient{
		closeFunc: func() error {
			// Spawn a goroutine that acquires the cache read-lock while Close()
			// is executing. If the write-lock were still held at this point,
			// this goroutine would block and the select below would time out.
			go func() {
				clientCacheMu.RLock()
				close(lockAcquired)
				clientCacheMu.RUnlock()
			}()
			select {
			case <-lockAcquired:
				// Lock was acquired during Close() — correct behaviour.
			case <-time.After(time.Second):
				// Timeout: the mutex is still held, which would cause a deadlock
				// in production. The test will catch this below.
			}
			return nil
		},
	}
	clientCacheMu.Lock()
	clientCache = map[string]backends.StoreClient{"key1": c}
	clientCacheMu.Unlock()

	if err := CloseAllCachedClients(); err != nil {
		t.Fatalf("CloseAllCachedClients returned unexpected error: %v", err)
	}

	select {
	case <-lockAcquired:
		// Pass: cache mutex was acquirable during client.Close().
	default:
		t.Error("cache mutex was still held during client.Close() — would deadlock in production")
	}
}
