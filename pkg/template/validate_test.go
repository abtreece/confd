package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateResourceFile(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "confd-validate-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	templateDir := filepath.Join(tmpDir, "templates")
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("Failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest: %v", err)
	}

	// Create a template file
	templateFile := filepath.Join(templateDir, "test.tmpl")
	if err := os.WriteFile(templateFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	tests := []struct {
		name        string
		content     string
		expectError bool
		errorFields []string
	}{
		{
			name: "valid config",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`,
			expectError: false,
		},
		{
			name: "missing src",
			content: `[template]
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`,
			expectError: true,
			errorFields: []string{"src"},
		},
		{
			name: "missing dest",
			content: `[template]
src = "test.tmpl"
keys = ["/app/test"]
`,
			expectError: true,
			errorFields: []string{"dest"},
		},
		{
			name: "missing keys",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
`,
			expectError: true,
			errorFields: []string{"keys"},
		},
		{
			name: "invalid mode",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
mode = "invalid"
`,
			expectError: true,
			errorFields: []string{"mode"},
		},
		{
			name: "valid octal mode",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
mode = "0644"
`,
			expectError: false,
		},
		{
			name: "template not found",
			content: `[template]
src = "nonexistent.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`,
			expectError: true,
			errorFields: []string{"src"},
		},
		{
			name: "dest dir not found",
			content: `[template]
src = "test.tmpl"
dest = "/nonexistent/path/test.conf"
keys = ["/app/test"]
`,
			expectError: true,
			errorFields: []string{"dest"},
		},
		{
			name: "empty key in array",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test", ""]
`,
			expectError: true,
			errorFields: []string{"keys[1]"},
		},
		{
			name: "valid config with backend section",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]

[backend]
backend = "env"
`,
			expectError: false,
		},
		{
			name: "backend section missing backend type",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]

[backend]
nodes = ["127.0.0.1:8500"]
`,
			expectError: true,
			errorFields: []string{"backend.backend"},
		},
		{
			name: "unknown backend type",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]

[backend]
backend = "unknown"
`,
			expectError: true,
			errorFields: []string{"backend.backend"},
		},
		{
			name: "valid output_format json",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "json"
`,
			expectError: false,
		},
		{
			name: "valid output_format yaml",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "yaml"
`,
			expectError: false,
		},
		{
			name: "valid output_format yml",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "yml"
`,
			expectError: false,
		},
		{
			name: "valid output_format toml",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "toml"
`,
			expectError: false,
		},
		{
			name: "valid output_format xml",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "xml"
`,
			expectError: false,
		},
		{
			name: "invalid output_format",
			content: `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
output_format = "invalid"
`,
			expectError: true,
			errorFields: []string{"output_format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write the test config file
			configFile := filepath.Join(confDir, "test.toml")
			if err := os.WriteFile(configFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}
			defer os.Remove(configFile)

			errs := validateResourceFile(configFile, templateDir)

			if tt.expectError && len(errs) == 0 {
				t.Error("Expected validation errors but got none")
			}
			if !tt.expectError && len(errs) > 0 {
				t.Errorf("Expected no validation errors but got: %v", errs)
			}

			// Check that expected error fields are present
			if tt.expectError {
				for _, expectedField := range tt.errorFields {
					found := false
					for _, err := range errs {
						if err.Field == expectedField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field %q but didn't find one. Got errors: %v", expectedField, errs)
					}
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "confd-validate-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	templateDir := filepath.Join(tmpDir, "templates")
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("Failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest: %v", err)
	}

	// Create template files
	if err := os.WriteFile(filepath.Join(templateDir, "valid.tmpl"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	t.Run("all configs valid", func(t *testing.T) {
		// Write valid config
		content := `[template]
src = "valid.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`
		if err := os.WriteFile(filepath.Join(confDir, "valid.toml"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		defer os.Remove(filepath.Join(confDir, "valid.toml"))

		err := ValidateConfig(tmpDir, "")
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
	})

	t.Run("specific resource file", func(t *testing.T) {
		content := `[template]
src = "valid.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`
		configFile := filepath.Join(confDir, "specific.toml")
		if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		defer os.Remove(configFile)

		err := ValidateConfig(tmpDir, "specific.toml")
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
	})

	t.Run("specific resource file not found", func(t *testing.T) {
		err := ValidateConfig(tmpDir, "nonexistent.toml")
		if err == nil {
			t.Error("Expected error for nonexistent resource file")
		}
	})

	t.Run("config dir not found", func(t *testing.T) {
		err := ValidateConfig("/nonexistent/path", "")
		if err == nil {
			t.Error("Expected error for nonexistent config directory")
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("with field", func(t *testing.T) {
		err := ValidationError{
			File:    "/path/to/config.toml",
			Field:   "src",
			Message: "required field is missing",
		}
		expected := "/path/to/config.toml: src: required field is missing"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("without field", func(t *testing.T) {
		err := ValidationError{
			File:    "/path/to/config.toml",
			Message: "TOML parse error",
		}
		expected := "/path/to/config.toml: TOML parse error"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})
}

func TestValidateTemplates(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "confd-validate-templates-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	templateDir := filepath.Join(tmpDir, "templates")
	destDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("Failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest: %v", err)
	}

	t.Run("valid template syntax", func(t *testing.T) {
		// Create template file with valid syntax
		templateContent := `server {
    listen {{ getv "/port" }};
    server_name {{ getv "/hostname" }};
}
`
		if err := os.WriteFile(filepath.Join(templateDir, "nginx.conf.tmpl"), []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}

		// Create resource file
		resourceContent := `[template]
src = "nginx.conf.tmpl"
dest = "` + filepath.Join(destDir, "nginx.conf") + `"
keys = ["/nginx"]
`
		if err := os.WriteFile(filepath.Join(confDir, "nginx.toml"), []byte(resourceContent), 0644); err != nil {
			t.Fatalf("Failed to create resource: %v", err)
		}
		defer os.Remove(filepath.Join(confDir, "nginx.toml"))

		err := ValidateTemplates(tmpDir, "", "")
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		// Create template file with invalid syntax
		templateContent := `server {
    listen {{ getv "/port" };
}
`
		if err := os.WriteFile(filepath.Join(templateDir, "bad.conf.tmpl"), []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
		defer os.Remove(filepath.Join(templateDir, "bad.conf.tmpl"))

		// Create resource file
		resourceContent := `[template]
src = "bad.conf.tmpl"
dest = "` + filepath.Join(destDir, "bad.conf") + `"
keys = ["/app"]
`
		if err := os.WriteFile(filepath.Join(confDir, "bad.toml"), []byte(resourceContent), 0644); err != nil {
			t.Fatalf("Failed to create resource: %v", err)
		}
		defer os.Remove(filepath.Join(confDir, "bad.toml"))

		err := ValidateTemplates(tmpDir, "bad.toml", "")
		if err == nil {
			t.Error("Expected error for invalid template syntax")
		}
	})

	t.Run("with mock data", func(t *testing.T) {
		// Create template file
		templateContent := `{
    "name": "{{ .name }}",
    "port": {{ .port }}
}
`
		if err := os.WriteFile(filepath.Join(templateDir, "config.json.tmpl"), []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
		defer os.Remove(filepath.Join(templateDir, "config.json.tmpl"))

		// Create resource file
		resourceContent := `[template]
src = "config.json.tmpl"
dest = "` + filepath.Join(destDir, "config.json") + `"
keys = ["/app"]
output_format = "json"
`
		if err := os.WriteFile(filepath.Join(confDir, "json.toml"), []byte(resourceContent), 0644); err != nil {
			t.Fatalf("Failed to create resource: %v", err)
		}
		defer os.Remove(filepath.Join(confDir, "json.toml"))

		// Create mock data file
		mockContent := `{"name": "myapp", "port": 8080}`
		mockFile := filepath.Join(tmpDir, "mock.json")
		if err := os.WriteFile(mockFile, []byte(mockContent), 0644); err != nil {
			t.Fatalf("Failed to create mock data: %v", err)
		}
		defer os.Remove(mockFile)

		err := ValidateTemplates(tmpDir, "json.toml", mockFile)
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
	})

	t.Run("config dir not found", func(t *testing.T) {
		err := ValidateTemplates("/nonexistent/path", "", "")
		if err == nil {
			t.Error("Expected error for nonexistent config directory")
		}
	})

	t.Run("invalid mock data", func(t *testing.T) {
		// Create a simple template for this test
		templateContent := `{{ .value }}`
		if err := os.WriteFile(filepath.Join(templateDir, "simple.tmpl"), []byte(templateContent), 0644); err != nil {
			t.Fatalf("Failed to create template: %v", err)
		}
		defer os.Remove(filepath.Join(templateDir, "simple.tmpl"))

		// Create resource file
		resourceContent := `[template]
src = "simple.tmpl"
dest = "` + filepath.Join(destDir, "simple.conf") + `"
keys = ["/test"]
`
		if err := os.WriteFile(filepath.Join(confDir, "simple.toml"), []byte(resourceContent), 0644); err != nil {
			t.Fatalf("Failed to create resource: %v", err)
		}
		defer os.Remove(filepath.Join(confDir, "simple.toml"))

		mockFile := filepath.Join(tmpDir, "invalid-mock.json")
		if err := os.WriteFile(mockFile, []byte(`{invalid json`), 0644); err != nil {
			t.Fatalf("Failed to create mock data: %v", err)
		}
		defer os.Remove(mockFile)

		err := ValidateTemplates(tmpDir, "", mockFile)
		if err == nil {
			t.Error("Expected error for invalid mock data")
		}
	})
}
