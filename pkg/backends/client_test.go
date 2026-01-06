package backends

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_InvalidBackend(t *testing.T) {
	config := Config{
		Backend: "invalid",
	}

	_, err := New(config)
	if err == nil {
		t.Error("New() expected error for invalid backend, got nil")
	}
	if err.Error() != "Invalid backend" {
		t.Errorf("New() error = %v, want 'Invalid backend'", err)
	}
}

func TestNew_EnvBackend(t *testing.T) {
	config := Config{
		Backend: "env",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client for env backend")
	}
}

func TestNew_FileBackend(t *testing.T) {
	// Create a temp YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := Config{
		Backend:  "file",
		YAMLFile: []string{yamlFile},
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client for file backend")
	}
}

func TestNew_DefaultBackendIsEtcd(t *testing.T) {
	// When backend is empty, it should default to etcd
	// We can't fully test this without an etcd server, but we can verify
	// the code path attempts etcd
	config := Config{
		Backend:      "",
		BackendNodes: []string{"http://localhost:2379"},
	}

	// This will fail to connect, but we can verify it tries etcd
	_, err := New(config)
	// Error is expected since there's no etcd server
	// The important thing is it doesn't return "Invalid backend"
	if err != nil && err.Error() == "Invalid backend" {
		t.Error("New() with empty backend should default to etcd, not return 'Invalid backend'")
	}
}

func TestStoreClient_Interface(t *testing.T) {
	// Verify that our backends implement the StoreClient interface
	config := Config{
		Backend: "env",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	// Test GetValues
	t.Setenv("TEST_KEY", "test_value")
	values, err := client.GetValues([]string{"/test/key"})
	if err != nil {
		t.Errorf("GetValues() unexpected error: %v", err)
	}
	if values["/test/key"] != "test_value" {
		t.Errorf("GetValues() = %v, want map with /test/key: test_value", values)
	}
}

func TestNew_FileBackend_GetValues(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  port: "5432"
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := Config{
		Backend:  "file",
		YAMLFile: []string{yamlFile},
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	values, err := client.GetValues([]string{"/database"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if values["/database/host"] != "localhost" {
		t.Errorf("GetValues()['/database/host'] = %s, want 'localhost'", values["/database/host"])
	}
	if values["/database/port"] != "5432" {
		t.Errorf("GetValues()['/database/port'] = %s, want '5432'", values["/database/port"])
	}
}

// Note: Testing other backends (consul, etcd, redis, zookeeper, vault, dynamodb, ssm)
// requires running backend services. These are covered by integration tests in
// .github/workflows/integration-tests.yml
