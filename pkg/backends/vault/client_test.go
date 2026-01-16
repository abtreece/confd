package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

// createMockResponse creates a mock vaultapi.Response with the given data
func createMockResponse(t *testing.T, data map[string]interface{}) *vaultapi.Response {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"data": data,
	})
	if err != nil {
		t.Fatalf("createMockResponse: failed to marshal data: %v", err)
	}
	return &vaultapi.Response{
		Response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
		},
	}
}

func TestGetMount(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "/secret/data/app",
			expected: "/secret",
		},
		{
			name:     "nested path",
			path:     "/kv/data/config/database",
			expected: "/kv",
		},
		{
			name:     "root path",
			path:     "/secret",
			expected: "/secret",
		},
		{
			name:     "deeply nested",
			path:     "/mount/a/b/c/d/e",
			expected: "/mount",
		},
		{
			name:     "with data segment",
			path:     "/secret/data/myapp/config",
			expected: "/secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMount(tt.path)
			if result != tt.expected {
				t.Errorf("getMount(%s) = %s, want %s", tt.path, result, tt.expected)
			}
		})
	}
}

func TestUniqMounts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"/secret", "/kv", "/database"},
			expected: []string{"/secret", "/kv", "/database"},
		},
		{
			name:     "with duplicates",
			input:    []string{"/secret", "/kv", "/secret", "/kv", "/database"},
			expected: []string{"/secret", "/kv", "/database"},
		},
		{
			name:     "all duplicates",
			input:    []string{"/secret", "/secret", "/secret"},
			expected: []string{"/secret"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"/secret"},
			expected: []string{"/secret"},
		},
		{
			name:     "preserve order",
			input:    []string{"/a", "/b", "/c", "/a", "/b"},
			expected: []string{"/a", "/b", "/c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqMounts(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("uniqMounts(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    interface{}
		mount    string
		expected map[string]string
	}{
		{
			name:     "string value",
			key:      "/secret/key",
			value:    "value123",
			mount:    "/secret",
			expected: map[string]string{"/secret/key": "value123"},
		},
		{
			name:  "nested map",
			key:   "/secret/config",
			value: map[string]interface{}{"db_host": "localhost", "db_port": "5432"},
			mount: "/secret",
			expected: map[string]string{
				"/secret/config/db_host": "localhost",
				"/secret/config/db_port": "5432",
			},
		},
		{
			name:  "deeply nested map",
			key:   "/secret/app",
			value: map[string]interface{}{"database": map[string]interface{}{"host": "localhost"}},
			mount: "/secret",
			expected: map[string]string{
				"/secret/app/database/host": "localhost",
			},
		},
		{
			name:     "removes data/ from key",
			key:      "/secret/data/config",
			value:    "value",
			mount:    "/secret",
			expected: map[string]string{"/secret/config": "value"},
		},
		{
			name:     "empty string value",
			key:      "/secret/empty",
			value:    "",
			mount:    "/secret",
			expected: map[string]string{"/secret/empty": ""},
		},
		{
			name:     "empty map",
			key:      "/secret/empty",
			value:    map[string]interface{}{},
			mount:    "/secret",
			expected: map[string]string{},
		},
		{
			name:  "multiple data segments",
			key:   "/secret/data/data/config",
			value: "value",
			mount: "/secret",
			expected: map[string]string{"/secret/config": "value"},
		},
		{
			name:  "three level nesting",
			key:   "/secret/app",
			value: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": "deep_value",
					},
				},
			},
			mount: "/secret",
			expected: map[string]string{
				"/secret/app/level1/level2/level3": "deep_value",
			},
		},
		{
			name:     "numeric value (integer)",
			key:      "/secret/port",
			value:    float64(5432),
			mount:    "/secret",
			expected: map[string]string{"/secret/port": "5432"},
		},
		{
			name:     "numeric value (float)",
			key:      "/secret/ratio",
			value:    float64(3.14159),
			mount:    "/secret",
			expected: map[string]string{"/secret/ratio": "3.14159"},
		},
		{
			name:     "boolean true",
			key:      "/secret/enabled",
			value:    true,
			mount:    "/secret",
			expected: map[string]string{"/secret/enabled": "true"},
		},
		{
			name:     "boolean false",
			key:      "/secret/disabled",
			value:    false,
			mount:    "/secret",
			expected: map[string]string{"/secret/disabled": "false"},
		},
		{
			name:     "array value",
			key:      "/secret/hosts",
			value:    []interface{}{"host1", "host2", "host3"},
			mount:    "/secret",
			expected: map[string]string{"/secret/hosts": `["host1","host2","host3"]`},
		},
		{
			name:     "mixed array",
			key:      "/secret/mixed",
			value:    []interface{}{"string", float64(42), true},
			mount:    "/secret",
			expected: map[string]string{"/secret/mixed": `["string",42,true]`},
		},
		{
			name: "nested map with mixed types",
			key:  "/secret/config",
			value: map[string]interface{}{
				"host":    "localhost",
				"port":    float64(5432),
				"enabled": true,
				"tags":    []interface{}{"prod", "primary"},
			},
			mount: "/secret",
			expected: map[string]string{
				"/secret/config/host":    "localhost",
				"/secret/config/port":    "5432",
				"/secret/config/enabled": "true",
				"/secret/config/tags":    `["prod","primary"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := make(map[string]string)
			flatten(tt.key, tt.value, tt.mount, vars)
			if !reflect.DeepEqual(vars, tt.expected) {
				t.Errorf("flatten() vars = %v, want %v", vars, tt.expected)
			}
		})
	}
}

func TestFlatten_NilValue(t *testing.T) {
	// Nil values should be silently skipped (valid JSON null)
	vars := make(map[string]string)
	flatten("/secret/key", nil, "/secret", vars)
	if len(vars) != 0 {
		t.Errorf("flatten() should skip nil values, got %v", vars)
	}
}

func TestFlatten_NumericEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"zero", 0, "0"},
		{"negative int", -42, "-42"},
		{"large int", 9007199254740991, "9007199254740991"},
		{"small float", 0.001, "0.001"},
		{"negative float", -3.14, "-3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := make(map[string]string)
			flatten("/secret/num", tt.value, "/secret", vars)
			if vars["/secret/num"] != tt.expected {
				t.Errorf("flatten() = %q, want %q", vars["/secret/num"], tt.expected)
			}
		})
	}
}

func TestFlatten_EmptyArray(t *testing.T) {
	vars := make(map[string]string)
	flatten("/secret/empty", []interface{}{}, "/secret", vars)
	if vars["/secret/empty"] != "[]" {
		t.Errorf("flatten() empty array = %q, want \"[]\"", vars["/secret/empty"])
	}
}

func TestFlatten_NestedArray(t *testing.T) {
	vars := make(map[string]string)
	nested := []interface{}{
		[]interface{}{"a", "b"},
		[]interface{}{float64(1), float64(2)},
	}
	flatten("/secret/nested", nested, "/secret", vars)
	expected := `[["a","b"],[1,2]]`
	if vars["/secret/nested"] != expected {
		t.Errorf("flatten() nested array = %q, want %q", vars["/secret/nested"], expected)
	}
}

func TestGetRequiredParameter_Success(t *testing.T) {
	params := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	result, err := getRequiredParameter("key1", params)
	if err != nil {
		t.Errorf("getRequiredParameter() unexpected error: %v", err)
	}
	if result != "value1" {
		t.Errorf("getRequiredParameter() = %s, want value1", result)
	}
}

func TestGetRequiredParameter_Missing(t *testing.T) {
	params := map[string]string{}

	_, err := getRequiredParameter("missing", params)
	if err == nil {
		t.Error("getRequiredParameter() expected error for missing key")
	}
}

func TestGetRequiredParameter_EmptyValue(t *testing.T) {
	params := map[string]string{
		"key1": "",
	}

	_, err := getRequiredParameter("key1", params)
	if err == nil {
		t.Error("getRequiredParameter() expected error for empty value")
	}
}

func TestWatchPrefix(t *testing.T) {
	client := &Client{client: nil}
	stopChan := make(chan bool, 1)

	// Send stop signal immediately
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/secret", []string{"/secret/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}

func TestGetConfig_Basic(t *testing.T) {
	conf, err := getConfig("http://localhost:8200", "", "", "")
	if err != nil {
		t.Fatalf("getConfig() unexpected error: %v", err)
	}

	if conf.Address != "http://localhost:8200" {
		t.Errorf("getConfig() address = %s, want http://localhost:8200", conf.Address)
	}
}

func TestGetConfig_InvalidCert(t *testing.T) {
	// Create temp dir for test files
	tmpDir := t.TempDir()

	// Try to load non-existent cert
	_, err := getConfig("http://localhost:8200", tmpDir+"/nonexistent.crt", tmpDir+"/nonexistent.key", "")
	if err == nil {
		t.Error("getConfig() expected error for non-existent cert")
	}
}

func TestGetConfig_InvalidCACert(t *testing.T) {
	tmpDir := t.TempDir()

	// Try to load non-existent CA cert
	_, err := getConfig("http://localhost:8200", "", "", tmpDir+"/nonexistent-ca.crt")
	if err == nil {
		t.Error("getConfig() expected error for non-existent CA cert")
	}
}

func TestGetConfig_WithCACert(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid PEM-encoded CA cert file (self-signed for testing)
	caCertPEM := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpegPjMCMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yMzAxMDEwMDAwMDBaFw0yNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC5hIP3lUOaTXwuOdPbx/CC
yvlSm/p7l0PFQT3PZ0LT/qRVKkYjF/P2cWoK8FShP0qPl6wPHKqvFbKMFwXH9S9H
AgMBAAGjUzBRMB0GA1UdDgQWBBQ8YF5gTVd1U7dD1Eh4NL+x7qmGRjAfBgNVHSME
GDAWgBQ8YF5gTVd1U7dD1Eh4NL+x7qmGRjAPBgNVHRMBAf8EBTADAQH/MA0GCSqG
SIb3DQEBCwUAA0EArXj9xm4+CXB0mVVPFdCDxrQK3Z0MbH2ZVNU1+/T/RxPHoVmJ
8LHZnGS6wFw5sRJbxFTcCXpCfvOZMqjV7wTa4Q==
-----END CERTIFICATE-----`
	caCertPath := tmpDir + "/ca.crt"
	if err := os.WriteFile(caCertPath, []byte(caCertPEM), 0644); err != nil {
		t.Fatalf("Failed to write CA cert: %v", err)
	}

	conf, err := getConfig("http://localhost:8200", "", "", caCertPath)
	if err != nil {
		t.Fatalf("getConfig() unexpected error: %v", err)
	}

	if conf.HttpClient.Transport == nil {
		t.Error("getConfig() transport should be set when CA cert is provided")
	}
}

func TestNew_MissingAuthType(t *testing.T) {
	_, err := New("http://localhost:8200", "", map[string]string{})
	if err == nil {
		t.Error("New() expected error for missing auth type")
	}
	if err.Error() != "you have to set the auth type when using the vault backend" {
		t.Errorf("New() error = %v, want auth type error", err)
	}
}

// mockVaultLogical implements the vaultLogical interface for testing
type mockVaultLogical struct {
	listFunc    func(path string) (*vaultapi.Secret, error)
	readFunc    func(path string) (*vaultapi.Secret, error)
	readRawFunc func(path string) (*vaultapi.Response, error)
	writeFunc   func(path string, data map[string]interface{}) (*vaultapi.Secret, error)
}

func (m *mockVaultLogical) List(path string) (*vaultapi.Secret, error) {
	if m.listFunc != nil {
		return m.listFunc(path)
	}
	return nil, nil
}

func (m *mockVaultLogical) Read(path string) (*vaultapi.Secret, error) {
	if m.readFunc != nil {
		return m.readFunc(path)
	}
	return nil, nil
}

func (m *mockVaultLogical) ReadRaw(path string) (*vaultapi.Response, error) {
	if m.readRawFunc != nil {
		return m.readRawFunc(path)
	}
	return nil, nil
}

func (m *mockVaultLogical) Write(path string, data map[string]interface{}) (*vaultapi.Secret, error) {
	if m.writeFunc != nil {
		return m.writeFunc(path, data)
	}
	return nil, nil
}

func TestListSecretWithLogical_Version1(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/mykey" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"key1", "key2"},
					},
				}, nil
			}
			return nil, nil
		},
	}

	result, err := listSecretWithLogical(mock, "/secret", "/mykey", "1")
	if err != nil {
		t.Fatalf("listSecretWithLogical() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("listSecretWithLogical() returned nil")
	}

	keys := result.Data["keys"].([]interface{})
	if len(keys) != 2 {
		t.Errorf("listSecretWithLogical() returned %d keys, want 2", len(keys))
	}
}

func TestListSecretWithLogical_Version2(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/metadata//mykey" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"key1", "key2", "key3"},
					},
				}, nil
			}
			return nil, nil
		},
	}

	result, err := listSecretWithLogical(mock, "/secret", "/mykey", "2")
	if err != nil {
		t.Fatalf("listSecretWithLogical() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("listSecretWithLogical() returned nil")
	}

	keys := result.Data["keys"].([]interface{})
	if len(keys) != 3 {
		t.Errorf("listSecretWithLogical() returned %d keys, want 3", len(keys))
	}
}

func TestListSecretWithLogical_EmptyVersion(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1"},
				},
			}, nil
		},
	}

	// Empty version should behave like version 1
	result, err := listSecretWithLogical(mock, "/secret", "/key", "")
	if err != nil {
		t.Fatalf("listSecretWithLogical() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("listSecretWithLogical() returned nil for empty version")
	}
}

func TestListSecretWithLogical_UnsupportedVersion(t *testing.T) {
	mock := &mockVaultLogical{}

	result, err := listSecretWithLogical(mock, "/secret", "/key", "unsupported")
	if err != nil {
		t.Errorf("listSecretWithLogical() unexpected error: %v", err)
	}
	if result != nil {
		t.Error("listSecretWithLogical() expected nil for unsupported version")
	}
}

func TestListSecretWithLogical_Error(t *testing.T) {
	expectedErr := errors.New("vault error")
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return nil, expectedErr
		},
	}

	_, err := listSecretWithLogical(mock, "/secret", "/key", "1")
	if err != expectedErr {
		t.Errorf("listSecretWithLogical() error = %v, want %v", err, expectedErr)
	}
}

func TestRecursiveListSecretWithLogical_SingleSecret_V1(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"mysecret"},
				},
			}, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "1")
	if len(result) == 0 {
		t.Error("recursiveListSecretWithLogical() returned empty result")
	}

	// Check that the path was added correctly
	found := false
	for _, p := range result {
		if p == "/secret/mysecret" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("recursiveListSecretWithLogical() missing expected path /secret/mysecret, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_SingleSecret_V2(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"mysecret"},
				},
			}, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "2")
	if len(result) == 0 {
		t.Error("recursiveListSecretWithLogical() returned empty result")
	}

	// For v2, paths should have /data prefix
	found := false
	for _, p := range result {
		if p == "/secret/data/mysecret" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("recursiveListSecretWithLogical() missing expected path /secret/data/mysecret, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_WithSubdirectory_V1(t *testing.T) {
	callCount := 0
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			callCount++
			if callCount == 1 {
				// First call returns a directory
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"subdir/", "secret1"},
					},
				}, nil
			}
			// Subsequent call for subdir returns secrets
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"nested_secret"},
				},
			}, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "1")
	if len(result) < 2 {
		t.Errorf("recursiveListSecretWithLogical() returned %d paths, want at least 2", len(result))
	}
}

func TestRecursiveListSecretWithLogical_NilSecretList_V1(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return nil, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "1")
	// When secretList is nil, an empty slice should be returned
	if len(result) != 0 {
		t.Errorf("recursiveListSecretWithLogical() expected empty slice when list returns nil, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_NilSecretList_V2(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return nil, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "2")
	// When secretList is nil, an empty slice should be returned
	if len(result) != 0 {
		t.Errorf("recursiveListSecretWithLogical() expected empty slice when list returns nil, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_UnsupportedVersion(t *testing.T) {
	mock := &mockVaultLogical{}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "unsupported")
	// For unsupported version, an empty slice should be returned since listSecretWithLogical returns nil
	if len(result) != 0 {
		t.Errorf("recursiveListSecretWithLogical() expected empty slice for unsupported version, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_KeysNotSlice(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": "not a slice", // Wrong type
				},
			}, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "1")
	if len(result) != 0 {
		t.Errorf("recursiveListSecretWithLogical() expected empty slice when keys is not a slice, got %v", result)
	}
}

func TestRecursiveListSecretWithLogical_KeyNotString(t *testing.T) {
	mock := &mockVaultLogical{
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{123, "valid_secret"}, // First key is not a string
				},
			}, nil
		},
	}

	result := recursiveListSecretWithLogical(mock, "/secret", "", "1")
	// Should skip the non-string key and only return the valid one
	if len(result) != 1 {
		t.Errorf("recursiveListSecretWithLogical() expected 1 result, got %d: %v", len(result), result)
	}
}

func TestBuildListPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		key      string
		version  string
		want     string
	}{
		{
			name:     "version 1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "1",
			want:     "/secret/mykey",
		},
		{
			name:     "version 2",
			basePath: "/secret",
			key:      "/mykey",
			version:  "2",
			want:     "/secret/metadata//mykey",
		},
		{
			name:     "empty version defaults to v1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "",
			want:     "/secret/mykey",
		},
		{
			name:     "unsupported version defaults to v1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "3",
			want:     "/secret/mykey",
		},
		{
			name:     "nested path v1",
			basePath: "/secret",
			key:      "/app/config/db",
			version:  "1",
			want:     "/secret/app/config/db",
		},
		{
			name:     "nested path v2",
			basePath: "/secret",
			key:      "/app/config/db",
			version:  "2",
			want:     "/secret/metadata//app/config/db",
		},
		{
			name:     "empty key v1",
			basePath: "/secret",
			key:      "",
			version:  "1",
			want:     "/secret",
		},
		{
			name:     "empty key v2",
			basePath: "/secret",
			key:      "",
			version:  "2",
			want:     "/secret/metadata/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildListPath(tt.basePath, tt.key, tt.version)
			if got != tt.want {
				t.Errorf("buildListPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSecretPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		key      string
		version  string
		want     string
	}{
		{
			name:     "version 1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "1",
			want:     "/secret/mykey",
		},
		{
			name:     "version 2",
			basePath: "/secret",
			key:      "/mykey",
			version:  "2",
			want:     "/secret/data/mykey",
		},
		{
			name:     "empty version defaults to v1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "",
			want:     "/secret/mykey",
		},
		{
			name:     "unsupported version defaults to v1",
			basePath: "/secret",
			key:      "/mykey",
			version:  "3",
			want:     "/secret/mykey",
		},
		{
			name:     "nested path v1",
			basePath: "/secret",
			key:      "/app/config/db",
			version:  "1",
			want:     "/secret/app/config/db",
		},
		{
			name:     "nested path v2",
			basePath: "/secret",
			key:      "/app/config/db",
			version:  "2",
			want:     "/secret/data/app/config/db",
		},
		{
			name:     "empty key v1",
			basePath: "/secret",
			key:      "",
			version:  "1",
			want:     "/secret",
		},
		{
			name:     "empty key v2",
			basePath: "/secret",
			key:      "",
			version:  "2",
			want:     "/secret/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSecretPath(tt.basePath, tt.key, tt.version)
			if got != tt.want {
				t.Errorf("buildSecretPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetKVVersion(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		wantVersion string
		wantErr     bool
	}{
		{
			name: "version 2 with options",
			data: map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "2",
				},
			},
			wantVersion: "2",
			wantErr:     false,
		},
		{
			name: "version 1 with options",
			data: map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			},
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name: "no options defaults to version 1",
			data: map[string]interface{}{
				"type": "kv",
			},
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name: "options without version defaults to version 1",
			data: map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"other_option": "value",
				},
			},
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name: "options is wrong type defaults to version 1",
			data: map[string]interface{}{
				"type":    "kv",
				"options": "not a map",
			},
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name: "version is not a string defaults to version 1",
			data: map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": 2, // int instead of string
				},
			},
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name:        "nil data",
			data:        nil,
			wantVersion: "1",
			wantErr:     false,
		},
		{
			name:        "empty data",
			data:        map[string]interface{}{},
			wantVersion: "1",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getKVVersion(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("getKVVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantVersion {
				t.Errorf("getKVVersion() = %v, want %v", got, tt.wantVersion)
			}
		})
	}
}

func TestGetValues_KVv1(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			if path == "/sys/internal/ui/mounts//secret" {
				return createMockResponse(t, map[string]interface{}{
					"type": "kv",
					"options": map[string]interface{}{
						"version": "1",
					},
				}), nil
			}
			return nil, nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"mykey"},
					},
				}, nil
			}
			return nil, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/mykey" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"username": "admin",
						"password": "secret123",
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/data"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Check that secrets were retrieved and flattened
	if vars["/secret/mykey/username"] != "admin" {
		t.Errorf("GetValues() username = %v, want admin", vars["/secret/mykey/username"])
	}
	if vars["/secret/mykey/password"] != "secret123" {
		t.Errorf("GetValues() password = %v, want secret123", vars["/secret/mykey/password"])
	}
}

func TestGetValues_KVv2(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			if path == "/sys/internal/ui/mounts//secret" {
				return createMockResponse(t, map[string]interface{}{
					"type": "kv",
					"options": map[string]interface{}{
						"version": "2",
					},
				}), nil
			}
			return nil, nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/metadata/" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"mykey"},
					},
				}, nil
			}
			return nil, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/data/mykey" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"data": map[string]interface{}{
							"username": "admin",
							"password": "secret123",
						},
						"metadata": map[string]interface{}{
							"version": 1,
						},
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/data"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Check that secrets were retrieved and flattened
	if vars["/secret/mykey/username"] != "admin" {
		t.Errorf("GetValues() username = %v, want admin", vars["/secret/mykey/username"])
	}
	if vars["/secret/mykey/password"] != "secret123" {
		t.Errorf("GetValues() password = %v, want secret123", vars["/secret/mykey/password"])
	}
}

func TestGetValues_ReadRawError(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return nil, errors.New("connection refused")
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/data"})
	// GetValues returns error for read failures while continuing to process
	if err == nil {
		t.Error("GetValues() expected error for read failure, got nil")
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() should return empty map on error, got %v", vars)
	}
}

func TestGetValues_EmptyResponse(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/data"})
	if err != nil {
		t.Errorf("GetValues() unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() should return empty map for nil response, got %v", vars)
	}
}

func TestGetValues_UnsupportedEngine(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "transit", // Not a kv engine
			}), nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/transit/data"})
	if err != nil {
		t.Errorf("GetValues() unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() should return empty map for unsupported engine, got %v", vars)
	}
}

func TestGetValues_MultiplePaths(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			switch path {
			case "/secret":
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"app1"},
					},
				}, nil
			case "/kv":
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"keys": []interface{}{"app2"},
					},
				}, nil
			}
			return nil, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			switch path {
			case "/secret/app1":
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"key1": "value1",
					},
				}, nil
			case "/kv/app2":
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"key2": "value2",
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/app", "/kv/app"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Should get secrets from both mounts
	if vars["/secret/app1/key1"] != "value1" {
		t.Errorf("GetValues() key1 = %v, want value1", vars["/secret/app1/key1"])
	}
	if vars["/kv/app2/key2"] != "value2" {
		t.Errorf("GetValues() key2 = %v, want value2", vars["/kv/app2/key2"])
	}
}

func TestGetValues_DuplicateMounts(t *testing.T) {
	callCount := 0
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			callCount++
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"key"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"value": "test",
				},
			}, nil
		},
	}

	client := &Client{logical: mock}
	// Multiple paths from same mount should only query mount info once
	_, err := client.GetValues(context.Background(), []string{"/secret/path1", "/secret/path2"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("GetValues() should deduplicate mounts, called readRaw %d times, want 1", callCount)
	}
}

func TestGetValues_SecretReadError(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1", "secret2"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/secret1" {
				return nil, errors.New("permission denied")
			}
			if path == "/secret/secret2" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"value": "test",
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err == nil {
		t.Error("GetValues() expected error when secret1 fails")
	}

	// Should still get secret2 even though secret1 failed (partial results)
	if vars["/secret/secret2/value"] != "test" {
		t.Errorf("GetValues() should continue after read error and return partial results, got %v", vars)
	}
}

func TestGetValues_NilSecretData(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"key"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: nil,
			}, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if len(vars) != 0 {
		t.Errorf("GetValues() should handle nil secret data, got %v", vars)
	}
}

func TestGetValues_KVv2_NilDataField(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "2",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"key"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			// KVv2 returns data under "data" key, but it's nil
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"data":     nil,
					"metadata": map[string]interface{}{},
				},
			}, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// When data field is nil, json.Marshal(nil) produces "null"
	// The secret path for KVv2 is /secret/data/<key>
	if vars["/secret/data/key"] != "null" {
		t.Errorf("GetValues() with nil data field: got %q, want \"null\"", vars["/secret/data/key"])
	}
}

func TestGetValues_EmptyPaths(t *testing.T) {
	mock := &mockVaultLogical{}
	client := &Client{logical: mock}

	vars, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() with empty paths should return empty map, got %v", vars)
	}
}

func TestGetValues_NilSecretFromParse(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			// Return a response that will parse to nil secret.Data
			body := []byte(`{}`) // Empty JSON, will result in nil Data
			return &vaultapi.Response{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(body)),
				},
			}, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/data"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() should handle nil secret data from parse, got %v", vars)
	}
}

func TestGetValues_KVv2_SecretReadError(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "2",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1", "secret2"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			if path == "/secret/data/secret1" {
				return nil, errors.New("permission denied")
			}
			if path == "/secret/data/secret2" {
				return &vaultapi.Secret{
					Data: map[string]interface{}{
						"data": map[string]interface{}{
							"value": "test",
						},
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err == nil {
		t.Error("GetValues() expected error when secret1 fails")
	}

	// Should still get secret2 even though secret1 failed (partial results)
	if vars["/secret/secret2/value"] != "test" {
		t.Errorf("GetValues() v2 should continue after read error and return partial results, got %v", vars)
	}
}

func TestGetValues_KVv2_NilSecretResponse(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "2",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			// Return nil secret
			return nil, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("GetValues() v2 should handle nil secret response, got %v", vars)
	}
}

func TestGetValues_KVv1_MarshalError(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "1",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			// Return data with a channel which can't be marshaled to JSON
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"unmarshalable": make(chan int),
				},
			}, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err == nil {
		t.Error("GetValues() expected error for marshal failure")
	}
	// Should handle marshal error gracefully (skip the secret, return empty but with error)
	if len(vars) != 0 {
		t.Errorf("GetValues() v1 should handle marshal error, got %v", vars)
	}
}

func TestGetValues_KVv2_MarshalError(t *testing.T) {
	mock := &mockVaultLogical{
		readRawFunc: func(path string) (*vaultapi.Response, error) {
			return createMockResponse(t, map[string]interface{}{
				"type": "kv",
				"options": map[string]interface{}{
					"version": "2",
				},
			}), nil
		},
		listFunc: func(path string) (*vaultapi.Secret, error) {
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"keys": []interface{}{"secret1"},
				},
			}, nil
		},
		readFunc: func(path string) (*vaultapi.Secret, error) {
			// Return data with a channel which can't be marshaled to JSON
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"data": make(chan int), // Can't marshal channel
				},
			}, nil
		},
	}

	client := &Client{logical: mock}
	vars, err := client.GetValues(context.Background(), []string{"/secret/"})
	if err == nil {
		t.Error("GetValues() expected error for marshal failure")
	}
	// Should handle marshal error gracefully (skip the secret, return empty but with error)
	if len(vars) != 0 {
		t.Errorf("GetValues() v2 should handle marshal error, got %v", vars)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	// Create a mock Vault server
	server := http.NewServeMux()
	server.HandleFunc("/v1/sys/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"initialized":true,"sealed":false,"standby":false}`))
	})
	ts := http.Server{
		Addr:    "127.0.0.1:0",
		Handler: server,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	go func() { _ = ts.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ts.Shutdown(ctx)
	}()

	// Create a real Vault client pointing at our mock server
	config := vaultapi.DefaultConfig()
	config.Address = "http://" + listener.Addr().String()
	vaultClient, err := vaultapi.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create vault client: %v", err)
	}

	client := &Client{client: vaultClient}
	err = client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_ServerError(t *testing.T) {
	// Create a mock Vault server that returns an error
	server := http.NewServeMux()
	server.HandleFunc("/v1/sys/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ts := http.Server{
		Addr:    "127.0.0.1:0",
		Handler: server,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	go func() { _ = ts.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ts.Shutdown(ctx)
	}()

	config := vaultapi.DefaultConfig()
	config.Address = "http://" + listener.Addr().String()
	vaultClient, err := vaultapi.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create vault client: %v", err)
	}

	client := &Client{client: vaultClient}
	err = client.HealthCheck(context.Background())
	// Server returns 503, which should result in an error
	if err == nil {
		t.Error("HealthCheck() expected error for unavailable server")
	}
}

func TestHealthCheck_ConnectionRefused(t *testing.T) {
	// Create a client pointing at a non-existent server
	config := vaultapi.DefaultConfig()
	config.Address = "http://127.0.0.1:1" // Port 1 should be refused
	vaultClient, err := vaultapi.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create vault client: %v", err)
	}

	client := &Client{client: vaultClient}
	err = client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error for connection refused")
	}
}

// Note: Full GetValues and authentication tests require a running Vault instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// The Vault client has complex authentication flows (app-role, github, kubernetes, etc.)
// that require an actual Vault server to test properly.
