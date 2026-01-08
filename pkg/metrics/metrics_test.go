package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordTemplateRender(t *testing.T) {
	// Reset metrics
	TemplateRendersTotal.Reset()
	TemplateRenderDuration.Reset()

	template := "/etc/nginx/nginx.conf"
	
	// Record success
	RecordTemplateRender(template, true, 0.5)
	
	// Check counter
	count := testutil.ToFloat64(TemplateRendersTotal.WithLabelValues(template, "success"))
	if count != 1.0 {
		t.Errorf("Expected counter to be 1.0, got %f", count)
	}
	
	// Record failure
	RecordTemplateRender(template, false, 0.1)
	
	// Check counter
	countError := testutil.ToFloat64(TemplateRendersTotal.WithLabelValues(template, "error"))
	if countError != 1.0 {
		t.Errorf("Expected error counter to be 1.0, got %f", countError)
	}
}

func TestRecordBackendRequest(t *testing.T) {
	// Reset metrics
	BackendRequestsTotal.Reset()
	BackendRequestDuration.Reset()

	backend := "etcd"
	operation := "GetValues"
	
	// Record success
	RecordBackendRequest(backend, operation, true, 0.05)
	
	// Check counter
	count := testutil.ToFloat64(BackendRequestsTotal.WithLabelValues(backend, operation, "success"))
	if count != 1.0 {
		t.Errorf("Expected counter to be 1.0, got %f", count)
	}
}

func TestRecordReload(t *testing.T) {
	// Reset metrics
	ReloadsTotal.Reset()
	ReloadDuration.Reset()

	template := "/etc/nginx/nginx.conf"
	
	// Record success
	RecordReload(template, true, 1.2)
	
	// Check counter
	count := testutil.ToFloat64(ReloadsTotal.WithLabelValues(template, "success"))
	if count != 1.0 {
		t.Errorf("Expected counter to be 1.0, got %f", count)
	}
}

func TestRecordConfigSync(t *testing.T) {
	// Reset metrics
	ConfigSyncsTotal.Reset()

	template := "/etc/nginx/nginx.conf"
	
	// Record success
	RecordConfigSync(template, true)
	
	// Check counter
	count := testutil.ToFloat64(ConfigSyncsTotal.WithLabelValues(template, "success"))
	if count != 1.0 {
		t.Errorf("Expected counter to be 1.0, got %f", count)
	}
}

func TestSetBackendConnected(t *testing.T) {
	backend := "consul"
	
	// Set connected
	SetBackendConnected(backend, true)
	
	// Check gauge
	value := testutil.ToFloat64(BackendConnected.WithLabelValues(backend))
	if value != 1.0 {
		t.Errorf("Expected gauge to be 1.0, got %f", value)
	}
	
	// Set disconnected
	SetBackendConnected(backend, false)
	
	// Check gauge
	value = testutil.ToFloat64(BackendConnected.WithLabelValues(backend))
	if value != 0.0 {
		t.Errorf("Expected gauge to be 0.0, got %f", value)
	}
}

func TestSetTemplatesLoaded(t *testing.T) {
	count := 5
	SetTemplatesLoaded(count)
	
	value := testutil.ToFloat64(TemplatesLoaded)
	if value != float64(count) {
		t.Errorf("Expected gauge to be %f, got %f", float64(count), value)
	}
}

func TestSetWatchedKeys(t *testing.T) {
	count := 10
	SetWatchedKeys(count)
	
	value := testutil.ToFloat64(WatchedKeys)
	if value != float64(count) {
		t.Errorf("Expected gauge to be %f, got %f", float64(count), value)
	}
}

func TestMetricsAreRegistered(t *testing.T) {
	// Check that metrics are registered with the default registry
	metrics := []prometheus.Collector{
		TemplateRendersTotal,
		BackendRequestsTotal,
		ReloadsTotal,
		ConfigSyncsTotal,
		WatchedKeys,
		TemplatesLoaded,
		BackendConnected,
		TemplateRenderDuration,
		BackendRequestDuration,
		ReloadDuration,
	}
	
	for _, metric := range metrics {
		if metric == nil {
			t.Error("Found nil metric")
		}
	}
}
