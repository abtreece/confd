// Package metrics provides Prometheus metrics instrumentation for confd.
// Metrics are optional and only initialized when --metrics-addr is specified.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const namespace = "confd"

// Registry holds all confd metrics. It is nil when metrics are disabled.
var Registry *prometheus.Registry

// Backend metrics
var (
	BackendRequestDuration *prometheus.HistogramVec
	BackendRequestTotal    *prometheus.CounterVec
	BackendErrorsTotal     *prometheus.CounterVec
	BackendHealthy         *prometheus.GaugeVec
)

// Template metrics
var (
	TemplateProcessDuration *prometheus.HistogramVec
	TemplateProcessTotal    *prometheus.CounterVec
	TemplateCacheHits       prometheus.Counter
	TemplateCacheMisses     prometheus.Counter
)

// Command metrics
var (
	CommandDuration  *prometheus.HistogramVec
	CommandTotal     *prometheus.CounterVec
	CommandExitCodes *prometheus.CounterVec
)

// File sync metrics
var (
	FileSyncTotal    *prometheus.CounterVec
	FileChangedTotal prometheus.Counter
)

// Initialize creates and registers all metrics with a new registry.
// Call this only when metrics are enabled (--metrics-addr is set).
func Initialize() {
	Registry = prometheus.NewRegistry()

	// Register Go runtime and process collectors
	Registry.MustRegister(collectors.NewGoCollector())
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Backend metrics
	BackendRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "backend_request_duration_seconds",
			Help:      "Duration of backend requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"backend", "operation"},
	)
	Registry.MustRegister(BackendRequestDuration)

	BackendRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backend_request_total",
			Help:      "Total number of backend requests.",
		},
		[]string{"backend", "operation"},
	)
	Registry.MustRegister(BackendRequestTotal)

	BackendErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backend_errors_total",
			Help:      "Total number of backend errors.",
		},
		[]string{"backend", "operation"},
	)
	Registry.MustRegister(BackendErrorsTotal)

	BackendHealthy = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "backend_healthy",
			Help:      "Whether the backend is healthy (1) or not (0).",
		},
		[]string{"backend"},
	)
	Registry.MustRegister(BackendHealthy)

	// Template metrics
	TemplateProcessDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "template_process_duration_seconds",
			Help:      "Duration of template processing in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"dest"},
	)
	Registry.MustRegister(TemplateProcessDuration)

	TemplateProcessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "template_process_total",
			Help:      "Total number of template processing operations.",
		},
		[]string{"dest", "status"},
	)
	Registry.MustRegister(TemplateProcessTotal)

	TemplateCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "template_cache_hits_total",
			Help:      "Total number of template cache hits.",
		},
	)
	Registry.MustRegister(TemplateCacheHits)

	TemplateCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "template_cache_misses_total",
			Help:      "Total number of template cache misses.",
		},
	)
	Registry.MustRegister(TemplateCacheMisses)

	// Command metrics
	CommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "command_duration_seconds",
			Help:      "Duration of command execution in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"type", "dest"},
	)
	Registry.MustRegister(CommandDuration)

	CommandTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "command_total",
			Help:      "Total number of command executions.",
		},
		[]string{"type", "dest"},
	)
	Registry.MustRegister(CommandTotal)

	CommandExitCodes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "command_exit_code_total",
			Help:      "Total number of commands by exit code.",
		},
		[]string{"type", "exit_code"},
	)
	Registry.MustRegister(CommandExitCodes)

	// File sync metrics
	FileSyncTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "file_sync_total",
			Help:      "Total number of file sync operations.",
		},
		[]string{"dest"},
	)
	Registry.MustRegister(FileSyncTotal)

	FileChangedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "file_changed_total",
			Help:      "Total number of files that were changed.",
		},
	)
	Registry.MustRegister(FileChangedTotal)
}

// Enabled returns true if metrics are enabled.
func Enabled() bool {
	return Registry != nil
}
