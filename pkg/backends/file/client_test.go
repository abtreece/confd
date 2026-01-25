package file

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewFileClient(t *testing.T) {
	paths := []string{"/path/to/file.yaml"}
	filter := "*.yaml"

	client, err := NewFileClient(paths, filter)
	if err != nil {
		t.Fatalf("NewFileClient() unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("NewFileClient() returned nil client")
	}
	if !reflect.DeepEqual(client.filepath, paths) {
		t.Errorf("client.filepath = %v, want %v", client.filepath, paths)
	}
	if client.filter != filter {
		t.Errorf("client.filter = %s, want %s", client.filter, filter)
	}
}

func TestGetValues_YAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
  port: "5432"
  name: mydb
app:
  debug: true
  timeout: 30
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/database"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/database/host": "localhost",
		"/database/port": "5432",
		"/database/name": "mydb",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_JSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")

	jsonContent := `{
	"server": {
		"host": "0.0.0.0",
		"port": 8080
	},
	"logging": {
		"level": "info"
	}
}`
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}

	client, _ := NewFileClient([]string{jsonFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/server"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/server/host": "0.0.0.0",
		"/server/port": "8080",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
database:
  host: localhost
cache:
  host: redis
other:
  value: ignored
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/database", "/cache"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/database/host": "localhost",
		"/cache/host":    "redis",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_NestedStructure(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
app:
  db:
    primary:
      host: primary-db
    replica:
      host: replica-db
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/app/db"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/db/primary/host": "primary-db",
		"/app/db/replica/host": "replica-db",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_ArrayValues(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
servers:
  - host: server1
  - host: server2
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/servers"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/servers/0/host": "server1",
		"/servers/1/host": "server2",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_WildcardKeys(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
servers:
  - host: server1
    port: 8080
  - host: server2
    port: 8081
database:
  host: dbhost
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	// Using wildcard key pattern
	result, err := client.GetValues(context.Background(), []string{"/servers/*"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/servers/0/host": "server1",
		"/servers/0/port": "8080",
		"/servers/1/host": "server2",
		"/servers/1/port": "8081",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}

	// Verify database is NOT included (filtered out)
	if _, ok := result["/database/host"]; ok {
		t.Error("Expected /database/host to be filtered out")
	}
}

func TestGetValues_DifferentTypes(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
values:
  string_val: hello
  int_val: 42
  float_val: 3.14
  bool_val: true
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/values"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/values/string_val": "hello",
		"/values/int_val":    "42",
		"/values/float_val":  "3.14",
		"/values/bool_val":   "true",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.yaml")
	file2 := filepath.Join(tmpDir, "file2.yaml")

	yaml1 := `
db:
  host: localhost
`
	yaml2 := `
cache:
  host: redis
`
	if err := os.WriteFile(file1, []byte(yaml1), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(yaml2), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	client, _ := NewFileClient([]string{file1, file2}, "")

	result, err := client.GetValues(context.Background(), []string{"/db", "/cache"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/db/host":    "localhost",
		"/cache/host": "redis",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_DirectoryWithFilter(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	jsonFile := filepath.Join(tmpDir, "other.json")

	yamlContent := `
yaml_key: yaml_value
`
	jsonContent := `{"json_key": "json_value"}`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}

	// Only get yaml files
	client, _ := NewFileClient([]string{tmpDir}, "*.yaml")

	result, err := client.GetValues(context.Background(), []string{"/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Should only contain YAML file content
	if _, ok := result["/yaml_key"]; !ok {
		t.Error("Expected /yaml_key to be present")
	}
	if _, ok := result["/json_key"]; ok {
		t.Error("Expected /json_key to NOT be present (filtered out)")
	}
}

func TestGetValues_MissingFile(t *testing.T) {
	client, _ := NewFileClient([]string{"/nonexistent/file.yaml"}, "")

	_, err := client.GetValues(context.Background(), []string{"/key"})
	if err == nil {
		t.Error("GetValues() expected error for missing file, got nil")
	}
}

func TestGetValues_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML content
	invalidContent := `
key: value
  bad indentation: error
`
	if err := os.WriteFile(yamlFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	_, err := client.GetValues(context.Background(), []string{"/key"})
	if err == nil {
		t.Error("GetValues() expected error for invalid YAML, got nil")
	}
}

func TestGetValues_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "invalid.json")

	// Invalid JSON content
	invalidContent := `{key: "missing quotes"}`
	if err := os.WriteFile(jsonFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{jsonFile}, "")

	_, err := client.GetValues(context.Background(), []string{"/key"})
	if err == nil {
		t.Error("GetValues() expected error for invalid JSON, got nil")
	}
}

func TestGetValues_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(yamlFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{"/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
key: value
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	result, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// With empty keys, nothing should match
	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestWatchPrefix_InitialCall(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(yamlFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")
	stopChan := make(chan bool)

	// waitIndex 0 should return immediately with 1
	index, err := client.WatchPrefix(context.Background(), "/", []string{"/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestWatchPrefix_StopChannel(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(yamlFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")
	stopChan := make(chan bool, 1)

	// Send stop signal
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/", []string{"/key"}, 1, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(yamlFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{yamlFile}, "")

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_MissingFile(t *testing.T) {
	client, _ := NewFileClient([]string{"/nonexistent/file.yaml"}, "")

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error for missing file, got nil")
	}
}

func TestHealthCheck_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.yaml")
	file2 := filepath.Join(tmpDir, "file2.yaml")

	if err := os.WriteFile(file1, []byte("key1: value1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("key2: value2"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	client, _ := NewFileClient([]string{file1, file2}, "")

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_PartialMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.yaml")

	if err := os.WriteFile(existingFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	client, _ := NewFileClient([]string{existingFile, "/nonexistent/file.yaml"}, "")

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error when one file is missing, got nil")
	}
}

func TestNormalizePrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/servers/*", "/servers"},
		{"/servers/", "/servers"},
		{"/servers", "/servers"},
		{"/*", ""},
		{"/", ""},
		{"/app/db/*", "/app/db"},
		{"/app/db/", "/app/db"},
		// Middle wildcards are preserved (only trailing is stripped)
		{"/a/*/b", "/a/*/b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePrefix(tt.input)
			if got != tt.want {
				t.Errorf("normalizePrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchesAnyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		prefixes []string
		want     bool
	}{
		{
			name:     "empty prefixes returns false",
			path:     "/database/host",
			prefixes: []string{},
			want:     false,
		},
		{
			name:     "single prefix match",
			path:     "/database/host",
			prefixes: []string{"/database"},
			want:     true,
		},
		{
			name:     "single prefix no match",
			path:     "/database/host",
			prefixes: []string{"/cache"},
			want:     false,
		},
		{
			name:     "multiple prefixes first matches",
			path:     "/database/host",
			prefixes: []string{"/database", "/cache", "/app"},
			want:     true,
		},
		{
			name:     "multiple prefixes last matches",
			path:     "/app/config",
			prefixes: []string{"/database", "/cache", "/app"},
			want:     true,
		},
		{
			name:     "multiple prefixes none match",
			path:     "/other/key",
			prefixes: []string{"/database", "/cache", "/app"},
			want:     false,
		},
		{
			name:     "prefix substring matches (HasPrefix behavior)",
			path:     "/database-backup/host",
			prefixes: []string{"/database"},
			want:     true,
		},
		{
			name:     "exact path match",
			path:     "/database",
			prefixes: []string{"/database"},
			want:     true,
		},
		{
			name:     "root prefix matches all",
			path:     "/database/host",
			prefixes: []string{"/"},
			want:     true,
		},
		{
			name:     "wildcard suffix matches nested path",
			path:     "/servers/0/host",
			prefixes: []string{"/servers/*"},
			want:     true,
		},
		{
			name:     "wildcard suffix matches immediate child",
			path:     "/servers/primary",
			prefixes: []string{"/servers/*"},
			want:     true,
		},
		{
			name:     "trailing slash is normalized",
			path:     "/database/host",
			prefixes: []string{"/database/"},
			want:     true,
		},
		{
			name:     "root wildcard matches all",
			path:     "/any/path/here",
			prefixes: []string{"/*"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAnyPrefix(tt.path, tt.prefixes)
			if got != tt.want {
				t.Errorf("matchesAnyPrefix(%q, %v) = %v, want %v",
					tt.path, tt.prefixes, got, tt.want)
			}
		})
	}
}

func TestNodeWalk(t *testing.T) {
	tests := []struct {
		name     string
		node     interface{}
		key      string
		expected map[string]string
	}{
		{
			name:     "string value",
			node:     "hello",
			key:      "/test",
			expected: map[string]string{"/test": "hello"},
		},
		{
			name:     "int value",
			node:     42,
			key:      "/num",
			expected: map[string]string{"/num": "42"},
		},
		{
			name:     "bool value",
			node:     true,
			key:      "/flag",
			expected: map[string]string{"/flag": "true"},
		},
		{
			name:     "float value",
			node:     3.14159,
			key:      "/pi",
			expected: map[string]string{"/pi": "3.14159"},
		},
		{
			name: "map[string]interface{}",
			node: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			key: "/root",
			expected: map[string]string{
				"/root/key1": "value1",
				"/root/key2": "value2",
			},
		},
		{
			name: "slice",
			node: []interface{}{"a", "b", "c"},
			key:  "/items",
			expected: map[string]string{
				"/items/0": "a",
				"/items/1": "b",
				"/items/2": "c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := make(map[string]string)
			nodeWalk(tt.node, tt.key, vars)
			if !reflect.DeepEqual(vars, tt.expected) {
				t.Errorf("nodeWalk() vars = %v, want %v", vars, tt.expected)
			}
		})
	}
}
