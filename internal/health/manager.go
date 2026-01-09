package health

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages multiple health checks and provides aggregated results.
type Manager struct {
	logger           *zap.Logger
	checkers         map[string]Checker
	cache            map[string]*cachedResult
	cacheMutex       sync.RWMutex
	cacheDuration    time.Duration
	checkTimeout     time.Duration
	serverChecker    *ServerChecker
	readinessChecker *ReadinessChecker
}

type cachedResult struct {
	result    CheckResult
	expiresAt time.Time
}

// NewManager creates a new health check manager.
func NewManager(logger *zap.Logger, cacheDuration, checkTimeout time.Duration) *Manager {
	return &Manager{
		logger:        logger,
		checkers:      make(map[string]Checker),
		cache:         make(map[string]*cachedResult),
		cacheDuration: cacheDuration,
		checkTimeout:  checkTimeout,
	}
}

// RegisterChecker registers a new health checker.
func (m *Manager) RegisterChecker(checker Checker) {
	m.checkers[checker.Name()] = checker

	// Keep references to special checkers for easy access
	switch c := checker.(type) {
	case *ServerChecker:
		m.serverChecker = c
	case *ReadinessChecker:
		m.readinessChecker = c
	}
}

// SetServersRunning marks the servers as running.
func (m *Manager) SetServersRunning(running bool) {
	if m.serverChecker != nil {
		m.serverChecker.SetRunning(running)
	}
	if m.readinessChecker != nil {
		m.readinessChecker.SetRunning(running)
	}
}

// SetShuttingDown marks the service as shutting down.
func (m *Manager) SetShuttingDown(shutDown bool) {
	if m.readinessChecker != nil {
		m.readinessChecker.SetShuttingDown(shutDown)
	}
}

// CheckAll runs all registered health checks concurrently.
func (m *Manager) CheckAll(ctx context.Context) []CheckResult {
	results := make([]CheckResult, 0, len(m.checkers))
	resultChan := make(chan CheckResult, len(m.checkers))

	var wg sync.WaitGroup

	for _, checker := range m.checkers {
		wg.Add(1)

		// Run each check in a goroutine
		go func(c Checker) {
			defer wg.Done()
			resultChan <- m.runCheck(ctx, c)
		}(checker)
	}

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// runCheck runs a single health check with timeout and caching.
func (m *Manager) runCheck(ctx context.Context, checker Checker) CheckResult {
	name := checker.Name()

	// Check cache first
	if cached := m.getCachedResult(name); cached != nil {
		return *cached
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, m.checkTimeout)
	defer cancel()

	// Run the check
	result := checker.Check(checkCtx)

	// Cache the result
	m.cacheResult(name, result)

	return result
}

// getCachedResult returns a cached result if it exists and hasn't expired.
func (m *Manager) getCachedResult(name string) *CheckResult {
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	if cached, ok := m.cache[name]; ok {
		if time.Now().Before(cached.expiresAt) {
			return &cached.result
		}
	}

	return nil
}

// cacheResult caches a check result.
func (m *Manager) cacheResult(name string, result CheckResult) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	m.cache[name] = &cachedResult{
		result:    result,
		expiresAt: time.Now().Add(m.cacheDuration),
	}
}

// GetStartupStatus returns the startup status of the service.
func (m *Manager) GetStartupStatus(ctx context.Context) StartupResponse {
	results := m.CheckAll(ctx)

	response := StartupResponse{
		Status:    StatusOK,
		Timestamp: time.Now(),
		Checks:    make(map[string]Status),
	}

	// Check if all checks passed
	allOK := true
	for _, result := range results {
		response.Checks[result.Name] = result.Status
		if result.Status != StatusOK {
			allOK = false
		}
		if result.Status == StatusStarting {
			response.Status = StatusStarting
		}
		if result.Status == StatusError {
			response.Status = StatusError
		}
	}

	if allOK {
		response.Status = StatusOK
	}

	return response
}

// GetLivenessStatus returns the liveness status of the service.
// Liveness is minimal - just confirms the goroutine is alive.
func (m *Manager) GetLivenessStatus() LivenessResponse {
	return LivenessResponse{
		Status:    StatusOK,
		Timestamp: time.Now(),
	}
}

// GetReadinessStatus returns the readiness status of the service.
func (m *Manager) GetReadinessStatus(ctx context.Context) ReadinessResponse {
	// Only check readiness checker
	var result CheckResult
	if m.readinessChecker != nil {
		result = m.runCheck(ctx, m.readinessChecker)
	} else {
		result = CheckResult{
			Name:      "readiness",
			Status:    StatusOK,
			Timestamp: time.Now(),
		}
	}

	response := ReadinessResponse{
		Status:    result.Status,
		Timestamp: result.Timestamp,
		Ready:     result.Status == StatusOK,
	}

	return response
}
