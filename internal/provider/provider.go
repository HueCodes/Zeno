package provider

import (
	"context"
	"time"
)

// Runner represents a runner instance managed by a provider
type Runner struct {
	ID          string
	Name        string
	Status      RunnerStatus
	Labels      []string
	Provider    string
	ProviderID  string
	CreatedAt   time.Time
	LastSeen    time.Time
	Metadata    map[string]string
}

// RunnerStatus represents the state of a runner
type RunnerStatus string

const (
	StatusPending      RunnerStatus = "pending"
	StatusProvisioning RunnerStatus = "provisioning"
	StatusRunning      RunnerStatus = "running"
	StatusIdle         RunnerStatus = "idle"
	StatusBusy         RunnerStatus = "busy"
	StatusTerminating  RunnerStatus = "terminating"
	StatusTerminated   RunnerStatus = "terminated"
	StatusFailed       RunnerStatus = "failed"
)

// CreateRunnerRequest contains parameters for creating a new runner
type CreateRunnerRequest struct {
	Name           string
	Labels         []string
	GitHubToken    string
	GitHubOrg      string
	GitHubRepo     string
	RunnerVersion  string
	Metadata       map[string]string
}

// Provider defines the interface for runner providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// ListRunners returns all runners managed by this provider
	ListRunners(ctx context.Context) ([]*Runner, error)

	// GetRunner returns a specific runner by ID
	GetRunner(ctx context.Context, id string) (*Runner, error)

	// CreateRunner provisions a new runner
	CreateRunner(ctx context.Context, req *CreateRunnerRequest) (*Runner, error)

	// RemoveRunner terminates and removes a runner
	RemoveRunner(ctx context.Context, id string, graceful bool) error

	// HealthCheck performs a health check on the provider
	HealthCheck(ctx context.Context) error

	// Close releases any resources held by the provider
	Close() error
}

// ProviderFactory creates a provider instance based on configuration
type ProviderFactory func(config interface{}) (Provider, error)
