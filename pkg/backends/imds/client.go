package imds

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	"github.com/abtreece/confd/pkg/backends/types"
	"github.com/abtreece/confd/pkg/log"
)

// imdsAPI defines the interface for IMDS operations (for testing)
type imdsAPI interface {
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
	GetDynamicData(ctx context.Context, params *imds.GetDynamicDataInput, optFns ...func(*imds.Options)) (*imds.GetDynamicDataOutput, error)
	GetUserData(ctx context.Context, params *imds.GetUserDataInput, optFns ...func(*imds.Options)) (*imds.GetUserDataOutput, error)
}

// Client provides access to AWS EC2 Instance Metadata Service
type Client struct {
	client   imdsAPI
	cache    *metadataCache
	cacheTTL time.Duration
}

// metadataCache provides thread-safe caching with TTL
type metadataCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

// cacheEntry stores a cached value with timestamp
type cacheEntry struct {
	value     string
	timestamp time.Time
}

// newMetadataCache creates a new metadata cache
func newMetadataCache() *metadataCache {
	return &metadataCache{
		entries: make(map[string]*cacheEntry),
	}
}

// get retrieves a cached value if it exists and hasn't expired
func (c *metadataCache) get(key string, ttl time.Duration) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > ttl {
		return "", false
	}

	return entry.value, true
}

// set stores a value in the cache with current timestamp
func (c *metadataCache) set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		value:     value,
		timestamp: time.Now(),
	}
}

// size returns the number of entries in the cache
func (c *metadataCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// New creates a new IMDS client
func New(cacheTTL, dialTimeout time.Duration) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create IMDS client with custom endpoint support for testing
	opts := []func(*imds.Options){
		func(o *imds.Options) {
			// Support custom endpoint override via IMDS_ENDPOINT for testing/non-production only.
			// WARNING: Overriding the IMDS endpoint can be a security risk in production and should
			// not be used there, as it may allow redirection of metadata requests to a malicious service.
			if endpoint := os.Getenv("IMDS_ENDPOINT"); endpoint != "" {
				o.Endpoint = endpoint
			}
		},
	}

	client := imds.NewFromConfig(cfg, opts...)

	return newWithClient(ctx, client, cacheTTL)
}

// newWithClient creates a new IMDS Client with the provided imdsAPI implementation.
// This is separated from New() to allow testing with mock clients.
func newWithClient(ctx context.Context, client imdsAPI, cacheTTL time.Duration) (*Client, error) {
	// Validate IMDS availability
	validationOutput, err := client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "",
	})
	if err != nil {
		return nil, fmt.Errorf("IMDS not available: %w", err)
	}
	if closeErr := validationOutput.Content.Close(); closeErr != nil {
		log.Debug("Failed to close IMDS validation response: %v", closeErr)
	}

	log.Info("Successfully connected to AWS EC2 IMDS")

	return &Client{
		client:   client,
		cache:    newMetadataCache(),
		cacheTTL: cacheTTL,
	}, nil
}

// GetValues retrieves values for the specified keys from IMDS
func (c *Client) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)

	// Process each requested key
	for _, key := range keys {
		// Normalize path: remove leading slash and "latest/" prefix
		path := strings.TrimPrefix(key, "/")
		path = strings.TrimPrefix(path, "latest/")

		// Determine category and extract the actual path
		category, metadataPath := c.categorizeKey(path)

		// Check cache first
		if cached, ok := c.cache.get(path, c.cacheTTL); ok {
			log.Debug("Cache hit for key=%s path=%s", key, path)
			result[key] = cached
			continue
		}

		// Fetch from IMDS and walk directory structure if needed
		values, err := c.walkPath(ctx, category, metadataPath)
		if err != nil {
			log.Error("Failed to fetch metadata for key=%s path=%s: %v", key, path, err)
			continue
		}

		// Cache and filter results
		for k, v := range values {
			fullPath := category
			if k != "" {
				fullPath += "/" + k
			}
			c.cache.set(fullPath, v)

			// Only include values that match the requested key prefix
			fullKey := "/" + fullPath
			if strings.HasPrefix(fullKey, key) || key == "/" || strings.HasPrefix(key, "/"+fullPath) {
				result[fullKey] = v
			}
		}

		// If the exact key was requested and found, add it
		if val, ok := values[metadataPath]; ok {
			result[key] = val
		}
	}

	return result, nil
}

// categorizeKey determines the IMDS category and extracts the path
func (c *Client) categorizeKey(path string) (category, metadataPath string) {
	if strings.HasPrefix(path, "meta-data/") {
		return "meta-data", strings.TrimPrefix(path, "meta-data/")
	} else if strings.HasPrefix(path, "dynamic/") {
		return "dynamic", strings.TrimPrefix(path, "dynamic/")
	} else if strings.HasPrefix(path, "user-data") {
		return "user-data", ""
	}
	// Default to meta-data for bare paths
	return "meta-data", path
}

// walkPath recursively fetches metadata, handling directory listings
func (c *Client) walkPath(ctx context.Context, category, path string) (map[string]string, error) {
	result := make(map[string]string)

	// Trim trailing slash for consistent API calls
	path = strings.TrimSuffix(path, "/")

	var content string
	var err error

	switch category {
	case "meta-data":
		content, err = c.getMetadata(ctx, path)
	case "dynamic":
		content, err = c.getDynamicData(ctx, path)
	case "user-data":
		content, err = c.getUserData(ctx)
		if err == nil {
			result[""] = content
			return result, nil
		}
	default:
		return nil, fmt.Errorf("unknown category: %s", category)
	}

	if err != nil {
		return nil, err
	}

	// Check if this is a directory listing (contains newlines)
	if strings.Contains(content, "\n") {
		// This is a directory, recursively fetch each entry
		entries := strings.Split(strings.TrimSpace(content), "\n")
		for _, entry := range entries {
			if entry == "" {
				continue
			}

			// Remove trailing slash if present
			entry = strings.TrimSuffix(entry, "/")

			// Build the full path for this entry
			var childPath string
			if path == "" {
				childPath = entry
			} else {
				childPath = path + "/" + entry
			}

			// Recursively walk this path
			childValues, err := c.walkPath(ctx, category, childPath)
			if err != nil {
				log.Debug("Failed to fetch child path=%s: %v", childPath, err)
				continue
			}

			// Merge child values into result
			for k, v := range childValues {
				result[k] = v
			}
		}
	} else {
		// This is a leaf value
		result[path] = content
	}

	return result, nil
}

// getMetadata fetches metadata from the meta-data category
func (c *Client) getMetadata(ctx context.Context, path string) (string, error) {
	output, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: path,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get metadata: %w", err)
	}
	defer func() {
		if closeErr := output.Content.Close(); closeErr != nil {
			log.Debug("Failed to close metadata response: %v", closeErr)
		}
	}()

	content, err := io.ReadAll(output.Content)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata content: %w", err)
	}

	return string(content), nil
}

// getDynamicData fetches data from the dynamic category
func (c *Client) getDynamicData(ctx context.Context, path string) (string, error) {
	output, err := c.client.GetDynamicData(ctx, &imds.GetDynamicDataInput{
		Path: path,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get dynamic data: %w", err)
	}
	defer func() {
		if closeErr := output.Content.Close(); closeErr != nil {
			log.Debug("Failed to close dynamic data response: %v", closeErr)
		}
	}()

	content, err := io.ReadAll(output.Content)
	if err != nil {
		return "", fmt.Errorf("failed to read dynamic data content: %w", err)
	}

	return string(content), nil
}

// getUserData fetches user data
func (c *Client) getUserData(ctx context.Context) (string, error) {
	output, err := c.client.GetUserData(ctx, &imds.GetUserDataInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get user data: %w", err)
	}
	defer func() {
		if closeErr := output.Content.Close(); closeErr != nil {
			log.Debug("Failed to close user data response: %v", closeErr)
		}
	}()

	content, err := io.ReadAll(output.Content)
	if err != nil {
		return "", fmt.Errorf("failed to read user data content: %w", err)
	}

	return string(content), nil
}

// WatchPrefix is not supported for IMDS (polling only)
func (c *Client) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	select {
	case <-ctx.Done():
		return waitIndex, ctx.Err()
	case <-stopChan:
		return waitIndex, nil
	}
}

// HealthCheck performs a basic health check
func (c *Client) HealthCheck(ctx context.Context) error {
	output, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "",
	})
	if err != nil {
		return fmt.Errorf("IMDS health check failed: %w", err)
	}
	if closeErr := output.Content.Close(); closeErr != nil {
		log.Debug("Failed to close IMDS health check response: %v", closeErr)
	}
	return nil
}

// HealthCheckDetailed performs a detailed health check
func (c *Client) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	start := time.Now()

	result := &types.HealthResult{
		Healthy:   true,
		CheckedAt: start,
		Details:   make(map[string]string),
	}

	// Try to list root metadata categories
	output, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "",
	})
	if err != nil {
		result.Healthy = false
		result.Message = fmt.Sprintf("IMDS unavailable: %v", err)
		result.Duration = types.DurationMillis(time.Since(start))
		return result, nil
	}
	if closeErr := output.Content.Close(); closeErr != nil {
		log.Debug("Failed to close IMDS response: %v", closeErr)
	}

	result.Duration = types.DurationMillis(time.Since(start))
	result.Message = "IMDS available"
	result.Details["cache_entries"] = fmt.Sprintf("%d", c.cache.size())
	result.Details["cache_ttl"] = c.cacheTTL.String()

	return result, nil
}

// Close closes the client (no-op for IMDS)
func (c *Client) Close() error {
	return nil
}
