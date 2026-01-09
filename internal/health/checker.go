package health

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServerChecker checks if the servers are running.
type ServerChecker struct {
	logger         *zap.Logger
	mu             sync.RWMutex
	serversRunning bool
}

// NewServerChecker creates a new server health checker.
func NewServerChecker(logger *zap.Logger) *ServerChecker {
	return &ServerChecker{
		logger:         logger,
		serversRunning: false,
	}
}

// Name returns the name of the health check.
func (s *ServerChecker) Name() string {
	return "servers"
}

// SetRunning marks the servers as running.
func (s *ServerChecker) SetRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serversRunning = running
}

// Check performs the health check.
func (s *ServerChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()

	s.mu.RLock()
	running := s.serversRunning
	s.mu.RUnlock()

	result := CheckResult{
		Name:      s.Name(),
		Status:    StatusOK,
		Message:   "All servers running",
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	if !running {
		result.Status = StatusStarting
		result.Message = "Servers starting"
	}

	return result
}

// ReadinessChecker checks if the service is ready to handle requests.
type ReadinessChecker struct {
	logger         *zap.Logger
	mu             sync.RWMutex
	shuttingDown   bool
	serversRunning bool
}

// NewReadinessChecker creates a new readiness health checker.
func NewReadinessChecker(logger *zap.Logger) *ReadinessChecker {
	return &ReadinessChecker{
		logger:         logger,
		shuttingDown:   false,
		serversRunning: false,
	}
}

// Name returns the name of the health check.
func (r *ReadinessChecker) Name() string {
	return "readiness"
}

// SetRunning marks the servers as running.
func (r *ReadinessChecker) SetRunning(running bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serversRunning = running
}

// SetShuttingDown marks the service as shutting down.
func (r *ReadinessChecker) SetShuttingDown(shutDown bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shuttingDown = shutDown
}

// Check performs the health check.
func (r *ReadinessChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()

	r.mu.RLock()
	shuttingDown := r.shuttingDown
	serversRunning := r.serversRunning
	r.mu.RUnlock()

	result := CheckResult{
		Name:      r.Name(),
		Status:    StatusOK,
		Message:   "Service ready",
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	if shuttingDown {
		result.Status = StatusNotReady
		result.Message = "Service shutting down"
	} else if !serversRunning {
		result.Status = StatusNotReady
		result.Message = "Service not ready"
	}

	return result
}
