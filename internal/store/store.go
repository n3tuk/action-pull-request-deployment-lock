package store

import (
	"context"
	"time"
)

// Store defines the interface for distributed key/value storage operations.
// It provides a clean abstraction for storing and retrieving deployment locks
// with support for TTL, cluster statistics, and health checks.
type Store interface {
	// Put stores a value with an optional TTL.
	// If ttl is 0, the key will not expire.
	// The value will be serialized using the underlying store's serialization mechanism.
	Put(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Get retrieves a value for the given key.
	// Returns an error if the key does not exist or if there's a connection issue.
	Get(ctx context.Context, key string) (interface{}, error)

	// Delete removes a value for the given key.
	// Returns nil if the key doesn't exist (idempotent operation).
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the store.
	// Returns true if the key exists, false otherwise.
	Exists(ctx context.Context, key string) (bool, error)

	// Ping verifies connectivity to the store.
	// This is used for health checks to ensure the store is reachable and responsive.
	Ping(ctx context.Context) error

	// Stats returns current statistics about the store.
	// This includes cluster membership, partition information, and storage metrics.
	Stats(ctx context.Context) (*StoreStats, error)

	// Close gracefully shuts down the store connection.
	// For embedded stores like Olric, this will also leave the cluster properly
	// and shut down the embedded server.
	Close(ctx context.Context) error
}

// StoreStats represents statistics about the distributed store.
// These metrics are useful for monitoring cluster health and performance.
type StoreStats struct {
	// ClusterMembers is the number of active members in the cluster.
	ClusterMembers int

	// PartitionCount is the total number of partitions in the cluster.
	// Partitions are used to distribute data across cluster members.
	PartitionCount int

	// BackupCount is the number of backup replicas for partitions.
	// Higher backup counts provide better fault tolerance.
	BackupCount int

	// ReplicationFactor is the number of copies of each partition.
	// This includes both primary and backup replicas.
	ReplicationFactor int

	// TotalKeys is the total number of keys stored across all partitions.
	TotalKeys int64

	// MemoryUsage is the total memory used by the store in bytes.
	// This includes all stored data and metadata.
	MemoryUsage int64
}
