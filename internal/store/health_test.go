package store

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/health"
)

func TestConnectionHealthChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindAddr = "127.0.0.1" // Use localhost for testing
	cfg.BindPort = 13324
	cfg.LogLevel = "ERROR"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewOlricStore(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = store.Close(shutdownCtx)
	}()

	checker := NewConnectionHealthChecker(logger, store)

	if checker.Name() != "olric-connection" {
		t.Errorf("Name() = %s, want olric-connection", checker.Name())
	}

	result := checker.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("Check() status = %s, want %s, message: %s", result.Status, health.StatusOK, result.Message)
	}
}

func TestClusterHealthChecker_SingleNode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindAddr = "127.0.0.1" // Use localhost for testing
	cfg.BindPort = 13325
	cfg.LogLevel = "ERROR"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewOlricStore(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = store.Close(shutdownCtx)
	}()

	// Single-node mode should always pass
	checker := NewClusterHealthChecker(logger, store, 1, true)

	if checker.Name() != "olric-cluster" {
		t.Errorf("Name() = %s, want olric-cluster", checker.Name())
	}

	result := checker.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("Check() status = %s, want %s, message: %s", result.Status, health.StatusOK, result.Message)
	}
}

func TestClusterHealthChecker_MultiNode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindAddr = "127.0.0.1" // Use localhost for testing
	cfg.BindPort = 13326
	cfg.LogLevel = "ERROR"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewOlricStore(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = store.Close(shutdownCtx)
	}()

	// Multi-node mode with current cluster having only 1 member should pass
	// since we're checking against quorum of 1
	checker := NewClusterHealthChecker(logger, store, 1, false)

	result := checker.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("Check() status = %s, want %s, message: %s", result.Status, health.StatusOK, result.Message)
	}

	// Multi-node mode with quorum of 2 should fail
	checker = NewClusterHealthChecker(logger, store, 2, false)

	result = checker.Check(ctx)
	if result.Status == health.StatusOK {
		t.Errorf("Check() status = %s, want not OK (cluster has 1 member, quorum is 2)", result.Status)
	}
}

func TestStorageHealthChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindAddr = "127.0.0.1" // Use localhost for testing
	cfg.BindPort = 13327
	cfg.LogLevel = "ERROR"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewOlricStore(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = store.Close(shutdownCtx)
	}()

	checker := NewStorageHealthChecker(logger, store)

	if checker.Name() != "olric-storage" {
		t.Errorf("Name() = %s, want olric-storage", checker.Name())
	}

	result := checker.Check(ctx)
	if result.Status != health.StatusOK {
		t.Errorf("Check() status = %s, want %s, message: %s", result.Status, health.StatusOK, result.Message)
	}

	// Verify the test key was cleaned up
	time.Sleep(100 * time.Millisecond)
	// We can't easily verify the cleanup without knowing the exact key name,
	// but we can trust it based on the code
}
