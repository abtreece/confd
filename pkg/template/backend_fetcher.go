package template

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/memkv"
	util "github.com/abtreece/confd/pkg/util"
)

// backendFetcher handles fetching data from backend stores and populating
// the in-memory key-value store used for template rendering.
type backendFetcher struct {
	storeClient    backends.StoreClient
	store          *memkv.Store
	prefix         string
	ctx            context.Context
	backendTimeout time.Duration
}

// backendFetcherConfig holds configuration for creating a backendFetcher.
type backendFetcherConfig struct {
	StoreClient    backends.StoreClient
	Store          *memkv.Store
	Prefix         string
	Ctx            context.Context
	BackendTimeout time.Duration
}

// newBackendFetcher creates a new backendFetcher instance.
func newBackendFetcher(config backendFetcherConfig) *backendFetcher {
	return &backendFetcher{
		storeClient:    config.StoreClient,
		store:          config.Store,
		prefix:         config.Prefix,
		ctx:            config.Ctx,
		backendTimeout: config.BackendTimeout,
	}
}

// fetchValues retrieves values for the specified keys from the backend store
// and populates the in-memory store with the results.
// Keys are combined with the configured prefix before fetching.
// The store is purged before populating with new values.
// Returns an error if the backend fetch operation fails.
func (b *backendFetcher) fetchValues(keys []string) error {
	log.Debug("Retrieving keys from store")
	log.Debug("Key prefix set to %s", b.prefix)

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

	// Fetch values from backend
	result, err := b.storeClient.GetValues(ctx, util.AppendPrefix(b.prefix, keys))
	if err != nil {
		return err
	}
	log.Debug("Got the following map from store: %v", result)

	// Purge existing values and populate with new ones
	b.store.Purge()

	for k, v := range result {
		// Normalize keys by removing the prefix
		normalizedKey := path.Join("/", strings.TrimPrefix(k, b.prefix))
		b.store.Set(normalizedKey, v)
	}

	return nil
}

// getStore returns the in-memory store instance.
// This is used by other components (e.g., TemplateRenderer) that need
// access to the populated store.
func (b *backendFetcher) getStore() *memkv.Store {
	return b.store
}
