package store

import (
	"fmt"
	"net"
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

	// AdvertiseAddr is the address to advertise to other cluster members.
	// Used for NAT traversal. If empty, uses BindAddr.
	// Default: "" (uses BindAddr)
	AdvertiseAddr string

	// AdvertisePort is the port to advertise to other cluster members.
	// Used for NAT traversal and testing multiple instances on same host.
	// If 0, uses BindPort.
	// Default: 0 (uses BindPort)
	AdvertisePort int

	// MemberlistBindPort is the port for memberlist gossip protocol.
	// Default: 0 (uses random available port)
	MemberlistBindPort int

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

const (
	// DefaultBindAddr is the default bind address for Olric
	DefaultBindAddr = "0.0.0.0"
	// DefaultBindPort is the default bind port for Olric
	DefaultBindPort = 3320
	// DefaultAdvertiseAddr is the default advertise address (empty means use BindAddr)
	DefaultAdvertiseAddr = ""
	// DefaultAdvertisePort is the default advertise port (0 means use BindPort)
	DefaultAdvertisePort = 0
	// DefaultMemberlistBindPort is the default memberlist bind port (0 means random)
	DefaultMemberlistBindPort = 0
	// DefaultReplicationMode is the default replication mode
	DefaultReplicationMode = "async"
	// DefaultReplicationFactor is the default replication factor for single-node
	DefaultReplicationFactor = 1
	// DefaultPartitionCount is the default number of partitions
	DefaultPartitionCount = 271
	// DefaultBackupCount is the default number of backups
	DefaultBackupCount = 1
	// DefaultBackupMode is the default backup mode
	DefaultBackupMode = "async"
	// DefaultMemberCountQuorum is the default member count quorum
	DefaultMemberCountQuorum = 1
	// DefaultJoinRetryInterval is the default join retry interval
	DefaultJoinRetryInterval = 1 * time.Second
	// DefaultMaxJoinAttempts is the default max join attempts
	DefaultMaxJoinAttempts = 30
	// DefaultLogLevel is the default log level for Olric
	DefaultLogLevel = "WARN"
	// DefaultKeepAlivePeriod is the default keep alive period
	DefaultKeepAlivePeriod = 30 * time.Second
	// DefaultRequestTimeout is the default request timeout
	DefaultRequestTimeout = 5 * time.Second
	// DefaultDMapName is the default DMap name
	DefaultDMapName = "deployment-locks"
)

// NewDefaultOlricConfig returns an OlricConfig with sensible defaults.
func NewDefaultOlricConfig() *OlricConfig {
	return &OlricConfig{
		BindAddr:          DefaultBindAddr,
		BindPort:          DefaultBindPort,
		AdvertiseAddr:     DefaultAdvertiseAddr,
		AdvertisePort:     DefaultAdvertisePort,
		MemberlistBindPort: DefaultMemberlistBindPort,
		JoinAddrs:         []string{},
		ReplicationMode:   DefaultReplicationMode,
		ReplicationFactor: DefaultReplicationFactor,
		PartitionCount:    DefaultPartitionCount,
		BackupCount:       DefaultBackupCount,
		BackupMode:        DefaultBackupMode,
		MemberCountQuorum: DefaultMemberCountQuorum,
		JoinRetryInterval: DefaultJoinRetryInterval,
		MaxJoinAttempts:   DefaultMaxJoinAttempts,
		LogLevel:          DefaultLogLevel,
		KeepAlivePeriod:   DefaultKeepAlivePeriod,
		RequestTimeout:    DefaultRequestTimeout,
		DMapName:          DefaultDMapName,
	}
}

// Validate checks if the Olric configuration is valid.
func (c *OlricConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("bind address cannot be empty")
	}

	// Validate bind address is a valid IPv4 or IPv6 address
	if net.ParseIP(c.BindAddr) == nil && c.BindAddr != "0.0.0.0" && c.BindAddr != "::" {
		return fmt.Errorf("bind address must be a valid IPv4 or IPv6 address, got: %s", c.BindAddr)
	}

	if c.BindPort < 1 || c.BindPort > 65535 {
		return fmt.Errorf("bind port must be between 1 and 65535, got: %d", c.BindPort)
	}

	// Validate advertise address if provided
	if c.AdvertiseAddr != "" {
		if net.ParseIP(c.AdvertiseAddr) == nil && c.AdvertiseAddr != "0.0.0.0" && c.AdvertiseAddr != "::" {
			return fmt.Errorf("advertise address must be a valid IPv4 or IPv6 address, got: %s", c.AdvertiseAddr)
		}
	}

	// Validate advertise port if provided
	if c.AdvertisePort != 0 && (c.AdvertisePort < 1 || c.AdvertisePort > 65535) {
		return fmt.Errorf("advertise port must be between 1 and 65535, got: %d", c.AdvertisePort)
	}

	// Validate memberlist bind port if provided
	if c.MemberlistBindPort != 0 && (c.MemberlistBindPort < 1 || c.MemberlistBindPort > 65535) {
		return fmt.Errorf("memberlist bind port must be between 1 and 65535, got: %d", c.MemberlistBindPort)
	}

	if c.ReplicationMode != "sync" && c.ReplicationMode != "async" {
		return fmt.Errorf("replication mode must be sync or async, got: %s", c.ReplicationMode)
	}

	if c.ReplicationFactor < 1 {
		return fmt.Errorf("replication factor must be at least 1, got: %d", c.ReplicationFactor)
	}

	if c.PartitionCount < 1 {
		return fmt.Errorf("partition count must be at least 1, got: %d", c.PartitionCount)
	}

	if c.BackupCount < 0 {
		return fmt.Errorf("backup count must be zero or greater, got: %d", c.BackupCount)
	}

	if c.BackupMode != "sync" && c.BackupMode != "async" {
		return fmt.Errorf("backup mode must be sync or async, got: %s", c.BackupMode)
	}

	if c.MemberCountQuorum < 1 {
		return fmt.Errorf("member count quorum must be at least 1, got: %d", c.MemberCountQuorum)
	}

	if c.JoinRetryInterval <= 0 {
		return fmt.Errorf("join retry interval must be positive, got: %v", c.JoinRetryInterval)
	}

	if c.MaxJoinAttempts < 1 {
		return fmt.Errorf("max join attempts must be at least 1, got: %d", c.MaxJoinAttempts)
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
