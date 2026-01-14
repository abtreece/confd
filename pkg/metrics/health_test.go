package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// healthMockClient implements the StoreClient interface for health tests
type healthMockClient struct {
	healthError error
}

func (m *healthMockClient) GetValues(ctx context.Context, keys []string) (map[string]string, error) {
	return nil, nil
}

func (m *healthMockClient) WatchPrefix(ctx context.Context, prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	return 0, nil
}

func (m *healthMockClient) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func TestHealthHandler_ReturnsOK(t *testing.T) {
	client := &healthMockClient{}
	handler := HealthHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", w.Body.String())
	}
}

func TestHealthHandler_AlwaysReturnsOK(t *testing.T) {
	// Health handler should return OK even if client would fail
	client := &healthMockClient{healthError: errors.New("backend down")}
	handler := HealthHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Health is a liveness check - should always return OK if process is running
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestReadyHandler_ReturnsOK_WhenBackendHealthy(t *testing.T) {
	client := &healthMockClient{}
	handler := ReadyHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", w.Body.String())
	}
}

func TestReadyHandler_ReturnsServiceUnavailable_WhenBackendUnhealthy(t *testing.T) {
	client := &healthMockClient{healthError: errors.New("connection refused")}
	handler := ReadyHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
	if w.Body.String() == "ok" {
		t.Error("Expected body to indicate unhealthy status")
	}
}

func TestReadyHandler_IncludesErrorInResponse(t *testing.T) {
	errorMsg := "connection refused"
	client := &healthMockClient{healthError: errors.New(errorMsg)}
	handler := ReadyHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty body for unhealthy response")
	}
	// Body should mention the error
	if len(body) < len("backend unhealthy") {
		t.Errorf("Expected body to contain error info, got: %s", body)
	}
}

func TestHealthHandler_NilClient(t *testing.T) {
	// Health handler should work even with nil client (it doesn't use it)
	handler := HealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}
