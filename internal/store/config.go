package store

import (
	"fmt"
	"time"
)

// OlricConfig holds the configuration for the Olric distributed store.
type OlricConfig struct {
	// BindAddr is the address to bind the Olric server to.
	// Default: "0.0.0.0:3320"
	BindAddr string

	// BindPort is the port to bind the Olric server to.
	// Default: 3320
	BindPort int

	// JoinAddrs is a list of addresses to join for cluster formation.
	// If empty, the node will start in single-node mode.
	// Example: ["node1:3320", "node2:3320"]
	JoinAddrs []string

	// ReplicationMode specifies the replication mode (sync or async).
	// Default: "async"
	ReplicationMode string

	// ReplicationFactor is the number of replicas for each partition.
	// Default: 1 (single-node), 2 (cluster)
	ReplicationFactor int

	// PartitionCount is the number of partitions in the cluster.
	// Default: 271 (prime number for good distribution)
	PartitionCount uint64

	// BackupCount is the number of backup replicas.
	// Default: 1
	BackupCount int

	// BackupMode specifies the backup mode (sync or async).
	// Default: "async"
	BackupMode string

	// MemberCountQuorum is the expected number of cluster members.
	// The cluster will wait for this many members before considering itself ready.
	// Default: 1 (single-node mode)
	MemberCountQuorum int

	// JoinRetryInterval is the interval between join retry attempts.
	// Default: 1s
	JoinRetryInterval time.Duration

	// MaxJoinAttempts is the maximum number of join attempts.
	// Default: 30
	MaxJoinAttempts int

	// LogLevel is the log level for Olric internals.
	// Valid values: "DEBUG", "INFO", "WARN", "ERROR"
	// Default: "WARN"
	LogLevel string

	// KeepAlivePeriod is the period for TCP keep-alive probes.
	// Default: 30s
	KeepAlivePeriod time.Duration

	// RequestTimeout is the timeout for Olric requests.
	// Default: 5s
	RequestTimeout time.Duration

	// DMapName is the name of the distributed map to use for storing locks.
	// Default: "deployment-locks"
	DMapName string
}

// NewDefaultOlricConfig returns an OlricConfig with sensible defaults.
func NewDefaultOlricConfig() *OlricConfig {
	return &OlricConfig{
		BindAddr:          "0.0.0.0",
		BindPort:          3320,
		JoinAddrs:         []string{},
		ReplicationMode:   "async",
		ReplicationFactor: 1,
		PartitionCount:    271,
		BackupCount:       1,
		BackupMode:        "async",
		MemberCountQuorum: 1,
		JoinRetryInterval: 1 * time.Second,
		MaxJoinAttempts:   30,
		LogLevel:          "WARN",
		KeepAlivePeriod:   30 * time.Second,
		RequestTimeout:    5 * time.Second,
		DMapName:          "deployment-locks",
	}
}

// Validate checks if the Olric configuration is valid.
func (c *OlricConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("bind address cannot be empty")
	}

	if c.BindPort < 1 || c.BindPort > 65535 {
		return fmt.Errorf("invalid bind port: %d (must be 1-65535)", c.BindPort)
	}

	if c.ReplicationMode != "sync" && c.ReplicationMode != "async" {
		return fmt.Errorf("invalid replication mode: %s (must be sync or async)", c.ReplicationMode)
	}

	if c.ReplicationFactor < 1 {
		return fmt.Errorf("replication factor must be at least 1")
	}

	if c.PartitionCount < 1 {
		return fmt.Errorf("partition count must be at least 1")
	}

	if c.BackupCount < 0 {
		return fmt.Errorf("backup count cannot be negative")
	}

	if c.BackupMode != "sync" && c.BackupMode != "async" {
		return fmt.Errorf("invalid backup mode: %s (must be sync or async)", c.BackupMode)
	}

	if c.MemberCountQuorum < 1 {
		return fmt.Errorf("member count quorum must be at least 1")
	}

	if c.JoinRetryInterval <= 0 {
		return fmt.Errorf("join retry interval must be positive")
	}

	if c.MaxJoinAttempts < 1 {
		return fmt.Errorf("max join attempts must be at least 1")
	}

	validLogLevels := map[string]bool{
		"DEBUG": true,
		"INFO":  true,
		"WARN":  true,
		"ERROR": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be DEBUG, INFO, WARN, or ERROR)", c.LogLevel)
	}

	if c.KeepAlivePeriod <= 0 {
		return fmt.Errorf("keep alive period must be positive")
	}

	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}

	if c.DMapName == "" {
		return fmt.Errorf("dmap name cannot be empty")
	}

	// Validate cluster configuration
	if len(c.JoinAddrs) > 0 {
		// Multi-node mode
		if c.MemberCountQuorum > len(c.JoinAddrs)+1 {
			return fmt.Errorf("member count quorum (%d) cannot be greater than number of join addresses + 1 (%d)",
				c.MemberCountQuorum, len(c.JoinAddrs)+1)
		}

		// In multi-node mode, replication factor should be at least 2
		if c.ReplicationFactor < 2 {
			return fmt.Errorf("replication factor should be at least 2 in multi-node mode (current: %d)", c.ReplicationFactor)
		}
	}

	return nil
}

// IsSingleNode returns true if this is configured for single-node mode.
func (c *OlricConfig) IsSingleNode() bool {
	return len(c.JoinAddrs) == 0
}
