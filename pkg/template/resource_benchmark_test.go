package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/abtreece/confd/pkg/backends/env"
	"github.com/abtreece/confd/pkg/log"
	"github.com/abtreece/confd/pkg/metrics"
)

// setupBenchmarkResource creates a minimal TemplateResource for benchmarking.
// It sets up temp directories, a simple template, and the env backend.
func setupBenchmarkResource(b *testing.B) (*TemplateResource, func()) {
	b.Helper()

	// Create temp directory structure
	confDir, err := os.MkdirTemp("", "confd-bench-")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	templatesDir := filepath.Join(confDir, "templates")
	confdDir := filepath.Join(confDir, "conf.d")
	destDir := filepath.Join(confDir, "dest")

	for _, dir := range []string{templatesDir, confdDir, destDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			os.RemoveAll(confDir)
			b.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create a simple template
	tmplContent := `# Config
key = "{{ getv "/benchmark/key" }}"
`
	tmplPath := filepath.Join(templatesDir, "bench.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		os.RemoveAll(confDir)
		b.Fatalf("Failed to write template: %v", err)
	}

	// Create resource config
	resourceContent := `[template]
src = "bench.tmpl"
dest = "` + filepath.Join(destDir, "bench.conf") + `"
keys = ["/benchmark/key"]
`
	resourcePath := filepath.Join(confdDir, "bench.toml")
	if err := os.WriteFile(resourcePath, []byte(resourceContent), 0644); err != nil {
		os.RemoveAll(confDir)
		b.Fatalf("Failed to write resource config: %v", err)
	}

	// Set up environment variable for the env backend
	os.Setenv("BENCHMARK_KEY", "test-value")

	// Create env backend client
	storeClient, err := env.NewEnvClient()
	if err != nil {
		os.RemoveAll(confDir)
		b.Fatalf("Failed to create env client: %v", err)
	}

	// Create the template resource
	config := Config{
		ConfDir:     confDir,
		ConfigDir:   confdDir,
		TemplateDir: templatesDir,
		StoreClient: storeClient,
		Prefix:      "",
	}

	tr, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		os.RemoveAll(confDir)
		b.Fatalf("Failed to create template resource: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(confDir)
		os.Unsetenv("BENCHMARK_KEY")
	}

	return tr, cleanup
}

// BenchmarkProcess_MetricsDisabled benchmarks process() without metrics.
// This measures the baseline performance without any metrics overhead.
func BenchmarkProcess_MetricsDisabled(b *testing.B) {
	log.SetLevel("error")

	// Ensure metrics are disabled
	if metrics.Enabled() {
		b.Skip("Metrics are enabled, skipping disabled metrics benchmark")
	}

	tr, cleanup := setupBenchmarkResource(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tr.process(); err != nil {
			b.Fatalf("process() failed: %v", err)
		}
	}
}

// BenchmarkProcess_MetricsEnabled benchmarks process() with metrics enabled.
// This measures the overhead of metrics collection.
func BenchmarkProcess_MetricsEnabled(b *testing.B) {
	log.SetLevel("error")

	// Save original registry and ensure it's restored
	origRegistry := metrics.Registry
	defer func() {
		metrics.Registry = origRegistry
	}()

	// Initialize metrics
	metrics.Initialize()

	tr, cleanup := setupBenchmarkResource(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tr.process(); err != nil {
			b.Fatalf("process() failed: %v", err)
		}
	}
}

// TestProcess_MetricsRecordedCorrectly verifies that the conditional defer
// correctly captures the error variable by reference, not by value.
// This addresses the concern that closures might capture err at defer registration time.
func TestProcess_MetricsRecordedCorrectly(t *testing.T) {
	log.SetLevel("error")

	// Save original registry and ensure it's restored
	origRegistry := metrics.Registry
	defer func() {
		metrics.Registry = origRegistry
	}()

	// Initialize metrics
	metrics.Initialize()

	t.Run("success_case", func(t *testing.T) {
		tr, cleanup := setupTestResource(t)
		defer cleanup()

		// Get initial counter value
		successBefore := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "success")
		errorBefore := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "error")

		err := tr.process()
		if err != nil {
			t.Fatalf("process() failed unexpectedly: %v", err)
		}

		// Verify success counter incremented
		successAfter := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "success")
		errorAfter := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "error")

		if successAfter != successBefore+1 {
			t.Errorf("success counter not incremented: before=%v, after=%v", successBefore, successAfter)
		}
		if errorAfter != errorBefore {
			t.Errorf("error counter should not change: before=%v, after=%v", errorBefore, errorAfter)
		}
	})

	t.Run("error_case", func(t *testing.T) {
		tr, cleanup := setupTestResource(t)
		defer cleanup()

		// Make process() fail by using an invalid template directory
		tr.templateDir = "/nonexistent/path"
		tr.Src = "missing.tmpl"

		// Get initial counter value
		successBefore := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "success")
		errorBefore := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "error")

		err := tr.process()
		if err == nil {
			t.Fatal("process() should have failed")
		}

		// Verify error counter incremented (proves closure captures err by reference)
		successAfter := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "success")
		errorAfter := getCounterValue(t, metrics.TemplateProcessTotal, tr.Dest, "error")

		if errorAfter != errorBefore+1 {
			t.Errorf("error counter not incremented: before=%v, after=%v (closure may not capture err by reference)", errorBefore, errorAfter)
		}
		if successAfter != successBefore {
			t.Errorf("success counter should not change: before=%v, after=%v", successBefore, successAfter)
		}
	})
}

// setupTestResource creates a minimal TemplateResource for testing (not benchmarking).
func setupTestResource(t *testing.T) (*TemplateResource, func()) {
	t.Helper()

	confDir, err := os.MkdirTemp("", "confd-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	templatesDir := filepath.Join(confDir, "templates")
	confdDir := filepath.Join(confDir, "conf.d")
	destDir := filepath.Join(confDir, "dest")

	for _, dir := range []string{templatesDir, confdDir, destDir} {
		if err := os.Mkdir(dir, 0755); err != nil {
			os.RemoveAll(confDir)
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	tmplContent := `# Config
key = "{{ getv "/test/key" }}"
`
	tmplPath := filepath.Join(templatesDir, "test.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		os.RemoveAll(confDir)
		t.Fatalf("Failed to write template: %v", err)
	}

	resourceContent := `[template]
src = "test.tmpl"
dest = "` + filepath.Join(destDir, "test.conf") + `"
keys = ["/test/key"]
`
	resourcePath := filepath.Join(confdDir, "test.toml")
	if err := os.WriteFile(resourcePath, []byte(resourceContent), 0644); err != nil {
		os.RemoveAll(confDir)
		t.Fatalf("Failed to write resource config: %v", err)
	}

	os.Setenv("TEST_KEY", "test-value")

	storeClient, err := env.NewEnvClient()
	if err != nil {
		os.RemoveAll(confDir)
		t.Fatalf("Failed to create env client: %v", err)
	}

	config := Config{
		ConfDir:     confDir,
		ConfigDir:   confdDir,
		TemplateDir: templatesDir,
		StoreClient: storeClient,
		Prefix:      "",
	}

	tr, err := NewTemplateResource(resourcePath, config)
	if err != nil {
		os.RemoveAll(confDir)
		t.Fatalf("Failed to create template resource: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(confDir)
		os.Unsetenv("TEST_KEY")
	}

	return tr, cleanup
}

// getCounterValue retrieves the current value of a counter metric.
func getCounterValue(t *testing.T, counter *prometheus.CounterVec, dest, status string) float64 {
	t.Helper()

	metric, err := counter.GetMetricWithLabelValues(dest, status)
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}

	// Use the Write method to get the current value
	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		t.Fatalf("Failed to write metric: %v", err)
	}

	return m.GetCounter().GetValue()
}

// BenchmarkDeferOverhead directly measures the overhead of defer registration.
// This isolates the defer cost from the rest of the process() function.
func BenchmarkDeferOverhead(b *testing.B) {
	// Benchmark with defer (unconditional)
	b.Run("unconditional_defer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			func() {
				defer func() {
					// Empty closure, simulating disabled metrics check
					if false {
						_ = i
					}
				}()
			}()
		}
	})

	// Benchmark without defer (conditional - metrics disabled path)
	b.Run("conditional_no_defer", func(b *testing.B) {
		metricsEnabled := false
		for i := 0; i < b.N; i++ {
			func() {
				if metricsEnabled {
					defer func() {
						_ = i
					}()
				}
			}()
		}
	})

	// Benchmark with defer (conditional - metrics enabled path)
	b.Run("conditional_with_defer", func(b *testing.B) {
		metricsEnabled := true
		for i := 0; i < b.N; i++ {
			func() {
				if metricsEnabled {
					defer func() {
						_ = i
					}()
				}
			}()
		}
	})
}
