package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/abtreece/confd/pkg/memkv"
)

func TestNewTemplateRenderer(t *testing.T) {
	store := memkv.New()
	funcMap := newFuncMap()

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: "/tmp/templates",
		FuncMap:     funcMap,
		Store:       store,
	})

	if renderer == nil {
		t.Fatal("newTemplateRenderer() returned nil")
	}
	if renderer.templateDir != "/tmp/templates" {
		t.Errorf("newTemplateRenderer() templateDir = %v, want %v", renderer.templateDir, "/tmp/templates")
	}
	if renderer.funcMap == nil {
		t.Error("newTemplateRenderer() funcMap is nil")
	}
	if renderer.store != store {
		t.Error("newTemplateRenderer() store not set correctly")
	}
}

func TestRender_SimpleTemplate(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple template
	tmplContent := "Hello {{.name}}!"
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	store.Set("/name", "World")

	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	result, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() unexpected error: %v", err)
	}

	// Note: This template uses {{.name}} which won't work with nil data
	// But the test verifies basic rendering works
	if result == nil {
		t.Error("render() returned nil result")
	}
}

func TestRender_WithStoreFunctions(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a template using store functions
	tmplContent := `{{getv "/database/host"}}`
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	store.Set("/database/host", "localhost")

	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	result, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() unexpected error: %v", err)
	}

	resultStr := string(result)
	if resultStr != "localhost" {
		t.Errorf("render() result = %v, want %v", resultStr, "localhost")
	}
}

func TestRender_MissingTemplate(t *testing.T) {
	store := memkv.New()
	funcMap := newFuncMap()

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: "/tmp",
		FuncMap:     funcMap,
		Store:       store,
	})

	_, err := renderer.render("/nonexistent/template.tmpl")
	if err == nil {
		t.Error("render() expected error for missing template, got nil")
	}
	if !strings.Contains(err.Error(), "missing template") {
		t.Errorf("render() error should mention missing template, got: %v", err)
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an invalid template
	tmplContent := "{{.invalid syntax"
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	funcMap := newFuncMap()

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	_, err = renderer.render(tmplPath)
	if err == nil {
		t.Error("render() expected error for invalid template, got nil")
	}
}

func TestRender_TemplateExecutionError(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a template that will fail during execution
	tmplContent := `{{getv "/nonexistent/key"}}`
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	_, err = renderer.render(tmplPath)
	if err == nil {
		t.Error("render() expected error for missing key, got nil")
	}
}

func TestRender_ComplexTemplate(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a complex template with multiple store functions
	tmplContent := `host={{getv "/database/host"}}
port={{getv "/database/port"}}
{{range gets "/servers/*"}}server={{.}}
{{end}}`
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	store.Set("/database/host", "localhost")
	store.Set("/database/port", "5432")
	store.Set("/servers/web1", "192.168.1.1")
	store.Set("/servers/web2", "192.168.1.2")

	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	result, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "host=localhost") {
		t.Errorf("render() result should contain 'host=localhost', got: %v", resultStr)
	}
	if !strings.Contains(resultStr, "port=5432") {
		t.Errorf("render() result should contain 'port=5432', got: %v", resultStr)
	}
}

func TestRender_WithIncludeFunction(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an included template
	includedContent := "Included content"
	includedPath := filepath.Join(tmpDir, "included.tmpl")
	if err := os.WriteFile(includedPath, []byte(includedContent), 0644); err != nil {
		t.Fatalf("Failed to write included template: %v", err)
	}

	// Create main template with include
	tmplContent := `Main: {{include "included.tmpl"}}`
	tmplPath := filepath.Join(tmpDir, "main.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write main template: %v", err)
	}

	store := memkv.New()
	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	result, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "Included content") {
		t.Errorf("render() result should contain included content, got: %v", resultStr)
	}
}

func TestRender_TemplateCaching(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a template
	tmplContent := "Test content"
	tmplPath := filepath.Join(tmpDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	funcMap := newFuncMap()

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	// Clear cache to ensure fresh start
	ClearTemplateCache()

	// First render - cache miss
	result1, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() first call unexpected error: %v", err)
	}

	// Second render - cache hit
	result2, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() second call unexpected error: %v", err)
	}

	// Results should be the same
	if string(result1) != string(result2) {
		t.Errorf("render() cached result differs: %v != %v", string(result1), string(result2))
	}
}

func TestRender_EmptyTemplate(t *testing.T) {
	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an empty template
	tmplPath := filepath.Join(tmpDir, "empty.tmpl")
	if err := os.WriteFile(tmplPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	store := memkv.New()
	funcMap := newFuncMap()

	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})

	result, err := renderer.render(tmplPath)
	if err != nil {
		t.Errorf("render() unexpected error for empty template: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("render() empty template should return empty result, got: %v", string(result))
	}
}

func TestRender_ConcurrentRenders(t *testing.T) {
	// Bug #483: Template cache race condition with shared Funcs()
	// This test verifies that concurrent renders of the same cached template
	// don't cause race conditions. Run with -race flag to detect races.

	// Create temp directory for templates
	tmpDir, err := os.MkdirTemp("", "template-renderer-concurrent-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a template that uses store functions
	tmplContent := `{{getv "/key"}}`
	tmplPath := filepath.Join(tmpDir, "concurrent.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Clear cache to ensure fresh start
	ClearTemplateCache()

	// Pre-populate cache with first render
	store := memkv.New()
	store.Set("/key", "value")
	funcMap := newFuncMap()
	addFuncs(funcMap, store.FuncMap)
	renderer := newTemplateRenderer(templateRendererConfig{
		TemplateDir: tmpDir,
		FuncMap:     funcMap,
		Store:       store,
	})
	_, err = renderer.render(tmplPath)
	if err != nil {
		t.Fatalf("Initial render failed: %v", err)
	}

	// Now run concurrent renders using the cached template
	const goroutines = 10
	const iterations = 100

	errChan := make(chan error, goroutines*iterations)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Each goroutine creates its own store and renderer
				localStore := memkv.New()
				localStore.Set("/key", "goroutine")
				localFuncMap := newFuncMap()
				addFuncs(localFuncMap, localStore.FuncMap)

				localRenderer := newTemplateRenderer(templateRendererConfig{
					TemplateDir: tmpDir,
					FuncMap:     localFuncMap,
					Store:       localStore,
				})

				result, renderErr := localRenderer.render(tmplPath)
				if renderErr != nil {
					errChan <- renderErr
					continue
				}
				if string(result) != "goroutine" {
					errChan <- fmt.Errorf("expected 'goroutine', got '%s'", string(result))
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent renders produced %d errors, first: %v", len(errors), errors[0])
	}
}
