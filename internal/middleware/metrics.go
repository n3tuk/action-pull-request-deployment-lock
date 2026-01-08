package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

// newResponseWriter creates a new response writer wrapper.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// MetricsMiddleware creates a middleware that records HTTP metrics.
func MetricsMiddleware(m *metrics.Metrics, logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get the route pattern (not the raw path)
			routePattern := getRoutePattern(r)

			// Track in-flight requests
			m.HTTPRequestsInFlight.WithLabelValues(r.Method, routePattern).Inc()
			defer m.HTTPRequestsInFlight.WithLabelValues(r.Method, routePattern).Dec()

			// Record request size
			if r.ContentLength > 0 {
				m.HTTPRequestSizeBytes.WithLabelValues(r.Method, routePattern).Observe(float64(r.ContentLength))
			}

			// Wrap response writer to capture status and size
			rw := newResponseWriter(w)

			// Handle panics
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic in HTTP handler",
						zap.String("method", r.Method),
						zap.String("path", routePattern),
						zap.Any("error", err),
					)
					
					// Record metrics for panic
					rw.statusCode = http.StatusInternalServerError
					duration := time.Since(start).Seconds()
					status := strconv.Itoa(rw.statusCode)
					
					m.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
					m.HTTPRequestDurationSeconds.WithLabelValues(r.Method, routePattern, status).Observe(duration)
					
					// Re-panic to let the recovery middleware handle it
					panic(err)
				}
			}()

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rw.statusCode)

			m.HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
			m.HTTPRequestDurationSeconds.WithLabelValues(r.Method, routePattern, status).Observe(duration)
			
			if rw.bytesWritten > 0 {
				m.HTTPResponseSizeBytes.WithLabelValues(r.Method, routePattern).Observe(float64(rw.bytesWritten))
			}
		})
	}
}

// getRoutePattern extracts the route pattern from the request context.
// Falls back to the raw path if no pattern is found.
func getRoutePattern(r *http.Request) string {
	// Try to get the route pattern from Chi router context
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}

	// Try to get from middleware
	if pattern := middleware.GetReqID(r.Context()); pattern != "" {
		// This won't work, but it's a fallback attempt
	}

	// Fall back to the path
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return path
}

// LoggingMiddleware creates a middleware that logs HTTP requests.
func LoggingMiddleware(logger *zap.Logger, serverName string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get the route pattern
			routePattern := getRoutePattern(r)

			// Wrap response writer to capture status
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			
			// Call the next handler
			next.ServeHTTP(ww, r)

			// Log the request
			logger.Info("HTTP request",
				zap.String("server", serverName),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("route", routePattern),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
				zap.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

// HealthCheckMetricsMiddleware creates a middleware that records health check metrics.
func HealthCheckMetricsMiddleware(m *metrics.Metrics, checkName string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			rw := newResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			m.HealthCheckDurationSeconds.WithLabelValues(checkName).Observe(duration)

			// Record status
			if rw.statusCode == http.StatusOK {
				m.HealthCheckStatus.WithLabelValues(checkName, "ok").Set(1)
				m.HealthCheckStatus.WithLabelValues(checkName, "error").Set(0)
				m.HealthCheckLastSuccessTimestamp.WithLabelValues(checkName).Set(float64(time.Now().Unix()))
			} else {
				m.HealthCheckStatus.WithLabelValues(checkName, "ok").Set(0)
				m.HealthCheckStatus.WithLabelValues(checkName, "error").Set(1)
				m.HealthCheckFailuresTotal.WithLabelValues(checkName).Inc()
			}
		})
	}
}

// RecovererMiddleware creates a middleware that recovers from panics.
func RecovererMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("error", err),
					)

					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(fmt.Sprintf("Internal Server Error: %v", err)))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
