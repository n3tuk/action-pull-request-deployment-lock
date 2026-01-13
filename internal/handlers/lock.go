package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/metrics"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/model"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/storage"
)

// LockHandlers provides HTTP handlers for lock operations.
type LockHandlers struct {
	lockManager storage.LockManager
	logger      *zap.Logger
	metrics     *metrics.Metrics
}

// NewLockHandlers creates a new LockHandlers instance.
func NewLockHandlers(lockManager storage.LockManager, logger *zap.Logger, metrics *metrics.Metrics) *LockHandlers {
	return &LockHandlers{
		lockManager: lockManager,
		logger:      logger,
		metrics:     metrics,
	}
}

// HandleLock handles POST /lock requests to acquire a deployment lock.
// Returns:
//   - 200 OK: Lock acquired successfully or already held by same branch
//   - 400 Bad Request: Invalid request body or validation error
//   - 409 Conflict: Lock held by another branch
//   - 500 Internal Server Error: Storage or internal error
func (h *LockHandlers) HandleLock(w http.ResponseWriter, r *http.Request) {
	var req model.LockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode lock request", zap.Error(err))
		h.recordMetric("lock", "failure")
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Project == "" {
		h.recordMetric("lock", "failure")
		h.respondError(w, http.StatusBadRequest, "Project name is required")
		return
	}
	if req.Branch == "" {
		h.recordMetric("lock", "failure")
		h.respondError(w, http.StatusBadRequest, "Branch name is required")
		return
	}

	// For now, use placeholder owner until OIDC is implemented
	owner := "placeholder-owner"

	// Attempt to acquire lock
	lock, err := h.lockManager.AcquireLock(r.Context(), req.Project, req.Branch, owner)
	if err != nil {
		if errors.Is(err, storage.ErrLockConflict) {
			h.logger.Debug("Lock conflict",
				zap.String("project", req.Project),
				zap.String("branch", req.Branch),
			)
			h.recordMetric("lock", "conflict")
			h.respondLock(w, http.StatusConflict, "locked", "Lock held by another branch", lock)
			return
		}

		h.logger.Error("Failed to acquire lock", zap.Error(err))
		h.recordMetric("lock", "failure")
		h.respondError(w, http.StatusInternalServerError, "Failed to acquire lock")
		return
	}

	h.recordMetric("lock", "success")
	h.respondLock(w, http.StatusOK, "locked", "Lock acquired successfully", lock)
}

// HandleUnlock handles POST /unlock requests to release a deployment lock.
// Returns:
//   - 200 OK: Lock released successfully or was already unlocked
//   - 400 Bad Request: Invalid request body or validation error
//   - 500 Internal Server Error: Storage or internal error
func (h *LockHandlers) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	var req model.UnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode unlock request", zap.Error(err))
		h.recordMetric("unlock", "failure")
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Project == "" {
		h.recordMetric("unlock", "failure")
		h.respondError(w, http.StatusBadRequest, "Project name is required")
		return
	}

	// For now, use placeholder owner until OIDC is implemented
	owner := "placeholder-owner"

	// Attempt to release lock
	err := h.lockManager.ReleaseLock(r.Context(), req.Project, req.Branch, owner)
	if err != nil {
		h.logger.Error("Failed to release lock", zap.Error(err))
		h.recordMetric("unlock", "failure")
		h.respondError(w, http.StatusInternalServerError, "Failed to release lock")
		return
	}

	h.recordMetric("unlock", "success")
	h.respondJSON(w, http.StatusOK, model.LockResponse{
		Status:  "unlocked",
		Message: "Lock released successfully",
	})
}

// HandleGetLock handles GET /lock/{project} requests to get lock status.
// Returns:
//   - 200 OK: Lock exists and status returned
//   - 404 Not Found: No lock exists for project
//   - 400 Bad Request: Invalid project parameter
//   - 500 Internal Server Error: Storage or internal error
func (h *LockHandlers) HandleGetLock(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	if project == "" {
		h.respondError(w, http.StatusBadRequest, "Project name is required")
		return
	}

	// Get lock status
	lock, err := h.lockManager.GetLock(r.Context(), project)
	if err != nil {
		if errors.Is(err, storage.ErrLockNotFound) {
			h.respondJSON(w, http.StatusNotFound, model.LockResponse{
				Status:  "unlocked",
				Message: "No lock exists for this project",
			})
			return
		}

		h.logger.Error("Failed to get lock", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get lock status")
		return
	}

	h.respondLock(w, http.StatusOK, "locked", "Lock exists", lock)
}

// respondError sends an error response.
func (h *LockHandlers) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, model.LockResponse{
		Status:  "error",
		Message: message,
	})
}

// respondLock sends a lock response.
func (h *LockHandlers) respondLock(w http.ResponseWriter, status int, statusStr, message string, lock *model.Lock) {
	h.respondJSON(w, status, model.LockResponse{
		Status:  statusStr,
		Message: message,
		Lock:    lock,
	})
}

// respondJSON sends a JSON response.
func (h *LockHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// recordMetric records a lock operation metric.
func (h *LockHandlers) recordMetric(operation, status string) {
	if h.metrics != nil && h.metrics.LockOperationsTotal != nil {
		h.metrics.LockOperationsTotal.WithLabelValues(operation, status).Inc()
	}
}
