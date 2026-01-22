package template

import (
	"sync"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

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
