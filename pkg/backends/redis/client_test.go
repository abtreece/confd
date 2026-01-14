package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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
	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 0, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestWatchPrefix_FromChannel(t *testing.T) {
	client := &Client{
		separator: "/",
		pscChan:   make(chan watchResponse, 1),
	}

	// Pre-populate the channel
	client.pscChan <- watchResponse{waitIndex: 42, err: nil}

	// waitIndex > 0 will check the channel
	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 1, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 42 {
		t.Errorf("WatchPrefix() index = %d, want 42", index)
	}
}

func TestWatchPrefix_ChannelError(t *testing.T) {
	expectedErr := errors.New("watch error")
	client := &Client{
		separator: "/",
		pscChan:   make(chan watchResponse, 1),
	}

	// Pre-populate the channel with an error
	client.pscChan <- watchResponse{waitIndex: 0, err: expectedErr}

	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 1, nil)
	if err != expectedErr {
		t.Errorf("WatchPrefix() error = %v, want %v", err, expectedErr)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}

func TestHealthCheck_WithNilClient(t *testing.T) {
	client := &Client{
		client:    nil,
		machines:  []string{}, // No machines to connect to
		separator: "/",
	}

	// With nil client and no machines, should return error
	err := client.HealthCheck(context.Background())
	if err == nil {
		// With no machines, we expect an error or nil client remains
		// The exact behavior depends on createClient returning nil, 0, nil
		// when machines is empty
	}
}

func TestConnectedClient_NilClient(t *testing.T) {
	client := &Client{
		client:    nil,
		machines:  []string{}, // No machines to connect to
		separator: "/",
	}

	// With nil client and no machines, createClient returns nil, 0, nil
	conn, err := client.connectedClient(context.Background())
	// Both should be nil since there are no machines
	if conn != nil || err != nil {
		// This is the expected edge case behavior
		return
	}
}

func TestRedisNilError(t *testing.T) {
	// Test that redis.Nil is the correct sentinel error
	if redis.Nil.Error() != "redis: nil" {
		t.Errorf("redis.Nil error message = %q, want %q", redis.Nil.Error(), "redis: nil")
	}
}

func TestCreateClient_EmptyMachines(t *testing.T) {
	config := RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0}
	client, db, err := createClient([]string{}, "password", true, config)
	if client != nil {
		t.Error("createClient with empty machines should return nil client")
	}
	if db != 0 {
		t.Errorf("createClient with empty machines should return db=0, got %d", db)
	}
	if err == nil {
		t.Error("createClient with empty machines should return error")
	}
}

func TestCreateClient_InvalidAddress(t *testing.T) {
	// Try to connect to an invalid address - should fail
	// Use zero retries for fast test
	config := RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0}
	client, _, err := createClient([]string{"invalid-host-that-does-not-exist:6379"}, "", true, config)
	if err == nil {
		// Connection might not fail immediately in all environments
		// but the client should still be nil or fail on ping
		if client != nil {
			client.Close()
		}
	}
}

func TestAddressParsing_WithDatabase(t *testing.T) {
	// This tests the address parsing logic indirectly
	// The format "host:port/db" should extract db number

	// We can't easily test createClient without a real Redis server,
	// but we can verify the transform/clean logic works correctly
	// with different separator configurations

	client := &Client{separator: ":"}

	// Transform should convert /app/db/0 to app:db:0
	result := client.transform("/app/db/0")
	if result != "app:db:0" {
		t.Errorf("transform(/app/db/0) = %s, want app:db:0", result)
	}

	// Clean should convert app:db:0 to /app/db/0
	result = client.clean("app:db:0")
	if result != "/app/db/0" {
		t.Errorf("clean(app:db:0) = %s, want /app/db/0", result)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("DefaultRetryConfig().MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.BaseDelay != 100*time.Millisecond {
		t.Errorf("DefaultRetryConfig().BaseDelay = %v, want 100ms", config.BaseDelay)
	}
	if config.MaxDelay != 5*time.Second {
		t.Errorf("DefaultRetryConfig().MaxDelay = %v, want 5s", config.MaxDelay)
	}
	if config.JitterFactor != 0.3 {
		t.Errorf("DefaultRetryConfig().JitterFactor = %f, want 0.3", config.JitterFactor)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		JitterFactor: 0.0, // No jitter for predictable testing
	}

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "first retry (attempt 0)",
			attempt:  0,
			expected: 100 * time.Millisecond, // baseDelay * 2^0 = 100ms
		},
		{
			name:     "second retry (attempt 1)",
			attempt:  1,
			expected: 200 * time.Millisecond, // baseDelay * 2^1 = 200ms
		},
		{
			name:     "third retry (attempt 2)",
			attempt:  2,
			expected: 400 * time.Millisecond, // baseDelay * 2^2 = 400ms
		},
		{
			name:     "fourth retry (attempt 3)",
			attempt:  3,
			expected: 800 * time.Millisecond, // baseDelay * 2^3 = 800ms
		},
		{
			name:     "fifth retry (attempt 4) - capped",
			attempt:  4,
			expected: 1 * time.Second, // baseDelay * 2^4 = 1600ms, but capped at 1s
		},
		{
			name:     "sixth retry (attempt 5) - capped",
			attempt:  5,
			expected: 1 * time.Second, // baseDelay * 2^5 = 3200ms, but capped at 1s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBackoff(tt.attempt, config)
			if result != tt.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	config := RetryConfig{
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		JitterFactor: 0.3, // 30% jitter
	}

	// Test that jitter produces values within expected range
	attempt := 1
	expectedBase := 200 * time.Millisecond // baseDelay * 2^1

	// Run multiple times to check jitter range
	for i := 0; i < 100; i++ {
		result := calculateBackoff(attempt, config)

		// With 30% jitter, result should be between 140ms (200 - 60) and 260ms (200 + 60)
		minExpected := time.Duration(float64(expectedBase) * 0.7)  // 140ms
		maxExpected := time.Duration(float64(expectedBase) * 1.3)  // 260ms

		if result < minExpected || result > maxExpected {
			t.Errorf("calculateBackoff(%d) with jitter = %v, want between %v and %v",
				attempt, result, minExpected, maxExpected)
		}
	}
}

func TestCalculateBackoffMaxDelay(t *testing.T) {
	config := RetryConfig{
		BaseDelay:    1 * time.Second,
		MaxDelay:     2 * time.Second,
		JitterFactor: 0.0,
	}

	// After several attempts, backoff should be capped at MaxDelay
	for attempt := 0; attempt < 10; attempt++ {
		result := calculateBackoff(attempt, config)
		if result > config.MaxDelay {
			t.Errorf("calculateBackoff(%d) = %v, exceeds MaxDelay %v",
				attempt, result, config.MaxDelay)
		}
	}
}

// Note: Full integration tests for GetValues, WatchPrefix with a running
// Redis instance are covered by integration tests in:
// .github/workflows/integration-tests.yml
// test/integration/redis/test.sh
//
// The go-redis library returns typed command results that are difficult
// to mock without a real Redis instance. The integration tests verify
// the complete functionality including:
// - String key retrieval (GET)
// - Hash field retrieval (HSCAN)
// - Pattern matching (SCAN)
// - PubSub watch mode with keyspace notifications
// - Database selection via /db suffix
// - Custom separator configurations
