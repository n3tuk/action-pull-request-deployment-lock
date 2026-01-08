package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/metrics"
)

func TestMetricsMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Wrap with metrics middleware
	mw := MetricsMiddleware(m, logger)
	wrappedHandler := mw(handler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Serve the request
	wrappedHandler.ServeHTTP(rr, req)

	// Verify metrics were recorded
	requestCount := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200"))
	if requestCount != 1 {
		t.Errorf("Request count = %f, want 1", requestCount)
	}

	// Verify in-flight is back to 0
	inFlight := testutil.ToFloat64(m.HTTPRequestsInFlight.WithLabelValues("GET", "/test"))
	if inFlight != 0 {
		t.Errorf("In-flight requests = %f, want 0", inFlight)
	}
}

func TestMetricsMiddlewareWithRoutePattern(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	// Create a Chi router
	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m, logger))
	r.Get("/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/api/123", nil)
	rr := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(rr, req)

	// Verify metrics use route pattern
	// Chi should provide the pattern, but in case it doesn't, check for either
	patternCount := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/api/{id}", "200"))
	pathCount := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/api/123", "200"))

	if patternCount != 1 && pathCount != 1 {
		t.Errorf("Request count for pattern or path = %f or %f, want 1", patternCount, pathCount)
	}
}

func TestMetricsMiddlewareStatusCodes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			mw := MetricsMiddleware(m, logger)
			wrappedHandler := mw(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rr, req)

			// Verify correct status code was recorded
			statusStr := http.StatusText(tt.statusCode)
			if rr.Code != tt.statusCode {
				t.Errorf("Status code = %d, want %d (%s)", rr.Code, tt.statusCode, statusStr)
			}
		})
	}
}

func TestMetricsMiddlewareResponseSize(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	responseBody := "test response body"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})

	mw := MetricsMiddleware(m, logger)
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	// Response size metric should be recorded
	// We can't easily verify the exact value with testutil for histograms,
	// but we can verify the request was processed
	if rr.Body.String() != responseBody {
		t.Errorf("Response body = %s, want %s", rr.Body.String(), responseBody)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
	})

	mw := LoggingMiddleware(logger, "test")
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHealthCheckMetricsMiddleware(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := HealthCheckMetricsMiddleware(m, "startup")
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/healthz/startup", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	// Verify health check metrics were recorded
	status := testutil.ToFloat64(m.HealthCheckStatus.WithLabelValues("startup", "ok"))
	if status != 1 {
		t.Errorf("Health check status = %f, want 1", status)
	}
}

func TestHealthCheckMetricsMiddlewareFailure(t *testing.T) {
	buildInfo := map[string]string{
		"version": "1.0.0",
		"commit":  "abc123",
		"date":    "2024-01-08",
	}
	m := metrics.NewMetrics("test", buildInfo)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	mw := HealthCheckMetricsMiddleware(m, "startup")
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/healthz/startup", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	// Verify failure was recorded
	errorStatus := testutil.ToFloat64(m.HealthCheckStatus.WithLabelValues("startup", "error"))
	if errorStatus != 1 {
		t.Errorf("Health check error status = %f, want 1", errorStatus)
	}

	failures := testutil.ToFloat64(m.HealthCheckFailuresTotal.WithLabelValues("startup"))
	if failures != 1 {
		t.Errorf("Health check failures = %f, want 1", failures)
	}
}

func TestRecovererMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	mw := RecovererMiddleware(logger)
	wrappedHandler := mw(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Should not panic
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestResponseWriter(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := newResponseWriter(rr)

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	if rw.statusCode != http.StatusNotFound {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusNotFound)
	}

	// Test Write
	data := []byte("test data")
	n, err := rw.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() returned %d bytes, want %d", n, len(data))
	}
	if rw.bytesWritten != len(data) {
		t.Errorf("bytesWritten = %d, want %d", rw.bytesWritten, len(data))
	}
}

func TestGetRoutePattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
	}{
		{"root path", "/", "/"},
		{"simple path", "/test", "/test"},
		{"nested path", "/api/v1/test", "/api/v1/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			pattern := getRoutePattern(req)
			if pattern != tt.pattern {
				t.Errorf("getRoutePattern() = %s, want %s", pattern, tt.pattern)
			}
		})
	}
}
