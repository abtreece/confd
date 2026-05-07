package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abtreece/confd/pkg/backends/types"
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

func (m *healthMockClient) Close() error {
	return nil
}

type detailedHealthMockClient struct {
	healthMockClient
	result *types.HealthResult
	err    error
}

func (m *detailedHealthMockClient) HealthCheckDetailed(ctx context.Context) (*types.HealthResult, error) {
	return m.result, m.err
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

func TestReadyDetailedHandler_ReturnsOK_WhenDetailedBackendHealthy(t *testing.T) {
	client := &detailedHealthMockClient{
		result: &types.HealthResult{
			Healthy:   true,
			Message:   "backend healthy",
			Duration:  types.DurationMillis(5 * time.Millisecond),
			CheckedAt: time.Now(),
			Details:   map[string]string{"version": "test"},
		},
	}
	handler := ReadyDetailedHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready?verbose=true", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var result types.HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !result.Healthy || result.Message != "backend healthy" {
		t.Fatalf("Response result = %#v", result)
	}
	if result.Details["version"] != "test" {
		t.Fatalf("Response details = %#v", result.Details)
	}
}

func TestReadyDetailedHandler_ReturnsUnavailable_WhenDetailedBackendUnhealthy(t *testing.T) {
	expectedErr := errors.New("backend unavailable")
	client := &detailedHealthMockClient{
		result: &types.HealthResult{
			Healthy:   false,
			Message:   expectedErr.Error(),
			CheckedAt: time.Now(),
			Details:   map[string]string{"error": expectedErr.Error()},
		},
		err: expectedErr,
	}
	handler := ReadyDetailedHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready?verbose=true", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var result types.HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result.Healthy {
		t.Fatalf("Response Healthy = true, want false")
	}
	if result.Details["error"] != expectedErr.Error() {
		t.Fatalf("Response details = %#v", result.Details)
	}
}

func TestReadyDetailedHandler_FallsBackToBasicHealthCheck(t *testing.T) {
	client := &healthMockClient{}
	handler := ReadyDetailedHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready?verbose=true", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result types.HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !result.Healthy {
		t.Fatalf("Response Healthy = false, want true")
	}
	if result.Message != "Backend does not support detailed health checks" {
		t.Fatalf("Response Message = %q", result.Message)
	}
}

func TestReadyDetailedHandler_FallbackReportsBasicHealthError(t *testing.T) {
	expectedErr := errors.New("connection refused")
	client := &healthMockClient{healthError: expectedErr}
	handler := ReadyDetailedHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/ready?verbose=true", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var result types.HealthResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result.Healthy {
		t.Fatalf("Response Healthy = true, want false")
	}
	if result.Message != expectedErr.Error() {
		t.Fatalf("Response Message = %q", result.Message)
	}
	if result.Details["error"] != expectedErr.Error() {
		t.Fatalf("Response details = %#v", result.Details)
	}
}
