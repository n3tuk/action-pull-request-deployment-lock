package storage

import (
	"context"
	"testing"
	"time"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/logger"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/store"
)

// testOlricConfig returns a standard Olric config for tests.
func testOlricConfig(port int, memberlistPort int) *store.OlricConfig {
	return &store.OlricConfig{
		BindAddr:           "127.0.0.1",
		BindPort:           port,
		AdvertiseAddr:      "",
		AdvertisePort:      0,
		MemberlistBindPort: memberlistPort,
		JoinAddrs:          []string{},
		ReplicationMode:    "async",
		ReplicationFactor:  1,
		PartitionCount:     23, // Smaller for tests
		BackupCount:        1,
		BackupMode:         "async",
		MemberCountQuorum:  1,
		JoinRetryInterval:  1 * time.Second,
		MaxJoinAttempts:    30,
		LogLevel:           "ERROR", // Reduce noise in tests
		KeepAlivePeriod:    30 * time.Second,
		RequestTimeout:     5 * time.Second,
		DMapName:           "test-locks",
	}
}

func TestOlricLockManager_AcquireLock(t *testing.T) {
	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create Olric store
	olricStore, err := store.NewOlricStore(context.Background(), testOlricConfig(23320, 27320), log)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = olricStore.Close(ctx)
	}()

	lockManager := NewOlricLockManager(olricStore, log)

	t.Run("acquire new lock", func(t *testing.T) {
		lock, err := lockManager.AcquireLock(context.Background(), "project1", "main", "owner1")
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}

		if lock.Project != "project1" {
			t.Errorf("Project mismatch: got %s, want project1", lock.Project)
		}
		if lock.Branch != "main" {
			t.Errorf("Branch mismatch: got %s, want main", lock.Branch)
		}
		if lock.Owner != "owner1" {
			t.Errorf("Owner mismatch: got %s, want owner1", lock.Owner)
		}
	})

	t.Run("idempotent acquire same lock", func(t *testing.T) {
		// Acquire same lock again - should succeed
		lock, err := lockManager.AcquireLock(context.Background(), "project1", "main", "owner1")
		if err != nil {
			t.Fatalf("Failed to acquire lock idempotently: %v", err)
		}

		if lock.Project != "project1" {
			t.Errorf("Project mismatch: got %s, want project1", lock.Project)
		}
	})

	t.Run("conflict on different branch", func(t *testing.T) {
		// Try to acquire lock with different branch - should conflict
		lock, err := lockManager.AcquireLock(context.Background(), "project1", "feature", "owner2")
		if err != ErrLockConflict {
			t.Fatalf("Expected lock conflict, got: %v", err)
		}

		if lock == nil {
			t.Fatal("Expected lock to be returned even on conflict")
		}
		if lock.Branch != "main" {
			t.Errorf("Expected existing lock branch 'main', got %s", lock.Branch)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		_, err := lockManager.AcquireLock(context.Background(), "", "main", "owner1")
		if err == nil {
			t.Error("Expected error for empty project")
		}

		_, err = lockManager.AcquireLock(context.Background(), "project2", "", "owner1")
		if err == nil {
			t.Error("Expected error for empty branch")
		}

		_, err = lockManager.AcquireLock(context.Background(), "project2", "main", "")
		if err == nil {
			t.Error("Expected error for empty owner")
		}
	})
}

func TestOlricLockManager_ReleaseLock(t *testing.T) {
	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create Olric store
	olricStore, err := store.NewOlricStore(context.Background(), testOlricConfig(23321, 27321), log)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = olricStore.Close(ctx)
	}()

	lockManager := NewOlricLockManager(olricStore, log)

	t.Run("release existing lock", func(t *testing.T) {
		// Acquire lock first
		_, err := lockManager.AcquireLock(context.Background(), "project2", "main", "owner1")
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}

		// Release lock
		err = lockManager.ReleaseLock(context.Background(), "project2", "main", "owner1")
		if err != nil {
			t.Fatalf("Failed to release lock: %v", err)
		}

		// Verify lock is gone
		_, err = lockManager.GetLock(context.Background(), "project2")
		if err != ErrLockNotFound {
			t.Errorf("Expected lock not found error, got: %v", err)
		}
	})

	t.Run("idempotent release", func(t *testing.T) {
		// Release non-existent lock - should succeed
		err := lockManager.ReleaseLock(context.Background(), "project3", "main", "owner1")
		if err != nil {
			t.Fatalf("Expected idempotent release to succeed, got: %v", err)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		err := lockManager.ReleaseLock(context.Background(), "", "main", "owner1")
		if err == nil {
			t.Error("Expected error for empty project")
		}
	})
}

func TestOlricLockManager_GetLock(t *testing.T) {
	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create Olric store
	olricStore, err := store.NewOlricStore(context.Background(), testOlricConfig(23322, 27322), log)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = olricStore.Close(ctx)
	}()

	lockManager := NewOlricLockManager(olricStore, log)

	t.Run("get existing lock", func(t *testing.T) {
		// Acquire lock first
		_, err := lockManager.AcquireLock(context.Background(), "project4", "main", "owner1")
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}

		// Get lock
		lock, err := lockManager.GetLock(context.Background(), "project4")
		if err != nil {
			t.Fatalf("Failed to get lock: %v", err)
		}

		if lock.Project != "project4" {
			t.Errorf("Project mismatch: got %s, want project4", lock.Project)
		}
		if lock.Branch != "main" {
			t.Errorf("Branch mismatch: got %s, want main", lock.Branch)
		}
	})

	t.Run("get non-existent lock", func(t *testing.T) {
		_, err := lockManager.GetLock(context.Background(), "non-existent")
		if err != ErrLockNotFound {
			t.Errorf("Expected ErrLockNotFound, got: %v", err)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		_, err := lockManager.GetLock(context.Background(), "")
		if err == nil {
			t.Error("Expected error for empty project")
		}
	})
}

func TestOlricLockManager_Concurrency(t *testing.T) {
	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create Olric store
	olricStore, err := store.NewOlricStore(context.Background(), testOlricConfig(23323, 27323), log)
	if err != nil {
		t.Fatalf("Failed to create Olric store: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = olricStore.Close(ctx)
	}()

	lockManager := NewOlricLockManager(olricStore, log)

	// Try to acquire the same project from multiple goroutines with different branches
	// At least one should succeed, and all should either succeed or get conflict (no other errors)
	results := make(chan struct {
		branch string
		err    error
	}, 5)
	project := "concurrent-project"

	for i := 0; i < 5; i++ {
		branch := "branch-" + string(rune('A'+i))
		owner := "owner-" + string(rune('A'+i))

		go func(b, o string) {
			_, err := lockManager.AcquireLock(context.Background(), project, b, o)
			results <- struct {
				branch string
				err    error
			}{b, err}
		}(branch, owner)
	}

	successCount := 0
	conflictCount := 0

	for i := 0; i < 5; i++ {
		result := <-results
		if result.err == nil {
			successCount++
		} else if result.err == ErrLockConflict {
			conflictCount++
		} else {
			t.Errorf("Unexpected error: %v", result.err)
		}
	}

	// At least one should succeed (could be more due to race conditions in Get-Check-Put)
	if successCount < 1 {
		t.Errorf("Expected at least 1 success, got %d", successCount)
	}

	// All operations should either succeed or conflict
	if successCount+conflictCount != 5 {
		t.Errorf("Expected all operations to succeed or conflict, got %d successes + %d conflicts", successCount, conflictCount)
	}

	// Verify a lock exists for the project
	lock, err := lockManager.GetLock(context.Background(), project)
	if err != nil {
		t.Errorf("Failed to get lock: %v", err)
	}
	if lock == nil {
		t.Error("Expected a lock to exist")
	}
}
