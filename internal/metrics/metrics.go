package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the service.
type Metrics struct {
	namespace string

	// Application metrics
	AppInfo                *prometheus.GaugeVec
	AppUptimeSeconds       prometheus.Counter
	AppStartTimeSeconds    prometheus.Gauge
	AppGoGoroutines        prometheus.Gauge
	AppGoThreads           prometheus.Gauge
	AppGoGCDurationSeconds prometheus.Summary

	// HTTP metrics
	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDurationSeconds *prometheus.HistogramVec
	HTTPRequestSizeBytes       *prometheus.HistogramVec
	HTTPResponseSizeBytes      *prometheus.HistogramVec
	HTTPRequestsInFlight       *prometheus.GaugeVec

	// Health check metrics
	HealthCheckStatus               *prometheus.GaugeVec
	HealthCheckDurationSeconds      *prometheus.HistogramVec
	HealthCheckLastSuccessTimestamp *prometheus.GaugeVec
	HealthCheckFailuresTotal        *prometheus.CounterVec

	// Lock operation metrics
	LockOperationsTotal *prometheus.CounterVec

	registry *prometheus.Registry
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(namespace string, buildInfo map[string]string) *Metrics {
	m := &Metrics{
		namespace: namespace,
		registry:  prometheus.NewRegistry(),
	}

	// Application metrics
	m.AppInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "app_info",
			Help:      "Application build information",
		},
		[]string{"version", "commit", "build_date", "go_version"},
	)

	m.AppUptimeSeconds = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "app_uptime_seconds",
			Help:      "Application uptime in seconds",
		},
	)

	m.AppStartTimeSeconds = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "app_start_time_seconds",
			Help:      "Unix timestamp of service start",
		},
	)

	m.AppGoGoroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "app_go_goroutines",
			Help:      "Number of goroutines",
		},
	)

	m.AppGoThreads = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "app_go_threads",
			Help:      "Number of OS threads",
		},
	)

	m.AppGoGCDurationSeconds = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "app_go_gc_duration_seconds",
			Help:      "GC pause durations",
		},
	)

	// HTTP metrics
	m.HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.HTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	m.HTTPRequestSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)

	m.HTTPResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)

	m.HTTPRequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Current number of HTTP requests being processed",
		},
		[]string{"method", "path"},
	)

	// Health check metrics
	m.HealthCheckStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "health_check_status",
			Help:      "Health check status (1 for healthy, 0 for unhealthy)",
		},
		[]string{"check_name", "status"},
	)

	m.HealthCheckDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "health_check_duration_seconds",
			Help:      "Health check duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"check_name"},
	)

	m.HealthCheckLastSuccessTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "health_check_last_success_timestamp",
			Help:      "Unix timestamp of last successful health check",
		},
		[]string{"check_name"},
	)

	m.HealthCheckFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "health_check_failures_total",
			Help:      "Total number of health check failures",
		},
		[]string{"check_name"},
	)

	// Lock operation metrics
	m.LockOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "lock_operations_total",
			Help:      "Total number of lock operations by operation type and status",
		},
		[]string{"operation", "status"},
	)

	// Register all metrics
	m.register()

	// Set initial values
	m.AppInfo.WithLabelValues(
		buildInfo["version"],
		buildInfo["commit"],
		buildInfo["date"],
		runtime.Version(),
	).Set(1)

	m.AppStartTimeSeconds.Set(float64(time.Now().Unix()))

	return m
}

// register registers all metrics with the registry.
func (m *Metrics) register() {
	m.registry.MustRegister(
		m.AppInfo,
		m.AppUptimeSeconds,
		m.AppStartTimeSeconds,
		m.AppGoGoroutines,
		m.AppGoThreads,
		m.AppGoGCDurationSeconds,
		m.HTTPRequestsTotal,
		m.HTTPRequestDurationSeconds,
		m.HTTPRequestSizeBytes,
		m.HTTPResponseSizeBytes,
		m.HTTPRequestsInFlight,
		m.HealthCheckStatus,
		m.HealthCheckDurationSeconds,
		m.HealthCheckLastSuccessTimestamp,
		m.HealthCheckFailuresTotal,
		m.LockOperationsTotal,
	)

	// Register Go collectors
	m.registry.MustRegister(prometheus.NewGoCollector())
	m.registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
}

// Registry returns the Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// UpdateRuntimeMetrics updates the runtime metrics (goroutines, threads, GC).
func (m *Metrics) UpdateRuntimeMetrics() {
	m.AppGoGoroutines.Set(float64(runtime.NumGoroutine()))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Track the number of OS threads created by the Go runtime
	// Note: This uses GOMAXPROCS as a proxy since there's no direct API for active thread count
	m.AppGoThreads.Set(float64(runtime.GOMAXPROCS(0)))

	// Get recent GC pause time
	if memStats.NumGC > 0 {
		// Use the most recent GC pause
		gcPause := float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e9
		m.AppGoGCDurationSeconds.Observe(gcPause)
	}
}
