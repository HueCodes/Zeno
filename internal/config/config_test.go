package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid config with org",
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
				"GITHUB_ORG":   "test-org",
			},
			wantErr: false,
		},
		{
			name: "valid config with repo",
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
				"GITHUB_REPO":  "owner/repo",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			envVars: map[string]string{
				"GITHUB_ORG": "test-org",
			},
			wantErr: true,
		},
		{
			name: "missing org and repo",
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env
			os.Clearenv()

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg == nil {
				t.Error("Load() returned nil config")
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				GitHub: GitHubConfig{
					Token:        "token",
					Organization: "org",
				},
				Runner: RunnerConfig{
					MinRunners:         1,
					MaxRunners:         10,
					ScaleUpThreshold:   5,
					ScaleDownThreshold: 0,
					CheckInterval:      30 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid min > max",
			cfg: &Config{
				GitHub: GitHubConfig{
					Token:        "token",
					Organization: "org",
				},
				Runner: RunnerConfig{
					MinRunners: 10,
					MaxRunners: 5,
				},
			},
			wantErr: true,
		},
		{
			name: "negative min runners",
			cfg: &Config{
				GitHub: GitHubConfig{
					Token:        "token",
					Organization: "org",
				},
				Runner: RunnerConfig{
					MinRunners: -1,
					MaxRunners: 10,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("GITHUB_TOKEN", "test-token")
	os.Setenv("GITHUB_ORG", "test-org")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Runner.MinRunners != 1 {
		t.Errorf("expected MinRunners=1, got %d", cfg.Runner.MinRunners)
	}

	if cfg.Runner.MaxRunners != 10 {
		t.Errorf("expected MaxRunners=10, got %d", cfg.Runner.MaxRunners)
	}

	if cfg.Runner.ScaleUpThreshold != 5 {
		t.Errorf("expected ScaleUpThreshold=5, got %d", cfg.Runner.ScaleUpThreshold)
	}

	if cfg.Runner.CheckInterval != 30*time.Second {
		t.Errorf("expected CheckInterval=30s, got %v", cfg.Runner.CheckInterval)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel=info, got %s", cfg.LogLevel)
	}
}
