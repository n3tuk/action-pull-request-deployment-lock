package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/health"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/metrics"
	internalMiddleware "github.com/n3tuk/action-pull-request-deployment-lock/internal/middleware"
)

// setupAPIRoutes configures the API server routes.
func setupAPIRoutes(r *chi.Mux, logger *zap.Logger) {
	r.Get("/ping", handlePing(logger))
}

// setupProbeRoutes configures the probe server routes with health checks.
func setupProbeRoutes(r *chi.Mux, logger *zap.Logger, healthManager *health.Manager, m *metrics.Metrics) {
	// Wrap each endpoint with health check metrics middleware
	r.With(internalMiddleware.HealthCheckMetricsMiddleware(m, "startup")).
		Get("/healthz/startup", handleStartupProbe(logger, healthManager))

	r.With(internalMiddleware.HealthCheckMetricsMiddleware(m, "liveness")).
		Get("/healthz/live", handleLivenessProbe(logger, healthManager))

	r.With(internalMiddleware.HealthCheckMetricsMiddleware(m, "readiness")).
		Get("/healthz/ready", handleReadinessProbe(logger, healthManager))
}

// handlePing handles the /ping endpoint.
func handlePing(logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{
			"status": "pong",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode ping response", zap.Error(err))
		}
	}
}

// handleStartupProbe handles the startup probe endpoint.
func handleStartupProbe(logger *zap.Logger, healthManager *health.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := healthManager.GetStartupStatus(r.Context())

		w.Header().Set("Content-Type", "application/json")

		// Set status code based on health status
		if response.Status == health.StatusOK {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode startup probe response", zap.Error(err))
		}
	}
}

// handleLivenessProbe handles the liveness probe endpoint.
func handleLivenessProbe(logger *zap.Logger, healthManager *health.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := healthManager.GetLivenessStatus()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode liveness probe response", zap.Error(err))
		}
	}
}

// handleReadinessProbe handles the readiness probe endpoint.
func handleReadinessProbe(logger *zap.Logger, healthManager *health.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := healthManager.GetReadinessStatus(r.Context())

		w.Header().Set("Content-Type", "application/json")

		// Set status code based on ready state
		if response.Ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode readiness probe response", zap.Error(err))
		}
	}
}
