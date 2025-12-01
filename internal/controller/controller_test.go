package controller

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/github"
	"Zeno/internal/metrics"
	"Zeno/internal/provider"
	"Zeno/internal/store"

	"github.com/prometheus/client_golang/prometheus"
)

// Mock provider for testing
type mockProvider struct {
	runners []*provider.Runner
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) ListRunners(ctx context.Context) ([]*provider.Runner, error) {
	return m.runners, nil
}

func (m *mockProvider) GetRunner(ctx context.Context, id string) (*provider.Runner, error) {
	for _, r := range m.runners {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockProvider) CreateRunner(ctx context.Context, req *provider.CreateRunnerRequest) (*provider.Runner, error) {
	runner := &provider.Runner{
		ID:         "test-" + time.Now().Format("20060102150405"),
		Name:       req.Name,
		Status:     provider.StatusRunning,
		Provider:   "mock",
		CreatedAt:  time.Now(),
	}
	m.runners = append(m.runners, runner)
	return runner, nil
}

func (m *mockProvider) RemoveRunner(ctx context.Context, id string, graceful bool) error {
	for i, r := range m.runners {
		if r.ID == id {
			m.runners = append(m.runners[:i], m.runners[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockProvider) Close() error {
	return nil
}

// Mock GitHub client for testing
type mockGitHubClient struct {
	queueDepth int
}

func (m *mockGitHubClient) GetQueuedWorkflowJobs(ctx context.Context) (int, error) {
	return m.queueDepth, nil
}

func (m *mockGitHubClient) GetRateLimitInfo() github.RateLimitInfo {
	return github.RateLimitInfo{
		Remaining: 5000,
		Reset:     time.Now().Add(time.Hour),
	}
}

func TestMakeScalingDecision(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := prometheus.NewRegistry()
	met := metrics.NewMetrics(registry)

	tests := []struct {
		name           string
		config         *config.ScalingConfig
		queueDepth     int
		currentCount   int
		wantAction     ScaleAction
		wantDesired    int
	}{
		{
			name: "scale up when queue exceeds threshold",
			config: &config.ScalingConfig{
				MinRunners:          1,
				MaxRunners:          10,
				ScaleUpThreshold:    5,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   1,
				ScaleDownHysteresis: 1,
				CooldownPeriod:      0,
			},
			queueDepth:   8,
			currentCount: 2,
			wantAction:   ScaleActionUp,
			wantDesired:  8,
		},
		{
			name: "scale down when queue below threshold",
			config: &config.ScalingConfig{
				MinRunners:          1,
				MaxRunners:          10,
				ScaleUpThreshold:    5,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   1,
				ScaleDownHysteresis: 1,
				CooldownPeriod:      0,
			},
			queueDepth:   0,
			currentCount: 5,
			wantAction:   ScaleActionDown,
			wantDesired:  1,
		},
		{
			name: "no action when in normal range",
			config: &config.ScalingConfig{
				MinRunners:          1,
				MaxRunners:          10,
				ScaleUpThreshold:    5,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   1,
				ScaleDownHysteresis: 1,
				CooldownPeriod:      0,
			},
			queueDepth:   3,
			currentCount: 3,
			wantAction:   ScaleActionNone,
			wantDesired:  3,
		},
		{
			name: "respect max runners limit",
			config: &config.ScalingConfig{
				MinRunners:          1,
				MaxRunners:          5,
				ScaleUpThreshold:    3,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   1,
				ScaleDownHysteresis: 1,
				CooldownPeriod:      0,
			},
			queueDepth:   100,
			currentCount: 2,
			wantAction:   ScaleActionUp,
			wantDesired:  5, // capped at max
		},
		{
			name: "respect min runners limit",
			config: &config.ScalingConfig{
				MinRunners:          3,
				MaxRunners:          10,
				ScaleUpThreshold:    5,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   1,
				ScaleDownHysteresis: 1,
				CooldownPeriod:      0,
			},
			queueDepth:   0,
			currentCount: 5,
			wantAction:   ScaleActionDown,
			wantDesired:  3, // capped at min
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &Controller{
				cfg: &config.Config{
					Scaling: *tt.config,
				},
				ghClient: &mockGitHubClient{queueDepth: tt.queueDepth},
				provider: &mockProvider{},
				metrics:  met,
				logger:   logger,
			}

			decision := ctrl.makeScalingDecision(tt.queueDepth, tt.currentCount)

			if decision.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v", decision.Action, tt.wantAction)
			}
			if decision.DesiredCount != tt.wantDesired {
				t.Errorf("DesiredCount = %d, want %d", decision.DesiredCount, tt.wantDesired)
			}
		})
	}
}

func TestScaleUpHysteresis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := prometheus.NewRegistry()
	met := metrics.NewMetrics(registry)

	ctrl := &Controller{
		cfg: &config.Config{
			Scaling: config.ScalingConfig{
				MinRunners:          1,
				MaxRunners:          10,
				ScaleUpThreshold:    5,
				ScaleDownThreshold:  0,
				ScaleUpHysteresis:   3,
				ScaleDownHysteresis: 2,
				CooldownPeriod:      0,
			},
		},
		ghClient: &mockGitHubClient{queueDepth: 7},
		provider: &mockProvider{},
		metrics:  met,
		logger:   logger,
	}

	// First check should not trigger scale up
	decision1 := ctrl.makeScalingDecision(7, 2)
	if decision1.Action != ScaleActionNone {
		t.Errorf("First check: Action = %v, want %v", decision1.Action, ScaleActionNone)
	}
	if !decision1.HysteresisHit {
		t.Error("First check: HysteresisHit should be true")
	}

	// Second check should not trigger scale up
	decision2 := ctrl.makeScalingDecision(7, 2)
	if decision2.Action != ScaleActionNone {
		t.Errorf("Second check: Action = %v, want %v", decision2.Action, ScaleActionNone)
	}

	// Third check should trigger scale up
	decision3 := ctrl.makeScalingDecision(7, 2)
	if decision3.Action != ScaleActionUp {
		t.Errorf("Third check: Action = %v, want %v", decision3.Action, ScaleActionUp)
	}
}

func TestPredictQueueGrowth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := prometheus.NewRegistry()
	met := metrics.NewMetrics(registry)

	ctrl := &Controller{
		cfg: &config.Config{
			Scaling: config.ScalingConfig{
				EnablePredictiveScaling: true,
			},
		},
		ghClient:     &mockGitHubClient{},
		provider:     &mockProvider{},
		metrics:      met,
		logger:       logger,
		queueHistory: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}

	predicted := ctrl.predictQueueGrowth()

	// With steadily growing queue (1->10), prediction should be positive
	if predicted <= 10 {
		t.Errorf("predictQueueGrowth() = %d, want > 10 for growing queue", predicted)
	}
}

func TestCooldownPeriod(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ctrl := &Controller{
		cfg: &config.Config{
			Scaling: config.ScalingConfig{
				CooldownPeriod: 5 * time.Minute,
			},
		},
		logger:          logger,
		lastScaleUpTime: time.Now().Add(-3 * time.Minute),
	}

	// Should be in cooldown
	if !ctrl.inCooldownPeriod() {
		t.Error("inCooldownPeriod() = false, want true (recent scale up)")
	}

	// Set last scale up to past cooldown period
	ctrl.lastScaleUpTime = time.Now().Add(-10 * time.Minute)

	// Should not be in cooldown
	if ctrl.inCooldownPeriod() {
		t.Error("inCooldownPeriod() = true, want false (cooldown expired)")
	}
}
