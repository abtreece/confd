package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestIncludeContext(t *testing.T) {
	t.Run("push and pop", func(t *testing.T) {
		ctx := NewIncludeContext()

		if err := ctx.Push("/path/to/template1.tmpl"); err != nil {
			t.Fatalf("unexpected error on first push: %v", err)
		}
		if ctx.Depth() != 1 {
			t.Errorf("expected depth 1, got %d", ctx.Depth())
		}

		if err := ctx.Push("/path/to/template2.tmpl"); err != nil {
			t.Fatalf("unexpected error on second push: %v", err)
		}
		if ctx.Depth() != 2 {
			t.Errorf("expected depth 2, got %d", ctx.Depth())
		}

		ctx.Pop()
		if ctx.Depth() != 1 {
			t.Errorf("expected depth 1 after pop, got %d", ctx.Depth())
		}

		ctx.Pop()
		if ctx.Depth() != 0 {
			t.Errorf("expected depth 0 after second pop, got %d", ctx.Depth())
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		ctx := NewIncludeContext()

		if err := ctx.Push("/path/to/template.tmpl"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err := ctx.Push("/path/to/template.tmpl")
		if err == nil {
			t.Error("expected cycle detection error")
		}
		if !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("expected cycle detection message, got: %v", err)
		}
	})

	t.Run("max depth", func(t *testing.T) {
		ctx := NewIncludeContext()

		// Push up to max depth
		for i := 0; i < maxIncludeDepth; i++ {
			if err := ctx.Push("/path/to/template" + string(rune('a'+i)) + ".tmpl"); err != nil {
				t.Fatalf("unexpected error at depth %d: %v", i, err)
			}
		}

		// Try to exceed max depth
		err := ctx.Push("/path/to/one-too-many.tmpl")
		if err == nil {
			t.Error("expected max depth error")
		}
		if !strings.Contains(err.Error(), "maximum include depth") {
			t.Errorf("expected max depth message, got: %v", err)
		}
	})
}

func TestNewIncludeFunc(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "include-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("simple include", func(t *testing.T) {
		// Create included template
		headerContent := `<!-- Header -->
<title>{{ .Title }}</title>
`
		if err := os.WriteFile(filepath.Join(tmpDir, "header.tmpl"), []byte(headerContent), 0644); err != nil {
			t.Fatalf("Failed to create header template: %v", err)
		}

		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		includeFunc := NewIncludeFunc(tmpDir, funcMap, ctx)

		result, err := includeFunc("header.tmpl", map[string]string{"Title": "Test Page"})
		if err != nil {
			t.Fatalf("include failed: %v", err)
		}

		if !strings.Contains(result, "Test Page") {
			t.Errorf("expected result to contain 'Test Page', got: %s", result)
		}
	})

	t.Run("nested include", func(t *testing.T) {
		// Create inner template
		innerContent := `Inner content`
		if err := os.WriteFile(filepath.Join(tmpDir, "inner.tmpl"), []byte(innerContent), 0644); err != nil {
			t.Fatalf("Failed to create inner template: %v", err)
		}

		// Create outer template that includes inner
		outerContent := `Outer: {{ include "inner.tmpl" }}`
		if err := os.WriteFile(filepath.Join(tmpDir, "outer.tmpl"), []byte(outerContent), 0644); err != nil {
			t.Fatalf("Failed to create outer template: %v", err)
		}

		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		funcMap["include"] = NewIncludeFunc(tmpDir, funcMap, ctx)

		// Parse and execute outer template
		tmpl, err := template.New("outer.tmpl").Funcs(funcMap).ParseFiles(filepath.Join(tmpDir, "outer.tmpl"))
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		var buf strings.Builder
		if err := tmpl.Execute(&buf, nil); err != nil {
			t.Fatalf("Failed to execute template: %v", err)
		}

		result := buf.String()
		if !strings.Contains(result, "Outer:") || !strings.Contains(result, "Inner content") {
			t.Errorf("unexpected result: %s", result)
		}
	})

	t.Run("include with subdirectory", func(t *testing.T) {
		// Create subdirectory
		subDir := filepath.Join(tmpDir, "partials")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		// Create template in subdirectory
		partialContent := `Partial content`
		if err := os.WriteFile(filepath.Join(subDir, "partial.tmpl"), []byte(partialContent), 0644); err != nil {
			t.Fatalf("Failed to create partial template: %v", err)
		}

		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		includeFunc := NewIncludeFunc(tmpDir, funcMap, ctx)

		result, err := includeFunc("partials/partial.tmpl")
		if err != nil {
			t.Fatalf("include failed: %v", err)
		}

		if result != "Partial content" {
			t.Errorf("expected 'Partial content', got: %s", result)
		}
	})

	t.Run("include not found", func(t *testing.T) {
		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		includeFunc := NewIncludeFunc(tmpDir, funcMap, ctx)

		_, err := includeFunc("nonexistent.tmpl")
		if err == nil {
			t.Error("expected error for nonexistent template")
		}
	})

	t.Run("directory traversal prevention", func(t *testing.T) {
		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		includeFunc := NewIncludeFunc(tmpDir, funcMap, ctx)

		_, err := includeFunc("../../../etc/passwd")
		if err == nil {
			t.Error("expected error for directory traversal")
		}
		if !strings.Contains(err.Error(), "outside template directory") {
			t.Errorf("expected directory traversal error, got: %v", err)
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		// Create template that includes itself
		cycleContent := `{{ include "cycle.tmpl" }}`
		if err := os.WriteFile(filepath.Join(tmpDir, "cycle.tmpl"), []byte(cycleContent), 0644); err != nil {
			t.Fatalf("Failed to create cycle template: %v", err)
		}

		funcMap := template.FuncMap{}
		ctx := NewIncludeContext()
		funcMap["include"] = NewIncludeFunc(tmpDir, funcMap, ctx)

		tmpl, err := template.New("cycle.tmpl").Funcs(funcMap).ParseFiles(filepath.Join(tmpDir, "cycle.tmpl"))
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		var buf strings.Builder
		err = tmpl.Execute(&buf, nil)
		if err == nil {
			t.Error("expected cycle detection error")
		}
		if !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("expected cycle detection message, got: %v", err)
		}
	})
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		dir      string
		expected bool
	}{
		{
			name:     "path within dir",
			path:     "/base/templates/file.tmpl",
			dir:      "/base/templates",
			expected: true,
		},
		{
			name:     "path in subdirectory",
			path:     "/base/templates/subdir/file.tmpl",
			dir:      "/base/templates",
			expected: true,
		},
		{
			name:     "path outside dir",
			path:     "/etc/passwd",
			dir:      "/base/templates",
			expected: false,
		},
		{
			name:     "path with traversal",
			path:     "/base/templates/../secrets/file",
			dir:      "/base/templates",
			expected: false,
		},
		{
			name:     "same path",
			path:     "/base/templates",
			dir:      "/base/templates",
			expected: false, // path is the dir itself, not within
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPathWithin(tt.path, tt.dir)
			if result != tt.expected {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v", tt.path, tt.dir, result, tt.expected)
			}
		})
	}
}
