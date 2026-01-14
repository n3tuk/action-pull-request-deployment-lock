package model

import (
	"time"
)

// Lock represents a deployment lock held by a branch on a project.
// It tracks which branch currently holds the lock and when it was created.
type Lock struct {
	// Project is the name of the project being locked.
	Project string `json:"project"`

	// Branch is the name of the branch that holds the lock.
	Branch string `json:"branch"`

	// CreatedAt is the timestamp when the lock was created.
	CreatedAt time.Time `json:"created_at"`

	// Owner is the identity of the entity that created the lock.
	// This is typically an OIDC subject or user ID.
	Owner string `json:"owner"`
}

// LockRequest represents a request to acquire a lock.
type LockRequest struct {
	// Project is the name of the project to lock.
	Project string `json:"project"`

	// Branch is the name of the branch requesting the lock.
	Branch string `json:"branch"`
}

// UnlockRequest represents a request to release a lock.
type UnlockRequest struct {
	// Project is the name of the project to unlock.
	Project string `json:"project"`

	// Branch is the name of the branch releasing the lock.
	Branch string `json:"branch"`
}

// LockResponse represents the response from lock/unlock operations.
type LockResponse struct {
	// Status indicates the overall status of the operation.
	// For successful operations this is:
	//   - "locked"   when a lock is currently held
	//   - "unlocked" when no active lock exists
	// For error responses produced by helper functions, this is:
	//   - "error"
	Status string `json:"status"`

	// Message provides additional context about the operation result.
	Message string `json:"message,omitempty"`

	// Lock contains the current lock state if applicable.
	Lock *Lock `json:"lock,omitempty"`
}
