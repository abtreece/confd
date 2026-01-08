package template

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/abtreece/confd/pkg/log"
)

func TestInitTemplateCache(t *testing.T) {
	log.SetLevel("warn")

	// Test enabling cache
	InitTemplateCache(true)
	if !TemplateCacheEnabled() {
		t.Error("Expected cache to be enabled")
	}

	// Test disabling cache
	InitTemplateCache(false)
	if TemplateCacheEnabled() {
		t.Error("Expected cache to be disabled")
	}

	// Re-enable for other tests
	InitTemplateCache(true)
}

func TestGetCachedTemplateMiss(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)
	ClearTemplateCache()

	// Non-existent path should return miss
	tmpl, hit := GetCachedTemplate("/nonexistent/path.tmpl")
	if hit {
		t.Error("Expected cache miss for non-existent path")
	}
	if tmpl != nil {
		t.Error("Expected nil template on cache miss")
	}
}

func TestPutAndGetCachedTemplate(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)
	ClearTemplateCache()

	// Create a temp file for mtime checking
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmpFile, []byte("{{.}}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	stat, _ := os.Stat(tmpFile)
	mtime := stat.ModTime()

	// Create and cache a template
	tmpl, err := template.New("test").Parse("{{.}}")
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	PutCachedTemplate(tmpFile, tmpl, mtime)

	// Should get cache hit
	cached, hit := GetCachedTemplate(tmpFile)
	if !hit {
		t.Error("Expected cache hit")
	}
	if cached != tmpl {
		t.Error("Expected same template instance from cache")
	}

	// Verify cache size
	if TemplateCacheSize() != 1 {
		t.Errorf("Expected cache size 1, got %d", TemplateCacheSize())
	}
}

func TestCachedTemplateMtimeInvalidation(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)
	ClearTemplateCache()

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmpFile, []byte("{{.}}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	stat, _ := os.Stat(tmpFile)
	mtime := stat.ModTime()

	// Cache template
	tmpl, _ := template.New("test").Parse("{{.}}")
	PutCachedTemplate(tmpFile, tmpl, mtime)

	// Should hit before modification
	_, hit := GetCachedTemplate(tmpFile)
	if !hit {
		t.Error("Expected cache hit before modification")
	}

	// Modify file (change mtime)
	time.Sleep(10 * time.Millisecond) // Ensure mtime difference
	if err := os.WriteFile(tmpFile, []byte("{{.}} modified"), 0644); err != nil {
		t.Fatalf("Failed to modify temp file: %v", err)
	}

	// Should miss after modification
	_, hit = GetCachedTemplate(tmpFile)
	if hit {
		t.Error("Expected cache miss after file modification")
	}
}

func TestClearTemplateCache(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmpFile, []byte("{{.}}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	stat, _ := os.Stat(tmpFile)
	mtime := stat.ModTime()

	// Cache a template
	tmpl, _ := template.New("test").Parse("{{.}}")
	PutCachedTemplate(tmpFile, tmpl, mtime)

	if TemplateCacheSize() != 1 {
		t.Errorf("Expected cache size 1, got %d", TemplateCacheSize())
	}

	// Clear cache
	ClearTemplateCache()

	if TemplateCacheSize() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", TemplateCacheSize())
	}

	// Should miss after clear
	_, hit := GetCachedTemplate(tmpFile)
	if hit {
		t.Error("Expected cache miss after clear")
	}
}

func TestCacheDisabled(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(false)

	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmpFile, []byte("{{.}}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	stat, _ := os.Stat(tmpFile)
	mtime := stat.ModTime()

	// Try to cache a template
	tmpl, _ := template.New("test").Parse("{{.}}")
	PutCachedTemplate(tmpFile, tmpl, mtime)

	// Should always miss when disabled
	_, hit := GetCachedTemplate(tmpFile)
	if hit {
		t.Error("Expected cache miss when disabled")
	}

	// Cache size should be 0
	if TemplateCacheSize() != 0 {
		t.Errorf("Expected cache size 0 when disabled, got %d", TemplateCacheSize())
	}
}

func TestCacheNilGlobalCache(t *testing.T) {
	log.SetLevel("warn")

	// Set global cache to nil
	globalTemplateCache = nil

	// Operations should not panic
	_, hit := GetCachedTemplate("/some/path.tmpl")
	if hit {
		t.Error("Expected miss with nil cache")
	}

	tmpl, _ := template.New("test").Parse("{{.}}")
	PutCachedTemplate("/some/path.tmpl", tmpl, time.Now())

	ClearTemplateCache()

	if TemplateCacheSize() != 0 {
		t.Error("Expected size 0 with nil cache")
	}

	if TemplateCacheEnabled() {
		t.Error("Expected disabled with nil cache")
	}

	// Restore for other tests
	InitTemplateCache(true)
}

func TestCacheConcurrency(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)
	ClearTemplateCache()

	// Create temp files
	tmpDir := t.TempDir()
	numFiles := 10
	files := make([]string, numFiles)
	mtimes := make([]time.Time, numFiles)

	for i := 0; i < numFiles; i++ {
		files[i] = filepath.Join(tmpDir, "test"+string(rune('0'+i))+".tmpl")
		if err := os.WriteFile(files[i], []byte("{{.}}"), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		stat, _ := os.Stat(files[i])
		mtimes[i] = stat.ModTime()
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent puts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fileIdx := idx % numFiles
			tmpl, _ := template.New("test").Parse("{{.}}")
			PutCachedTemplate(files[fileIdx], tmpl, mtimes[fileIdx])
		}(i)
	}

	wg.Wait()

	// Concurrent gets
	hits := make([]bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fileIdx := idx % numFiles
			_, hits[idx] = GetCachedTemplate(files[fileIdx])
		}(i)
	}

	wg.Wait()

	// All gets should be hits
	for i, hit := range hits {
		if !hit {
			t.Errorf("Goroutine %d expected cache hit", i)
		}
	}

	// Cache should have exactly numFiles entries
	if TemplateCacheSize() != numFiles {
		t.Errorf("Expected cache size %d, got %d", numFiles, TemplateCacheSize())
	}
}

func TestCacheFileStatError(t *testing.T) {
	log.SetLevel("warn")
	InitTemplateCache(true)
	ClearTemplateCache()

	// Cache a template for a file that will be deleted
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmpFile, []byte("{{.}}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	stat, _ := os.Stat(tmpFile)
	mtime := stat.ModTime()

	tmpl, _ := template.New("test").Parse("{{.}}")
	PutCachedTemplate(tmpFile, tmpl, mtime)

	// Delete the file
	os.Remove(tmpFile)

	// Should miss because os.Stat will fail
	_, hit := GetCachedTemplate(tmpFile)
	if hit {
		t.Error("Expected cache miss when file is deleted")
	}
}
