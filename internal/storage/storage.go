package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/model"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/store"
)

// Common errors returned by the storage layer.
var (
	// ErrLockNotFound is returned when a lock does not exist.
	ErrLockNotFound = errors.New("lock not found")

	// ErrLockConflict is returned when attempting to acquire a lock held by another branch.
	ErrLockConflict = errors.New("lock conflict: project locked by another branch")
)

// LockManager defines the interface for managing deployment locks.
// It provides atomic operations for acquiring, releasing, and querying locks.
type LockManager interface {
	// AcquireLock attempts to acquire a lock for the given project and branch.
	// If the lock is already held by the same branch/owner, it returns success (idempotent).
	// If the lock is held by a different branch, it returns ErrLockConflict.
	// On success, it returns the current lock state.
	AcquireLock(ctx context.Context, project, branch, owner string) (*model.Lock, error)

	// ReleaseLock releases the lock for the given project.
	// This operation is idempotent - if the lock doesn't exist, it returns success.
	// The branch parameter is provided for future ownership verification.
	ReleaseLock(ctx context.Context, project, branch, owner string) error

	// GetLock retrieves the current lock state for a project.
	// Returns ErrLockNotFound if no lock exists.
	GetLock(ctx context.Context, project string) (*model.Lock, error)
}

// OlricLockManager implements LockManager using Olric distributed store.
type OlricLockManager struct {
	store  store.Store
	logger *zap.Logger
}

// NewOlricLockManager creates a new OlricLockManager.
func NewOlricLockManager(store store.Store, logger *zap.Logger) *OlricLockManager {
	return &OlricLockManager{
		store:  store,
		logger: logger,
	}
}

// AcquireLock attempts to acquire a lock for the given project and branch.
// This operation uses PutIfAbsent which provides better atomicity than the previous
// Get-Check-Put pattern, though due to limitations in Olric v0.7.2, there remains
// a small race window. This is acceptable for deployment locking use cases where
// the cost of a rare race is low (one deployment wins, others retry).
func (m *OlricLockManager) AcquireLock(ctx context.Context, project, branch, owner string) (*model.Lock, error) {
	if project == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if branch == "" {
		return nil, fmt.Errorf("branch name cannot be empty")
	}
	if owner == "" {
		return nil, fmt.Errorf("owner cannot be empty")
	}

	m.logger.Debug("Attempting to acquire lock",
		zap.String("project", project),
		zap.String("branch", branch),
		zap.String("owner", owner),
	)

	// Try to get existing lock first to check for idempotency
	existingLock, err := m.GetLock(ctx, project)
	if err != nil && err != ErrLockNotFound {
		return nil, fmt.Errorf("failed to check existing lock: %w", err)
	}

	// If lock exists, check if same branch and owner - idempotent success
	if existingLock != nil {
		if existingLock.Branch == branch && existingLock.Owner == owner {
			m.logger.Debug("Lock already held by same branch/owner",
				zap.String("project", project),
				zap.String("branch", branch),
			)
			return existingLock, nil
		}

		// Lock held by different branch - conflict
		m.logger.Debug("Lock held by different branch",
			zap.String("project", project),
			zap.String("existing_branch", existingLock.Branch),
			zap.String("requested_branch", branch),
		)
		return existingLock, ErrLockConflict
	}

	// Create new lock
	newLock := &model.Lock{
		Project:   project,
		Branch:    branch,
		CreatedAt: time.Now(),
		Owner:     owner,
	}

	// Serialize lock
	lockData, err := json.Marshal(newLock)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize lock: %w", err)
	}

	// Atomically store the lock if it doesn't exist
	stored, err := m.store.PutIfAbsent(ctx, project, string(lockData), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to store lock: %w", err)
	}

	if !stored {
		// Another process acquired the lock between our check and PutIfAbsent
		// Retrieve the lock that was stored
		existingLock, err := m.GetLock(ctx, project)
		if err != nil {
			return nil, fmt.Errorf("failed to get lock after race: %w", err)
		}

		// Check if it's the same branch/owner (race with ourselves)
		if existingLock.Branch == branch && existingLock.Owner == owner {
			m.logger.Debug("Lock acquired by concurrent request for same branch/owner",
				zap.String("project", project),
				zap.String("branch", branch),
			)
			return existingLock, nil
		}

		// Different branch won the race
		m.logger.Debug("Lock acquired by different branch in race",
			zap.String("project", project),
			zap.String("existing_branch", existingLock.Branch),
			zap.String("requested_branch", branch),
		)
		return existingLock, ErrLockConflict
	}

	m.logger.Info("Lock acquired successfully",
		zap.String("project", project),
		zap.String("branch", branch),
		zap.String("owner", owner),
	)

	return newLock, nil
}

// ReleaseLock releases the lock for the given project.
// This operation is idempotent - if the lock doesn't exist, it returns success.
func (m *OlricLockManager) ReleaseLock(ctx context.Context, project, branch, owner string) error {
	if project == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	m.logger.Debug("Attempting to release lock",
		zap.String("project", project),
		zap.String("branch", branch),
		zap.String("owner", owner),
	)

	// Delete the lock - this is idempotent in the store
	err := m.store.Delete(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to delete lock: %w", err)
	}

	m.logger.Info("Lock released successfully",
		zap.String("project", project),
	)

	return nil
}

// GetLock retrieves the current lock state for a project.
// Returns ErrLockNotFound if no lock exists.
func (m *OlricLockManager) GetLock(ctx context.Context, project string) (*model.Lock, error) {
	if project == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	// Get lock data from store
	value, err := m.store.Get(ctx, project)
	if err != nil {
		// Check if this is a "key not found" error from Olric
		// Use strings.Contains for more robust error matching since Olric
		// error messages may change across versions
		if strings.Contains(err.Error(), "key not found") {
			return nil, ErrLockNotFound
		}
		return nil, fmt.Errorf("failed to get lock: %w", err)
	}

	// Deserialize lock
	lockData, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid lock data type: expected string, got %T", value)
	}

	var lock model.Lock
	if err := json.Unmarshal([]byte(lockData), &lock); err != nil {
		return nil, fmt.Errorf("failed to deserialize lock: %w", err)
	}

	return &lock, nil
}
