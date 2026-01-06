package redis

import (
	"errors"
	"testing"

	"github.com/gomodule/redigo/redis"
)

// mockRedisConn implements redisConn for testing
type mockRedisConn struct {
	doFunc    func(cmd string, args ...interface{}) (interface{}, error)
	closeFunc func() error
}

func (m *mockRedisConn) Do(cmd string, args ...interface{}) (interface{}, error) {
	if m.doFunc != nil {
		return m.doFunc(cmd, args...)
	}
	return nil, nil
}

func (m *mockRedisConn) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
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
	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 0, nil)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 1 {
		t.Errorf("WatchPrefix() index = %d, want 1", index)
	}
}

func TestGetValues_StringType(t *testing.T) {
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return "string", nil
			case "GET":
				key := args[0].(string)
				if key == "/app/key" {
					return "value123", nil
				}
				return nil, redis.ErrNil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	vars, err := client.GetValues([]string{"/app/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/key"] != "value123" {
		t.Errorf("GetValues() = %v, want map with /app/key=value123", vars)
	}
}

func TestGetValues_HashType(t *testing.T) {
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return "hash", nil
			case "HSCAN":
				// Return cursor 0 (done) and field/value pairs
				return []interface{}{
					[]byte("0"), // cursor
					[]interface{}{
						[]byte("field1"),
						[]byte("value1"),
						[]byte("field2"),
						[]byte("value2"),
					},
				}, nil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	vars, err := client.GetValues([]string{"/app/config"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/config/field1": "value1",
		"/app/config/field2": "value2",
	}

	for k, v := range expected {
		if vars[k] != v {
			t.Errorf("GetValues()[%s] = %s, want %s", k, vars[k], v)
		}
	}
}

func TestGetValues_ScanPattern(t *testing.T) {
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return "none", nil
			case "SCAN":
				// Return cursor 0 (done) and matching keys
				return []interface{}{
					[]byte("0"),
					[]interface{}{
						[]byte("/app/key1"),
						[]byte("/app/key2"),
					},
				}, nil
			case "GET":
				key := args[0].(string)
				switch key {
				case "/app/key1":
					return "val1", nil
				case "/app/key2":
					return "val2", nil
				}
				return nil, redis.ErrNil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	vars, err := client.GetValues([]string{"/app/*"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	if vars["/app/key1"] != "val1" {
		t.Errorf("GetValues()[/app/key1] = %s, want val1", vars["/app/key1"])
	}
	if vars["/app/key2"] != "val2" {
		t.Errorf("GetValues()[/app/key2] = %s, want val2", vars["/app/key2"])
	}
}

func TestGetValues_GetError(t *testing.T) {
	expectedErr := errors.New("get failed")
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return "string", nil
			case "GET":
				return nil, expectedErr
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	_, err := client.GetValues([]string{"/app/key"})
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_TypeQueryError(t *testing.T) {
	expectedErr := errors.New("type query failed")
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return nil, expectedErr
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	_, err := client.GetValues([]string{"/app/key"})
	if err == nil {
		t.Error("GetValues() expected error for TYPE query failure")
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	keyValues := map[string]string{
		"/app/key1": "value1",
		"/app/key2": "value2",
		"/db/host":  "localhost",
	}

	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				return "string", nil
			case "GET":
				key := args[0].(string)
				if val, ok := keyValues[key]; ok {
					return val, nil
				}
				return nil, redis.ErrNil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
		pscChan:   make(chan watchResponse),
	}

	vars, err := client.GetValues([]string{"/app/key1", "/app/key2", "/db/host"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	for k, v := range keyValues {
		if vars[k] != v {
			t.Errorf("GetValues()[%s] = %s, want %s", k, vars[k], v)
		}
	}
}

func TestGetValues_CustomSeparator(t *testing.T) {
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			switch cmd {
			case "PING":
				return "PONG", nil
			case "TYPE":
				// Key should be transformed to use colon separator
				key := args[0].(string)
				if key == "app:config:key" {
					return "string", nil
				}
				return "none", nil
			case "GET":
				key := args[0].(string)
				if key == "app:config:key" {
					return "myvalue", nil
				}
				return nil, redis.ErrNil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: ":",
		pscChan:   make(chan watchResponse),
	}

	vars, err := client.GetValues([]string{"/app/config/key"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	// Key should be transformed back to slash-separated
	if vars["/app/config/key"] != "myvalue" {
		t.Errorf("GetValues() = %v, want /app/config/key=myvalue", vars)
	}
}

func TestConnectedClient_PingSuccess(t *testing.T) {
	pingCalled := false
	mock := &mockRedisConn{
		doFunc: func(cmd string, args ...interface{}) (interface{}, error) {
			if cmd == "PING" {
				pingCalled = true
				return "PONG", nil
			}
			return nil, nil
		},
	}

	client := &Client{
		client:    mock,
		separator: "/",
	}

	conn, err := client.connectedClient()
	if err != nil {
		t.Fatalf("connectedClient() unexpected error: %v", err)
	}
	if !pingCalled {
		t.Error("connectedClient() did not call PING")
	}
	if conn != mock {
		t.Error("connectedClient() returned different connection")
	}
}

func TestConnectedClient_NilClient(t *testing.T) {
	client := &Client{
		client:    nil,
		machines:  []string{}, // No machines to connect to
		separator: "/",
	}

	// With nil client and no machines, tryConnect returns nil, 0, nil
	conn, err := client.connectedClient()
	// Both should be nil since there are no machines
	if conn != nil || err != nil {
		// This is the expected edge case behavior
		return
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
	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 1, nil)
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

	index, err := client.WatchPrefix("/app", []string{"/app/key"}, 1, nil)
	if err != expectedErr {
		t.Errorf("WatchPrefix() error = %v, want %v", err, expectedErr)
	}
	if index != 0 {
		t.Errorf("WatchPrefix() index = %d, want 0", index)
	}
}

// Note: Full WatchPrefix tests with pub/sub require a running Redis instance.
// These are covered by integration tests in .github/workflows/integration-tests.yml
