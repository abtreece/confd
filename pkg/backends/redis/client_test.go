package redis

import (
	"testing"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		input     string
		expected  string
	}{
		{
			name:      "default separator (no change)",
			separator: "/",
			input:     "/app/config/key",
			expected:  "/app/config/key",
		},
		{
			name:      "colon separator",
			separator: ":",
			input:     "/app/config/key",
			expected:  "app:config:key",
		},
		{
			name:      "dot separator",
			separator: ".",
			input:     "/db/host",
			expected:  "db.host",
		},
		{
			name:      "underscore separator",
			separator: "_",
			input:     "/service/name",
			expected:  "service_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{separator: tt.separator}
			result := client.transform(tt.input)
			if result != tt.expected {
				t.Errorf("transform(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		input     string
		expected  string
	}{
		{
			name:      "default separator (no change)",
			separator: "/",
			input:     "/app/config/key",
			expected:  "/app/config/key",
		},
		{
			name:      "colon separator to slash",
			separator: ":",
			input:     "app:config:key",
			expected:  "/app/config/key",
		},
		{
			name:      "add leading slash",
			separator: "/",
			input:     "app/config",
			expected:  "/app/config",
		},
		{
			name:      "dot separator to slash",
			separator: ".",
			input:     "db.host.name",
			expected:  "/db/host/name",
		},
		{
			name:      "already has leading slash with colon",
			separator: ":",
			input:     "/app:config",
			expected:  "/app/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{separator: tt.separator}
			result := client.clean(tt.input)
			if result != tt.expected {
				t.Errorf("clean(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWatchPrefix_InitialCall(t *testing.T) {
	client := &Client{
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	// waitIndex 0 should return immediately with 1
	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 0, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

// Note: Full GetValues and WatchPrefix tests require a running Redis instance
// or significant refactoring to support mocking. These are covered by
// integration tests in .github/workflows/integration-tests.yml
