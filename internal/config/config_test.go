package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	// Reset viper state before each test
	defer viper.Reset()

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "default configuration",
			setup: func() {
				viper.Reset()
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.APIPort != 8080 {
					t.Errorf("APIPort = %d, want 8080", cfg.APIPort)
				}
				if cfg.ProbePort != 8081 {
					t.Errorf("ProbePort = %d, want 8081", cfg.ProbePort)
				}
				if cfg.MetricsPort != 9090 {
					t.Errorf("MetricsPort = %d, want 9090", cfg.MetricsPort)
				}
				if cfg.LogLevel != "info" {
					t.Errorf("LogLevel = %s, want info", cfg.LogLevel)
				}
				if cfg.LogFormat != "json" {
					t.Errorf("LogFormat = %s, want json", cfg.LogFormat)
				}
				if cfg.ShutdownTimeout != 30*time.Second {
					t.Errorf("ShutdownTimeout = %s, want 30s", cfg.ShutdownTimeout)
				}
			},
		},
		{
			name: "custom configuration via viper",
			setup: func() {
				viper.Reset()
				viper.Set("api.port", 9000)
				viper.Set("probe.port", 9001)
				viper.Set("metrics.port", 9002)
				viper.Set("log.level", "debug")
				viper.Set("log.format", "console")
				viper.Set("shutdown.timeout", "60s")
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.APIPort != 9000 {
					t.Errorf("APIPort = %d, want 9000", cfg.APIPort)
				}
				if cfg.ProbePort != 9001 {
					t.Errorf("ProbePort = %d, want 9001", cfg.ProbePort)
				}
				if cfg.MetricsPort != 9002 {
					t.Errorf("MetricsPort = %d, want 9002", cfg.MetricsPort)
				}
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
				}
				if cfg.LogFormat != "console" {
					t.Errorf("LogFormat = %s, want console", cfg.LogFormat)
				}
				if cfg.ShutdownTimeout != 60*time.Second {
					t.Errorf("ShutdownTimeout = %s, want 60s", cfg.ShutdownTimeout)
				}
			},
		},
		{
			name: "TLS configuration",
			setup: func() {
				viper.Reset()
				viper.Set("tls.enabled", true)
				viper.Set("tls.cert", "/path/to/cert.pem")
				viper.Set("tls.key", "/path/to/key.pem")
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if !cfg.TLSEnabled {
					t.Error("TLSEnabled = false, want true")
				}
				if cfg.TLSCert != "/path/to/cert.pem" {
					t.Errorf("TLSCert = %s, want /path/to/cert.pem", cfg.TLSCert)
				}
				if cfg.TLSKey != "/path/to/key.pem" {
					t.Errorf("TLSKey = %s, want /path/to/key.pem", cfg.TLSKey)
				}
			},
		},
		{
			name: "invalid shutdown timeout",
			setup: func() {
				viper.Reset()
				viper.Set("shutdown.timeout", "invalid")
			},
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid API port - too low",
			config: &Config{
				APIPort:         0,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid API port - too high",
			config: &Config{
				APIPort:         65536,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid probe port",
			config: &Config{
				APIPort:         8080,
				ProbePort:       -1,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid metrics port",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     70000,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "TLS enabled but no cert",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				TLSEnabled:      true,
				TLSCert:         "",
				TLSKey:          "/path/to/key",
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "TLS enabled but no key",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				TLSEnabled:      true,
				TLSCert:         "/path/to/cert",
				TLSKey:          "",
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "invalid",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid log format",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "invalid",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative shutdown timeout",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "info",
				LogFormat:       "json",
				ShutdownTimeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "all log levels are valid",
			config: &Config{
				APIPort:         8080,
				ProbePort:       8081,
				MetricsPort:     9090,
				LogLevel:        "debug",
				LogFormat:       "json",
				ShutdownTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Save current environment and restore at the end
	oldEnv := make(map[string]string)
	envVars := map[string]string{
		"LOCK_API_PORT":         "9000",
		"LOCK_PROBE_PORT":       "9001",
		"LOCK_METRICS_PORT":     "9002",
		"LOCK_LOG_LEVEL":        "debug",
		"LOCK_LOG_FORMAT":       "console",
		"LOCK_TLS_ENABLED":      "true",
		"LOCK_TLS_CERT":         "/test/cert.pem",
		"LOCK_TLS_KEY":          "/test/key.pem",
		"LOCK_SHUTDOWN_TIMEOUT": "45s",
	}

	for key := range envVars {
		oldEnv[key] = os.Getenv(key)
	}

	// Clean up at the end
	defer func() {
		for key, value := range oldEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
		viper.Reset()
	}()

	// Set environment variables
	for key, value := range envVars {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set env var %s: %v", key, err)
		}
	}

	// Reset viper to pick up environment variables
	viper.Reset()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIPort != 9000 {
		t.Errorf("APIPort = %d, want 9000", cfg.APIPort)
	}
	if cfg.ProbePort != 9001 {
		t.Errorf("ProbePort = %d, want 9001", cfg.ProbePort)
	}
	if cfg.MetricsPort != 9002 {
		t.Errorf("MetricsPort = %d, want 9002", cfg.MetricsPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "console" {
		t.Errorf("LogFormat = %s, want console", cfg.LogFormat)
	}
	if !cfg.TLSEnabled {
		t.Error("TLSEnabled = false, want true")
	}
	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 45s", cfg.ShutdownTimeout)
	}
}
