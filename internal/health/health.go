package health

import (
	"context"
	"time"
)

// Status represents the health status of a check.
type Status string

const (
	// StatusOK indicates the check passed.
	StatusOK Status = "ok"
	// StatusStarting indicates the service is still starting.
	StatusStarting Status = "starting"
	// StatusNotReady indicates the service is not ready to handle requests.
	StatusNotReady Status = "not-ready"
	// StatusError indicates the check failed.
	StatusError Status = "error"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	// Name is the name of the health check.
	Name string `json:"name"`
	// Status is the status of the health check.
	Status Status `json:"status"`
	// Message provides additional context about the check result.
	Message string `json:"message,omitempty"`
	// Timestamp is when the check was performed.
	Timestamp time.Time `json:"timestamp"`
	// Duration is how long the check took to execute.
	Duration time.Duration `json:"duration"`
}

// Checker is the interface that health checks must implement.
type Checker interface {
	// Name returns the name of the health check.
	Name() string
	// Check performs the health check and returns the result.
	Check(ctx context.Context) CheckResult
}

// StartupResponse represents the response from the startup probe endpoint.
type StartupResponse struct {
	Status    Status            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]Status `json:"checks"`
}

// LivenessResponse represents the response from the liveness probe endpoint.
type LivenessResponse struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadinessResponse represents the response from the readiness probe endpoint.
type ReadinessResponse struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Ready     bool      `json:"ready"`
}
