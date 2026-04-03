package template

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
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
//
// Operational parameters (timeouts, retry config) are zeroed before hashing
// because they don't affect client identity - two configs pointing to the
// same backend should share a client regardless of timeout settings.
func configHash(cfg backends.Config) string {
	// Zero operational parameters - they don't affect client identity.
	// cfg is passed by value, so these modifications don't affect the caller.
	cfg.DialTimeout = 0
	cfg.ReadTimeout = 0
	cfg.WriteTimeout = 0
	cfg.RetryMaxAttempts = 0
	cfg.RetryBaseDelay = 0
	cfg.RetryMaxDelay = 0
	cfg.IMDSCacheTTL = 0

	// Marshal config to JSON for consistent hashing
	data, err := json.Marshal(cfg)
	if err != nil {
		// Fallback to a simple string representation
		return fmt.Sprintf("%s-%v", cfg.Backend, cfg.BackendNodes)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for shorter key
}

// CloseAllCachedClients closes every client in the cache and resets it.
// It should be called during shutdown after the processor has stopped,
// to release connections held by per-resource backend configurations.
// All clients are attempted regardless of individual close errors; if any
// close fails, a combined error wrapping all failures is returned so
// callers can inspect every failure via errors.Is/As.
//
// The map is swapped out under the lock and closed outside the critical
// section so that client.Close() (which may block on network I/O) does
// not hold the mutex.
func CloseAllCachedClients() error {
	clientCacheMu.Lock()
	cached := clientCache
	clientCache = make(map[string]backends.StoreClient)
	clientCacheMu.Unlock()

	var errs []error
	for hash, client := range cached {
		if err := client.Close(); err != nil {
			log.Warning("Failed to close cached backend client %s: %v", hash, err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("closed cached clients with %d error(s): %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// clearClientCache clears the client cache without closing clients.
// This is used for testing only.
func clearClientCache() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	clientCache = make(map[string]backends.StoreClient)
}
