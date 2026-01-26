package env

import (
	"context"
	"reflect"
	"testing"
)

func TestNewEnvClient(t *testing.T) {
	client, err := NewEnvClient()
	if err != nil {
		t.Errorf("NewEnvClient() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("NewEnvClient() returned nil client")
	}
}

func TestGetValues(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		keys     []string
		expected map[string]string
	}{
		{
			name: "single key match",
			envVars: map[string]string{
				"FOO": "bar",
			},
			keys:     []string{"/foo"},
			expected: map[string]string{"/foo": "bar"},
		},
		{
			name: "prefix match",
			envVars: map[string]string{
				"APP_DB_HOST": "localhost",
				"APP_DB_PORT": "5432",
				"OTHER_VAR":   "ignored",
			},
			keys: []string{"/app/db"},
			expected: map[string]string{
				"/app/db/host": "localhost",
				"/app/db/port": "5432",
			},
		},
		{
			name: "multiple keys",
			envVars: map[string]string{
				"FOO": "foo_value",
				"BAR": "bar_value",
			},
			keys: []string{"/foo", "/bar"},
			expected: map[string]string{
				"/foo": "foo_value",
				"/bar": "bar_value",
			},
		},
		{
			name: "key not found",
			envVars: map[string]string{
				"OTHER": "value",
			},
			keys:     []string{"/missing"},
			expected: map[string]string{},
		},
		{
			name: "case insensitive matching",
			envVars: map[string]string{
				"MY_VAR": "value",
			},
			keys:     []string{"/my/var"},
			expected: map[string]string{"/my/var": "value"},
		},
		{
			name:     "empty keys",
			envVars:  map[string]string{"FOO": "bar"},
			keys:     []string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for this test
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			client, err := NewEnvClient()
			if err != nil {
				t.Fatalf("NewEnvClient() error: %v", err)
			}

			result, err := client.GetValues(context.Background(), tt.keys)
			if err != nil {
				t.Errorf("GetValues(%v) unexpected error: %v", tt.keys, err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetValues(%v) = %v, want %v", tt.keys, result, tt.expected)
			}
		})
	}
}

func TestTransform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple key",
			input:    "/foo",
			expected: "FOO",
		},
		{
			name:     "nested key",
			input:    "/foo/bar/baz",
			expected: "FOO_BAR_BAZ",
		},
		{
			name:     "already uppercase",
			input:    "/FOO",
			expected: "FOO",
		},
		{
			name:     "no leading slash",
			input:    "foo",
			expected: "FOO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transform(tt.input)
			if result != tt.expected {
				t.Errorf("transform(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple key",
			input:    "FOO",
			expected: "/foo",
		},
		{
			name:     "nested key with underscores",
			input:    "FOO_BAR_BAZ",
			expected: "/foo/bar/baz",
		},
		{
			name:     "already lowercase",
			input:    "foo",
			expected: "/foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clean(tt.input)
			if result != tt.expected {
				t.Errorf("clean(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWatchPrefix(t *testing.T) {
	client, err := NewEnvClient()
	if err != nil {
		t.Fatalf("NewEnvClient() error: %v", err)
	}

	stopChan := make(chan bool, 1)

	// Send stop signal immediately
	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/test", []string{"/test/key"}, 0, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}

func TestWatchPrefix_ContextCancellation(t *testing.T) {
	client, err := NewEnvClient()
	if err != nil {
		t.Fatalf("NewEnvClient() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stopChan := make(chan bool)
	waitIndex := uint64(42)

	index, err := client.WatchPrefix(ctx, "/test", []string{"/test/key"}, waitIndex, stopChan)
	if err != context.Canceled {
		t.Errorf("WatchPrefix() error = %v, want context.Canceled", err)
	}
	if index != waitIndex {
		t.Errorf("WatchPrefix() index = %d, want %d", index, waitIndex)
	}
}

func TestWatchPrefix_ReturnsWaitIndex(t *testing.T) {
	client, err := NewEnvClient()
	if err != nil {
		t.Fatalf("NewEnvClient() error: %v", err)
	}

	stopChan := make(chan bool, 1)
	waitIndex := uint64(123)

	go func() {
		stopChan <- true
	}()

	index, err := client.WatchPrefix(context.Background(), "/test", []string{"/test/key"}, waitIndex, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != waitIndex {
		t.Errorf("WatchPrefix() index = %d, want %d", index, waitIndex)
	}
}

func TestHealthCheck(t *testing.T) {
	client, err := NewEnvClient()
	if err != nil {
		t.Fatalf("NewEnvClient() error: %v", err)
	}

	// HealthCheck for env backend should always return nil
	err = client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}
