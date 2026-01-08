package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDiff(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		oldContent  string
		newContent  string
		expectEmpty bool
		expectAdded bool
		expectRemoved bool
	}{
		{
			name:        "identical files",
			oldContent:  "line1\nline2\nline3\n",
			newContent:  "line1\nline2\nline3\n",
			expectEmpty: true,
		},
		{
			name:        "line added",
			oldContent:  "line1\nline2\n",
			newContent:  "line1\nline2\nline3\n",
			expectEmpty: false,
			expectAdded: true,
		},
		{
			name:        "line removed",
			oldContent:  "line1\nline2\nline3\n",
			newContent:  "line1\nline2\n",
			expectEmpty: false,
			expectRemoved: true,
		},
		{
			name:        "line changed",
			oldContent:  "line1\nline2\nline3\n",
			newContent:  "line1\nmodified\nline3\n",
			expectEmpty: false,
			expectAdded: true,
			expectRemoved: true,
		},
		{
			name:        "empty old file",
			oldContent:  "",
			newContent:  "new content\n",
			expectEmpty: false,
			expectAdded: true,
		},
		{
			name:        "empty new file",
			oldContent:  "old content\n",
			newContent:  "",
			expectEmpty: false,
			expectRemoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldPath := filepath.Join(tmpDir, "old")
			newPath := filepath.Join(tmpDir, "new")

			if tt.oldContent != "" {
				if err := os.WriteFile(oldPath, []byte(tt.oldContent), 0644); err != nil {
					t.Fatalf("Failed to write old file: %v", err)
				}
			} else {
				os.Remove(oldPath)
			}

			if err := os.WriteFile(newPath, []byte(tt.newContent), 0644); err != nil {
				t.Fatalf("Failed to write new file: %v", err)
			}

			diff, err := GenerateDiff(newPath, oldPath, 3)
			if err != nil {
				t.Fatalf("GenerateDiff failed: %v", err)
			}

			if tt.expectEmpty && diff != "" {
				t.Errorf("Expected empty diff, got:\n%s", diff)
			}
			if !tt.expectEmpty && diff == "" {
				t.Error("Expected non-empty diff, got empty")
			}
			if tt.expectAdded && !strings.Contains(diff, "+") {
				t.Error("Expected added lines (+) in diff")
			}
			if tt.expectRemoved && !strings.Contains(diff, "-") {
				t.Error("Expected removed lines (-) in diff")
			}
		})
	}
}

func TestGenerateDiff_NonExistentDest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "new")
	destPath := filepath.Join(tmpDir, "nonexistent")

	if err := os.WriteFile(srcPath, []byte("new content\n"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	diff, err := GenerateDiff(srcPath, destPath, 3)
	if err != nil {
		t.Fatalf("GenerateDiff failed: %v", err)
	}

	if !strings.Contains(diff, "+new content") {
		t.Errorf("Expected diff to show added content, got:\n%s", diff)
	}
}

func TestColorizeDiff(t *testing.T) {
	diff := `--- /etc/nginx/nginx.conf
+++ /tmp/.nginx.conf.12345
@@ -1,3 +1,4 @@
 server {
-    listen 80;
+    listen 8080;
+    listen 443 ssl;
 }
`

	colorized := ColorizeDiff(diff)

	// Check that color codes are present
	if !strings.Contains(colorized, "\033[32m+") {
		t.Error("Expected green color for added lines")
	}
	if !strings.Contains(colorized, "\033[31m-") {
		t.Error("Expected red color for removed lines")
	}
	if !strings.Contains(colorized, "\033[36m@@") {
		t.Error("Expected cyan color for hunk headers")
	}
	if !strings.Contains(colorized, "\033[1m---") {
		t.Error("Expected bold for file headers")
	}
}

func TestColorizeDiff_Empty(t *testing.T) {
	result := ColorizeDiff("")
	if result != "" {
		t.Errorf("Expected empty string, got: %s", result)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "unix endings",
			input:    "line1\nline2\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "windows endings",
			input:    "line1\r\nline2\r\nline3\r\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "mixed endings",
			input:    "line1\r\nline2\nline3\r\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "no trailing newline",
			input:    "line1\nline2",
			expected: []string{"line1", "line2"},
		},
		{
			name:     "empty",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines([]byte(tt.input))
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Line %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestComputeLCS(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "identical",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "partial match",
			a:        []string{"a", "b", "c", "d"},
			b:        []string{"a", "x", "c", "d"},
			expected: []string{"a", "c", "d"},
		},
		{
			name:     "no match",
			a:        []string{"a", "b"},
			b:        []string{"c", "d"},
			expected: nil,
		},
		{
			name:     "empty first",
			a:        []string{},
			b:        []string{"a", "b"},
			expected: nil,
		},
		{
			name:     "empty second",
			a:        []string{"a", "b"},
			b:        []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeLCS(tt.a, tt.b)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Expected %v, got %v", tt.expected, result)
					break
				}
			}
		})
	}
}
