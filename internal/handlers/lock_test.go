package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/logger"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/metrics"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/model"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/storage"
)

// mockLockManager implements storage.LockManager for testing.
type mockLockManager struct {
	acquireFunc func(ctx context.Context, project, branch, owner string) (*model.Lock, error)
	releaseFunc func(ctx context.Context, project, branch, owner string) error
	getFunc     func(ctx context.Context, project string) (*model.Lock, error)
}

func (m *mockLockManager) AcquireLock(ctx context.Context, project, branch, owner string) (*model.Lock, error) {
	if m.acquireFunc != nil {
		return m.acquireFunc(ctx, project, branch, owner)
	}
	return nil, errors.New("not implemented")
}

func (m *mockLockManager) ReleaseLock(ctx context.Context, project, branch, owner string) error {
	if m.releaseFunc != nil {
		return m.releaseFunc(ctx, project, branch, owner)
	}
	return errors.New("not implemented")
}

func (m *mockLockManager) GetLock(ctx context.Context, project string) (*model.Lock, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, project)
	}
	return nil, errors.New("not implemented")
}

func testLogger() *zap.Logger {
	log, _ := logger.New("error", "json")
	return log
}

func testMetrics() *metrics.Metrics {
	return metrics.NewMetrics("test", map[string]string{
		"version": "test",
		"commit":  "test",
		"date":    "test",
	})
}

func TestHandleLock(t *testing.T) {
	t.Run("successful lock acquisition", func(t *testing.T) {
		mockLock := &model.Lock{
			Project:   "test-project",
			Branch:    "main",
			CreatedAt: time.Now(),
			Owner:     "placeholder-owner",
		}

		mock := &mockLockManager{
			acquireFunc: func(ctx context.Context, project, branch, owner string) (*model.Lock, error) {
				return mockLock, nil
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.LockRequest{
			Project: "test-project",
			Branch:  "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleLock(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var resp model.LockResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "locked" {
			t.Errorf("Expected status 'locked', got '%s'", resp.Status)
		}
		if resp.Lock == nil {
			t.Error("Expected lock in response")
		}
	})

	t.Run("lock conflict", func(t *testing.T) {
		existingLock := &model.Lock{
			Project:   "test-project",
			Branch:    "other-branch",
			CreatedAt: time.Now(),
			Owner:     "other-owner",
		}

		mock := &mockLockManager{
			acquireFunc: func(ctx context.Context, project, branch, owner string) (*model.Lock, error) {
				return existingLock, storage.ErrLockConflict
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.LockRequest{
			Project: "test-project",
			Branch:  "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleLock(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("Expected status 409, got %d", rec.Code)
		}

		var resp model.LockResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "locked" {
			t.Errorf("Expected status 'locked', got '%s'", resp.Status)
		}
		if resp.Lock == nil {
			t.Error("Expected existing lock in response")
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		mock := &mockLockManager{}
		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader([]byte("invalid json")))
		rec := httptest.NewRecorder()

		handlers.HandleLock(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		mock := &mockLockManager{}
		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.LockRequest{
			Branch: "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleLock(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		mock := &mockLockManager{
			acquireFunc: func(ctx context.Context, project, branch, owner string) (*model.Lock, error) {
				return nil, errors.New("storage failure")
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.LockRequest{
			Project: "test-project",
			Branch:  "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/lock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleLock(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

func TestHandleUnlock(t *testing.T) {
	t.Run("successful unlock", func(t *testing.T) {
		mock := &mockLockManager{
			releaseFunc: func(ctx context.Context, project, branch, owner string) error {
				return nil
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.UnlockRequest{
			Project: "test-project",
			Branch:  "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/unlock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleUnlock(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var resp model.LockResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "unlocked" {
			t.Errorf("Expected status 'unlocked', got '%s'", resp.Status)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		mock := &mockLockManager{}
		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		req := httptest.NewRequest(http.MethodPost, "/unlock", bytes.NewReader([]byte("invalid json")))
		rec := httptest.NewRecorder()

		handlers.HandleUnlock(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		mock := &mockLockManager{}
		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.UnlockRequest{
			Branch: "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/unlock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleUnlock(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		mock := &mockLockManager{
			releaseFunc: func(ctx context.Context, project, branch, owner string) error {
				return errors.New("storage failure")
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		reqBody := model.UnlockRequest{
			Project: "test-project",
			Branch:  "main",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/unlock", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handlers.HandleUnlock(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

func TestHandleGetLock(t *testing.T) {
	t.Run("lock exists", func(t *testing.T) {
		mockLock := &model.Lock{
			Project:   "test-project",
			Branch:    "main",
			CreatedAt: time.Now(),
			Owner:     "test-owner",
		}

		mock := &mockLockManager{
			getFunc: func(ctx context.Context, project string) (*model.Lock, error) {
				return mockLock, nil
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		req := httptest.NewRequest(http.MethodGet, "/lock/test-project", nil)
		rec := httptest.NewRecorder()

		// Set up chi context for URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("project", "test-project")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handlers.HandleGetLock(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var resp model.LockResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "locked" {
			t.Errorf("Expected status 'locked', got '%s'", resp.Status)
		}
		if resp.Lock == nil {
			t.Error("Expected lock in response")
		}
	})

	t.Run("lock not found", func(t *testing.T) {
		mock := &mockLockManager{
			getFunc: func(ctx context.Context, project string) (*model.Lock, error) {
				return nil, storage.ErrLockNotFound
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		req := httptest.NewRequest(http.MethodGet, "/lock/test-project", nil)
		rec := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("project", "test-project")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handlers.HandleGetLock(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rec.Code)
		}

		var resp model.LockResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Status != "unlocked" {
			t.Errorf("Expected status 'unlocked', got '%s'", resp.Status)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		mock := &mockLockManager{
			getFunc: func(ctx context.Context, project string) (*model.Lock, error) {
				return nil, errors.New("storage failure")
			},
		}

		handlers := NewLockHandlers(mock, testLogger(), testMetrics())

		req := httptest.NewRequest(http.MethodGet, "/lock/test-project", nil)
		rec := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("project", "test-project")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handlers.HandleGetLock(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}
