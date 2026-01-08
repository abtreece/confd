package template

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

// TemplateCache caches compiled Go templates to avoid re-parsing on every render.
type TemplateCache struct {
	mu       sync.RWMutex
	cache    map[string]*cachedTemplate
	maxSize  int
	policy   string // "lru", "lfu", or "fifo"
	enabled  bool
	lruList  []string // Track access order for LRU
	hits     int64
	misses   int64
	evictions int64
}

// cachedTemplate represents a cached compiled template with metadata.
type cachedTemplate struct {
	template  *template.Template
	mtime     time.Time
	lastUsed  time.Time
	useCount  int64
	addedAt   time.Time // For FIFO policy
}

// NewTemplateCache creates a new template cache with the specified configuration.
func NewTemplateCache(enabled bool, maxSize int, policy string) *TemplateCache {
	if maxSize <= 0 {
		maxSize = 100 // Default
	}
	if policy == "" {
		policy = "lru" // Default
	}
	return &TemplateCache{
		cache:   make(map[string]*cachedTemplate),
		maxSize: maxSize,
		policy:  policy,
		enabled: enabled,
		lruList: make([]string, 0, maxSize),
	}
}

// Get retrieves a compiled template from the cache, or compiles it if not cached
// or if the source file has been modified.
func (c *TemplateCache) Get(path string, funcMap map[string]interface{}) (*template.Template, error) {
	if !c.enabled {
		// Cache disabled, always compile
		return c.compile(path, funcMap)
	}

	// Get file modification time
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat template %s: %w", path, err)
	}
	currentMtime := stat.ModTime()

	// Check cache with read lock
	c.mu.RLock()
	cached, ok := c.cache[path]
	c.mu.RUnlock()

	if ok {
		// Check if file has been modified
		if cached.mtime.Equal(currentMtime) {
			// Cache hit - update access metadata
			c.mu.Lock()
			cached.lastUsed = time.Now()
			cached.useCount++
			c.hits++
			c.updateLRU(path)
			c.mu.Unlock()

			log.Debug("Template cache hit for %s", path)
			return cached.template, nil
		}
		// File modified, need to recompile
		log.Debug("Template cache stale for %s (mtime changed)", path)
	}

	// Cache miss or stale - compile and cache
	c.mu.Lock()
	defer c.mu.Unlock()
	c.misses++

	// Double-check after acquiring write lock (another goroutine might have cached it)
	if cached, ok := c.cache[path]; ok && cached.mtime.Equal(currentMtime) {
		cached.lastUsed = time.Now()
		cached.useCount++
		c.updateLRU(path)
		return cached.template, nil
	}

	tmpl, err := c.compile(path, funcMap)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.cache[path] = &cachedTemplate{
		template: tmpl,
		mtime:    currentMtime,
		lastUsed: time.Now(),
		useCount: 1,
		addedAt:  time.Now(),
	}
	c.updateLRU(path)

	// Evict if cache is full
	if len(c.cache) > c.maxSize {
		c.evict()
	}

	log.Debug("Template cached for %s", path)
	return tmpl, nil
}

// compile parses a template file with the given function map.
func (c *TemplateCache) compile(path string, funcMap map[string]interface{}) (*template.Template, error) {
	// Clone the funcMap to avoid sharing state between templates
	funcMapCopy := make(map[string]interface{})
	for k, v := range funcMap {
		funcMapCopy[k] = v
	}

	// Use the file base name as the template name, consistent with the original implementation
	baseName := filepath.Base(path)
	tmpl, err := template.New(baseName).Funcs(funcMapCopy).ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("unable to process template %s: %w", path, err)
	}
	return tmpl, nil
}

// updateLRU updates the LRU list when a template is accessed.
func (c *TemplateCache) updateLRU(path string) {
	if c.policy != "lru" {
		return
	}

	// Remove path from current position
	for i, p := range c.lruList {
		if p == path {
			c.lruList = append(c.lruList[:i], c.lruList[i+1:]...)
			break
		}
	}
	// Add to end (most recently used)
	c.lruList = append(c.lruList, path)
}

// evict removes the least valuable entry based on the eviction policy.
func (c *TemplateCache) evict() {
	if len(c.cache) == 0 {
		return
	}

	var victimPath string

	switch c.policy {
	case "lru":
		// Remove least recently used (first in list)
		if len(c.lruList) > 0 {
			victimPath = c.lruList[0]
			c.lruList = c.lruList[1:]
		}
	case "lfu":
		// Remove least frequently used
		var minCount int64 = -1
		for path, entry := range c.cache {
			if minCount == -1 || entry.useCount < minCount {
				minCount = entry.useCount
				victimPath = path
			}
		}
	case "fifo":
		// Remove oldest entry
		var oldestTime time.Time
		for path, entry := range c.cache {
			if oldestTime.IsZero() || entry.addedAt.Before(oldestTime) {
				oldestTime = entry.addedAt
				victimPath = path
			}
		}
	default:
		// Fallback to LRU
		if len(c.lruList) > 0 {
			victimPath = c.lruList[0]
			c.lruList = c.lruList[1:]
		}
	}

	if victimPath != "" {
		delete(c.cache, victimPath)
		c.evictions++
		log.Debug("Evicted template from cache: %s (policy: %s)", victimPath, c.policy)
	}
}

// Clear removes all entries from the cache.
func (c *TemplateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cachedTemplate)
	c.lruList = make([]string, 0, c.maxSize)
	log.Info("Template cache cleared")
}

// Stats returns cache statistics.
func (c *TemplateCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Size:      len(c.cache),
		Evictions: c.evictions,
		Enabled:   c.enabled,
	}
}

// CacheStats contains cache performance metrics.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Size      int
	Evictions int64
	Enabled   bool
}

// globalTemplateCache is the global instance used by all template resources
var (
	globalTemplateCache   *TemplateCache
	globalTemplateCacheMu sync.RWMutex
)

// InitGlobalTemplateCache initializes the global template cache.
func InitGlobalTemplateCache(enabled bool, maxSize int, policy string) {
	globalTemplateCacheMu.Lock()
	defer globalTemplateCacheMu.Unlock()
	globalTemplateCache = NewTemplateCache(enabled, maxSize, policy)
	log.Info("Template cache initialized (enabled: %v, maxSize: %d, policy: %s)", enabled, maxSize, policy)
}

// GetGlobalTemplateCache returns the global template cache instance.
func GetGlobalTemplateCache() *TemplateCache {
	globalTemplateCacheMu.RLock()
	defer globalTemplateCacheMu.RUnlock()
	if globalTemplateCache == nil {
		// Initialize with defaults if not already initialized
		globalTemplateCacheMu.RUnlock()
		InitGlobalTemplateCache(true, 100, "lru")
		globalTemplateCacheMu.RLock()
	}
	return globalTemplateCache
}
