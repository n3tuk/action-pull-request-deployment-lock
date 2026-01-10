package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/config"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/logger"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/server"
	"github.com/n3tuk/action-pull-request-deployment-lock/internal/store"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "service",
	Short: "Deployment lock service",
	Long:  `A distributed locking service for managing deployment locks.`,
	RunE:  runServer,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit:  %s\n", commit)
		fmt.Printf("Built:   %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Configuration flags
	rootCmd.Flags().Int("api-port", 8080, "API server port")
	rootCmd.Flags().String("api-host", "0.0.0.0", "API server host")
	rootCmd.Flags().Int("probe-port", 8081, "Probe server port")
	rootCmd.Flags().String("probe-host", "0.0.0.0", "Probe server host")
	rootCmd.Flags().Int("metrics-port", 9090, "Metrics server port")
	rootCmd.Flags().String("metrics-host", "0.0.0.0", "Metrics server host")
	rootCmd.Flags().Bool("tls-enabled", false, "Enable TLS for API server")
	rootCmd.Flags().String("tls-cert", "", "Path to TLS certificate")
	rootCmd.Flags().String("tls-key", "", "Path to TLS key")
	rootCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().String("log-format", "json", "Log format (json, console)")
	rootCmd.Flags().Duration("shutdown-timeout", 30*time.Second, "Graceful shutdown timeout (e.g., 30s)")
	rootCmd.Flags().Duration("health-check-timeout", 5*time.Second, "Health check timeout (e.g., 5s)")
	rootCmd.Flags().Duration("health-cache-duration", 10*time.Second, "Health check cache duration (e.g., 10s)")

	// Olric configuration flags
	rootCmd.Flags().String("olric-host", store.DefaultBindAddr, "Olric bind host")
	rootCmd.Flags().Int("olric-port", store.DefaultBindPort, "Olric bind port")
	rootCmd.Flags().StringSlice("olric-join-addrs", []string{}, "Olric cluster join addresses")
	rootCmd.Flags().String("olric-replication-mode", store.DefaultReplicationMode, "Olric replication mode (sync/async)")
	rootCmd.Flags().Int("olric-replication-factor", store.DefaultReplicationFactor, "Olric replication factor")
	rootCmd.Flags().Int("olric-partition-count", int(store.DefaultPartitionCount), "Olric partition count")
	rootCmd.Flags().Int("olric-backup-count", store.DefaultBackupCount, "Olric backup count")
	rootCmd.Flags().String("olric-backup-mode", store.DefaultBackupMode, "Olric backup mode (sync/async)")
	rootCmd.Flags().Int("olric-member-count-quorum", store.DefaultMemberCountQuorum, "Olric member count quorum")
	rootCmd.Flags().Duration("olric-join-retry-interval", store.DefaultJoinRetryInterval, "Olric join retry interval")
	rootCmd.Flags().Int("olric-max-join-attempts", store.DefaultMaxJoinAttempts, "Olric max join attempts")
	rootCmd.Flags().String("olric-log-level", "", "Olric log level (DEBUG/INFO/WARN/ERROR, defaults to main log level)")
	rootCmd.Flags().Duration("olric-keep-alive-period", store.DefaultKeepAlivePeriod, "Olric keep alive period")
	rootCmd.Flags().Duration("olric-request-timeout", store.DefaultRequestTimeout, "Olric request timeout")
	rootCmd.Flags().String("olric-dmap-name", store.DefaultDMapName, "Olric DMap name")

	// Bind flags to viper
	_ = viper.BindPFlag("api.port", rootCmd.Flags().Lookup("api-port"))
	_ = viper.BindPFlag("api.host", rootCmd.Flags().Lookup("api-host"))
	_ = viper.BindPFlag("probe.port", rootCmd.Flags().Lookup("probe-port"))
	_ = viper.BindPFlag("probe.host", rootCmd.Flags().Lookup("probe-host"))
	_ = viper.BindPFlag("metrics.port", rootCmd.Flags().Lookup("metrics-port"))
	_ = viper.BindPFlag("metrics.host", rootCmd.Flags().Lookup("metrics-host"))
	_ = viper.BindPFlag("tls.enabled", rootCmd.Flags().Lookup("tls-enabled"))
	_ = viper.BindPFlag("tls.cert", rootCmd.Flags().Lookup("tls-cert"))
	_ = viper.BindPFlag("tls.key", rootCmd.Flags().Lookup("tls-key"))
	_ = viper.BindPFlag("log.level", rootCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("log.format", rootCmd.Flags().Lookup("log-format"))
	_ = viper.BindPFlag("shutdown.timeout", rootCmd.Flags().Lookup("shutdown-timeout"))
	_ = viper.BindPFlag("health.check_timeout", rootCmd.Flags().Lookup("health-check-timeout"))
	_ = viper.BindPFlag("health.cache_duration", rootCmd.Flags().Lookup("health-cache-duration"))
	_ = viper.BindPFlag("olric.host", rootCmd.Flags().Lookup("olric-host"))
	_ = viper.BindPFlag("olric.port", rootCmd.Flags().Lookup("olric-port"))
	_ = viper.BindPFlag("olric.join_addrs", rootCmd.Flags().Lookup("olric-join-addrs"))
	_ = viper.BindPFlag("olric.replication_mode", rootCmd.Flags().Lookup("olric-replication-mode"))
	_ = viper.BindPFlag("olric.replication_factor", rootCmd.Flags().Lookup("olric-replication-factor"))
	_ = viper.BindPFlag("olric.partition_count", rootCmd.Flags().Lookup("olric-partition-count"))
	_ = viper.BindPFlag("olric.backup_count", rootCmd.Flags().Lookup("olric-backup-count"))
	_ = viper.BindPFlag("olric.backup_mode", rootCmd.Flags().Lookup("olric-backup-mode"))
	_ = viper.BindPFlag("olric.member_count_quorum", rootCmd.Flags().Lookup("olric-member-count-quorum"))
	_ = viper.BindPFlag("olric.join_retry_interval", rootCmd.Flags().Lookup("olric-join-retry-interval"))
	_ = viper.BindPFlag("olric.max_join_attempts", rootCmd.Flags().Lookup("olric-max-join-attempts"))
	_ = viper.BindPFlag("olric.log_level", rootCmd.Flags().Lookup("olric-log-level"))
	_ = viper.BindPFlag("olric.keep_alive_period", rootCmd.Flags().Lookup("olric-keep-alive-period"))
	_ = viper.BindPFlag("olric.request_timeout", rootCmd.Flags().Lookup("olric-request-timeout"))
	_ = viper.BindPFlag("olric.dmap_name", rootCmd.Flags().Lookup("olric-dmap-name"))
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	log.Info("Starting deployment lock service",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
	)

	// Create server with build info
	buildInfo := map[string]string{
		"version": version,
		"commit":  commit,
		"date":    date,
	}
	srv, err := server.New(cfg, log, buildInfo)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	log.Info("Service started successfully")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Error during shutdown", zap.Error(err))
		return err
	}

	log.Info("Service stopped gracefully")
	return nil
}
