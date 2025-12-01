package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	GitHub         GitHubConfig         `mapstructure:"github"`
	Scaling        ScalingConfig        `mapstructure:"scaling"`
	Provider       ProviderConfig       `mapstructure:"provider"`
	Observability  ObservabilityConfig  `mapstructure:"observability"`
	LeaderElection LeaderElectionConfig `mapstructure:"leader_election"`
	Store          StoreConfig          `mapstructure:"store"`
	DryRun         bool                 `mapstructure:"dry_run"`
	LogLevel       string               `mapstructure:"log_level"`
}

type ServerConfig struct {
	Address       string        `mapstructure:"address"`
	Port          int           `mapstructure:"port"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	APIKey        string        `mapstructure:"api_key"`
	EnableAuth    bool          `mapstructure:"enable_auth"`
	RateLimitRPS  int           `mapstructure:"rate_limit_rps"`
}

type GitHubConfig struct {
	Token                string        `mapstructure:"token"`
	Organization         string        `mapstructure:"organization"`
	Repository           string        `mapstructure:"repository"`
	RunnerLabels         []string      `mapstructure:"runner_labels"`
	RequestTimeout       time.Duration `mapstructure:"request_timeout"`
	MaxRetries           int           `mapstructure:"max_retries"`
	RetryBackoffBase     time.Duration `mapstructure:"retry_backoff_base"`
	RetryBackoffMax      time.Duration `mapstructure:"retry_backoff_max"`
	CacheTTL             time.Duration `mapstructure:"cache_ttl"`
	RateLimitBuffer      int           `mapstructure:"rate_limit_buffer"`
}

type ScalingConfig struct {
	MinRunners              int           `mapstructure:"min_runners"`
	MaxRunners              int           `mapstructure:"max_runners"`
	ScaleUpThreshold        int           `mapstructure:"scale_up_threshold"`
	ScaleDownThreshold      int           `mapstructure:"scale_down_threshold"`
	ScaleUpHysteresis       int           `mapstructure:"scale_up_hysteresis"`
	ScaleDownHysteresis     int           `mapstructure:"scale_down_hysteresis"`
	CheckInterval           time.Duration `mapstructure:"check_interval"`
	CooldownPeriod          time.Duration `mapstructure:"cooldown_period"`
	EnablePredictiveScaling bool          `mapstructure:"enable_predictive_scaling"`
	PredictionWindow        time.Duration `mapstructure:"prediction_window"`
	GracefulTermination     bool          `mapstructure:"graceful_termination"`
	TerminationTimeout      time.Duration `mapstructure:"termination_timeout"`
}

type ProviderConfig struct {
	Type   string        `mapstructure:"type"`
	Docker DockerConfig  `mapstructure:"docker"`
	AWS    AWSConfig     `mapstructure:"aws"`
}

type DockerConfig struct {
	Host               string            `mapstructure:"host"`
	Image              string            `mapstructure:"image"`
	RunnerWorkDir      string            `mapstructure:"runner_work_dir"`
	Network            string            `mapstructure:"network"`
	CPULimit           float64           `mapstructure:"cpu_limit"`
	MemoryLimit        int64             `mapstructure:"memory_limit"`
	Labels             map[string]string `mapstructure:"labels"`
	Volumes            []string          `mapstructure:"volumes"`
	RegistryAuth       string            `mapstructure:"registry_auth"`
	PullPolicy         string            `mapstructure:"pull_policy"`
}

type AWSConfig struct {
	Region                string            `mapstructure:"region"`
	InstanceType          string            `mapstructure:"instance_type"`
	AMI                   string            `mapstructure:"ami"`
	SubnetID              string            `mapstructure:"subnet_id"`
	SecurityGroupIDs      []string          `mapstructure:"security_group_ids"`
	KeyName               string            `mapstructure:"key_name"`
	IAMInstanceProfile    string            `mapstructure:"iam_instance_profile"`
	UseSpot               bool              `mapstructure:"use_spot"`
	SpotMaxPrice          string            `mapstructure:"spot_max_price"`
	Tags                  map[string]string `mapstructure:"tags"`
	UserDataScript        string            `mapstructure:"user_data_script"`
	VolumeSize            int32             `mapstructure:"volume_size"`
	VolumeType            string            `mapstructure:"volume_type"`
}

type ObservabilityConfig struct {
	EnableMetrics     bool   `mapstructure:"enable_metrics"`
	MetricsPath       string `mapstructure:"metrics_path"`
	EnableTracing     bool   `mapstructure:"enable_tracing"`
	TracingEndpoint   string `mapstructure:"tracing_endpoint"`
	HealthCheckPath   string `mapstructure:"health_check_path"`
	ReadinessPath     string `mapstructure:"readiness_path"`
}

type LeaderElectionConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	LockFilePath   string        `mapstructure:"lock_file_path"`
	LeaseDuration  time.Duration `mapstructure:"lease_duration"`
	RenewDeadline  time.Duration `mapstructure:"renew_deadline"`
	RetryPeriod    time.Duration `mapstructure:"retry_period"`
}

type StoreConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Type      string `mapstructure:"type"`
	Path      string `mapstructure:"path"`
	MaxEvents int    `mapstructure:"max_events"`
}

// Load reads configuration from environment variables and optional config file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Environment variables
	v.SetEnvPrefix("ZENO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file (optional)
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.address", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.enable_auth", false)
	v.SetDefault("server.rate_limit_rps", 100)

	// GitHub defaults
	v.SetDefault("github.request_timeout", 30*time.Second)
	v.SetDefault("github.max_retries", 3)
	v.SetDefault("github.retry_backoff_base", 1*time.Second)
	v.SetDefault("github.retry_backoff_max", 30*time.Second)
	v.SetDefault("github.cache_ttl", 30*time.Second)
	v.SetDefault("github.rate_limit_buffer", 100)
	v.SetDefault("github.runner_labels", []string{})

	// Scaling defaults
	v.SetDefault("scaling.min_runners", 1)
	v.SetDefault("scaling.max_runners", 10)
	v.SetDefault("scaling.scale_up_threshold", 5)
	v.SetDefault("scaling.scale_down_threshold", 0)
	v.SetDefault("scaling.scale_up_hysteresis", 2)
	v.SetDefault("scaling.scale_down_hysteresis", 2)
	v.SetDefault("scaling.check_interval", 30*time.Second)
	v.SetDefault("scaling.cooldown_period", 60*time.Second)
	v.SetDefault("scaling.enable_predictive_scaling", false)
	v.SetDefault("scaling.prediction_window", 5*time.Minute)
	v.SetDefault("scaling.graceful_termination", true)
	v.SetDefault("scaling.termination_timeout", 60*time.Second)

	// Provider defaults
	v.SetDefault("provider.type", "docker")
	v.SetDefault("provider.docker.host", "unix:///var/run/docker.sock")
	v.SetDefault("provider.docker.image", "myoung34/github-runner:latest")
	v.SetDefault("provider.docker.runner_work_dir", "/runner/_work")
	v.SetDefault("provider.docker.network", "bridge")
	v.SetDefault("provider.docker.cpu_limit", 1.0)
	v.SetDefault("provider.docker.memory_limit", 2147483648) // 2GB
	v.SetDefault("provider.docker.pull_policy", "always")
	v.SetDefault("provider.aws.region", "us-east-1")
	v.SetDefault("provider.aws.instance_type", "t3.medium")
	v.SetDefault("provider.aws.use_spot", true)
	v.SetDefault("provider.aws.volume_size", 30)
	v.SetDefault("provider.aws.volume_type", "gp3")

	// Observability defaults
	v.SetDefault("observability.enable_metrics", true)
	v.SetDefault("observability.metrics_path", "/metrics")
	v.SetDefault("observability.enable_tracing", false)
	v.SetDefault("observability.health_check_path", "/health")
	v.SetDefault("observability.readiness_path", "/ready")

	// Leader election defaults
	v.SetDefault("leader_election.enabled", false)
	v.SetDefault("leader_election.lock_file_path", "/tmp/zeno-leader.lock")
	v.SetDefault("leader_election.lease_duration", 15*time.Second)
	v.SetDefault("leader_election.renew_deadline", 10*time.Second)
	v.SetDefault("leader_election.retry_period", 2*time.Second)

	// Store defaults
	v.SetDefault("store.enabled", false)
	v.SetDefault("store.type", "file")
	v.SetDefault("store.path", "/tmp/zeno-events.json")
	v.SetDefault("store.max_events", 1000)

	// General defaults
	v.SetDefault("dry_run", false)
	v.SetDefault("log_level", "info")
}

func (c *Config) Validate() error {
	// GitHub validation
	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	if c.GitHub.Organization == "" && c.GitHub.Repository == "" {
		return fmt.Errorf("either github.organization or github.repository must be set")
	}
	if c.GitHub.MaxRetries < 0 {
		return fmt.Errorf("github.max_retries must be >= 0")
	}
	if c.GitHub.CacheTTL < 0 {
		return fmt.Errorf("github.cache_ttl must be >= 0")
	}

	// Scaling validation
	if c.Scaling.MinRunners < 0 {
		return fmt.Errorf("scaling.min_runners must be >= 0")
	}
	if c.Scaling.MaxRunners < c.Scaling.MinRunners {
		return fmt.Errorf("scaling.max_runners must be >= scaling.min_runners")
	}
	if c.Scaling.ScaleDownThreshold < 0 {
		return fmt.Errorf("scaling.scale_down_threshold must be >= 0")
	}
	if c.Scaling.ScaleUpThreshold <= c.Scaling.ScaleDownThreshold {
		return fmt.Errorf("scaling.scale_up_threshold must be > scaling.scale_down_threshold")
	}
	if c.Scaling.CheckInterval <= 0 {
		return fmt.Errorf("scaling.check_interval must be > 0")
	}
	if c.Scaling.ScaleUpHysteresis < 0 {
		return fmt.Errorf("scaling.scale_up_hysteresis must be >= 0")
	}
	if c.Scaling.ScaleDownHysteresis < 0 {
		return fmt.Errorf("scaling.scale_down_hysteresis must be >= 0")
	}

	// Provider validation
	if c.Provider.Type != "docker" && c.Provider.Type != "ec2" {
		return fmt.Errorf("provider.type must be either 'docker' or 'ec2'")
	}

	if c.Provider.Type == "docker" {
		if c.Provider.Docker.Image == "" {
			return fmt.Errorf("provider.docker.image is required when using docker provider")
		}
	}

	if c.Provider.Type == "ec2" {
		if c.Provider.AWS.Region == "" {
			return fmt.Errorf("provider.aws.region is required when using ec2 provider")
		}
		if c.Provider.AWS.AMI == "" {
			return fmt.Errorf("provider.aws.ami is required when using ec2 provider")
		}
		if c.Provider.AWS.SubnetID == "" {
			return fmt.Errorf("provider.aws.subnet_id is required when using ec2 provider")
		}
		if len(c.Provider.AWS.SecurityGroupIDs) == 0 {
			return fmt.Errorf("provider.aws.security_group_ids is required when using ec2 provider")
		}
	}

	// Server validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if c.Server.EnableAuth && c.Server.APIKey == "" {
		return fmt.Errorf("server.api_key is required when server.enable_auth is true")
	}

	// Leader election validation
	if c.LeaderElection.Enabled {
		if c.LeaderElection.LockFilePath == "" {
			return fmt.Errorf("leader_election.lock_file_path is required when enabled")
		}
		if c.LeaderElection.LeaseDuration <= 0 {
			return fmt.Errorf("leader_election.lease_duration must be > 0")
		}
		if c.LeaderElection.RenewDeadline <= 0 {
			return fmt.Errorf("leader_election.renew_deadline must be > 0")
		}
		if c.LeaderElection.RenewDeadline >= c.LeaderElection.LeaseDuration {
			return fmt.Errorf("leader_election.renew_deadline must be < lease_duration")
		}
	}

	return nil
}
