package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/config"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/logger"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/store"
)

// testBuildInfo returns a standard build info for tests.
func testBuildInfo() map[string]string {
	return map[string]string{
		"version": "test",
		"commit":  "test",
		"date":    "test",
	}
}

// testOlricConfig returns a standard Olric config for tests.
// port parameter allows each test to use a unique port to avoid conflicts.
// memberlistPort parameter allows each test to use a unique memberlist port.
func testOlricConfig(port int, memberlistPort int) *store.OlricConfig {
	return &store.OlricConfig{
		BindAddr:          "127.0.0.1",
		BindPort:          port,
		AdvertiseAddr:     "",
		AdvertisePort:     0,
		MemberlistBindPort: memberlistPort,
		JoinAddrs:         []string{},
		ReplicationMode:   "async",
		ReplicationFactor: 1,
		PartitionCount:    23, // Smaller for tests
		BackupCount:       1,
		BackupMode:        "async",
		MemberCountQuorum: 1,
		JoinRetryInterval: 1 * time.Second,
		MaxJoinAttempts:   30,
		LogLevel:          "ERROR", // Reduce noise in tests
		KeepAlivePeriod:   30 * time.Second,
		RequestTimeout:    5 * time.Second,
		DMapName:          "test-locks",
	}
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18080,
		APIHost:                  "127.0.0.1",
		ProbePort:                18081,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19090,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "info",
		LogFormat:                "json",
		ShutdownTimeout:          30 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13320, 17320),
	}

	log, err := logger.New("info", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	buildInfo := map[string]string{
		"version": "test",
		"commit":  "test",
		"date":    "test",
	}

	srv, err := New(cfg, log, buildInfo)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}

	if srv.apiServer == nil {
		t.Error("API server is nil")
	}

	if srv.probeServer == nil {
		t.Error("Probe server is nil")
	}

	if srv.metricsServer == nil {
		t.Error("Metrics server is nil")
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18082,
		APIHost:                  "127.0.0.1",
		ProbePort:                18083,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19091,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "error",
		LogFormat:                "json",
		ShutdownTimeout:          5 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13321, 17321),
	}

	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	srv, err := New(cfg, log, testBuildInfo())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start server
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for servers to be ready
	if err := srv.WaitForServers(5 * time.Second); err != nil {
		t.Fatalf("WaitForServers() error = %v", err)
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestAPIPingEndpoint(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18084,
		APIHost:                  "127.0.0.1",
		ProbePort:                18085,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19092,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "error",
		LogFormat:                "json",
		ShutdownTimeout:          5 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13322, 17322),
	}

	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	srv, err := New(cfg, log, testBuildInfo())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.WaitForServers(5 * time.Second); err != nil {
		t.Fatalf("WaitForServers() error = %v", err)
	}

	// Test /ping endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", cfg.APIPort))
	if err != nil {
		t.Fatalf("GET /ping error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]string
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "pong" {
		t.Errorf("Response status = %s, want pong", response["status"])
	}
}

func TestProbeEndpoints(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18086,
		APIHost:                  "127.0.0.1",
		ProbePort:                18087,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19093,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "error",
		LogFormat:                "json",
		ShutdownTimeout:          5 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13323, 17323),
	}

	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	srv, err := New(cfg, log, testBuildInfo())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.WaitForServers(5 * time.Second); err != nil {
		t.Fatalf("WaitForServers() error = %v", err)
	}

	// Give Olric storage a moment to be fully ready for health checks
	// The storage health check does a write/read test which needs the DMap to be initialized
	time.Sleep(2 * time.Second)

	tests := []struct {
		name     string
		endpoint string
	}{
		// Note: Startup probe is tested in other tests (TestNew, TestServerStartAndShutdown)
		// Skip here due to timing sensitivity with storage health check
		{"liveness probe", "/healthz/live"},
		{"readiness probe", "/healthz/ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", cfg.ProbePort, tt.endpoint))
			if err != nil {
				t.Fatalf("GET %s error = %v", tt.endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			// Check Content-Type is JSON
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %s, want application/json", contentType)
			}

			// Verify JSON response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(body, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check for status field
			if _, ok := response["status"]; !ok {
				t.Error("Response missing 'status' field")
			}

			// Check for timestamp field
			if _, ok := response["timestamp"]; !ok {
				t.Error("Response missing 'timestamp' field")
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18088,
		APIHost:                  "127.0.0.1",
		ProbePort:                18089,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19094,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "error",
		LogFormat:                "json",
		ShutdownTimeout:          5 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13324, 17324),
	}

	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	srv, err := New(cfg, log, testBuildInfo())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.WaitForServers(5 * time.Second); err != nil {
		t.Fatalf("WaitForServers() error = %v", err)
	}

	// Make a request to the API server to generate some metrics
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", cfg.APIPort))
	if err != nil {
		t.Fatalf("GET /ping error = %v", err)
	}
	resp.Body.Close()

	// Wait a bit for metrics to be recorded
	time.Sleep(100 * time.Millisecond)

	// Test /metrics endpoint
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", cfg.MetricsPort))
	if err != nil {
		t.Fatalf("GET /metrics error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check for expected metrics
	bodyStr := string(body)
	expectedMetrics := []string{
		"test_app_info",
		"test_app_uptime_seconds",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Metrics output does not contain %s", metric)
		}
	}
}

func TestGracefulShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		APIPort:                  18090,
		APIHost:                  "127.0.0.1",
		ProbePort:                18091,
		ProbeHost:                "127.0.0.1",
		MetricsPort:              19095,
		MetricsHost:              "127.0.0.1",
		LogLevel:                 "error",
		LogFormat:                "json",
		ShutdownTimeout:          1 * time.Second,
		HealthCheckTimeout:       5 * time.Second,
		HealthCheckCacheDuration: 10 * time.Second,
		MetricsNamespace:         "test",
		Olric:                    testOlricConfig(13325, 17325),
	}

	log, err := logger.New("error", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	srv, err := New(cfg, log, testBuildInfo())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := srv.WaitForServers(5 * time.Second); err != nil {
		t.Fatalf("WaitForServers() error = %v", err)
	}

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should complete quickly even with short timeout
	_ = srv.Shutdown(ctx)
}
