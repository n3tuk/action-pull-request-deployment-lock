package logger

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		format    string
		wantErr   bool
		checkFunc func(*testing.T, *zap.Logger)
	}{
		{
			name:    "json format with info level",
			level:   "info",
			format:  "json",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
		{
			name:    "json format with debug level",
			level:   "debug",
			format:  "json",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
		{
			name:    "json format with warn level",
			level:   "warn",
			format:  "json",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
		{
			name:    "json format with error level",
			level:   "error",
			format:  "json",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
		{
			name:    "console format with info level",
			level:   "info",
			format:  "console",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
		{
			name:    "invalid log level",
			level:   "invalid",
			format:  "json",
			wantErr: true,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger != nil {
					t.Error("expected nil logger for invalid level")
				}
			},
		},
		{
			name:    "uppercase log level",
			level:   "INFO",
			format:  "json",
			wantErr: false,
			checkFunc: func(t *testing.T, logger *zap.Logger) {
				if logger == nil {
					t.Error("logger is nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.level, tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, logger)
			}

			if logger != nil {
				_ = logger.Sync()
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	// Test that the logger can actually log messages
	logger, err := New("info", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// These should not panic
	logger.Info("test info message", zap.String("key", "value"))
	logger.Debug("test debug message") // Should not appear with info level
	logger.Warn("test warn message")
	logger.Error("test error message")
}

func TestLoggerLevelEnabled(t *testing.T) {
	tests := []struct {
		name          string
		configLevel   string
		testLevel     zapcore.Level
		shouldBeEnabled bool
	}{
		{
			name:          "debug level should enable debug",
			configLevel:   "debug",
			testLevel:     zapcore.DebugLevel,
			shouldBeEnabled: true,
		},
		{
			name:          "info level should not enable debug",
			configLevel:   "info",
			testLevel:     zapcore.DebugLevel,
			shouldBeEnabled: false,
		},
		{
			name:          "info level should enable info",
			configLevel:   "info",
			testLevel:     zapcore.InfoLevel,
			shouldBeEnabled: true,
		},
		{
			name:          "warn level should not enable info",
			configLevel:   "warn",
			testLevel:     zapcore.InfoLevel,
			shouldBeEnabled: false,
		},
		{
			name:          "error level should enable error",
			configLevel:   "error",
			testLevel:     zapcore.ErrorLevel,
			shouldBeEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.configLevel, "json")
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Sync()

			core := logger.Core()
			enabled := core.Enabled(tt.testLevel)

			if enabled != tt.shouldBeEnabled {
				t.Errorf("Level %s enabled = %v, want %v", tt.testLevel, enabled, tt.shouldBeEnabled)
			}
		})
	}
}
