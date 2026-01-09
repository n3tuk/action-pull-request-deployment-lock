package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the service.
type Config struct {
	// API server settings
	APIPort int
	APIHost string

	// Probe server settings
	ProbePort int
	ProbeHost string

	// Metrics server settings
	MetricsPort int
	MetricsHost string

	// TLS settings
	TLSEnabled bool
	TLSCert    string
	TLSKey     string

	// Logging settings
	LogLevel  string
	LogFormat string

	// Graceful shutdown timeout
	ShutdownTimeout time.Duration

	// Health check settings
	HealthCheckTimeout       time.Duration
	HealthCheckCacheDuration time.Duration

	// Metrics settings
	MetricsNamespace string
}

// Load reads configuration from environment variables, config file, and flags.
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("api.port", 8080)
	viper.SetDefault("api.host", "0.0.0.0")
	viper.SetDefault("probe.port", 8081)
	viper.SetDefault("probe.host", "0.0.0.0")
	viper.SetDefault("metrics.port", 9090)
	viper.SetDefault("metrics.host", "0.0.0.0")
	viper.SetDefault("tls.enabled", false)
	viper.SetDefault("tls.cert", "")
	viper.SetDefault("tls.key", "")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("shutdown.timeout", "30s")
	viper.SetDefault("health.check_timeout", "5s")
	viper.SetDefault("health.cache_duration", "10s")

	// Enable environment variable support with automatic replacement
	viper.SetEnvPrefix("LOCK")
	viper.AutomaticEnv()
	// Replace . with _ in environment variable names (e.g., api.port -> LOCK_API_PORT)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Try to read config file if it exists
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/deployment-lock/")

	// Reading config file is optional
	_ = viper.ReadInConfig()

	// Parse configuration
	cfg := &Config{
		APIPort:          viper.GetInt("api.port"),
		APIHost:          viper.GetString("api.host"),
		ProbePort:        viper.GetInt("probe.port"),
		ProbeHost:        viper.GetString("probe.host"),
		MetricsPort:      viper.GetInt("metrics.port"),
		MetricsHost:      viper.GetString("metrics.host"),
		TLSEnabled:       viper.GetBool("tls.enabled"),
		TLSCert:          viper.GetString("tls.cert"),
		TLSKey:           viper.GetString("tls.key"),
		LogLevel:         viper.GetString("log.level"),
		LogFormat:        viper.GetString("log.format"),
		MetricsNamespace: "deployment_lock", // Fixed value, not configurable
	}

	// Parse shutdown timeout
	timeout, err := time.ParseDuration(viper.GetString("shutdown.timeout"))
	if err != nil {
		return nil, fmt.Errorf("invalid shutdown timeout: %w", err)
	}
	cfg.ShutdownTimeout = timeout

	// Parse health check timeout
	healthTimeout, err := time.ParseDuration(viper.GetString("health.check_timeout"))
	if err != nil {
		return nil, fmt.Errorf("invalid health check timeout: %w", err)
	}
	cfg.HealthCheckTimeout = healthTimeout

	// Parse health check cache duration
	cacheDuration, err := time.ParseDuration(viper.GetString("health.cache_duration"))
	if err != nil {
		return nil, fmt.Errorf("invalid health check cache duration: %w", err)
	}
	cfg.HealthCheckCacheDuration = cacheDuration

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.APIPort < 1 || c.APIPort > 65535 {
		return fmt.Errorf("invalid API port: %d", c.APIPort)
	}
	if c.ProbePort < 1 || c.ProbePort > 65535 {
		return fmt.Errorf("invalid probe port: %d", c.ProbePort)
	}
	if c.MetricsPort < 1 || c.MetricsPort > 65535 {
		return fmt.Errorf("invalid metrics port: %d", c.MetricsPort)
	}

	if c.TLSEnabled {
		if c.TLSCert == "" {
			return fmt.Errorf("TLS enabled but no certificate path provided")
		}
		if c.TLSKey == "" {
			return fmt.Errorf("TLS enabled but no key path provided")
		}
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	validLogFormats := map[string]bool{
		"json":    true,
		"console": true,
	}
	if !validLogFormats[c.LogFormat] {
		return fmt.Errorf("invalid log format: %s (must be json or console)", c.LogFormat)
	}

	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("invalid shutdown timeout: %s (must be positive)", c.ShutdownTimeout)
	}

	if c.HealthCheckTimeout <= 0 {
		return fmt.Errorf("invalid health check timeout: %s (must be positive)", c.HealthCheckTimeout)
	}

	if c.HealthCheckCacheDuration < 0 {
		return fmt.Errorf("invalid health check cache duration: %s (must be non-negative, zero disables caching)", c.HealthCheckCacheDuration)
	}

	if c.MetricsNamespace == "" {
		return fmt.Errorf("metrics namespace cannot be empty")
	}

	return nil
}
