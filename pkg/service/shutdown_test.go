package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockStoreClient is a minimal StoreClient for shutdown tests.
type mockStoreClient struct {
	closeErr error
	closed   bool
}

func (m *mockStoreClient) GetValues(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}

func (m *mockStoreClient) WatchPrefix(_ context.Context, _ string, _ []string, _ uint64, _ chan bool) (uint64, error) {
	return 0, nil
}

func (m *mockStoreClient) HealthCheck(_ context.Context) error {
	return nil
}

func (m *mockStoreClient) Close() error {
	m.closed = true
	return m.closeErr
}

func TestNewShutdownManager(t *testing.T) {
	client := &mockStoreClient{}
	mgr := NewShutdownManager(30*time.Second, nil, client)
	if mgr == nil {
		t.Fatal("expected non-nil ShutdownManager")
	}
	if mgr.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", mgr.timeout)
	}
	if mgr.storeClient != client {
		t.Error("storeClient not set correctly")
	}
}

func TestShutdown_ClosesStoreClient(t *testing.T) {
	client := &mockStoreClient{}
	mgr := NewShutdownManager(5*time.Second, nil, client)

	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if !client.closed {
		t.Error("expected storeClient.Close() to be called")
	}
}

func TestShutdown_StoreClientCloseError(t *testing.T) {
	closeErr := errors.New("connection reset")
	client := &mockStoreClient{closeErr: closeErr}
	mgr := NewShutdownManager(5*time.Second, nil, client)

	err := mgr.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected error from Shutdown, got nil")
	}
	if !errors.Is(err, closeErr) {
		t.Errorf("error = %v, want to wrap %v", err, closeErr)
	}
}

func TestShutdown_NilStoreClient(t *testing.T) {
	mgr := NewShutdownManager(5*time.Second, nil, nil)
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown with nil storeClient returned error: %v", err)
	}
}

func TestShutdown_WithMetricsServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	// Convert httptest.Server to http.Server for shutdown
	httpSrv := &http.Server{Addr: srv.Listener.Addr().String()}
	httpSrv.Handler = http.DefaultServeMux
	// Start the server on the test listener
	go func() { _ = httpSrv.Serve(srv.Listener) }()

	client := &mockStoreClient{}
	mgr := NewShutdownManager(5*time.Second, httpSrv, client)

	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if !client.closed {
		t.Error("expected storeClient.Close() to be called")
	}
}

func TestShutdown_NilMetricsServer(t *testing.T) {
	client := &mockStoreClient{}
	mgr := NewShutdownManager(5*time.Second, nil, client)

	// Should not panic with nil metrics server
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestShutdown_TimeoutPropagated(t *testing.T) {
	// A very short timeout should still complete without panic when
	// there is no metrics server and the store client closes quickly.
	client := &mockStoreClient{}
	mgr := NewShutdownManager(1*time.Millisecond, nil, client)

	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}
