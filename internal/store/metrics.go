package store

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// OlricMetrics holds Prometheus metrics for Olric.
type OlricMetrics struct {
	// Cluster metrics
	ClusterMembers    prometheus.Gauge
	ClusterPartitions prometheus.Gauge
	ClusterBackups    prometheus.Gauge
	ClusterCoordinator prometheus.Gauge

	// Storage metrics
	StorageKeys          prometheus.Gauge
	StorageMemoryBytes   prometheus.Gauge
	StorageAllocatedBytes prometheus.Gauge

	// Operation metrics
	OperationsTotal        *prometheus.CounterVec
	OperationDuration      *prometheus.HistogramVec
	OperationErrorsTotal   *prometheus.CounterVec

	// Replication metrics
	ReplicationLag       prometheus.Gauge
	BackupOperationsTotal *prometheus.CounterVec
}

// NewOlricMetrics creates a new OlricMetrics instance.
func NewOlricMetrics(namespace string, registry *prometheus.Registry) *OlricMetrics {
	m := &OlricMetrics{
		ClusterMembers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_cluster_members",
				Help:      "Number of cluster members",
			},
		),
		ClusterPartitions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_cluster_partitions",
				Help:      "Number of partitions in the cluster",
			},
		),
		ClusterBackups: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_cluster_backups",
				Help:      "Number of backup replicas",
			},
		),
		ClusterCoordinator: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_cluster_coordinator",
				Help:      "1 if this node is coordinator, 0 otherwise",
			},
		),
		StorageKeys: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_storage_keys_total",
				Help:      "Total number of keys stored",
			},
		),
		StorageMemoryBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_storage_memory_bytes",
				Help:      "Memory usage in bytes",
			},
		),
		StorageAllocatedBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_storage_allocated_memory_bytes",
				Help:      "Allocated memory in bytes",
			},
		),
		OperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "olric_operations_total",
				Help:      "Total number of Olric operations",
			},
			[]string{"operation", "status"},
		),
		OperationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "olric_operation_duration_seconds",
				Help:      "Olric operation duration in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"operation"},
		),
		OperationErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "olric_operation_errors_total",
				Help:      "Total number of Olric operation errors",
			},
			[]string{"operation", "error_type"},
		),
		ReplicationLag: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "olric_replication_lag_seconds",
				Help:      "Replication lag in seconds",
			},
		),
		BackupOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "olric_backup_operations_total",
				Help:      "Total number of backup operations",
			},
			[]string{"status"},
		),
	}

	// Register all metrics
	registry.MustRegister(
		m.ClusterMembers,
		m.ClusterPartitions,
		m.ClusterBackups,
		m.ClusterCoordinator,
		m.StorageKeys,
		m.StorageMemoryBytes,
		m.StorageAllocatedBytes,
		m.OperationsTotal,
		m.OperationDuration,
		m.OperationErrorsTotal,
		m.ReplicationLag,
		m.BackupOperationsTotal,
	)

	return m
}

// OlricMetricsCollector collects Olric metrics periodically.
type OlricMetricsCollector struct {
	logger   *zap.Logger
	store    Store
	metrics  *OlricMetrics
	interval time.Duration
	stopChan chan struct{}
	doneChan chan struct{}
	olricDB  interface{} // Will be *olric.Olric for coordinator check
}

// NewOlricMetricsCollector creates a new metrics collector.
func NewOlricMetricsCollector(
	logger *zap.Logger,
	store Store,
	metrics *OlricMetrics,
	interval time.Duration,
	olricDB interface{},
) *OlricMetricsCollector {
	return &OlricMetricsCollector{
		logger:   logger,
		store:    store,
		metrics:  metrics,
		interval: interval,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		olricDB:  olricDB,
	}
}

// Start begins collecting metrics.
func (c *OlricMetricsCollector) Start() {
	go c.run()
}

// Stop stops the metrics collector.
func (c *OlricMetricsCollector) Stop() {
	close(c.stopChan)
	<-c.doneChan
}

// run is the main loop for collecting metrics.
func (c *OlricMetricsCollector) run() {
	defer close(c.doneChan)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect initial metrics
	c.collect()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopChan:
			c.logger.Info("Stopping Olric metrics collector")
			return
		}
	}
}

// collect collects and updates all Olric metrics.
func (c *OlricMetricsCollector) collect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats, err := c.store.Stats(ctx)
	if err != nil {
		c.logger.Error("Failed to collect Olric stats", zap.Error(err))
		return
	}

	// Update cluster metrics
	c.metrics.ClusterMembers.Set(float64(stats.ClusterMembers))
	c.metrics.ClusterPartitions.Set(float64(stats.PartitionCount))
	c.metrics.ClusterBackups.Set(float64(stats.BackupCount))

	// Check if this node is coordinator
	// We can check the Coordinator field in members
	if olricStore, ok := c.store.(*OlricStore); ok {
		if olricStore.client != nil {
			members, err := olricStore.client.Members(ctx)
			if err == nil && len(members) > 0 {
				// Find if any member is marked as coordinator
				for _, member := range members {
					if member.Coordinator {
						c.metrics.ClusterCoordinator.Set(1)
						break
					}
				}
			}
		}
	}

	// Update storage metrics
	c.metrics.StorageKeys.Set(float64(stats.TotalKeys))
	c.metrics.StorageMemoryBytes.Set(float64(stats.MemoryUsage))
	// Note: StorageAllocatedBytes is set from runtime stats in Stats()

	c.logger.Debug("Collected Olric metrics",
		zap.Int("cluster_members", stats.ClusterMembers),
		zap.Int64("total_keys", stats.TotalKeys),
		zap.Int64("memory_bytes", stats.MemoryUsage),
	)
}

// RecordOperation records an operation metric.
func (m *OlricMetrics) RecordOperation(operation, status string, duration time.Duration) {
	m.OperationsTotal.WithLabelValues(operation, status).Inc()
	m.OperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordError records an operation error.
func (m *OlricMetrics) RecordError(operation, errorType string) {
	m.OperationErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// RecordBackupOperation records a backup operation.
func (m *OlricMetrics) RecordBackupOperation(status string) {
	m.BackupOperationsTotal.WithLabelValues(status).Inc()
}
