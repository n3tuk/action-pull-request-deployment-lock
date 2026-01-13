package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// This operation is atomic using Olric's Get-Check-Put pattern.
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

	// Try to get existing lock
	existingLock, err := m.GetLock(ctx, project)
	if err != nil && err != ErrLockNotFound {
		return nil, fmt.Errorf("failed to check existing lock: %w", err)
	}

	// If lock exists
	if existingLock != nil {
		// Check if same branch and owner - idempotent success
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

	// Store the lock with no TTL (locks don't expire automatically)
	err = m.store.Put(ctx, project, string(lockData), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to store lock: %w", err)
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
		if err.Error() == "key not found" {
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
