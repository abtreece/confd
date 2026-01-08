package template

import (
"os"
"path/filepath"
"testing"
)

// BenchmarkTemplateCacheEnabled benchmarks template processing with cache enabled
func BenchmarkTemplateCacheEnabled(b *testing.B) {
tmpDir := b.TempDir()
tmplPath := filepath.Join(tmpDir, "bench.tmpl")
content := "Hello {{.Name}}"
if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
b.Fatalf("failed to create template: %v", err)
}

cache := NewTemplateCache(true, 100, "lru")
funcMap := make(map[string]interface{})

b.ResetTimer()
for i := 0; i < b.N; i++ {
_, err := cache.Get(tmplPath, funcMap)
if err != nil {
b.Fatalf("Get failed: %v", err)
}
}
}

// BenchmarkTemplateCacheDisabled benchmarks template processing with cache disabled
func BenchmarkTemplateCacheDisabled(b *testing.B) {
tmpDir := b.TempDir()
tmplPath := filepath.Join(tmpDir, "bench.tmpl")
content := "Hello {{.Name}}"
if err := os.WriteFile(tmplPath, []byte(content), 0644); err != nil {
b.Fatalf("failed to create template: %v", err)
}

cache := NewTemplateCache(false, 100, "lru")
funcMap := make(map[string]interface{})

b.ResetTimer()
for i := 0; i < b.N; i++ {
_, err := cache.Get(tmplPath, funcMap)
if err != nil {
b.Fatalf("Get failed: %v", err)
}
}
}
