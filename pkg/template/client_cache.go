package template

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

var (
	clientCache   = make(map[string]backends.StoreClient)
	clientCacheMu sync.RWMutex
)

// getOrCreateClient returns a cached StoreClient for the given config,
// or creates a new one if not cached. This prevents duplicate connections
// when multiple template resources use identical backend configurations.
func getOrCreateClient(cfg backends.Config) (backends.StoreClient, error) {
	hash := configHash(cfg)

	// Check cache first with read lock
	clientCacheMu.RLock()
	if client, ok := clientCache[hash]; ok {
		clientCacheMu.RUnlock()
		log.Debug("Using cached backend client for %s", cfg.Backend)
		return client, nil
	}
	clientCacheMu.RUnlock()

	// Create new client with write lock
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := clientCache[hash]; ok {
		log.Debug("Using cached backend client for %s", cfg.Backend)
		return client, nil
	}

	log.Info("Creating new backend client for %s", cfg.Backend)
	client, err := backends.New(cfg)
	if err != nil {
		return nil, err
	}

	clientCache[hash] = client
	return client, nil
}

// configHash generates a unique hash for a backend configuration.
// This is used as the cache key to identify identical configurations.
func configHash(cfg backends.Config) string {
	// Marshal config to JSON for consistent hashing
	data, err := json.Marshal(cfg)
	if err != nil {
		// Fallback to a simple string representation
		return fmt.Sprintf("%s-%v", cfg.Backend, cfg.BackendNodes)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for shorter key
}

// clearClientCache clears the client cache. This is primarily used for testing.
func clearClientCache() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	clientCache = make(map[string]backends.StoreClient)
}
