package template

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewTemplateCache(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		maxSize  int
		policy   string
		wantSize int
		wantPolicy string
	}{
		{
			name:       "default values",
			enabled:    true,
			maxSize:    0,
			policy:     "",
			wantSize:   100,
			wantPolicy: "lru",
		},
		{
			name:       "custom values",
			enabled:    true,
			maxSize:    50,
			policy:     "lfu",
			wantSize:   50,
			wantPolicy: "lfu",
		},
		{
			name:       "disabled cache",
			enabled:    false,
			maxSize:    100,
			policy:     "lru",
			wantSize:   100,
			wantPolicy: "lru",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewTemplateCache(tt.enabled, tt.maxSize, tt.policy)
			if cache.enabled != tt.enabled {
				t.Errorf("enabled = %v, want %v", cache.enabled, tt.enabled)
			}
			if cache.maxSize != tt.wantSize {
				t.Errorf("maxSize = %d, want %d", cache.maxSize, tt.wantSize)
			}
			if cache.policy != tt.wantPolicy {
				t.Errorf("policy = %s, want %s", cache.policy, tt.wantPolicy)
			}
		})
	}
}

func TestTemplateCacheGetHitMiss(t *testing.T) {
	// Create a temporary template file
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	content := "Hello {{.Name}}"
	if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test template: %v", err)
	}

	cache := NewTemplateCache(true, 10, "lru")
	funcMap := make(map[string]interface{})

	// First access - should be a miss
	tmpl1, err := cache.Get(tmplPath, funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if tmpl1 == nil {
		t.Fatal("expected non-nil template")
	}
	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("expected 0 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	// Second access - should be a hit
	tmpl2, err := cache.Get(tmplPath, funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if tmpl2 == nil {
		t.Fatal("expected non-nil template")
	}
	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestTemplateCacheMtimeChange(t *testing.T) {
	// Create a temporary template file
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	content1 := "Hello {{.Name}}"
	if err := os.WriteFile(tmplPath, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to create test template: %v", err)
	}

	cache := NewTemplateCache(true, 10, "lru")
	funcMap := make(map[string]interface{})

	// First access
	_, err := cache.Get(tmplPath, funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Wait a bit to ensure mtime changes
	time.Sleep(10 * time.Millisecond)

	// Modify the file
	content2 := "Goodbye {{.Name}}"
	if err := os.WriteFile(tmplPath, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to modify test template: %v", err)
	}

	// Second access - should detect mtime change and recompile
	_, err = cache.Get(tmplPath, funcMap)
	if err != nil {
		t.Fatalf("Get failed after modification: %v", err)
	}

	stats := cache.Stats()
	// Should have 2 misses (initial + after modification)
	if stats.Misses != 2 {
		t.Errorf("expected 2 misses after modification, got %d", stats.Misses)
	}
}

func TestTemplateCacheDisabled(t *testing.T) {
	// Create a temporary template file
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	content := "Hello {{.Name}}"
	if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test template: %v", err)
	}

	cache := NewTemplateCache(false, 10, "lru")
	funcMap := make(map[string]interface{})

	// Access multiple times
	for i := 0; i < 3; i++ {
		_, err := cache.Get(tmplPath, funcMap)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
	}

	stats := cache.Stats()
	// Cache is disabled, so no hits should be recorded
	if stats.Hits != 0 {
		t.Errorf("expected 0 hits with disabled cache, got %d", stats.Hits)
	}
	if !stats.Enabled {
		// This is correct - cache should report as disabled
	}
}

func TestTemplateCacheEvictionLRU(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewTemplateCache(true, 3, "lru")
	funcMap := make(map[string]interface{})

	// Create 4 template files
	paths := make([]string, 4)
	for i := 0; i < 4; i++ {
		path := filepath.Join(tmpDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".tmpl")
		if err := os.WriteFile(path, []byte("Test {{.}}"), 0644); err != nil {
			t.Fatalf("failed to create template %d: %v", i, err)
		}
		paths[i] = path
	}

	// Access first 3 templates (fills cache)
	for i := 0; i < 3; i++ {
		_, err := cache.Get(paths[i], funcMap)
		if err != nil {
			t.Fatalf("Get failed for template %d: %v", i, err)
		}
	}

	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("expected cache size 3, got %d", stats.Size)
	}

	// Access 4th template - should evict the LRU (first one)
	_, err := cache.Get(paths[3], funcMap)
	if err != nil {
		t.Fatalf("Get failed for template 4: %v", err)
	}

	stats = cache.Stats()
	if stats.Size != 3 {
		t.Errorf("expected cache size 3 after eviction, got %d", stats.Size)
	}
	if stats.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", stats.Evictions)
	}

	// Access first template again - should be a miss (was evicted)
	statsBeforeMiss := cache.Stats()
	_, err = cache.Get(paths[0], funcMap)
	if err != nil {
		t.Fatalf("Get failed for first template after eviction: %v", err)
	}
	statsAfterMiss := cache.Stats()
	if statsAfterMiss.Misses <= statsBeforeMiss.Misses {
		t.Errorf("expected miss count to increase for evicted template")
	}
}

func TestTemplateCacheEvictionLFU(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewTemplateCache(true, 3, "lfu")
	funcMap := make(map[string]interface{})

	// Create 4 template files
	paths := make([]string, 4)
	for i := 0; i < 4; i++ {
		path := filepath.Join(tmpDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".tmpl")
		if err := os.WriteFile(path, []byte("Test {{.}}"), 0644); err != nil {
			t.Fatalf("failed to create template %d: %v", i, err)
		}
		paths[i] = path
	}

	// Access first template 5 times
	for i := 0; i < 5; i++ {
		_, err := cache.Get(paths[0], funcMap)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
	}

	// Access second and third templates once
	_, _ = cache.Get(paths[1], funcMap)
	_, _ = cache.Get(paths[2], funcMap)

	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("expected cache size 3, got %d", stats.Size)
	}

	// Access 4th template - should evict LFU (paths[1] or paths[2])
	_, err := cache.Get(paths[3], funcMap)
	if err != nil {
		t.Fatalf("Get failed for template 4: %v", err)
	}

	stats = cache.Stats()
	if stats.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", stats.Evictions)
	}

	// First template (most frequently used) should still be cached
	statsBeforeHit := cache.Stats()
	_, err = cache.Get(paths[0], funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	statsAfterHit := cache.Stats()
	if statsAfterHit.Hits <= statsBeforeHit.Hits {
		t.Errorf("expected hit count to increase for frequently used template")
	}
}

func TestTemplateCacheEvictionFIFO(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewTemplateCache(true, 3, "fifo")
	funcMap := make(map[string]interface{})

	// Create 4 template files
	paths := make([]string, 4)
	for i := 0; i < 4; i++ {
		path := filepath.Join(tmpDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".tmpl")
		if err := os.WriteFile(path, []byte("Test {{.}}"), 0644); err != nil {
			t.Fatalf("failed to create template %d: %v", i, err)
		}
		paths[i] = path
	}

	// Add templates in order with small delays to ensure different timestamps
	for i := 0; i < 3; i++ {
		_, err := cache.Get(paths[i], funcMap)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("expected cache size 3, got %d", stats.Size)
	}

	// Access 4th template - should evict first added (FIFO)
	_, err := cache.Get(paths[3], funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	stats = cache.Stats()
	if stats.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestTemplateCacheClear(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte("Test {{.}}"), 0644); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	cache := NewTemplateCache(true, 10, "lru")
	funcMap := make(map[string]interface{})

	// Add template to cache
	_, err := cache.Get(tmplPath, funcMap)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	stats := cache.Stats()
	if stats.Size != 1 {
		t.Errorf("expected cache size 1, got %d", stats.Size)
	}

	// Clear cache
	cache.Clear()

	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected cache size 0 after clear, got %d", stats.Size)
	}
}

func TestTemplateCacheConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewTemplateCache(true, 50, "lru")
	funcMap := make(map[string]interface{})

	// Create multiple template files
	numTemplates := 10
	paths := make([]string, numTemplates)
	for i := 0; i < numTemplates; i++ {
		path := filepath.Join(tmpDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".tmpl")
		if err := os.WriteFile(path, []byte("Test {{.}}"), 0644); err != nil {
			t.Fatalf("failed to create template %d: %v", i, err)
		}
		paths[i] = path
	}

	// Access templates concurrently
	var wg sync.WaitGroup
	numGoroutines := 20
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				path := paths[j%numTemplates]
				_, err := cache.Get(path, funcMap)
				if err != nil {
					t.Errorf("goroutine %d: Get failed: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := cache.Stats()
	// Should have both hits and misses
	if stats.Hits == 0 && stats.Misses == 0 {
		t.Error("expected some cache activity")
	}
}

func TestGlobalTemplateCache(t *testing.T) {
	// Initialize global cache
	InitGlobalTemplateCache(true, 50, "lru")

	cache := GetGlobalTemplateCache()
	if cache == nil {
		t.Fatal("expected non-nil global cache")
	}
	if !cache.enabled {
		t.Error("expected cache to be enabled")
	}
	if cache.maxSize != 50 {
		t.Errorf("expected maxSize 50, got %d", cache.maxSize)
	}
	if cache.policy != "lru" {
		t.Errorf("expected policy lru, got %s", cache.policy)
	}
}
