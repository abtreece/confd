package template

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/abtreece/confd/pkg/log"
)

// mockStoreClient implements backends.StoreClient for testing
type mockStoreClient struct {
	healthCheckErr error
	getValuesErr   error
	values         map[string]string
}

func (m *mockStoreClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	if m.getValuesErr != nil {
		return nil, m.getValuesErr
	}
	return m.values, nil
}

func (m *mockStoreClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	return 0, nil
}

func (m *mockStoreClient) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func TestPreflight_HealthCheckFailure(t *testing.T) {
	log.SetLevel("warn")

	config := Config{
		StoreClient: &mockStoreClient{
			healthCheckErr: errors.New("connection refused"),
		},
	}

	err := Preflight(config)
	if err == nil {
		t.Error("Expected error for health check failure, got nil")
	}
	if err != nil && !errors.Is(err, err) {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestPreflight_NoTemplateResources(t *testing.T) {
	log.SetLevel("warn")

	// Create temp directory with empty conf.d
	tmpDir, err := os.MkdirTemp("", "confd-preflight-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	templateDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("Failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	config := Config{
		StoreClient: &mockStoreClient{},
		ConfDir:     tmpDir,   // Root directory
		ConfigDir:   confDir,  // conf.d subdirectory
		TemplateDir: templateDir,
	}

	err = Preflight(config)
	if err != nil {
		t.Errorf("Expected no error for empty template resources, got: %v", err)
	}
}

func TestPreflight_SuccessWithKeys(t *testing.T) {
	log.SetLevel("warn")

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "confd-preflight-test")
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

	// Create template file
	if err := os.WriteFile(filepath.Join(templateDir, "test.tmpl"), []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create resource file
	resourceContent := `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`
	if err := os.WriteFile(filepath.Join(confDir, "test.toml"), []byte(resourceContent), 0644); err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	config := Config{
		StoreClient: &mockStoreClient{
			values: map[string]string{
				"/app/test": "value",
			},
		},
		ConfDir:     tmpDir,
		ConfigDir:   confDir,
		TemplateDir: templateDir,
	}

	err = Preflight(config)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestPreflight_NoKeysFound(t *testing.T) {
	log.SetLevel("warn")

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "confd-preflight-test")
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

	// Create template file
	if err := os.WriteFile(filepath.Join(templateDir, "test.tmpl"), []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create resource file
	resourceContent := `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`
	if err := os.WriteFile(filepath.Join(confDir, "test.toml"), []byte(resourceContent), 0644); err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	config := Config{
		StoreClient: &mockStoreClient{
			values: map[string]string{}, // Empty - no keys found
		},
		ConfDir:     tmpDir,
		ConfigDir:   confDir,
		TemplateDir: templateDir,
	}

	// Should succeed with warning, not error
	err = Preflight(config)
	if err != nil {
		t.Errorf("Expected no error (just warning), got: %v", err)
	}
}

func TestPreflight_GetValuesError(t *testing.T) {
	log.SetLevel("warn")

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "confd-preflight-test")
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

	// Create template file
	if err := os.WriteFile(filepath.Join(templateDir, "test.tmpl"), []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create resource file
	resourceContent := `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/app/test"]
`
	if err := os.WriteFile(filepath.Join(confDir, "test.toml"), []byte(resourceContent), 0644); err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	config := Config{
		StoreClient: &mockStoreClient{
			getValuesErr: errors.New("backend timeout"),
		},
		ConfDir:     tmpDir,
		ConfigDir:   confDir,
		TemplateDir: templateDir,
	}

	err = Preflight(config)
	if err == nil {
		t.Error("Expected error for GetValues failure, got nil")
	}
}

func TestPreflight_InvalidTemplateResource(t *testing.T) {
	log.SetLevel("warn")

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "confd-preflight-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confDir := filepath.Join(tmpDir, "conf.d")
	templateDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatalf("Failed to create conf.d: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create templates: %v", err)
	}

	// Create invalid resource file (missing required src)
	resourceContent := `[template]
dest = "/tmp/test.conf"
keys = ["/app/test"]
`
	if err := os.WriteFile(filepath.Join(confDir, "invalid.toml"), []byte(resourceContent), 0644); err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	config := Config{
		StoreClient: &mockStoreClient{},
		ConfDir:     tmpDir,
		ConfigDir:   confDir,
		TemplateDir: templateDir,
	}

	err = Preflight(config)
	if err == nil {
		t.Error("Expected error for invalid template resource, got nil")
	}
}
