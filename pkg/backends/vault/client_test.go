package vault

import (
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

// Note: Full GetValues and authentication tests require a running Vault instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
//
// The Vault client has complex authentication flows (app-role, github, kubernetes, etc.)
// that require an actual Vault server to test properly.
