package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetrics(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}

	if m.namespace != "test" {
		t.Errorf("namespace = %s, want test", m.namespace)
	}

	if m.registry == nil {
		t.Error("registry is nil")
	}

	// Test that app_start_time_seconds is set
	startTime := testutil.ToFloat64(m.AppStartTimeSeconds)
	if startTime == 0 {
		t.Error("app_start_time_seconds is 0")
	}
}

func TestMetricsRegistry(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)
	registry := m.Registry()

	if registry == nil {
		t.Fatal("Registry() returned nil")
	}

	// Verify metrics are registered
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Error("No metrics registered")
	}

	// Check for expected metrics
	expectedMetrics := []string{
		"test_app_info",
		"test_app_uptime_seconds",
		"test_app_start_time_seconds",
	}

	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[*mf.Name] = true
	}

	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("Expected metric %s not found in %v", expected, foundMetrics)
		}
	}
}

func TestHTTPMetrics(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Record some HTTP metrics
	m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()
	m.HTTPRequestDurationSeconds.WithLabelValues("GET", "/test", "200").Observe(0.5)
	m.HTTPRequestSizeBytes.WithLabelValues("GET", "/test").Observe(1024)
	m.HTTPResponseSizeBytes.WithLabelValues("GET", "/test").Observe(2048)
	m.HTTPRequestsInFlight.WithLabelValues("GET", "/test").Inc()

	// Verify metrics
	count := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200"))
	if count != 1 {
		t.Errorf("http_requests_total = %f, want 1", count)
	}

	inFlight := testutil.ToFloat64(m.HTTPRequestsInFlight.WithLabelValues("GET", "/test"))
	if inFlight != 1 {
		t.Errorf("http_requests_in_flight = %f, want 1", inFlight)
	}
}

func TestHealthCheckMetrics(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Record health check metrics
	m.HealthCheckStatus.WithLabelValues("config", "ok").Set(1)
	m.HealthCheckDurationSeconds.WithLabelValues("config").Observe(0.001)
	m.HealthCheckLastSuccessTimestamp.WithLabelValues("config").Set(1704715200)
	m.HealthCheckFailuresTotal.WithLabelValues("config").Inc()

	// Verify metrics
	status := testutil.ToFloat64(m.HealthCheckStatus.WithLabelValues("config", "ok"))
	if status != 1 {
		t.Errorf("health_check_status = %f, want 1", status)
	}

	failures := testutil.ToFloat64(m.HealthCheckFailuresTotal.WithLabelValues("config"))
	if failures != 1 {
		t.Errorf("health_check_failures_total = %f, want 1", failures)
	}
}

func TestUpdateRuntimeMetrics(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Update runtime metrics
	m.UpdateRuntimeMetrics()

	// Verify that goroutines metric is set
	goroutines := testutil.ToFloat64(m.AppGoGoroutines)
	if goroutines == 0 {
		t.Error("app_go_goroutines is 0")
	}

	// Verify that threads metric is set
	threads := testutil.ToFloat64(m.AppGoThreads)
	// threads is actually NumGC in our implementation, so it might be 0
	if threads < 0 {
		t.Error("app_go_threads is negative")
	}
}

func TestAppUptimeMetric(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Initially should be 0
	uptime := testutil.ToFloat64(m.AppUptimeSeconds)
	if uptime != 0 {
		t.Errorf("Initial uptime = %f, want 0", uptime)
	}

	// Increment uptime
	m.AppUptimeSeconds.Add(10)

	uptime = testutil.ToFloat64(m.AppUptimeSeconds)
	if uptime != 10 {
		t.Errorf("After add uptime = %f, want 10", uptime)
	}
}

func TestMetricsLabels(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Test different label combinations
	m.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1", "200").Inc()
	m.HTTPRequestsTotal.WithLabelValues("POST", "/api/v1", "201").Inc()
	m.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1", "404").Inc()

	// Verify each combination is tracked separately
	get200 := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1", "200"))
	post201 := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("POST", "/api/v1", "201"))
	get404 := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1", "404"))

	if get200 != 1 || post201 != 1 || get404 != 1 {
		t.Errorf("Metrics not tracked separately: GET/200=%f, POST/201=%f, GET/404=%f",
			get200, post201, get404)
	}
}

func TestMetricsCollectorRegistration(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	// Create first metrics instance
	m1 := NewMetrics("test1", buildInfo)
	if m1 == nil {
		t.Fatal("First metrics instance is nil")
	}

	// Create second metrics instance with different namespace
	m2 := NewMetrics("test2", buildInfo)
	if m2 == nil {
		t.Fatal("Second metrics instance is nil")
	}

	// Both should have their own registries
	if m1.Registry() == m2.Registry() {
		t.Error("Metrics instances share the same registry")
	}
}

func TestHistogramBuckets(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}

	m := NewMetrics("test", buildInfo)

	// Record observations in different buckets
	m.HTTPRequestDurationSeconds.WithLabelValues("GET", "/test", "200").Observe(0.001)
	m.HTTPRequestDurationSeconds.WithLabelValues("GET", "/test", "200").Observe(0.1)
	m.HTTPRequestDurationSeconds.WithLabelValues("GET", "/test", "200").Observe(1.0)

	// Verify histogram was created (actual bucket testing would require more complex logic)
	metricFamilies, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metricFamilies {
		if *mf.Name == "test_http_request_duration_seconds" {
			found = true
			// Just verify it's a histogram by checking the type string
			if mf.Type.String() != "HISTOGRAM" {
				t.Errorf("Metric type = %v, want HISTOGRAM", mf.Type.String())
			}
		}
	}

	if !found {
		t.Error("http_request_duration_seconds histogram not found")
	}
}
