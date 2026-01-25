package consul

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
)

// mockConsulKV implements the consulKVAPI interface for testing
type mockConsulKV struct {
	listFunc func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
}

func (m *mockConsulKV) List(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
	if m.listFunc != nil {
		return m.listFunc(prefix, q)
	}
	return nil, &api.QueryMeta{}, nil
}

// newTestClient creates a ConsulClient with a mock KV for testing
func newTestClient(mock *mockConsulKV) *ConsulClient {
	return &ConsulClient{
		client: mock,
	}
}

func TestGetValues_SingleKey(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			if prefix == "app/config" {
				return api.KVPairs{
					&api.KVPair{Key: "app/config/key", Value: []byte("value")},
				}, &api.QueryMeta{}, nil
			}
			return nil, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/app/config"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{"/app/config/key": "value"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_MultipleKeys(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			switch prefix {
			case "db":
				return api.KVPairs{
					&api.KVPair{Key: "db/host", Value: []byte("localhost")},
				}, &api.QueryMeta{}, nil
			case "cache":
				return api.KVPairs{
					&api.KVPair{Key: "cache/host", Value: []byte("redis")},
				}, &api.QueryMeta{}, nil
			}
			return nil, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)

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

func TestGetValues_MultiplePairs(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return api.KVPairs{
				&api.KVPair{Key: "app/db/host", Value: []byte("localhost")},
				&api.KVPair{Key: "app/db/port", Value: []byte("5432")},
				&api.KVPair{Key: "app/db/name", Value: []byte("mydb")},
			}, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/app/db"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{
		"/app/db/host": "localhost",
		"/app/db/port": "5432",
		"/app/db/name": "mydb",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_EmptyResult(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{"/missing"})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_Error(t *testing.T) {
	expectedErr := errors.New("consul error")
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, nil, expectedErr
		},
	}

	client := newTestClient(mock)

	_, err := client.GetValues(context.Background(), []string{"/app"})
	if err == nil {
		t.Error("GetValues() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("GetValues() error = %v, want %v", err, expectedErr)
	}
}

func TestGetValues_EmptyKeys(t *testing.T) {
	mock := &mockConsulKV{}
	client := newTestClient(mock)

	result, err := client.GetValues(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetValues() unexpected error: %v", err)
	}

	expected := map[string]string{}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetValues() = %v, want %v", result, expected)
	}
}

func TestGetValues_LeadingSlashHandling(t *testing.T) {
	var capturedPrefix string
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			capturedPrefix = prefix
			return nil, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)
	client.GetValues(context.Background(), []string{"/app/config"})

	// Leading slash should be trimmed
	if capturedPrefix != "app/config" {
		t.Errorf("Expected prefix 'app/config', got '%s'", capturedPrefix)
	}
}

func TestWatchPrefix_StopChannel(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			// This should not complete before stopChan
			return nil, &api.QueryMeta{LastIndex: 100}, nil
		},
	}

	client := newTestClient(mock)
	stopChan := make(chan bool, 1)

	// Send stop signal before watch can complete
	stopChan <- true

	index, err := client.WatchPrefix(context.Background(), "/app", []string{"/app/key"}, 10, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 10 {
		t.Errorf("WatchPrefix() index = %d, want 10 (unchanged)", index)
	}
}

func TestWatchPrefix_NewIndex(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, &api.QueryMeta{LastIndex: 200}, nil
		},
	}

	client := newTestClient(mock)
	stopChan := make(chan bool)

	index, err := client.WatchPrefix(context.Background(), "app", []string{"/app/key"}, 100, stopChan)
	if err != nil {
		t.Errorf("WatchPrefix() unexpected error: %v", err)
	}
	if index != 200 {
		t.Errorf("WatchPrefix() index = %d, want 200", index)
	}
}

func TestWatchPrefix_Error(t *testing.T) {
	expectedErr := errors.New("watch error")
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, nil, expectedErr
		},
	}

	client := newTestClient(mock)
	stopChan := make(chan bool)

	_, err := client.WatchPrefix(context.Background(), "app", []string{"/app/key"}, 100, stopChan)
	if err == nil {
		t.Error("WatchPrefix() expected error, got nil")
	}
}

func TestNew_BasicConfiguration(t *testing.T) {
	// Test New with basic configuration
	client, err := New([]string{"localhost:8500"}, "http", "", "", "", false, "", "")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client")
	}
}

func TestNew_WithNodes(t *testing.T) {
	// Test New with multiple nodes (should use first node)
	client, err := New([]string{"node1:8500", "node2:8500", "node3:8500"}, "http", "", "", "", false, "", "")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client")
	}
}

func TestNew_EmptyNodes(t *testing.T) {
	// Test New with empty nodes list (should use default Consul address)
	client, err := New([]string{}, "http", "", "", "", false, "", "")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client")
	}
}

func TestNew_WithBasicAuth(t *testing.T) {
	// Test New with basic authentication
	client, err := New([]string{"localhost:8500"}, "http", "", "", "", true, "user", "pass")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client")
	}
}

func TestNew_WithHTTPS(t *testing.T) {
	// Test New with HTTPS scheme
	client, err := New([]string{"localhost:8500"}, "https", "", "", "", false, "", "")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if client == nil {
		t.Error("New() returned nil client")
	}
}

// Note: TLS configuration tests with valid certificates require integration tests
// as the Consul SDK validates certificate content. The basic New() tests above
// cover the config setup paths that don't require certificate validation.

func TestHealthCheck_Success(t *testing.T) {
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, &api.QueryMeta{}, nil
		},
	}

	client := newTestClient(mock)

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() unexpected error: %v", err)
	}
}

func TestHealthCheck_Error(t *testing.T) {
	expectedErr := errors.New("connection refused")
	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			return nil, nil, expectedErr
		},
	}

	client := newTestClient(mock)

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("HealthCheck() error = %v, want %v", err, expectedErr)
	}
}

func TestWatchPrefix_ContextCancel_NoLeak(t *testing.T) {
	// This test verifies that when context is cancelled while the goroutine
	// is blocked on List(), WatchPrefix returns promptly and no goroutine leaks.
	// The fix uses a buffered channel (size 1) so the goroutine can send its
	// response and exit even when no receiver is waiting.

	listStarted := make(chan struct{})
	listCanProceed := make(chan struct{})

	mock := &mockConsulKV{
		listFunc: func(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
			// Signal that List() has started
			close(listStarted)
			// Block until test allows us to proceed
			<-listCanProceed
			return nil, &api.QueryMeta{LastIndex: 200}, nil
		},
	}

	client := newTestClient(mock)
	stopChan := make(chan bool)

	// Record goroutine count before test
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())

	// Start WatchPrefix in a goroutine
	resultChan := make(chan struct {
		index uint64
		err   error
	})
	go func() {
		index, err := client.WatchPrefix(ctx, "app", []string{"/app/key"}, 100, stopChan)
		resultChan <- struct {
			index uint64
			err   error
		}{index, err}
	}()

	// Wait for List() to start (goroutine is now blocked)
	<-listStarted

	// Cancel the context while goroutine is blocked on List()
	cancel()

	// WatchPrefix should return promptly with context error
	select {
	case result := <-resultChan:
		if !errors.Is(result.err, context.Canceled) {
			t.Errorf("WatchPrefix() error = %v, want context.Canceled", result.err)
		}
		if result.index != 100 {
			t.Errorf("WatchPrefix() index = %d, want 100 (unchanged)", result.index)
		}
	case <-time.After(time.Second):
		t.Fatal("WatchPrefix() did not return promptly after context cancellation")
	}

	// Allow the blocked goroutine to complete
	close(listCanProceed)

	// Give the goroutine time to send on the buffered channel and exit
	time.Sleep(50 * time.Millisecond)

	// Verify no goroutine leak - count should return to initial (or very close)
	// Allow for some variance due to runtime goroutines
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+1 {
		t.Errorf("Possible goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}
