package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// setupAPIRoutes configures the API server routes.
func setupAPIRoutes(r *chi.Mux, logger *zap.Logger) {
	r.Get("/ping", handlePing(logger))
}

// setupProbeRoutes configures the probe server routes.
func setupProbeRoutes(r *chi.Mux, logger *zap.Logger) {
	r.Get("/healthz/startup", handleHealthz(logger, "startup"))
	r.Get("/healthz/live", handleHealthz(logger, "live"))
	r.Get("/healthz/ready", handleHealthz(logger, "ready"))
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

// handleHealthz handles the health check endpoints.
func handleHealthz(logger *zap.Logger, checkType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, all health checks pass
		// In the future, we can add actual health checks here
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok\n")); err != nil {
			logger.Error("Failed to write health check response",
				zap.String("type", checkType),
				zap.Error(err))
		}
	}
}
