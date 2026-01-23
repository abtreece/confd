//go:build e2e

package operations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestMetrics_PrometheusFormat verifies that the /metrics endpoint returns
// valid Prometheus format (lines starting with # HELP).
func TestMetrics_PrometheusFormat(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("metrics.conf")

	// Write template
	env.WriteTemplate("metrics.tmpl", `service: {{ getv "/service/name" }}`)

	// Write config
	env.WriteConfig("metrics.toml", fmt.Sprintf(`[template]
src = "metrics.tmpl"
dest = "%s"
keys = ["/service/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("SERVICE_NAME", "metrics-test")
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

	// Test /metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		t.Fatalf("Failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	metricsContent := string(body)

	// Verify Prometheus format (should contain # HELP lines)
	if !strings.Contains(metricsContent, "# HELP") {
		t.Errorf("Response doesn't look like Prometheus format (missing # HELP lines)")
		t.Logf("Response preview: %s", truncate(metricsContent, 500))
	}
}

// TestMetrics_ConfdMetricsPresent verifies that key confd metrics are exposed.
func TestMetrics_ConfdMetricsPresent(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("confd-metrics.conf")

	// Write template
	env.WriteTemplate("confd-metrics.tmpl", `service: {{ getv "/service/name" }}`)

	// Write config
	env.WriteConfig("confd-metrics.toml", fmt.Sprintf(`[template]
src = "confd-metrics.tmpl"
dest = "%s"
keys = ["/service/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("SERVICE_NAME", "confd-metrics-test")
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

	// Trigger a health check to populate backend metrics
	http.Get(fmt.Sprintf("http://%s/ready", addr))
	time.Sleep(100 * time.Millisecond)

	// Fetch metrics
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		t.Fatalf("Failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	metricsContent := string(body)

	// Check for key confd metrics
	requiredMetrics := []string{
		"confd_templates_loaded",
		"confd_backend_healthy",
		"confd_template_cache_hits_total",
	}

	for _, metric := range requiredMetrics {
		if !strings.Contains(metricsContent, metric) {
			t.Errorf("Missing metric: %s", metric)
		}
	}
}

// TestMetrics_GoRuntimeMetrics verifies that Go runtime metrics are exposed.
func TestMetrics_GoRuntimeMetrics(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("go-metrics.conf")

	// Write template
	env.WriteTemplate("go-metrics.tmpl", `service: {{ getv "/service/name" }}`)

	// Write config
	env.WriteConfig("go-metrics.toml", fmt.Sprintf(`[template]
src = "go-metrics.tmpl"
dest = "%s"
keys = ["/service/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("SERVICE_NAME", "go-metrics-test")
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

	// Fetch metrics
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		t.Fatalf("Failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	metricsContent := string(body)

	// Check for Go runtime metrics
	if !strings.Contains(metricsContent, "go_goroutines") {
		t.Error("Missing Go runtime metric: go_goroutines")
	}
}

// TestMetrics_BackendHealthValue verifies that the confd_backend_healthy metric
// has value 1 (healthy) when the backend is functioning.
func TestMetrics_BackendHealthValue(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	port := GetFreePort(t)
	metricsAddr := fmt.Sprintf(":%d", port)
	destPath := env.DestPath("health-value.conf")

	// Write template
	env.WriteTemplate("health-value.tmpl", `service: {{ getv "/service/name" }}`)

	// Write config
	env.WriteConfig("health-value.toml", fmt.Sprintf(`[template]
src = "health-value.tmpl"
dest = "%s"
keys = ["/service/name"]
`, destPath))

	// Start confd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	confd := NewConfdBinary(t)
	confd.SetEnv("SERVICE_NAME", "health-value-test")
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

	// Trigger a health check to populate backend metrics
	http.Get(fmt.Sprintf("http://%s/ready", addr))
	time.Sleep(100 * time.Millisecond)

	// Fetch metrics
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		t.Fatalf("Failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	metricsContent := string(body)

	// Parse confd_backend_healthy metric value
	// Format: confd_backend_healthy{backend="env"} 1
	re := regexp.MustCompile(`(?m)^confd_backend_healthy\{[^}]*\}\s+(\d+)`)
	matches := re.FindStringSubmatch(metricsContent)
	if len(matches) < 2 {
		t.Fatalf("Could not find confd_backend_healthy metric in:\n%s", truncate(metricsContent, 1000))
	}

	if matches[1] != "1" {
		t.Errorf("Expected backend health=1, got %s", matches[1])
	}
}

// truncate returns the first n characters of s, or s if it's shorter than n.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
