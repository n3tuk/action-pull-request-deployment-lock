package store

import (
	"testing"
	"time"
)

func TestNewDefaultOlricConfig(t *testing.T) {
	cfg := NewDefaultOlricConfig()

	if cfg.BindAddr != "0.0.0.0" {
		t.Errorf("BindAddr = %s, want 0.0.0.0", cfg.BindAddr)
	}

	if cfg.BindPort != 3320 {
		t.Errorf("BindPort = %d, want 3320", cfg.BindPort)
	}

	if cfg.ReplicationMode != "async" {
		t.Errorf("ReplicationMode = %s, want async", cfg.ReplicationMode)
	}

	if cfg.ReplicationFactor != 1 {
		t.Errorf("ReplicationFactor = %d, want 1", cfg.ReplicationFactor)
	}

	if cfg.PartitionCount != 271 {
		t.Errorf("PartitionCount = %d, want 271", cfg.PartitionCount)
	}

	if cfg.BackupCount != 1 {
		t.Errorf("BackupCount = %d, want 1", cfg.BackupCount)
	}

	if cfg.MemberCountQuorum != 1 {
		t.Errorf("MemberCountQuorum = %d, want 1", cfg.MemberCountQuorum)
	}

	if cfg.LogLevel != "WARN" {
		t.Errorf("LogLevel = %s, want WARN", cfg.LogLevel)
	}

	if cfg.DMapName != "deployment-locks" {
		t.Errorf("DMapName = %s, want deployment-locks", cfg.DMapName)
	}
}

func TestOlricConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *OlricConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default config",
			cfg:     NewDefaultOlricConfig(),
			wantErr: false,
		},
		{
			name: "empty bind address",
			cfg: &OlricConfig{
				BindAddr:          "",
				BindPort:          3320,
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "bind address cannot be empty",
		},
		{
			name: "invalid port - too low",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          0,
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "bind port must be between",
		},
		{
			name: "invalid port - too high",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          65536,
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "bind port must be between",
		},
		{
			name: "invalid replication mode",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				ReplicationMode:   "invalid",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "replication mode must be",
		},
		{
			name: "invalid replication factor",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				ReplicationMode:   "async",
				ReplicationFactor: 0,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "replication factor must be at least 1",
		},
		{
			name: "invalid log level",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "INVALID",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "multi-node config with low replication factor",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				JoinAddrs:         []string{"node1:3320", "node2:3320"},
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 1,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "replication factor should be at least 2 in multi-node mode",
		},
		{
			name: "quorum greater than 1 requires join addresses",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				JoinAddrs:         []string{}, // No join addresses
				ReplicationMode:   "async",
				ReplicationFactor: 1,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 2, // Quorum > 1 but no join addresses
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: true,
			errMsg:  "member count quorum is 2 but no join addresses provided",
		},
		{
			name: "valid multi-node config",
			cfg: &OlricConfig{
				BindAddr:          "0.0.0.0",
				BindPort:          3320,
				JoinAddrs:         []string{"node1:3320", "node2:3320"},
				ReplicationMode:   "async",
				ReplicationFactor: 2,
				PartitionCount:    271,
				BackupCount:       1,
				BackupMode:        "async",
				MemberCountQuorum: 2,
				JoinRetryInterval: 1 * time.Second,
				MaxJoinAttempts:   30,
				LogLevel:          "WARN",
				KeepAlivePeriod:   30 * time.Second,
				RequestTimeout:    5 * time.Second,
				DMapName:          "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error()[:len(tt.errMsg)] != tt.errMsg {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestOlricConfig_IsSingleNode(t *testing.T) {
	tests := []struct {
		name string
		cfg  *OlricConfig
		want bool
	}{
		{
			name: "single node",
			cfg: &OlricConfig{
				JoinAddrs: []string{},
			},
			want: true,
		},
		{
			name: "multi node",
			cfg: &OlricConfig{
				JoinAddrs: []string{"node1:3320"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsSingleNode(); got != tt.want {
				t.Errorf("IsSingleNode() = %v, want %v", got, tt.want)
			}
		})
	}
}
