package store

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	dmapMu sync.RWMutex
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

	// Start the embedded server in a goroutine
	// Olric's Start() method blocks until shutdown
	startChan := make(chan error, 1)
	go func() {
		startChan <- db.Start()
	}()

	// Wait briefly to ensure Olric has started
	select {
	case err := <-startChan:
		// If Start() returns immediately, it's an error
		return nil, fmt.Errorf("olric start returned prematurely: %w", err)
	case <-time.After(500 * time.Millisecond):
		// Olric is running
	}

	store.db = db

	// Get embedded client
	client := db.NewEmbeddedClient()
	store.client = client

	// Give Olric a moment to fully initialize
	time.Sleep(100 * time.Millisecond)

	// Wait for cluster to be ready
	if err := store.waitForCluster(ctx); err != nil {
		// Clean up on failure
		_ = db.Shutdown(context.Background())
		return nil, fmt.Errorf("cluster not ready: %w", err)
	}

	// Note: DMap will be created lazily on first use to avoid hanging during initialization

	// Get member count for logging
	members, err := client.Members(ctx)
	if err != nil {
		logger.Warn("Failed to get members", zap.Error(err))
	}

	logger.Info("Olric store initialized successfully",
		zap.Int("cluster_members", len(members)),
	)

	// Eagerly create the DMap to avoid lazy initialization issues with health checks
	_, err = store.getDMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create DMap during initialization: %w", err)
	}

	return store, nil
}

// createOlricConfig creates an Olric configuration from the OlricConfig.
func (s *OlricStore) createOlricConfig() (*config.Config, error) {
	c := config.New("local")  // Use "local" for single-node, less network overhead
	c.BindAddr = s.config.BindAddr
	c.BindPort = s.config.BindPort
	c.KeepAlivePeriod = s.config.KeepAlivePeriod
	c.PartitionCount = s.config.PartitionCount
	c.ReplicaCount = s.config.ReplicationFactor
	c.ReadQuorum = 1
	c.WriteQuorum = s.config.MemberCountQuorum // Match write quorum to member count quorum for safety
	c.MemberCountQuorum = int32(s.config.MemberCountQuorum)
	c.LogLevel = s.config.LogLevel
	c.Logger = zap.NewStdLog(s.logger) // Route Olric logs through our configured logger
	c.JoinRetryInterval = s.config.JoinRetryInterval
	c.MaxJoinAttempts = s.config.MaxJoinAttempts
	c.BootstrapTimeout = 5 * time.Second // Reduce bootstrap timeout for faster startup

	// Configure memberlist for cluster discovery
	mc, err := config.NewMemberlistConfig("local")
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist config: %w", err)
	}
	
	// Set bind address for memberlist to match Olric bind address
	mc.BindAddr = s.config.BindAddr
	
	// Set bind port for memberlist (0 defaults to 7946)
	if s.config.MemberlistBindPort != 0 {
		mc.BindPort = s.config.MemberlistBindPort
	}
	
	// Set advertise address and port for NAT traversal and multi-instance testing
	if s.config.AdvertiseAddr != "" {
		mc.AdvertiseAddr = s.config.AdvertiseAddr
	}
	if s.config.AdvertisePort != 0 {
		mc.AdvertisePort = s.config.AdvertisePort
	}
	
	c.MemberlistConfig = mc

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

// getDMap returns the DMap, creating it lazily if needed.
func (s *OlricStore) getDMap(ctx context.Context) (olric.DMap, error) {
	s.dmapMu.RLock()
	if s.dmap != nil {
		defer s.dmapMu.RUnlock()
		return s.dmap, nil
	}
	s.dmapMu.RUnlock()

	s.dmapMu.Lock()
	defer s.dmapMu.Unlock()

	// Double-check after acquiring write lock
	if s.dmap != nil {
		return s.dmap, nil
	}

	// Create DMap
	dmap, err := s.client.NewDMap(s.config.DMapName)
	if err != nil {
		return nil, fmt.Errorf("failed to create dmap: %w", err)
	}

	s.dmap = dmap
	s.logger.Debug("Created DMap",
		zap.String("name", s.config.DMapName),
	)

	return s.dmap, nil
}

// Put stores a value with an optional TTL.
func (s *OlricStore) Put(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	dmap, err := s.getDMap(ctx)
	if err != nil {
		return err
	}
	
	if ttl > 0 {
		return dmap.Put(ctx, key, value, olric.EX(ttl))
	}
	return dmap.Put(ctx, key, value)
}

// Get retrieves a value for the given key.
func (s *OlricStore) Get(ctx context.Context, key string) (interface{}, error) {
	dmap, err := s.getDMap(ctx)
	if err != nil {
		return nil, err
	}
	
	resp, err := dmap.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	// Scan into a string since that's what we're storing
	// Olric requires concrete types for unmarshaling
	var result string
	if err := resp.Scan(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// isKeyNotFoundError checks if an error indicates a key was not found.
// Olric returns errors with "key not found" message for missing keys.
// This helper provides a centralized check that's easier to maintain if
// Olric's error handling changes in future versions.
func isKeyNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check the error message as Olric doesn't export a specific error constant
	return err.Error() == "key not found"
}

// Delete removes a value for the given key.
func (s *OlricStore) Delete(ctx context.Context, key string) error {
	dmap, err := s.getDMap(ctx)
	if err != nil {
		return err
	}
	
	// Olric returns "key not found" error if the key doesn't exist, which is fine
	// We want Delete to be idempotent
	_, err = dmap.Delete(ctx, key)
	if err != nil && !isKeyNotFoundError(err) {
		return err
	}
	return nil
}

// Exists checks if a key exists in the store.
func (s *OlricStore) Exists(ctx context.Context, key string) (bool, error) {
	dmap, err := s.getDMap(ctx)
	if err != nil {
		return false, err
	}
	
	_, err = dmap.Get(ctx, key)
	if err != nil {
		if isKeyNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Ping verifies connectivity to the store.
func (s *OlricStore) Ping(ctx context.Context) error {
	// For embedded Olric, verify health by checking if we can get members
	// This doesn't require network connectivity like the Ping command
	if s.client == nil {
		return fmt.Errorf("olric client is nil")
	}

	// Create a context with 2 second timeout
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	// Use Members() call as a health check for embedded client
	// This verifies the Olric instance is running and responsive
	_, err := s.client.Members(pingCtx)
	if err != nil {
		return fmt.Errorf("failed to get olric members: %w", err)
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
	
	// For now, set basic stats from cluster information.
	// TotalKeys and MemoryUsage are not currently available from Olric v0.7+ stats API.
	// These fields will remain at their default zero values, which callers should
	// treat as "not available" per the StoreStats documentation.
	_ = rtStats // Reserved for future use when detailed stats parsing is implemented
	stats.TotalKeys = 0
	stats.MemoryUsage = 0

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
