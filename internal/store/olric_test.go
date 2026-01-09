package store

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestOlricStore_SingleNode(t *testing.T) {
	// Skip in short mode as this test starts an actual Olric server
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	// Create a single-node configuration
	cfg := NewDefaultOlricConfig()
	cfg.BindPort = 13320 // Use a different port for testing
	cfg.LogLevel = "ERROR" // Reduce log noise in tests

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewOlricStore(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := store.Close(shutdownCtx); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	}()

	// Test Put
	key := "test-key"
	value := "test-value"
	if err := store.Put(ctx, key, value, 0); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Test Get
	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got != value {
		t.Errorf("Get() = %v, want %v", got, value)
	}

	// Test Exists
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Test Delete
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify key is deleted
	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() after delete failed: %v", err)
	}
	if exists {
		t.Error("Exists() after delete = true, want false")
	}

	// Test Get on non-existent key
	_, err = store.Get(ctx, key)
	if err == nil {
		t.Error("Get() on deleted key should return error")
	}

	// Test Put with TTL
	ttlKey := "ttl-key"
	ttlValue := "ttl-value"
	if err := store.Put(ctx, ttlKey, ttlValue, 2*time.Second); err != nil {
		t.Fatalf("Put() with TTL failed: %v", err)
	}

	// Verify key exists
	exists, err = store.Exists(ctx, ttlKey)
	if err != nil {
		t.Fatalf("Exists() for TTL key failed: %v", err)
	}
	if !exists {
		t.Error("Exists() for TTL key = false, want true")
	}

	// Wait for TTL to expire
	time.Sleep(3 * time.Second)

	// Verify key is gone
	exists, err = store.Exists(ctx, ttlKey)
	if err != nil {
		t.Fatalf("Exists() after TTL failed: %v", err)
	}
	if exists {
		t.Error("Exists() after TTL expiry = true, want false")
	}
}

func TestOlricStore_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindPort = 13321
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

	// Test Ping
	if err := store.Ping(ctx); err != nil {
		t.Errorf("Ping() failed: %v", err)
	}
}

func TestOlricStore_Stats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindPort = 13322
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

	// Test Stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() failed: %v", err)
	}

	if stats.ClusterMembers != 1 {
		t.Errorf("Stats().ClusterMembers = %d, want 1", stats.ClusterMembers)
	}

	if stats.PartitionCount != int(cfg.PartitionCount) {
		t.Errorf("Stats().PartitionCount = %d, want %d", stats.PartitionCount, cfg.PartitionCount)
	}

	if stats.ReplicationFactor != cfg.ReplicationFactor {
		t.Errorf("Stats().ReplicationFactor = %d, want %d", stats.ReplicationFactor, cfg.ReplicationFactor)
	}
}

func TestOlricStore_DeleteIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger, _ := zap.NewDevelopment()

	cfg := NewDefaultOlricConfig()
	cfg.BindPort = 13323
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

	// Delete a non-existent key should not error
	key := "non-existent-key"
	if err := store.Delete(ctx, key); err != nil {
		t.Errorf("Delete() on non-existent key failed: %v", err)
	}

	// Second delete should also not error (idempotent)
	if err := store.Delete(ctx, key); err != nil {
		t.Errorf("Second Delete() on non-existent key failed: %v", err)
	}
}
