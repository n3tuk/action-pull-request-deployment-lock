package store

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/hashicorp/logutils"
	"github.com/olric-data/olric"
	"github.com/olric-data/olric/config"
	"go.uber.org/zap"
)

// OlricStore implements the Store interface using Olric distributed key/value store.
// It runs an embedded Olric server and provides distributed storage with replication.
type OlricStore struct {
	config *OlricConfig
	logger *zap.Logger
	db     *olric.Olric
	client *olric.EmbeddedClient
	dmap   olric.DMap
}

// NewOlricStore creates a new Olric-based store.
// It initializes and starts an embedded Olric server, optionally joining a cluster.
func NewOlricStore(ctx context.Context, cfg *OlricConfig, logger *zap.Logger) (*OlricStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid olric configuration: %w", err)
	}

	store := &OlricStore{
		config: cfg,
		logger: logger,
	}

	// Create Olric configuration
	olricCfg, err := store.createOlricConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create olric config: %w", err)
	}

	logger.Info("Starting Olric embedded server",
		zap.String("bind_addr", fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.BindPort)),
		zap.Bool("single_node", cfg.IsSingleNode()),
		zap.Strings("join_addrs", cfg.JoinAddrs),
		zap.Int("replication_factor", cfg.ReplicationFactor),
		zap.Uint64("partition_count", cfg.PartitionCount),
	)

	// Start Olric
	db, err := olric.New(olricCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create olric instance: %w", err)
	}

	// Start the embedded server
	if err := db.Start(); err != nil {
		return nil, fmt.Errorf("failed to start olric: %w", err)
	}

	store.db = db

	// Get embedded client
	client := db.NewEmbeddedClient()
	store.client = client

	// Wait for cluster to be ready
	if err := store.waitForCluster(ctx); err != nil {
		// Clean up on failure
		_ = db.Shutdown(context.Background())
		return nil, fmt.Errorf("cluster not ready: %w", err)
	}

	// Get DMap for storing locks
	dmap, err := client.NewDMap(cfg.DMapName)
	if err != nil {
		_ = db.Shutdown(context.Background())
		return nil, fmt.Errorf("failed to create dmap: %w", err)
	}
	store.dmap = dmap

	// Get member count for logging
	members, err := client.Members(ctx)
	if err != nil {
		logger.Warn("Failed to get members", zap.Error(err))
	}

	logger.Info("Olric store initialized successfully",
		zap.Int("cluster_members", len(members)),
	)

	return store, nil
}

// createOlricConfig creates an Olric configuration from the OlricConfig.
func (s *OlricStore) createOlricConfig() (*config.Config, error) {
	// Create Olric logger with appropriate level
	logFilter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(s.config.LogLevel),
		Writer:   io.Discard, // We'll use our own logger via zap
	}

	// If log level is DEBUG or INFO, route to stdout
	if s.config.LogLevel == "DEBUG" || s.config.LogLevel == "INFO" {
		logFilter.Writer = os.Stdout
	}

	olricLogger := log.New(logFilter, "", log.LstdFlags)

	c := config.New("lan")
	c.BindAddr = s.config.BindAddr
	c.BindPort = s.config.BindPort
	c.KeepAlivePeriod = s.config.KeepAlivePeriod
	c.PartitionCount = s.config.PartitionCount
	c.ReplicaCount = s.config.ReplicationFactor
	c.ReadQuorum = 1
	c.WriteQuorum = 1
	c.MemberCountQuorum = int32(s.config.MemberCountQuorum)
	c.LogLevel = s.config.LogLevel
	c.Logger = olricLogger
	c.JoinRetryInterval = s.config.JoinRetryInterval
	c.MaxJoinAttempts = s.config.MaxJoinAttempts

	// Set replication mode
	if s.config.ReplicationMode == "sync" {
		c.ReplicationMode = config.SyncReplicationMode
	} else {
		c.ReplicationMode = config.AsyncReplicationMode
	}

	// Add join addresses if in cluster mode
	if len(s.config.JoinAddrs) > 0 {
		c.Peers = s.config.JoinAddrs
	}

	return c, nil
}

// waitForCluster waits for the cluster to be ready based on member count quorum.
func (s *OlricStore) waitForCluster(ctx context.Context) error {
	// If single-node mode, we're immediately ready
	if s.config.IsSingleNode() {
		s.logger.Info("Running in single-node mode, cluster ready")
		return nil
	}

	// Wait for expected number of members
	ticker := time.NewTicker(s.config.JoinRetryInterval)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			attempts++
			
			// Get current member count
			members, err := s.client.Members(context.Background())
			memberCount := len(members)
			if err != nil {
				s.logger.Warn("Failed to get members", zap.Error(err))
				memberCount = 0
			}

			s.logger.Debug("Waiting for cluster members",
				zap.Int("current_members", memberCount),
				zap.Int("required_members", s.config.MemberCountQuorum),
				zap.Int("attempt", attempts),
			)

			if memberCount >= s.config.MemberCountQuorum {
				s.logger.Info("Cluster member quorum reached",
					zap.Int("member_count", memberCount),
					zap.Int("quorum", s.config.MemberCountQuorum),
				)
				return nil
			}

			if attempts >= s.config.MaxJoinAttempts {
				return fmt.Errorf("max join attempts (%d) reached, only %d/%d members present",
					s.config.MaxJoinAttempts, memberCount, s.config.MemberCountQuorum)
			}
		}
	}
}

// Put stores a value with an optional TTL.
func (s *OlricStore) Put(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl > 0 {
		return s.dmap.Put(ctx, key, value, olric.EX(ttl))
	}
	return s.dmap.Put(ctx, key, value)
}

// Get retrieves a value for the given key.
func (s *OlricStore) Get(ctx context.Context, key string) (interface{}, error) {
	resp, err := s.dmap.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	var result interface{}
	if err := resp.Scan(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// Delete removes a value for the given key.
func (s *OlricStore) Delete(ctx context.Context, key string) error {
	// Olric returns olric.ErrKeyNotFound if the key doesn't exist, which is fine
	// We want Delete to be idempotent
	_, err := s.dmap.Delete(ctx, key)
	if err != nil && err.Error() != "key not found" {
		return err
	}
	return nil
}

// Exists checks if a key exists in the store.
func (s *OlricStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.dmap.Get(ctx, key)
	if err != nil {
		if err.Error() == "key not found" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Ping verifies connectivity to the store.
func (s *OlricStore) Ping(ctx context.Context) error {
	// Check if we can reach the Olric service by doing a simple operation
	// We'll try to check if a test key exists (which will return quickly)
	addr := net.JoinHostPort(s.config.BindAddr, fmt.Sprintf("%d", s.config.BindPort))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to olric: %w", err)
	}
	defer conn.Close()

	// Also verify the db is not nil
	if s.db == nil {
		return fmt.Errorf("olric db is nil")
	}

	return nil
}

// Stats returns current statistics about the store.
func (s *OlricStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{}

	// Get cluster members
	members, err := s.client.Members(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}
	stats.ClusterMembers = len(members)

	// Get partition count from config
	stats.PartitionCount = int(s.config.PartitionCount)

	// Set backup and replication info
	stats.BackupCount = s.config.BackupCount
	stats.ReplicationFactor = s.config.ReplicationFactor

	// Get runtime stats from Olric
	// Note: Stats() requires an address parameter, use first member or local address
	var rtStats interface{}
	if len(members) > 0 {
		// Try to get stats from a member
		rtStats, err = s.client.Stats(ctx, members[0].Name)
		if err != nil {
			s.logger.Debug("Failed to get detailed stats", zap.Error(err))
		}
	}
	
	// For now, set basic stats
	// Olric v0.7+ uses different stats structure
	_ = rtStats // Will use when we understand the stats structure better
	stats.TotalKeys = 0 // Will be populated when we scan or have better stats API
	stats.MemoryUsage = 0 // Will be populated from runtime stats

	return stats, nil
}

// Close gracefully shuts down the store.
func (s *OlricStore) Close(ctx context.Context) error {
	s.logger.Info("Shutting down Olric store")

	if s.db == nil {
		return nil
	}

	// Shutdown Olric with context
	if err := s.db.Shutdown(ctx); err != nil {
		s.logger.Error("Error shutting down Olric", zap.Error(err))
		return err
	}

	s.logger.Info("Olric store shut down successfully")
	return nil
}
