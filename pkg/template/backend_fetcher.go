package template

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/memkv"
)

// backendFetcher handles fetching data from backend stores and populating
// the in-memory key-value store used for template rendering.
type backendFetcher struct {
	storeClient    backends.StoreClient
	store          *memkv.Store
	prefix         string
	prefixedKeys   []string // Pre-computed keys with prefix applied
	ctx            context.Context
	backendTimeout time.Duration
}

// backendFetcherConfig holds configuration for creating a backendFetcher.
type backendFetcherConfig struct {
	StoreClient    backends.StoreClient
	Store          *memkv.Store
	Prefix         string
	PrefixedKeys   []string // Pre-computed keys with prefix applied
	Ctx            context.Context
	BackendTimeout time.Duration
}

// newBackendFetcher creates a new backendFetcher instance.
func newBackendFetcher(config backendFetcherConfig) *backendFetcher {
	return &backendFetcher{
		storeClient:    config.StoreClient,
		store:          config.Store,
		prefix:         config.Prefix,
		prefixedKeys:   config.PrefixedKeys,
		ctx:            config.Ctx,
		backendTimeout: config.BackendTimeout,
	}
}

// fetchValues retrieves values for the pre-computed prefixed keys from the backend store
// and populates the in-memory store with the results.
// The store is purged before populating with new values.
// Returns an error if the backend fetch operation fails.
func (b *backendFetcher) fetchValues() error {
	start := time.Now()
	logger := log.With("prefix", b.prefix, "key_count", len(b.prefixedKeys))
	logger.DebugContext(b.ctx, "Starting backend fetch")

	// Use context with timeout if configured
	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if b.backendTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.backendTimeout)
		defer cancel()
	}

	// Fetch values from backend using pre-computed prefixed keys
	fetchStart := time.Now()
	result, err := b.storeClient.GetValues(ctx, b.prefixedKeys)
	fetchDuration := time.Since(fetchStart)

	if err != nil {
		logger.ErrorContext(ctx, "Backend fetch failed",
			"fetch_duration_ms", fetchDuration.Milliseconds(),
			"total_duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error())
		return err
	}

	logger.DebugContext(ctx, "Backend fetch completed",
		"fetch_duration_ms", fetchDuration.Milliseconds(),
		"value_count", len(result))

	// Purge existing values and populate with new ones
	updateStart := time.Now()
	b.store.Purge()

	for k, v := range result {
		// Normalize keys by removing the prefix
		normalizedKey := path.Join("/", strings.TrimPrefix(k, b.prefix))
		b.store.Set(normalizedKey, v)
	}
	updateDuration := time.Since(updateStart)

	logger.InfoContext(ctx, "Backend fetch and store update completed",
		"total_duration_ms", time.Since(start).Milliseconds(),
		"fetch_duration_ms", fetchDuration.Milliseconds(),
		"update_duration_ms", updateDuration.Milliseconds(),
		"value_count", len(result))

	return nil
}

// getStore returns the in-memory store instance.
// This is used by other components (e.g., TemplateRenderer) that need
// access to the populated store.
func (b *backendFetcher) getStore() *memkv.Store {
	return b.store
}
