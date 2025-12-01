package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/provider"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

const (
	runnerLabelPrefix = "zeno.runner"
	labelRunnerID     = runnerLabelPrefix + ".id"
	labelRunnerName   = runnerLabelPrefix + ".name"
	labelManagedBy    = runnerLabelPrefix + ".managed-by"
)

type DockerProvider struct {
	client *client.Client
	config config.DockerConfig
	logger *slog.Logger
	mu     sync.RWMutex
}

// New creates a new Docker provider
func New(cfg config.DockerConfig, logger *slog.Logger) (*DockerProvider, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(cfg.Host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerProvider{
		client: cli,
		config: cfg,
		logger: logger.With("provider", "docker"),
	}, nil
}

func (p *DockerProvider) Name() string {
	return "docker"
}

func (p *DockerProvider) ListRunners(ctx context.Context) ([]*provider.Runner, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	containers, err := p.client.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var runners []*provider.Runner
	for _, c := range containers {
		if c.Labels[labelManagedBy] != "zeno" {
			continue
		}

		status := mapContainerState(c.State)
		runners = append(runners, &provider.Runner{
			ID:         c.Labels[labelRunnerID],
			Name:       c.Labels[labelRunnerName],
			Status:     status,
			Provider:   "docker",
			ProviderID: c.ID,
			CreatedAt:  time.Unix(c.Created, 0),
			Metadata: map[string]string{
				"container_id": c.ID,
				"image":        c.Image,
				"state":        c.State,
			},
		})
	}

	return runners, nil
}

func (p *DockerProvider) GetRunner(ctx context.Context, id string) (*provider.Runner, error) {
	runners, err := p.ListRunners(ctx)
	if err != nil {
		return nil, err
	}

	for _, r := range runners {
		if r.ID == id {
			return r, nil
		}
	}

	return nil, fmt.Errorf("runner %s not found", id)
}

func (p *DockerProvider) CreateRunner(ctx context.Context, req *provider.CreateRunnerRequest) (*provider.Runner, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	runnerID := uuid.New().String()
	containerName := fmt.Sprintf("zeno-runner-%s", runnerID[:8])

	p.logger.Info("creating runner", "id", runnerID, "name", req.Name)

	// Pull image if needed
	if p.config.PullPolicy == "always" || p.config.PullPolicy == "if-not-present" {
		if err := p.pullImage(ctx); err != nil {
			return nil, fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// Build environment variables
	env := p.buildEnv(req)

	// Build labels
	labels := p.buildLabels(runnerID, req)

	// Create container config
	containerConfig := &container.Config{
		Image:  p.config.Image,
		Env:    env,
		Labels: labels,
	}

	// Create host config
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(p.config.Network),
		Resources: container.Resources{
			NanoCPUs: int64(p.config.CPULimit * 1e9),
			Memory:   p.config.MemoryLimit,
		},
	}

	// Add volumes
	if len(p.config.Volumes) > 0 {
		hostConfig.Binds = p.config.Volumes
	}

	// Create container
	resp, err := p.client.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up container on start failure
		_ = p.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	p.logger.Info("runner created successfully",
		"id", runnerID,
		"container_id", resp.ID,
		"name", req.Name,
	)

	return &provider.Runner{
		ID:         runnerID,
		Name:       req.Name,
		Status:     provider.StatusProvisioning,
		Labels:     req.Labels,
		Provider:   "docker",
		ProviderID: resp.ID,
		CreatedAt:  time.Now(),
		Metadata: map[string]string{
			"container_id": resp.ID,
			"image":        p.config.Image,
		},
	}, nil
}

func (p *DockerProvider) RemoveRunner(ctx context.Context, id string, graceful bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	runner, err := p.GetRunner(ctx, id)
	if err != nil {
		return err
	}

	p.logger.Info("removing runner",
		"id", id,
		"container_id", runner.ProviderID,
		"graceful", graceful,
	)

	var timeout *int
	if graceful {
		t := 30
		timeout = &t
	}

	removeOpts := container.RemoveOptions{
		Force:         !graceful,
		RemoveVolumes: true,
	}

	if graceful {
		// Try graceful shutdown first
		if err := p.client.ContainerStop(ctx, runner.ProviderID, container.StopOptions{
			Timeout: timeout,
		}); err != nil {
			p.logger.Warn("graceful stop failed, forcing removal", "error", err)
			removeOpts.Force = true
		}
	}

	if err := p.client.ContainerRemove(ctx, runner.ProviderID, removeOpts); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	p.logger.Info("runner removed successfully", "id", id)
	return nil
}

func (p *DockerProvider) HealthCheck(ctx context.Context) error {
	_, err := p.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker health check failed: %w", err)
	}
	return nil
}

func (p *DockerProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func (p *DockerProvider) pullImage(ctx context.Context) error {
	p.logger.Info("pulling image", "image", p.config.Image)

	reader, err := p.client.ImagePull(ctx, p.config.Image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Consume the output to ensure pull completes
	_, err = io.Copy(io.Discard, reader)
	return err
}

func (p *DockerProvider) buildEnv(req *provider.CreateRunnerRequest) []string {
	env := []string{
		fmt.Sprintf("RUNNER_NAME=%s", req.Name),
		fmt.Sprintf("RUNNER_WORKDIR=%s", p.config.RunnerWorkDir),
	}

	if req.GitHubToken != "" {
		env = append(env, fmt.Sprintf("ACCESS_TOKEN=%s", req.GitHubToken))
	}

	if req.GitHubOrg != "" {
		env = append(env, fmt.Sprintf("RUNNER_SCOPE=org"))
		env = append(env, fmt.Sprintf("ORG_NAME=%s", req.GitHubOrg))
	} else if req.GitHubRepo != "" {
		env = append(env, fmt.Sprintf("RUNNER_SCOPE=repo"))
		env = append(env, fmt.Sprintf("REPO_URL=https://github.com/%s", req.GitHubRepo))
	}

	if len(req.Labels) > 0 {
		env = append(env, fmt.Sprintf("LABELS=%s", strings.Join(req.Labels, ",")))
	}

	return env
}

func (p *DockerProvider) buildLabels(runnerID string, req *provider.CreateRunnerRequest) map[string]string {
	labels := map[string]string{
		labelRunnerID:   runnerID,
		labelRunnerName: req.Name,
		labelManagedBy:  "zeno",
	}

	// Merge custom labels from config
	for k, v := range p.config.Labels {
		labels[k] = v
	}

	// Add request metadata
	for k, v := range req.Metadata {
		labels[runnerLabelPrefix+"."+k] = v
	}

	return labels
}

func mapContainerState(state string) provider.RunnerStatus {
	switch state {
	case "running":
		return provider.StatusRunning
	case "exited", "dead":
		return provider.StatusTerminated
	case "paused":
		return provider.StatusIdle
	case "restarting":
		return provider.StatusProvisioning
	case "removing":
		return provider.StatusTerminating
	case "created":
		return provider.StatusPending
	default:
		return provider.StatusFailed
	}
}
