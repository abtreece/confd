//go:build e2e

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestHealthcheck_HealthEndpoint verifies that the /health endpoint returns
// HTTP 200 with body "ok".
func TestHealthcheck_HealthEndpoint(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("health.conf")

	// Write template
	env.WriteTemplate("health.tmpl", `app: {{ getv "/app/name" }}
status: running
`)

	// Write config
	env.WriteConfig("health.toml", fmt.Sprintf(`[template]
mode = "0644"
src = "health.tmpl"
dest = "%s"
keys = ["/app/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("APP_NAME", "healthcheck-test")
	err := confd.Start(ctx, "env", "--watch", "--confdir", env.ConfDir, "--metrics-addr", metricsAddr, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for confd to be ready
	addr := fmt.Sprintf("localhost:%d", port)
	if err := confd.WaitForReady(ctx, addr, 10*time.Second); err != nil {
		t.Fatalf("Confd did not become ready: %v", err)
	}

	// Test /health endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "ok" {
		t.Errorf("Expected body 'ok', got %q", string(body))
	}
}

// TestHealthcheck_ReadyEndpoint verifies that the /ready endpoint returns
// HTTP 200 when the backend is healthy.
func TestHealthcheck_ReadyEndpoint(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("ready.conf")

	// Write template
	env.WriteTemplate("ready.tmpl", `app: {{ getv "/app/name" }}`)

	// Write config
	env.WriteConfig("ready.toml", fmt.Sprintf(`[template]
src = "ready.tmpl"
dest = "%s"
keys = ["/app/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("APP_NAME", "ready-test")
	err := confd.Start(ctx, "env", "--watch", "--confdir", env.ConfDir, "--metrics-addr", metricsAddr, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for confd to be ready
	addr := fmt.Sprintf("localhost:%d", port)
	if err := confd.WaitForReady(ctx, addr, 10*time.Second); err != nil {
		t.Fatalf("Confd did not become ready: %v", err)
	}

	// Test /ready endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/ready", addr))
	if err != nil {
		t.Fatalf("Failed to GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestHealthcheck_ReadyDetailedEndpoint verifies that the /ready/detailed endpoint
// returns valid JSON with all required fields.
func TestHealthcheck_ReadyDetailedEndpoint(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("detailed.conf")

	// Write template
	env.WriteTemplate("detailed.tmpl", `app: {{ getv "/app/name" }}`)

	// Write config
	env.WriteConfig("detailed.toml", fmt.Sprintf(`[template]
src = "detailed.tmpl"
dest = "%s"
keys = ["/app/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("APP_NAME", "detailed-test")
	err := confd.Start(ctx, "env", "--watch", "--confdir", env.ConfDir, "--metrics-addr", metricsAddr, "--log-level", "error")
	if err != nil {
		t.Fatalf("Failed to start confd: %v", err)
	}
	defer confd.Stop()

	// Wait for confd to be ready
	addr := fmt.Sprintf("localhost:%d", port)
	if err := confd.WaitForReady(ctx, addr, 10*time.Second); err != nil {
		t.Fatalf("Confd did not become ready: %v", err)
	}

	// Test /ready/detailed endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/ready/detailed", addr))
	if err != nil {
		t.Fatalf("Failed to GET /ready/detailed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Parse JSON response
	var result struct {
		Healthy    bool                   `json:"healthy"`
		Message    string                 `json:"message"`
		DurationMs float64                `json:"duration_ms"`
		Details    map[string]interface{} `json:"details"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, string(body))
	}

	// Verify required fields
	if !result.Healthy {
		t.Errorf("Expected healthy=true, got %v", result.Healthy)
	}

	if result.Message == "" {
		t.Error("Expected non-empty message field")
	}

	// DurationMs should be present (can be 0 or greater)
	if result.DurationMs < 0 {
		t.Errorf("Expected non-negative duration_ms, got %v", result.DurationMs)
	}

	if result.Details == nil {
		t.Error("Expected details field to be present")
	}
}
