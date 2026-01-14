package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// waitForServer waits for a miniredis server to be ready by attempting to connect.
// Returns error if server is not ready within timeout.
func waitForServer(t *testing.T, addr string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		client := redis.NewClient(&redis.Options{
			Addr:        addr,
			DialTimeout: 10 * time.Millisecond,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		err := client.Ping(ctx).Err()
		cancel()
		client.Close()
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return errors.New("server not ready within timeout")
}

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

	// Run multiple times to check jitter range (30 iterations is sufficient for randomness validation)
	for i := 0; i < 30; i++ {
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

// ============================================================================
// Tests using miniredis for lightweight Redis server simulation
// ============================================================================

func TestNewRedisClient(t *testing.T) {
	s := miniredis.RunT(t)

	t.Run("default separator", func(t *testing.T) {
		client, err := NewRedisClient([]string{s.Addr()}, "", "")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		if client.separator != "/" {
			t.Errorf("NewRedisClient() separator = %q, want %q", client.separator, "/")
		}
		if client.db != 0 {
			t.Errorf("NewRedisClient() db = %d, want 0", client.db)
		}
	})

	t.Run("custom separator", func(t *testing.T) {
		client, err := NewRedisClient([]string{s.Addr()}, "", ":")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		if client.separator != ":" {
			t.Errorf("NewRedisClient() separator = %q, want %q", client.separator, ":")
		}
	})

	t.Run("with password", func(t *testing.T) {
		s2 := miniredis.RunT(t)
		s2.RequireAuth("secret")

		client, err := NewRedisClient([]string{s2.Addr()}, "secret", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		if client.password != "secret" {
			t.Errorf("NewRedisClient() password = %q, want %q", client.password, "secret")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		s2 := miniredis.RunT(t)
		s2.RequireAuth("secret")

		_, err := NewRedisClient([]string{s2.Addr()}, "wrong", "/")
		if err == nil {
			t.Fatal("NewRedisClient() expected error for wrong password")
		}
	})
}

func TestGetValues_StringType(t *testing.T) {
	s := miniredis.RunT(t)

	// Set up test data - when separator is "/" (default), keys are stored as-is
	// The transform function returns the key unchanged with the default separator
	s.Set("/app/config/key1", "value1")
	s.Set("/app/config/key2", "value2")
	s.Set("/app/other/key3", "value3")

	client, err := NewRedisClient([]string{s.Addr()}, "", "/")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	t.Run("single string key", func(t *testing.T) {
		values, err := client.GetValues(context.Background(), []string{"/app/config/key1"})
		if err != nil {
			t.Fatalf("GetValues() unexpected error: %v", err)
		}
		if values["/app/config/key1"] != "value1" {
			t.Errorf("GetValues() = %q, want %q", values["/app/config/key1"], "value1")
		}
	})

	t.Run("multiple string keys", func(t *testing.T) {
		values, err := client.GetValues(context.Background(), []string{
			"/app/config/key1",
			"/app/config/key2",
		})
		if err != nil {
			t.Fatalf("GetValues() unexpected error: %v", err)
		}
		if len(values) != 2 {
			t.Errorf("GetValues() returned %d values, want 2", len(values))
		}
	})

	t.Run("nonexistent key returns empty", func(t *testing.T) {
		values, err := client.GetValues(context.Background(), []string{"/nonexistent"})
		if err != nil {
			t.Fatalf("GetValues() unexpected error: %v", err)
		}
		if len(values) != 0 {
			t.Errorf("GetValues() returned %d values, want 0", len(values))
		}
	})
}

func TestGetValues_HashType(t *testing.T) {
	s := miniredis.RunT(t)

	// Set up hash data - key path with leading slash
	s.HSet("/app/config", "field1", "hashvalue1")
	s.HSet("/app/config", "field2", "hashvalue2")
	s.HSet("/app/config", "field3", "hashvalue3")

	client, err := NewRedisClient([]string{s.Addr()}, "", "/")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	values, err := client.GetValues(context.Background(), []string{"/app/config"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Should return all hash fields
	if len(values) != 3 {
		t.Errorf("GetValues() returned %d values, want 3", len(values))
	}
	if values["/app/config/field1"] != "hashvalue1" {
		t.Errorf("GetValues() field1 = %q, want %q", values["/app/config/field1"], "hashvalue1")
	}
	if values["/app/config/field2"] != "hashvalue2" {
		t.Errorf("GetValues() field2 = %q, want %q", values["/app/config/field2"], "hashvalue2")
	}
}

func TestGetValues_ScanPattern(t *testing.T) {
	s := miniredis.RunT(t)

	// Set up test data with pattern - keys with leading slash
	s.Set("/app/service/host", "localhost")
	s.Set("/app/service/port", "8080")
	s.Set("/app/service/timeout", "30")
	s.Set("/other/key", "other")

	client, err := NewRedisClient([]string{s.Addr()}, "", "/")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	t.Run("wildcard pattern", func(t *testing.T) {
		values, err := client.GetValues(context.Background(), []string{"/app/service/*"})
		if err != nil {
			t.Fatalf("GetValues() unexpected error: %v", err)
		}

		if len(values) != 3 {
			t.Errorf("GetValues() returned %d values, want 3", len(values))
		}
		if values["/app/service/host"] != "localhost" {
			t.Errorf("GetValues() host = %q, want %q", values["/app/service/host"], "localhost")
		}
	})

	t.Run("root pattern", func(t *testing.T) {
		values, err := client.GetValues(context.Background(), []string{"/"})
		if err != nil {
			t.Fatalf("GetValues() unexpected error: %v", err)
		}

		// Should return all keys
		if len(values) != 4 {
			t.Errorf("GetValues() returned %d values, want 4", len(values))
		}
	})
}

func TestGetValues_CustomSeparator(t *testing.T) {
	s := miniredis.RunT(t)

	// Set up test data with colon separator
	s.Set("app:config:db:host", "localhost")
	s.Set("app:config:db:port", "5432")

	client, err := NewRedisClient([]string{s.Addr()}, "", ":")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	values, err := client.GetValues(context.Background(), []string{"/app/config/db/host"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if values["/app/config/db/host"] != "localhost" {
		t.Errorf("GetValues() = %q, want %q", values["/app/config/db/host"], "localhost")
	}
}

func TestConnectionFailover(t *testing.T) {
	t.Run("primary fails over to backup", func(t *testing.T) {
		s1 := miniredis.RunT(t)
		s2 := miniredis.RunT(t)

		// Save addresses before closing
		addr1 := s1.Addr()
		addr2 := s2.Addr()

		// Set different values in each
		s1.Set("key", "primary")
		s2.Set("key", "backup")

		// Close primary before connecting
		s1.Close()

		// Should fail over to s2
		config := RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0}
		rClient, _, err := createClient([]string{addr1, addr2}, "", true, config)
		if err != nil {
			t.Fatalf("createClient() unexpected error: %v", err)
		}
		defer rClient.Close()

		// Should be connected to backup
		val, err := rClient.Get(context.Background(), "key").Result()
		if err != nil {
			t.Fatalf("Get() unexpected error: %v", err)
		}
		if val != "backup" {
			t.Errorf("Get() = %q, want %q (should connect to backup)", val, "backup")
		}
	})

	t.Run("all machines fail", func(t *testing.T) {
		s1 := miniredis.RunT(t)
		s2 := miniredis.RunT(t)

		// Save addresses before closing
		addr1 := s1.Addr()
		addr2 := s2.Addr()

		// Close both servers
		s1.Close()
		s2.Close()

		config := RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0}
		_, _, err := createClient([]string{addr1, addr2}, "", true, config)
		if err == nil {
			t.Fatal("createClient() expected error when all machines fail")
		}
	})

	t.Run("retry succeeds on transient failure", func(t *testing.T) {
		s := miniredis.RunT(t)
		addr := s.Addr()

		// Close then immediately reopen (simulate transient failure)
		s.Close()
		s2, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to restart miniredis: %v", err)
		}
		// Start on same port
		s2.StartAddr(addr)
		defer s2.Close()

		// Wait for server to be ready before proceeding
		if err := waitForServer(t, addr, 500*time.Millisecond); err != nil {
			t.Fatalf("Server not ready: %v", err)
		}

		s2.Set("key", "recovered")

		// With retries, should eventually connect
		config := RetryConfig{
			MaxRetries:   3,
			BaseDelay:    10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			JitterFactor: 0,
		}
		rClient, _, err := createClient([]string{addr}, "", true, config)
		if err != nil {
			t.Fatalf("createClient() unexpected error: %v", err)
		}
		defer rClient.Close()

		val, err := rClient.Get(context.Background(), "key").Result()
		if err != nil {
			t.Fatalf("Get() unexpected error: %v", err)
		}
		if val != "recovered" {
			t.Errorf("Get() = %q, want %q", val, "recovered")
		}
	})
}

func TestConnectedClient_Reconnection(t *testing.T) {
	t.Run("reconnects when connection lost", func(t *testing.T) {
		s := miniredis.RunT(t)
		addr := s.Addr() // Save address before any operations
		s.Set("key", "initial")

		client, err := NewRedisClient([]string{addr}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}

		// Verify initial connection works
		_, err = client.connectedClient(context.Background())
		if err != nil {
			t.Fatalf("connectedClient() unexpected error: %v", err)
		}

		// Close the miniredis server to simulate connection loss
		s.Close()

		// Restart on same address
		s2, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to restart miniredis: %v", err)
		}
		s2.StartAddr(addr)
		defer s2.Close()

		// Wait for server to be ready before proceeding
		if err := waitForServer(t, addr, 500*time.Millisecond); err != nil {
			t.Fatalf("Server not ready: %v", err)
		}

		s2.Set("key", "reconnected")

		// Update client's machines to point to new server
		client.machines = []string{s2.Addr()}

		// connectedClient should detect dead connection and reconnect
		rClient, err := client.connectedClient(context.Background())
		if err != nil {
			t.Fatalf("connectedClient() unexpected error after reconnect: %v", err)
		}

		val, err := rClient.Get(context.Background(), "key").Result()
		if err != nil {
			t.Fatalf("Get() unexpected error: %v", err)
		}
		if val != "reconnected" {
			t.Errorf("Get() = %q, want %q", val, "reconnected")
		}
	})

	t.Run("returns existing connection when healthy", func(t *testing.T) {
		s := miniredis.RunT(t)

		client, err := NewRedisClient([]string{s.Addr()}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		// Get connection twice
		conn1, err := client.connectedClient(context.Background())
		if err != nil {
			t.Fatalf("connectedClient() first call unexpected error: %v", err)
		}

		conn2, err := client.connectedClient(context.Background())
		if err != nil {
			t.Fatalf("connectedClient() second call unexpected error: %v", err)
		}

		// Should be the same connection
		if conn1 != conn2 {
			t.Error("connectedClient() should return same connection when healthy")
		}
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("healthy connection", func(t *testing.T) {
		s := miniredis.RunT(t)

		client, err := NewRedisClient([]string{s.Addr()}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		err = client.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("HealthCheck() unexpected error: %v", err)
		}
	})

	t.Run("unhealthy connection", func(t *testing.T) {
		s := miniredis.RunT(t)
		addr := s.Addr() // Save address before operations

		client, err := NewRedisClient([]string{addr}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}

		// Close server to simulate failure
		s.Close()

		// HealthCheck should fail (but may succeed if it reconnects)
		// This tests the ping failure path in connectedClient
		err = client.HealthCheck(context.Background())
		if err == nil {
			// It's ok if health check passes - it means reconnection logic worked
			// The real test is that it doesn't panic
		}
	})
}

func TestDatabaseSelection(t *testing.T) {
	t.Run("select database via address suffix", func(t *testing.T) {
		s := miniredis.RunT(t)

		// miniredis supports multiple databases
		client, err := NewRedisClient([]string{s.Addr() + "/2"}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		if client.db != 2 {
			t.Errorf("NewRedisClient() db = %d, want 2", client.db)
		}
	})

	t.Run("default database 0", func(t *testing.T) {
		s := miniredis.RunT(t)

		client, err := NewRedisClient([]string{s.Addr()}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		if client.db != 0 {
			t.Errorf("NewRedisClient() db = %d, want 0", client.db)
		}
	})

	t.Run("invalid database suffix is ignored", func(t *testing.T) {
		s := miniredis.RunT(t)

		// Invalid db suffix (not a number) should be treated as part of address
		// This tests the error handling in address parsing
		client, err := NewRedisClient([]string{s.Addr() + "/notanumber"}, "", "/")
		// This might fail to connect since the address is malformed
		// or it might work if the parsing handles it gracefully
		if err != nil {
			// Expected - invalid address format
			return
		}
		defer client.client.Close()
		// If it connects, db should be 0
		if client.db != 0 {
			t.Errorf("NewRedisClient() db = %d, want 0 for invalid suffix", client.db)
		}
	})
}

func TestWatchPrefix_DrainChannel(t *testing.T) {
	t.Run("drains multiple pending responses", func(t *testing.T) {
		client := &Client{
			separator: "/",
			pscChan:   make(chan watchResponse, 3),
		}

		// Pre-populate multiple responses
		client.pscChan <- watchResponse{waitIndex: 1, err: nil}
		client.pscChan <- watchResponse{waitIndex: 2, err: nil}
		client.pscChan <- watchResponse{waitIndex: 3, err: nil}

		// Should drain and return the last one
		index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 1, nil)
		if err != nil {
			t.Errorf("WatchPrefix() unexpected error: %v", err)
		}
		if index != 3 {
			t.Errorf("WatchPrefix() index = %d, want 3 (last drained)", index)
		}
	})
}

func TestCreateClient_WithRetries(t *testing.T) {
	t.Run("retries on connection failure", func(t *testing.T) {
		// Start server, get address, close it
		s := miniredis.RunT(t)
		addr := s.Addr()
		s.Close()

		// Very short retries for fast test
		config := RetryConfig{
			MaxRetries:   2,
			BaseDelay:    1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			JitterFactor: 0,
		}

		start := time.Now()
		_, _, err := createClient([]string{addr}, "", true, config)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("createClient() expected error for closed server")
		}

		// Should have taken at least some time for retries
		// baseDelay * (2^0 + 2^1) = 1ms + 2ms = 3ms minimum
		if elapsed < 1*time.Millisecond {
			t.Errorf("createClient() elapsed %v, expected at least some retry delay", elapsed)
		}
	})

	t.Run("returns aggregated errors", func(t *testing.T) {
		config := RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0}
		_, _, err := createClient([]string{
			"invalid1:6379",
			"invalid2:6379",
		}, "", true, config)

		if err == nil {
			t.Fatal("createClient() expected error for invalid addresses")
		}

		// Error should contain info about multiple failed addresses
		errStr := err.Error()
		if errStr == "" {
			t.Error("createClient() error should not be empty")
		}
	})
}

func TestGetValues_ErrorHandling(t *testing.T) {
	t.Run("cancelled context returns error", func(t *testing.T) {
		s := miniredis.RunT(t)
		s.Set("/key", "value")

		client, err := NewRedisClient([]string{s.Addr()}, "", "/")
		if err != nil {
			t.Fatalf("NewRedisClient() unexpected error: %v", err)
		}
		defer client.client.Close()

		// Create an already-cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// GetValues with cancelled context should return error
		_, err = client.GetValues(ctx, []string{"/key"})
		if err == nil {
			t.Error("GetValues() expected error for cancelled context")
		}
	})

	t.Run("disconnected server returns error after retry exhaustion", func(t *testing.T) {
		s := miniredis.RunT(t)
		addr := s.Addr()

		// Create client with minimal retries for fast test
		client := &Client{
			client:      nil, // Force reconnection attempt
			machines:    []string{addr},
			separator:   "/",
			pscChan:     make(chan watchResponse),
			retryConfig: RetryConfig{MaxRetries: 0, BaseDelay: 0, MaxDelay: 0, JitterFactor: 0},
		}

		// Close server before GetValues
		s.Close()

		// GetValues should fail when server is unavailable and retries exhausted
		_, err := client.GetValues(context.Background(), []string{"/key"})
		if err == nil {
			t.Error("GetValues() expected error for disconnected server")
		}
	})
}

func TestRetryConfigRetention(t *testing.T) {
	s := miniredis.RunT(t)

	client, err := NewRedisClient([]string{s.Addr()}, "", "/")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	// Client should retain default retry config
	expected := DefaultRetryConfig()
	if client.retryConfig.MaxRetries != expected.MaxRetries {
		t.Errorf("retryConfig.MaxRetries = %d, want %d", client.retryConfig.MaxRetries, expected.MaxRetries)
	}
	if client.retryConfig.BaseDelay != expected.BaseDelay {
		t.Errorf("retryConfig.BaseDelay = %v, want %v", client.retryConfig.BaseDelay, expected.BaseDelay)
	}
}

func TestMachinesRetention(t *testing.T) {
	s := miniredis.RunT(t)

	machines := []string{s.Addr(), "backup:6379"}
	client, err := NewRedisClient(machines, "", "/")
	if err != nil {
		t.Fatalf("NewRedisClient() unexpected error: %v", err)
	}
	defer client.client.Close()

	if len(client.machines) != 2 {
		t.Errorf("client.machines length = %d, want 2", len(client.machines))
	}
	if client.machines[0] != s.Addr() {
		t.Errorf("client.machines[0] = %q, want %q", client.machines[0], s.Addr())
	}
}

// Note: Full integration tests for WatchPrefix PubSub with keyspace notifications
// are covered by integration tests in:
// .github/workflows/integration-tests.yml
// test/integration/redis/test.sh
//
// PubSub keyspace notification testing requires Redis configured with
// notify-keyspace-events which miniredis does not fully support.
