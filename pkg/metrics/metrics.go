package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Counter metrics
	TemplateRendersTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "confd_template_renders_total",
			Help: "Total number of template renders",
		},
		[]string{"template", "status"},
	)

	BackendRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "confd_backend_requests_total",
			Help: "Total number of backend API calls",
		},
		[]string{"backend", "operation", "status"},
	)

	ReloadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "confd_reloads_total",
			Help: "Total number of reload command executions",
		},
		[]string{"template", "status"},
	)

	ConfigSyncsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "confd_config_syncs_total",
			Help: "Total number of config file syncs",
		},
		[]string{"template", "status"},
	)

	// Gauge metrics
	WatchedKeys = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "confd_watched_keys",
			Help: "Number of keys being watched",
		},
	)

	TemplatesLoaded = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "confd_templates_loaded",
			Help: "Number of loaded template resources",
		},
	)

	BackendConnected = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "confd_backend_connected",
			Help: "Backend connection status (0=disconnected, 1=connected)",
		},
		[]string{"backend"},
	)

	// Histogram metrics
	TemplateRenderDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "confd_template_render_duration_seconds",
			Help:    "Template rendering latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"template"},
	)

	BackendRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "confd_backend_request_duration_seconds",
			Help:    "Backend request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend", "operation"},
	)

	ReloadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "confd_reload_duration_seconds",
			Help:    "Reload command execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"template"},
	)
)

// RecordTemplateRender records a template render attempt
func RecordTemplateRender(template string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	TemplateRendersTotal.WithLabelValues(template, status).Inc()
	TemplateRenderDuration.WithLabelValues(template).Observe(duration)
}

// RecordBackendRequest records a backend request
func RecordBackendRequest(backend, operation string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	BackendRequestsTotal.WithLabelValues(backend, operation, status).Inc()
	BackendRequestDuration.WithLabelValues(backend, operation).Observe(duration)
}

// RecordReload records a reload command execution
func RecordReload(template string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	ReloadsTotal.WithLabelValues(template, status).Inc()
	ReloadDuration.WithLabelValues(template).Observe(duration)
}

// RecordConfigSync records a config file sync
func RecordConfigSync(template string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	ConfigSyncsTotal.WithLabelValues(template, status).Inc()
}

// SetBackendConnected sets the backend connection status
func SetBackendConnected(backend string, connected bool) {
	value := 0.0
	if connected {
		value = 1.0
	}
	BackendConnected.WithLabelValues(backend).Set(value)
}

// SetTemplatesLoaded sets the number of loaded templates
func SetTemplatesLoaded(count int) {
	TemplatesLoaded.Set(float64(count))
}

// SetWatchedKeys sets the number of watched keys
func SetWatchedKeys(count int) {
	WatchedKeys.Set(float64(count))
}
