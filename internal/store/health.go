package store

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/health"
)

// ConnectionHealthChecker checks if the Olric store connection is healthy.
type ConnectionHealthChecker struct {
	logger *zap.Logger
	store  Store
}

// NewConnectionHealthChecker creates a new connection health checker.
func NewConnectionHealthChecker(logger *zap.Logger, store Store) *ConnectionHealthChecker {
	return &ConnectionHealthChecker{
		logger: logger,
		store:  store,
	}
}

// Name returns the name of the health check.
func (c *ConnectionHealthChecker) Name() string {
	return "olric-connection"
}

// Check performs the health check.
func (c *ConnectionHealthChecker) Check(ctx context.Context) health.CheckResult {
	start := time.Now()

	// Create a context with 2 second timeout
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := c.store.Ping(checkCtx)

	result := health.CheckResult{
		Name:      c.Name(),
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	if err != nil {
		result.Status = health.StatusError
		result.Message = fmt.Sprintf("Olric connection failed: %v", err)
		c.logger.Warn("Olric connection check failed", zap.Error(err))
	} else {
		result.Status = health.StatusOK
		result.Message = "Olric connection healthy"
	}

	return result
}

// ClusterHealthChecker checks if the Olric cluster is healthy.
type ClusterHealthChecker struct {
	logger     *zap.Logger
	store      Store
	quorum     int
	singleNode bool
}

// NewClusterHealthChecker creates a new cluster health checker.
// If singleNode is true, this check will always pass.
// quorum is the minimum number of members required for the cluster to be healthy.
func NewClusterHealthChecker(logger *zap.Logger, store Store, quorum int, singleNode bool) *ClusterHealthChecker {
	return &ClusterHealthChecker{
		logger:     logger,
		store:      store,
		quorum:     quorum,
		singleNode: singleNode,
	}
}

// Name returns the name of the health check.
func (c *ClusterHealthChecker) Name() string {
	return "olric-cluster"
}

// Check performs the health check.
func (c *ClusterHealthChecker) Check(ctx context.Context) health.CheckResult {
	start := time.Now()

	result := health.CheckResult{
		Name:      c.Name(),
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	// In single-node mode, cluster check always passes
	if c.singleNode {
		result.Status = health.StatusOK
		result.Message = "Running in single-node mode"
		return result
	}

	// Create a context with 5 second timeout
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get cluster stats
	stats, err := c.store.Stats(checkCtx)
	if err != nil {
		result.Status = health.StatusError
		result.Message = fmt.Sprintf("Failed to get cluster stats: %v", err)
		c.logger.Warn("Cluster health check failed", zap.Error(err))
		return result
	}

	// Check if we have quorum for safe reads and writes
	if stats.ClusterMembers < c.quorum {
		result.Status = health.StatusNotReady
		result.Message = fmt.Sprintf("Cluster has %d members, quorum requires %d",
			stats.ClusterMembers, c.quorum)
		c.logger.Warn("Cluster member count below quorum",
			zap.Int("current", stats.ClusterMembers),
			zap.Int("quorum", c.quorum),
		)
		return result
	}

	result.Status = health.StatusOK
	result.Message = fmt.Sprintf("Cluster healthy with %d members (quorum: %d)", stats.ClusterMembers, c.quorum)
	return result
}

// StorageHealthChecker checks if the Olric storage is working.
type StorageHealthChecker struct {
	logger *zap.Logger
	store  Store
}

// NewStorageHealthChecker creates a new storage health checker.
func NewStorageHealthChecker(logger *zap.Logger, store Store) *StorageHealthChecker {
	return &StorageHealthChecker{
		logger: logger,
		store:  store,
	}
}

// Name returns the name of the health check.
func (s *StorageHealthChecker) Name() string {
	return "olric-storage"
}

// Check performs the health check.
func (s *StorageHealthChecker) Check(ctx context.Context) health.CheckResult {
	start := time.Now()

	result := health.CheckResult{
		Name:      s.Name(),
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	// Create a context with 3 second timeout
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Test key for health check
	testKey := fmt.Sprintf("health-check-%d", time.Now().UnixNano())
	testValue := "healthy"

	// Try to write a test key
	if err := s.store.Put(checkCtx, testKey, testValue, 5*time.Second); err != nil {
		result.Status = health.StatusError
		result.Message = fmt.Sprintf("Failed to write test key: %v", err)
		s.logger.Warn("Storage write health check failed", zap.Error(err))
		return result
	}

	// Try to read the test key
	value, err := s.store.Get(checkCtx, testKey)
	if err != nil {
		result.Status = health.StatusError
		result.Message = fmt.Sprintf("Failed to read test key: %v", err)
		s.logger.Warn("Storage read health check failed", zap.Error(err))
		// Try to clean up
		_ = s.store.Delete(context.Background(), testKey)
		return result
	}

	// Verify the value
	if value != testValue {
		result.Status = health.StatusError
		result.Message = fmt.Sprintf("Test key value mismatch: got %v, want %v", value, testValue)
		s.logger.Warn("Storage value health check failed",
			zap.Any("got", value),
			zap.String("want", testValue),
		)
		// Try to clean up
		_ = s.store.Delete(context.Background(), testKey)
		return result
	}

	// Clean up the test key
	if err := s.store.Delete(checkCtx, testKey); err != nil {
		s.logger.Warn("Failed to clean up test key", zap.Error(err))
		// Not a critical error, just log it
	}

	result.Status = health.StatusOK
	result.Message = "Storage read/write operations working"
	return result
}
