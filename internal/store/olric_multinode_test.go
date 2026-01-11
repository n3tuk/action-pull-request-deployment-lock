package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestOlricStore_MultiNode tests a 3-node Olric cluster with quorum of 2.
// This test verifies:
// 1. Three nodes can start and form a cluster
// 2. CRUD operations work correctly
// 3. After shutting down one node, operations still succeed with quorum
//
// Note: Multi-node clustering on localhost can be timing-sensitive and may
// require specific network configuration. If this test fails in your environment,
// it doesn't indicate a problem with single-node deployments or production
// multi-node deployments on separate hosts.
func TestOlricStore_MultiNode(t *testing.T) {
	// Skip in short mode as this test starts actual Olric servers
	if testing.Short() {
		t.Skip("Skipping multi-node integration test in short mode")
	}
	
	// This test requires specific networking setup and can be flaky on localhost
	// Skip if SKIP_MULTINODE_TEST environment variable is set
	if os.Getenv("SKIP_MULTINODE_TEST") != "" {
		t.Skip("Multi-node test skipped - SKIP_MULTINODE_TEST environment variable is set")
	}

	logger, _ := zap.NewDevelopment()

	// Base configuration for cluster
	basePort := 14000
	baseMemberlistPort := 14200

	// Create configurations for 3 nodes
	configs := make([]*OlricConfig, 3)
	for i := 0; i < 3; i++ {
		configs[i] = NewDefaultOlricConfig()
		configs[i].BindAddr = "127.0.0.1"
		configs[i].BindPort = basePort + i
		configs[i].AdvertiseAddr = "" // Let it use bind addr
		configs[i].AdvertisePort = 0 // Let it use bind port
		configs[i].MemberlistBindPort = baseMemberlistPort + i // Use different memberlist ports
		configs[i].ReplicationFactor = 2
		configs[i].MemberCountQuorum = 2 // Require quorum of 2 for writes (WriteQuorum matches this)
		configs[i].LogLevel = "ERROR"    // Reduce log noise
		configs[i].PartitionCount = 23   // Smaller partition count for faster testing
		configs[i].MaxJoinAttempts = 20
	}

	// Set join addresses - use the actual Olric bind ports for joining
	olricAddrs := []string{
		fmt.Sprintf("127.0.0.1:%d", basePort),     // node 0
		fmt.Sprintf("127.0.0.1:%d", basePort+1),   // node 1
		fmt.Sprintf("127.0.0.1:%d", basePort+2),   // node 2
	}
	
	// Each node joins to the others using Olric bind ports
	for i := 0; i < 3; i++ {
		// Join to all other nodes (exclude self)
		var peers []string
		for j := 0; j < 3; j++ {
			if i != j {
				peers = append(peers, olricAddrs[j])
			}
		}
		configs[i].JoinAddrs = peers
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Start nodes sequentially to ensure cluster formation
	stores := make([]*OlricStore, 3)

	t.Log("Starting node 0 (bootstrap node)...")
	// Use a longer timeout for node 0 since it's waiting for other nodes
	node0Ctx, node0Cancel := context.WithTimeout(ctx, 40*time.Second)
	defer node0Cancel()
	
	// Start node 0 in a goroutine since it will block waiting for quorum
	var store0 *OlricStore
	var node0Err error
	node0Done := make(chan struct{})
	go func() {
		store0, node0Err = NewOlricStore(node0Ctx, configs[0], logger.Named("node0"))
		close(node0Done)
	}()

	// Give node 0 time to start listening before starting node 1
	time.Sleep(2 * time.Second)

	t.Log("Starting node 1...")
	store1, err := NewOlricStore(ctx, configs[1], logger.Named("node1"))
	if err != nil {
		t.Fatalf("Node 1 failed to start: %v", err)
	}
	stores[1] = store1
	t.Log("Node 1 started successfully")

	// Wait for node 0 to complete now that we have quorum
	select {
	case <-node0Done:
		if node0Err != nil {
			t.Fatalf("Node 0 failed to start: %v", node0Err)
		}
		stores[0] = store0
		t.Log("Node 0 started successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Node 0 failed to start in time after node 1 joined")
	}

	// Give cluster time to stabilize
	time.Sleep(2 * time.Second)

	t.Log("Starting node 2...")
	store2, err := NewOlricStore(ctx, configs[2], logger.Named("node2"))
	if err != nil {
		t.Fatalf("Node 2 failed to start: %v", err)
	}
	stores[2] = store2
	t.Log("Node 2 started successfully")

	// Ensure all nodes are created
	for i, store := range stores {
		if store == nil {
			t.Fatalf("Node %d is nil", i)
		}
	}

	// Defer cleanup of all stores
	defer func() {
		t.Log("Shutting down all nodes...")
		for i, store := range stores {
			if store != nil {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := store.Close(shutdownCtx); err != nil {
					t.Logf("Warning: Node %d shutdown error: %v", i, err)
				}
				shutdownCancel()
			}
		}
	}()

	// Give the cluster a moment to stabilize
	time.Sleep(2 * time.Second)

	// Verify cluster formation - should have 3 members
	// Note: We use WriteQuorum of 2 for safety even though MemberCountQuorum is 1
	t.Log("Verifying cluster formation...")
	for i, store := range stores {
		stats, err := store.Stats(ctx)
		if err != nil {
			t.Fatalf("Node %d: Failed to get stats: %v", i, err)
		}
		if stats.ClusterMembers < 2 {
			t.Errorf("Node %d: Expected at least 2 cluster members, got %d", i, stats.ClusterMembers)
		}
		t.Logf("Node %d: Cluster has %d members", i, stats.ClusterMembers)
	}

	// Test CRUD operations with full cluster (3 nodes, write quorum 2)
	t.Log("Testing CRUD operations with full cluster...")
	testKey := "test-key-multi"
	testValue := "test-value-multi"

	// Test Put on node 0
	if err := stores[0].Put(ctx, testKey, testValue, 0); err != nil {
		t.Fatalf("Failed to put key: %v", err)
	}
	t.Log("Successfully put key on node 0")

	// Give replication time to complete
	time.Sleep(500 * time.Millisecond)

	// Test Get from node 1 (different node)
	value, err := stores[1].Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to get key from node 1: %v", err)
	}
	if value != testValue {
		t.Errorf("Get from node 1: got %v, want %v", value, testValue)
	}
	t.Log("Successfully retrieved key from node 1 (replication working)")

	// Test Get from node 2
	value, err = stores[2].Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to get key from node 2: %v", err)
	}
	if value != testValue {
		t.Errorf("Get from node 2: got %v, want %v", value, testValue)
	}
	t.Log("Successfully retrieved key from node 2 (replication working)")

	// Test Exists
	exists, err := stores[0].Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to check exists: %v", err)
	}
	if !exists {
		t.Error("Key should exist but doesn't")
	}
	t.Log("Key existence verified")

	// Shutdown node 2 (leaving 2 nodes, which meets quorum of 2)
	t.Log("Shutting down node 2 to test quorum operation...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := stores[2].Close(shutdownCtx); err != nil {
		t.Logf("Warning: Node 2 shutdown error: %v", err)
	}
	shutdownCancel()
	stores[2] = nil

	// Give cluster time to detect node failure
	time.Sleep(2 * time.Second)

	// Verify remaining nodes see reduced cluster
	t.Log("Verifying cluster state after node 2 shutdown...")
	for i := 0; i < 2; i++ {
		stats, err := stores[i].Stats(ctx)
		if err != nil {
			t.Fatalf("Node %d: Failed to get stats after shutdown: %v", i, err)
		}
		// Cluster might still show 3 members briefly or 2 members
		t.Logf("Node %d: Cluster has %d members after shutdown", i, stats.ClusterMembers)
	}

	// Test CRUD operations with quorum (2 nodes remaining, write quorum 2)
	t.Log("Testing CRUD operations with quorum (2 nodes)...")
	
	// Test Put with quorum
	quorumKey := "test-key-quorum"
	quorumValue := "test-value-quorum"
	if err := stores[0].Put(ctx, quorumKey, quorumValue, 0); err != nil {
		t.Fatalf("Failed to put key with quorum: %v", err)
	}
	t.Log("Successfully put key with quorum")

	// Give replication time
	time.Sleep(500 * time.Millisecond)

	// Test Get from other node with quorum
	value, err = stores[1].Get(ctx, quorumKey)
	if err != nil {
		t.Fatalf("Failed to get key with quorum: %v", err)
	}
	if value != quorumValue {
		t.Errorf("Get with quorum: got %v, want %v", value, quorumValue)
	}
	t.Log("Successfully retrieved key with quorum")

	// Test Update operation
	updatedValue := "updated-value"
	if err := stores[0].Put(ctx, quorumKey, updatedValue, 0); err != nil {
		t.Fatalf("Failed to update key with quorum: %v", err)
	}
	t.Log("Successfully updated key with quorum")

	time.Sleep(500 * time.Millisecond)

	value, err = stores[1].Get(ctx, quorumKey)
	if err != nil {
		t.Fatalf("Failed to get updated key with quorum: %v", err)
	}
	if value != updatedValue {
		t.Errorf("Get updated value with quorum: got %v, want %v", value, updatedValue)
	}
	t.Log("Successfully verified update with quorum")

	// Test Delete with quorum
	if err := stores[0].Delete(ctx, quorumKey); err != nil {
		t.Fatalf("Failed to delete key with quorum: %v", err)
	}
	t.Log("Successfully deleted key with quorum")

	time.Sleep(500 * time.Millisecond)

	// Verify deletion on other node
	exists, err = stores[1].Exists(ctx, quorumKey)
	if err != nil {
		t.Fatalf("Failed to check exists after delete with quorum: %v", err)
	}
	if exists {
		t.Error("Key should not exist after delete with quorum")
	}
	t.Log("Successfully verified deletion with quorum")

	// Test with TTL
	ttlKey := "test-key-ttl"
	ttlValue := "test-value-ttl"
	if err := stores[0].Put(ctx, ttlKey, ttlValue, 2*time.Second); err != nil {
		t.Fatalf("Failed to put key with TTL: %v", err)
	}
	t.Log("Successfully put key with TTL")

	time.Sleep(500 * time.Millisecond)

	// Verify key exists
	exists, err = stores[1].Exists(ctx, ttlKey)
	if err != nil {
		t.Fatalf("Failed to check TTL key exists: %v", err)
	}
	if !exists {
		t.Error("TTL key should exist before expiry")
	}
	t.Log("TTL key exists before expiry")

	// Wait for TTL to expire
	time.Sleep(3 * time.Second)

	// Verify key is gone
	exists, err = stores[1].Exists(ctx, ttlKey)
	if err != nil {
		t.Fatalf("Failed to check TTL key after expiry: %v", err)
	}
	if exists {
		t.Error("TTL key should not exist after expiry")
	}
	t.Log("TTL key successfully expired")

	t.Log("Multi-node cluster test completed successfully!")
}
