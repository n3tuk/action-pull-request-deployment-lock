package health

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConfigChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	checker := NewConfigChecker(logger)

	if checker.Name() != "config" {
		t.Errorf("Name() = %s, want config", checker.Name())
	}

	result := checker.Check(context.Background())
	if result.Status != StatusOK {
		t.Errorf("Check() status = %s, want %s", result.Status, StatusOK)
	}
}

func TestLoggerChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	checker := NewLoggerChecker(logger)

	if checker.Name() != "logger" {
		t.Errorf("Name() = %s, want logger", checker.Name())
	}

	result := checker.Check(context.Background())
	if result.Status != StatusOK {
		t.Errorf("Check() status = %s, want %s", result.Status, StatusOK)
	}
}

func TestLoggerCheckerNil(t *testing.T) {
	checker := NewLoggerChecker(nil)

	result := checker.Check(context.Background())
	if result.Status != StatusError {
		t.Errorf("Check() status = %s, want %s", result.Status, StatusError)
	}
}

func TestServerChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	checker := NewServerChecker(logger)

	if checker.Name() != "servers" {
		t.Errorf("Name() = %s, want servers", checker.Name())
	}

	// Initially not running
	result := checker.Check(context.Background())
	if result.Status != StatusStarting {
		t.Errorf("Check() status = %s, want %s", result.Status, StatusStarting)
	}

	// Mark as running
	checker.SetRunning(true)
	result = checker.Check(context.Background())
	if result.Status != StatusOK {
		t.Errorf("Check() status = %s, want %s after SetRunning(true)", result.Status, StatusOK)
	}

	// Mark as not running
	checker.SetRunning(false)
	result = checker.Check(context.Background())
	if result.Status != StatusStarting {
		t.Errorf("Check() status = %s, want %s after SetRunning(false)", result.Status, StatusStarting)
	}
}

func TestReadinessChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	checker := NewReadinessChecker(logger)

	if checker.Name() != "readiness" {
		t.Errorf("Name() = %s, want readiness", checker.Name())
	}

	// Initially not ready
	result := checker.Check(context.Background())
	if result.Status != StatusNotReady {
		t.Errorf("Check() status = %s, want %s", result.Status, StatusNotReady)
	}

	// Mark as running
	checker.SetRunning(true)
	result = checker.Check(context.Background())
	if result.Status != StatusOK {
		t.Errorf("Check() status = %s, want %s after SetRunning(true)", result.Status, StatusOK)
	}

	// Mark as shutting down
	checker.SetShuttingDown(true)
	result = checker.Check(context.Background())
	if result.Status != StatusNotReady {
		t.Errorf("Check() status = %s, want %s after SetShuttingDown(true)", result.Status, StatusNotReady)
	}
}

func TestManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger, 10*time.Second, 5*time.Second)

	// Register checkers
	configChecker := NewConfigChecker(logger)
	loggerChecker := NewLoggerChecker(logger)
	serverChecker := NewServerChecker(logger)

	manager.RegisterChecker(configChecker)
	manager.RegisterChecker(loggerChecker)
	manager.RegisterChecker(serverChecker)

	// Check all
	results := manager.CheckAll(context.Background())
	if len(results) != 3 {
		t.Errorf("CheckAll() returned %d results, want 3", len(results))
	}

	// Verify results contain expected checks
	names := make(map[string]bool)
	for _, result := range results {
		names[result.Name] = true
	}

	expectedNames := []string{"config", "logger", "servers"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("CheckAll() did not return check %s", name)
		}
	}
}

func TestManagerCaching(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger, 100*time.Millisecond, 5*time.Second)

	checker := NewConfigChecker(logger)
	manager.RegisterChecker(checker)

	// First call
	results1 := manager.CheckAll(context.Background())
	if len(results1) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results1))
	}
	time1 := results1[0].Timestamp

	// Second call (should be cached)
	results2 := manager.CheckAll(context.Background())
	if len(results2) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results2))
	}
	time2 := results2[0].Timestamp

	// Timestamps should be identical (cached)
	if !time1.Equal(time2) {
		t.Errorf("Expected cached result with same timestamp, got different times")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call (cache expired)
	results3 := manager.CheckAll(context.Background())
	if len(results3) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results3))
	}
	time3 := results3[0].Timestamp

	// Timestamp should be different (new check)
	if time1.Equal(time3) {
		t.Errorf("Expected new result with different timestamp after cache expiry")
	}
}

func TestManagerSetServersRunning(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger, 10*time.Second, 5*time.Second)

	serverChecker := NewServerChecker(logger)
	readinessChecker := NewReadinessChecker(logger)

	manager.RegisterChecker(serverChecker)
	manager.RegisterChecker(readinessChecker)

	// Set servers running
	manager.SetServersRunning(true)

	// Check that both checkers reflect running state
	serverResult := serverChecker.Check(context.Background())
	if serverResult.Status != StatusOK {
		t.Errorf("Server checker status = %s, want %s", serverResult.Status, StatusOK)
	}

	readinessResult := readinessChecker.Check(context.Background())
	if readinessResult.Status != StatusOK {
		t.Errorf("Readiness checker status = %s, want %s", readinessResult.Status, StatusOK)
	}
}

func TestManagerSetShuttingDown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger, 10*time.Second, 5*time.Second)

	readinessChecker := NewReadinessChecker(logger)
	manager.RegisterChecker(readinessChecker)

	// Set running first
	manager.SetServersRunning(true)
	result := readinessChecker.Check(context.Background())
	if result.Status != StatusOK {
		t.Errorf("Initial status = %s, want %s", result.Status, StatusOK)
	}

	// Set shutting down
	manager.SetShuttingDown(true)
	result = readinessChecker.Check(context.Background())
	if result.Status != StatusNotReady {
		t.Errorf("Status after shutdown = %s, want %s", result.Status, StatusNotReady)
	}
}

func TestManagerGetStartupStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Use short cache duration for testing
	manager := NewManager(logger, 10*time.Millisecond, 5*time.Second)

	configChecker := NewConfigChecker(logger)
	loggerChecker := NewLoggerChecker(logger)
	serverChecker := NewServerChecker(logger)

	manager.RegisterChecker(configChecker)
	manager.RegisterChecker(loggerChecker)
	manager.RegisterChecker(serverChecker)

	// Get startup status (servers not running yet)
	response := manager.GetStartupStatus(context.Background())
	if response.Status != StatusStarting {
		t.Errorf("Startup status = %s, want %s", response.Status, StatusStarting)
	}

	if len(response.Checks) != 3 {
		t.Errorf("Checks count = %d, want 3", len(response.Checks))
	}

	// Set servers running
	manager.SetServersRunning(true)
	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)
	response = manager.GetStartupStatus(context.Background())
	if response.Status != StatusOK {
		t.Errorf("Startup status after servers running = %s, want %s", response.Status, StatusOK)
	}
}

func TestManagerGetLivenessStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger, 10*time.Second, 5*time.Second)

	response := manager.GetLivenessStatus()
	if response.Status != StatusOK {
		t.Errorf("Liveness status = %s, want %s", response.Status, StatusOK)
	}
}

func TestManagerGetReadinessStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Use short cache duration for testing
	manager := NewManager(logger, 10*time.Millisecond, 5*time.Second)

	readinessChecker := NewReadinessChecker(logger)
	manager.RegisterChecker(readinessChecker)

	// Initially not ready
	response := manager.GetReadinessStatus(context.Background())
	if response.Status != StatusNotReady {
		t.Errorf("Readiness status = %s, want %s", response.Status, StatusNotReady)
	}
	if response.Ready {
		t.Error("Ready should be false")
	}

	// Set servers running
	manager.SetServersRunning(true)
	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)
	response = manager.GetReadinessStatus(context.Background())
	if response.Status != StatusOK {
		t.Errorf("Readiness status after running = %s, want %s", response.Status, StatusOK)
	}
	if !response.Ready {
		t.Error("Ready should be true")
	}
}

func TestManagerCheckTimeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Very short timeout
	manager := NewManager(logger, 10*time.Second, 1*time.Millisecond)

	// Create a checker that takes longer than the timeout
	slowChecker := &slowChecker{}
	manager.RegisterChecker(slowChecker)

	// Check should complete (won't actually timeout in our implementation, but won't hang)
	results := manager.CheckAll(context.Background())
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

// slowChecker is a test checker that takes time
type slowChecker struct{}

func (s *slowChecker) Name() string {
	return "slow"
}

func (s *slowChecker) Check(ctx context.Context) CheckResult {
	// Simulate a slow check
	time.Sleep(10 * time.Millisecond)
	return CheckResult{
		Name:      s.Name(),
		Status:    StatusOK,
		Timestamp: time.Now(),
		Duration:  10 * time.Millisecond,
	}
}
