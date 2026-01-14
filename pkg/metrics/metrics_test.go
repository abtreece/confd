package metrics

import (
	"testing"
)

func TestEnabled_BeforeInitialize(t *testing.T) {
	// Reset state
	Registry = nil

	if Enabled() {
		t.Error("Enabled() should return false before Initialize()")
	}
}

func TestEnabled_AfterInitialize(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	if !Enabled() {
		t.Error("Enabled() should return true after Initialize()")
	}

	// Cleanup
	Registry = nil
}

func TestInitialize_RegistersMetrics(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	if Registry == nil {
		t.Fatal("Registry should not be nil after Initialize()")
	}

	// Verify some metrics are registered by trying to gather them
	metricFamilies, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Should have at least the Go collector metrics plus our custom metrics
	if len(metricFamilies) == 0 {
		t.Error("Expected some metric families to be registered")
	}

	// Verify our custom namespace prefix
	hasConfdMetric := false
	for _, mf := range metricFamilies {
		name := mf.GetName()
		if len(name) >= 5 && name[:5] == "confd" {
			hasConfdMetric = true
			break
		}
	}
	if !hasConfdMetric {
		t.Error("Expected at least one metric with 'confd' prefix")
	}

	// Cleanup
	Registry = nil
}

func TestBackendMetrics_Initialized(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	// Verify backend metrics are not nil
	if BackendRequestDuration == nil {
		t.Error("BackendRequestDuration should not be nil")
	}
	if BackendRequestTotal == nil {
		t.Error("BackendRequestTotal should not be nil")
	}
	if BackendErrorsTotal == nil {
		t.Error("BackendErrorsTotal should not be nil")
	}
	if BackendHealthy == nil {
		t.Error("BackendHealthy should not be nil")
	}

	// Cleanup
	Registry = nil
}

func TestTemplateMetrics_Initialized(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	// Verify template metrics are not nil
	if TemplateProcessDuration == nil {
		t.Error("TemplateProcessDuration should not be nil")
	}
	if TemplateProcessTotal == nil {
		t.Error("TemplateProcessTotal should not be nil")
	}
	if TemplateCacheHits == nil {
		t.Error("TemplateCacheHits should not be nil")
	}
	if TemplateCacheMisses == nil {
		t.Error("TemplateCacheMisses should not be nil")
	}

	// Cleanup
	Registry = nil
}

func TestCommandMetrics_Initialized(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	// Verify command metrics are not nil
	if CommandDuration == nil {
		t.Error("CommandDuration should not be nil")
	}
	if CommandTotal == nil {
		t.Error("CommandTotal should not be nil")
	}
	if CommandExitCodes == nil {
		t.Error("CommandExitCodes should not be nil")
	}

	// Cleanup
	Registry = nil
}

func TestFileMetrics_Initialized(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	// Verify file metrics are not nil
	if FileSyncTotal == nil {
		t.Error("FileSyncTotal should not be nil")
	}
	if FileChangedTotal == nil {
		t.Error("FileChangedTotal should not be nil")
	}

	// Cleanup
	Registry = nil
}

func TestMetrics_CanBeRecorded(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()

	// Test that we can record metrics without panicking
	BackendRequestDuration.WithLabelValues("vault", "get_values").Observe(0.5)
	BackendRequestTotal.WithLabelValues("vault", "get_values").Inc()
	BackendErrorsTotal.WithLabelValues("vault", "get_values").Inc()
	BackendHealthy.WithLabelValues("vault").Set(1)

	TemplateProcessDuration.WithLabelValues("/etc/nginx/nginx.conf").Observe(0.1)
	TemplateProcessTotal.WithLabelValues("/etc/nginx/nginx.conf", "success").Inc()
	TemplateCacheHits.Inc()
	TemplateCacheMisses.Inc()

	CommandDuration.WithLabelValues("check", "/etc/nginx/nginx.conf").Observe(0.05)
	CommandTotal.WithLabelValues("check", "/etc/nginx/nginx.conf").Inc()
	CommandExitCodes.WithLabelValues("check", "0").Inc()

	FileSyncTotal.WithLabelValues("/etc/nginx/nginx.conf").Inc()
	FileChangedTotal.Inc()

	// Verify we can gather metrics
	metricFamilies, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics after recording: %v", err)
	}

	// Find and verify some of our recorded metrics
	found := make(map[string]bool)
	for _, mf := range metricFamilies {
		found[mf.GetName()] = true
	}

	expectedMetrics := []string{
		"confd_backend_request_duration_seconds",
		"confd_backend_request_total",
		"confd_template_process_total",
		"confd_command_total",
	}

	for _, metric := range expectedMetrics {
		if !found[metric] {
			t.Errorf("Expected metric %q to be present in gathered metrics", metric)
		}
	}

	// Cleanup
	Registry = nil
}

func TestMetrics_DoubleInitializeCreatesNewRegistry(t *testing.T) {
	// Reset state
	Registry = nil

	Initialize()
	firstRegistry := Registry

	// Record a metric
	BackendRequestTotal.WithLabelValues("etcd", "get_values").Inc()

	// Initialize again (simulates restart or re-init)
	Initialize()
	secondRegistry := Registry

	// The registry should be a new instance
	if firstRegistry == secondRegistry {
		t.Error("Second Initialize() should create a new Registry")
	}

	// The new registry should still be valid
	if !Enabled() {
		t.Error("Enabled() should return true after second Initialize()")
	}

	// Cleanup
	Registry = nil
}

