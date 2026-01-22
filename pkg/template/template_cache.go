package template

import (
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
)

// cachedTemplate stores a compiled template with its modification time
type cachedTemplate struct {
	tmpl     *template.Template
	mtime    time.Time
	statTime time.Time // When we last verified mtime via os.Stat()
}

// TemplateCache caches compiled Go templates keyed by file path
type TemplateCache struct {
	mu      sync.RWMutex
	cache   map[string]*cachedTemplate
	enabled bool
	statTTL time.Duration // TTL for stat cache (0 = always stat)
}

var globalTemplateCache *TemplateCache

// InitTemplateCache initializes the global template cache
func InitTemplateCache(enabled bool, statTTL time.Duration) {
	globalTemplateCache = &TemplateCache{
		cache:   make(map[string]*cachedTemplate),
		enabled: enabled,
		statTTL: statTTL,
	}
	if enabled {
		log.Debug("Template cache enabled with stat TTL %v", statTTL)
	} else {
		log.Debug("Template cache disabled")
	}
}

// GetCachedTemplate returns a cached template if valid, or nil if miss/stale
func GetCachedTemplate(path string) (*template.Template, bool) {
	if globalTemplateCache == nil || !globalTemplateCache.enabled {
		return nil, false
	}

	globalTemplateCache.mu.RLock()
	cached, ok := globalTemplateCache.cache[path]
	statTTL := globalTemplateCache.statTTL
	var cachedStatTime time.Time
	if ok {
		cachedStatTime = cached.statTime
	}
	globalTemplateCache.mu.RUnlock()

	if !ok {
		if metrics.Enabled() {
			metrics.TemplateCacheMisses.Inc()
		}
		return nil, false
	}

	// Skip stat if checked recently (within TTL)
	if statTTL > 0 && time.Since(cachedStatTime) < statTTL {
		if metrics.Enabled() {
			metrics.TemplateCacheHits.Inc()
			metrics.TemplateStatCacheHits.Inc()
		}
		return cached.tmpl, true
	}

	// Check if file mtime changed
	stat, err := os.Stat(path)
	if err != nil || !stat.ModTime().Equal(cached.mtime) {
		if metrics.Enabled() {
			metrics.TemplateCacheMisses.Inc()
		}
		return nil, false
	}

	// Update statTime (requires write lock)
	globalTemplateCache.mu.Lock()
	if c, ok := globalTemplateCache.cache[path]; ok {
		c.statTime = time.Now()
	}
	globalTemplateCache.mu.Unlock()

	if metrics.Enabled() {
		metrics.TemplateCacheHits.Inc()
	}
	return cached.tmpl, true
}

// PutCachedTemplate stores a compiled template
func PutCachedTemplate(path string, tmpl *template.Template, mtime time.Time) {
	if globalTemplateCache == nil || !globalTemplateCache.enabled {
		return
	}

	globalTemplateCache.mu.Lock()
	defer globalTemplateCache.mu.Unlock()
	globalTemplateCache.cache[path] = &cachedTemplate{
		tmpl:     tmpl,
		mtime:    mtime,
		statTime: time.Now(), // Just verified via os.Stat in caller
	}
}

// ClearTemplateCache removes all cached templates
func ClearTemplateCache() {
	if globalTemplateCache == nil {
		return
	}
	globalTemplateCache.mu.Lock()
	defer globalTemplateCache.mu.Unlock()
	globalTemplateCache.cache = make(map[string]*cachedTemplate)
	log.Debug("Template cache cleared")
}

// TemplateCacheSize returns the number of cached templates
func TemplateCacheSize() int {
	if globalTemplateCache == nil {
		return 0
	}
	globalTemplateCache.mu.RLock()
	defer globalTemplateCache.mu.RUnlock()
	return len(globalTemplateCache.cache)
}

// TemplateCacheEnabled returns whether the template cache is enabled
func TemplateCacheEnabled() bool {
	return globalTemplateCache != nil && globalTemplateCache.enabled
}
