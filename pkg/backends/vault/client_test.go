package vault

import (
	"os"
	"reflect"
	"testing"
)

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

func TestFlatten_UnsupportedType(t *testing.T) {
	// Unsupported types should be ignored (logged as warning)
	vars := make(map[string]string)

	// Integer type - unsupported
	flatten("/secret/key", 123, "/secret", vars)
	if len(vars) != 0 {
		t.Errorf("flatten() should ignore unsupported int type, got %v", vars)
	}

	// Boolean type - unsupported
	flatten("/secret/key", true, "/secret", vars)
	if len(vars) != 0 {
		t.Errorf("flatten() should ignore unsupported bool type, got %v", vars)
	}

	// Slice type - unsupported
	flatten("/secret/key", []string{"a", "b"}, "/secret", vars)
	if len(vars) != 0 {
		t.Errorf("flatten() should ignore unsupported slice type, got %v", vars)
	}

	// Nil type - unsupported
	flatten("/secret/key", nil, "/secret", vars)
	if len(vars) != 0 {
		t.Errorf("flatten() should ignore nil type, got %v", vars)
	}
}

func TestGetParameter_Success(t *testing.T) {
	params := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	result := getParameter("key1", params)
	if result != "value1" {
		t.Errorf("getParameter() = %s, want value1", result)
	}
}

func TestGetParameter_Missing(t *testing.T) {
	params := map[string]string{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("getParameter() expected panic for missing key")
		}
	}()

	getParameter("missing", params)
}

func TestGetParameter_EmptyValue(t *testing.T) {
	params := map[string]string{
		"key1": "",
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("getParameter() expected panic for empty value")
		}
	}()

	getParameter("key1", params)
}

func TestPanicToError_StringPanic(t *testing.T) {
	var err error

	func() {
		defer panicToError(&err)
		panic("test error")
	}()

	if err == nil {
		t.Error("panicToError() expected error, got nil")
	}
	if err.Error() != "test error" {
		t.Errorf("panicToError() error = %v, want 'test error'", err)
	}
}

func TestPanicToError_ErrorPanic(t *testing.T) {
	var err error
	expectedErr := &testError{msg: "test error"}

	func() {
		defer panicToError(&err)
		panic(expectedErr)
	}()

	if err != expectedErr {
		t.Errorf("panicToError() error = %v, want %v", err, expectedErr)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestPanicToError_NoPanic(t *testing.T) {
	var err error

	func() {
		defer panicToError(&err)
		// No panic
	}()

	if err != nil {
		t.Errorf("panicToError() error = %v, want nil", err)
	}
}

func TestPanicToError_UnknownPanicType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("panicToError() should re-panic for unknown types")
		}
	}()

	var err error
	func() {
		defer panicToError(&err)
		panic(12345) // integer panic - unknown type
	}()
}

func TestWatchPrefix(t *testing.T) {
	client := &Client{client: nil}
	stopChan := make(chan bool, 1)

	// Send stop signal immediately
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix("/secret", []string{"/secret/key"}, 0, stopChan)
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

func TestListSecret_UnsupportedVersion(t *testing.T) {
	// ListSecret with unsupported version returns nil
	result, err := ListSecret(nil, "/secret", "/key", "unsupported")
	if err != nil {
		t.Errorf("ListSecret() unexpected error: %v", err)
	}
	if result != nil {
		t.Error("ListSecret() expected nil for unsupported version")
	}
}

func TestRecursiveListSecret_UnsupportedVersion(t *testing.T) {
	// RecursiveListSecret with unsupported version returns nil
	result := RecursiveListSecret(nil, "/secret", "/key", "unsupported")
	if result != nil {
		t.Error("RecursiveListSecret() expected nil for unsupported version")
	}
}

// Note: Full GetValues and authentication tests require a running Vault instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// The Vault client has complex authentication flows (app-role, github, kubernetes, etc.)
// that require an actual Vault server to test properly.
