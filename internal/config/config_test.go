package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid minimal config",
			envVars: map[string]string{
				"ZENO_GITHUB_TOKEN":        "test-token",
				"ZENO_GITHUB_ORGANIZATION": "test-org",
			},
			wantErr: false,
		},
		{
			name: "missing github token",
			envVars: map[string]string{
				"ZENO_GITHUB_ORGANIZATION": "test-org",
			},
			wantErr:     true,
			errContains: "github.token is required",
		},
		{
			name: "missing org and repo",
			envVars: map[string]string{
				"ZENO_GITHUB_TOKEN": "test-token",
			},
			wantErr:     true,
			errContains: "either github.organization or github.repository must be set",
		},
		{
			name: "invalid provider type",
			envVars: map[string]string{
				"ZENO_GITHUB_TOKEN":        "test-token",
				"ZENO_GITHUB_ORGANIZATION": "test-org",
				"ZENO_PROVIDER_TYPE":       "invalid",
			},
			wantErr:     true,
			errContains: "provider.type must be either 'docker' or 'ec2'",
		},
		{
			name: "invalid scaling config",
			envVars: map[string]string{
				"ZENO_GITHUB_TOKEN":        "test-token",
				"ZENO_GITHUB_ORGANIZATION": "test-org",
				"ZENO_SCALING_MIN_RUNNERS": "10",
				"ZENO_SCALING_MAX_RUNNERS": "5",
			},
			wantErr:     true,
			errContains: "scaling.max_runners must be >= scaling.min_runners",
		},
		{
			name: "ec2 provider missing required fields",
			envVars: map[string]string{
				"ZENO_GITHUB_TOKEN":        "test-token",
				"ZENO_GITHUB_ORGANIZATION": "test-org",
				"ZENO_PROVIDER_TYPE":       "ec2",
			},
			wantErr:     true,
			errContains: "provider.aws.ami is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()

			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load("")
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Load() error = %v, want error containing %q", err, tt.errContains)
				}
			}

			if !tt.wantErr && cfg == nil {
				t.Error("Load() returned nil config without error")
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("ZENO_GITHUB_TOKEN", "test-token")
	os.Setenv("ZENO_GITHUB_ORGANIZATION", "test-org")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Scaling.MinRunners != 1 {
		t.Errorf("Scaling.MinRunners = %d, want 1", cfg.Scaling.MinRunners)
	}
	if cfg.Scaling.MaxRunners != 10 {
		t.Errorf("Scaling.MaxRunners = %d, want 10", cfg.Scaling.MaxRunners)
	}
	if cfg.Scaling.CheckInterval != 30*time.Second {
		t.Errorf("Scaling.CheckInterval = %v, want 30s", cfg.Scaling.CheckInterval)
	}
	if cfg.Provider.Type != "docker" {
		t.Errorf("Provider.Type = %s, want docker", cfg.Provider.Type)
	}
	if !cfg.Observability.EnableMetrics {
		t.Error("Observability.EnableMetrics should be true by default")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			cfg: &Config{
				GitHub: GitHubConfig{
					Token:        "token",
					Organization: "org",
				},
				Scaling: ScalingConfig{
					MinRunners:         1,
					MaxRunners:         10,
					ScaleUpThreshold:   5,
					ScaleDownThreshold: 0,
					CheckInterval:      30 * time.Second,
				},
				Provider: ProviderConfig{
					Type: "docker",
					Docker: DockerConfig{
						Image: "test-image",
					},
				},
				Server: ServerConfig{
					Port: 8080,
				},
			},
			wantErr: false,
		},
		{
			name: "leader election invalid config",
			cfg: &Config{
				GitHub: GitHubConfig{
					Token:        "token",
					Organization: "org",
				},
				Scaling: ScalingConfig{
					MinRunners:         1,
					MaxRunners:         10,
					ScaleUpThreshold:   5,
					ScaleDownThreshold: 0,
					CheckInterval:      30 * time.Second,
				},
				Provider: ProviderConfig{
					Type: "docker",
					Docker: DockerConfig{
						Image: "test-image",
					},
				},
				Server: ServerConfig{
					Port: 8080,
				},
				LeaderElection: LeaderElectionConfig{
					Enabled:       true,
					LeaseDuration: 10 * time.Second,
					RenewDeadline: 15 * time.Second,
				},
			},
			wantErr:     true,
			errContains: "renew_deadline must be < lease_duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}
