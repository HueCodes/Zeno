package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/github"
	"Zeno/internal/metrics"
	"Zeno/internal/provider"
	"Zeno/internal/store"
)

type Controller struct {
	cfg      *config.Config
	ghClient *github.Client
	provider provider.Provider
	store    *store.Store
	metrics  *metrics.Metrics
	logger   *slog.Logger

	// Scaling state
	lastScaleUpTime   time.Time
	lastScaleDownTime time.Time
	scaleUpCounter    int
	scaleDownCounter  int
	queueHistory      []int

	mu sync.RWMutex
}

type ScaleDecision struct {
	Action        ScaleAction
	Reason        string
	CurrentCount  int
	DesiredCount  int
	QueueDepth    int
	HysteresisHit bool
}

type ScaleAction string

const (
	ScaleActionNone ScaleAction = "none"
	ScaleActionUp   ScaleAction = "up"
	ScaleActionDown ScaleAction = "down"
)

// New creates a new controller instance
func New(
	cfg *config.Config,
	ghClient *github.Client,
	prov provider.Provider,
	st *store.Store,
	met *metrics.Metrics,
	logger *slog.Logger,
) *Controller {
	return &Controller{
		cfg:          cfg,
		ghClient:     ghClient,
		provider:     prov,
		store:        st,
		metrics:      met,
		logger:       logger.With("component", "controller"),
		queueHistory: make([]int, 0, 100),
	}
}

// Run starts the controller reconciliation loop
func (c *Controller) Run(ctx context.Context) error {
	c.logger.Info("controller starting",
		"check_interval", c.cfg.Scaling.CheckInterval,
		"min_runners", c.cfg.Scaling.MinRunners,
		"max_runners", c.cfg.Scaling.MaxRunners,
	)

	// Initial reconcile
	if err := c.reconcile(ctx); err != nil {
		c.logger.Error("initial reconcile failed", "error", err)
	}

	ticker := time.NewTicker(c.cfg.Scaling.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("controller stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := c.reconcile(ctx); err != nil {
				c.logger.Error("reconcile failed", "error", err)
				c.metrics.ReconcileErrors.WithLabelValues("reconcile_error").Inc()
			}
		}
	}
}

func (c *Controller) reconcile(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		c.metrics.ReconcileDuration.WithLabelValues("success").Observe(duration.Seconds())
	}()

	c.logger.Debug("starting reconciliation")

	// Get queue depth
	queueDepth, err := c.ghClient.GetQueuedWorkflowJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get queue depth: %w", err)
	}

	c.metrics.QueueDepth.Set(float64(queueDepth))
	c.metrics.QueueDepthSamples.Observe(float64(queueDepth))

	// Update queue history for predictive scaling
	c.updateQueueHistory(queueDepth)

	// Get current runners
	runners, err := c.provider.ListRunners(ctx)
	if err != nil {
		return fmt.Errorf("failed to list runners: %w", err)
	}

	currentCount := len(runners)
	c.metrics.RunnersCurrent.Set(float64(currentCount))

	// Update runner status metrics
	c.updateRunnerStatusMetrics(runners)

	// Make scaling decision
	decision := c.makeScalingDecision(queueDepth, currentCount)

	c.logger.Info("scaling decision",
		"action", decision.Action,
		"reason", decision.Reason,
		"current", decision.CurrentCount,
		"desired", decision.DesiredCount,
		"queue_depth", decision.QueueDepth,
		"hysteresis_hit", decision.HysteresisHit,
	)

	c.metrics.RunnersDesired.Set(float64(decision.DesiredCount))

	// Execute scaling action
	if err := c.executeScaling(ctx, decision); err != nil {
		return fmt.Errorf("failed to execute scaling: %w", err)
	}

	// Update rate limit metrics
	rateLimitInfo := c.ghClient.GetRateLimitInfo()
	c.metrics.GitHubAPIRateLimit.Set(float64(rateLimitInfo.Remaining))
	if !rateLimitInfo.Reset.IsZero() {
		c.metrics.GitHubAPIRateLimitReset.Set(float64(rateLimitInfo.Reset.Unix()))
	}

	c.metrics.ReconcileTotal.WithLabelValues("success").Inc()
	return nil
}

func (c *Controller) makeScalingDecision(queueDepth, currentCount int) ScaleDecision {
	decision := ScaleDecision{
		Action:       ScaleActionNone,
		CurrentCount: currentCount,
		DesiredCount: currentCount,
		QueueDepth:   queueDepth,
	}

	// Check if in cooldown period
	if c.inCooldownPeriod() {
		decision.Reason = "in_cooldown_period"
		return decision
	}

	// Predictive scaling
	if c.cfg.Scaling.EnablePredictiveScaling {
		predictedQueue := c.predictQueueGrowth()
		if predictedQueue > queueDepth {
			c.logger.Debug("predictive scaling",
				"current_queue", queueDepth,
				"predicted_queue", predictedQueue,
			)
			queueDepth = predictedQueue
		}
	}

	// Determine desired count based on queue depth
	desiredCount := currentCount

	// Scale up logic
	if queueDepth >= c.cfg.Scaling.ScaleUpThreshold {
		// Simple strategy: one runner per queued job, up to max
		desiredCount = min(queueDepth, c.cfg.Scaling.MaxRunners)
		desiredCount = max(desiredCount, c.cfg.Scaling.MinRunners)

		if desiredCount > currentCount {
			// Check hysteresis
			c.mu.Lock()
			c.scaleUpCounter++
			if c.scaleUpCounter >= c.cfg.Scaling.ScaleUpHysteresis {
				decision.Action = ScaleActionUp
				decision.DesiredCount = desiredCount
				decision.Reason = "queue_above_threshold"
				c.scaleUpCounter = 0
				c.scaleDownCounter = 0
			} else {
				decision.HysteresisHit = true
				decision.Reason = fmt.Sprintf("hysteresis_check_%d_of_%d",
					c.scaleUpCounter, c.cfg.Scaling.ScaleUpHysteresis)
			}
			c.mu.Unlock()
		}
	} else if queueDepth <= c.cfg.Scaling.ScaleDownThreshold {
		// Scale down logic
		desiredCount = max(queueDepth, c.cfg.Scaling.MinRunners)

		if desiredCount < currentCount {
			// Check hysteresis
			c.mu.Lock()
			c.scaleDownCounter++
			if c.scaleDownCounter >= c.cfg.Scaling.ScaleDownHysteresis {
				decision.Action = ScaleActionDown
				decision.DesiredCount = desiredCount
				decision.Reason = "queue_below_threshold"
				c.scaleDownCounter = 0
				c.scaleUpCounter = 0
			} else {
				decision.HysteresisHit = true
				decision.Reason = fmt.Sprintf("hysteresis_check_%d_of_%d",
					c.scaleDownCounter, c.cfg.Scaling.ScaleDownHysteresis)
			}
			c.mu.Unlock()
		}
	} else {
		// Reset counters if in normal range
		c.mu.Lock()
		c.scaleUpCounter = 0
		c.scaleDownCounter = 0
		c.mu.Unlock()
		decision.Reason = "queue_in_normal_range"
	}

	return decision
}

func (c *Controller) executeScaling(ctx context.Context, decision ScaleDecision) error {
	if decision.Action == ScaleActionNone {
		return nil
	}

	if c.cfg.DryRun {
		c.logger.Info("dry-run mode: would execute scaling",
			"action", decision.Action,
			"from", decision.CurrentCount,
			"to", decision.DesiredCount,
		)
		return nil
	}

	switch decision.Action {
	case ScaleActionUp:
		return c.scaleUp(ctx, decision)
	case ScaleActionDown:
		return c.scaleDown(ctx, decision)
	}

	return nil
}

func (c *Controller) scaleUp(ctx context.Context, decision ScaleDecision) error {
	startTime := time.Now()
	defer func() {
		c.metrics.ScaleUpDuration.Observe(time.Since(startTime).Seconds())
	}()

	count := decision.DesiredCount - decision.CurrentCount
	c.logger.Info("scaling up", "count", count)

	for i := 0; i < count; i++ {
		req := &provider.CreateRunnerRequest{
			Name:        fmt.Sprintf("zeno-runner-%d", time.Now().UnixNano()),
			Labels:      c.cfg.GitHub.RunnerLabels,
			GitHubToken: c.cfg.GitHub.Token,
			GitHubOrg:   c.cfg.GitHub.Organization,
			GitHubRepo:  c.cfg.GitHub.Repository,
		}

		runner, err := c.provider.CreateRunner(ctx, req)
		if err != nil {
			c.logger.Error("failed to create runner", "error", err)
			c.metrics.ProviderErrors.WithLabelValues(
				c.provider.Name(),
				"create",
				"creation_error",
			).Inc()
			continue
		}

		c.logger.Info("runner created", "id", runner.ID, "name", runner.Name)
		c.metrics.ScaleUpEvents.WithLabelValues(decision.Reason).Inc()

		// Record event
		if c.store != nil {
			_ = c.store.RecordScaleEvent(store.ScaleEvent{
				Timestamp:    time.Now(),
				Action:       "scale_up",
				Reason:       decision.Reason,
				QueueDepth:   decision.QueueDepth,
				RunnersBefore: decision.CurrentCount,
				RunnersAfter: decision.CurrentCount + i + 1,
			})
		}
	}

	c.mu.Lock()
	c.lastScaleUpTime = time.Now()
	c.mu.Unlock()

	return nil
}

func (c *Controller) scaleDown(ctx context.Context, decision ScaleDecision) error {
	startTime := time.Now()
	defer func() {
		c.metrics.ScaleDownDuration.Observe(time.Since(startTime).Seconds())
	}()

	count := decision.CurrentCount - decision.DesiredCount
	c.logger.Info("scaling down", "count", count)

	// Get current runners
	runners, err := c.provider.ListRunners(ctx)
	if err != nil {
		return fmt.Errorf("failed to list runners: %w", err)
	}

	// Remove oldest idle runners first
	removed := 0
	for _, runner := range runners {
		if removed >= count {
			break
		}

		if runner.Status == provider.StatusIdle || runner.Status == provider.StatusRunning {
			graceful := c.cfg.Scaling.GracefulTermination

			if err := c.provider.RemoveRunner(ctx, runner.ID, graceful); err != nil {
				c.logger.Error("failed to remove runner",
					"id", runner.ID,
					"error", err,
				)
				c.metrics.ProviderErrors.WithLabelValues(
					c.provider.Name(),
					"remove",
					"removal_error",
				).Inc()
				continue
			}

			c.logger.Info("runner removed", "id", runner.ID, "name", runner.Name)
			c.metrics.ScaleDownEvents.WithLabelValues(decision.Reason).Inc()
			removed++

			// Record event
			if c.store != nil {
				_ = c.store.RecordScaleEvent(store.ScaleEvent{
					Timestamp:     time.Now(),
					Action:        "scale_down",
					Reason:        decision.Reason,
					QueueDepth:    decision.QueueDepth,
					RunnersBefore: decision.CurrentCount,
					RunnersAfter:  decision.CurrentCount - removed,
				})
			}
		}
	}

	c.mu.Lock()
	c.lastScaleDownTime = time.Now()
	c.mu.Unlock()

	return nil
}

func (c *Controller) inCooldownPeriod() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	if now.Sub(c.lastScaleUpTime) < c.cfg.Scaling.CooldownPeriod {
		return true
	}
	if now.Sub(c.lastScaleDownTime) < c.cfg.Scaling.CooldownPeriod {
		return true
	}

	return false
}

func (c *Controller) updateQueueHistory(queueDepth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.queueHistory = append(c.queueHistory, queueDepth)

	// Keep only recent history (last 100 samples)
	if len(c.queueHistory) > 100 {
		c.queueHistory = c.queueHistory[1:]
	}
}

func (c *Controller) predictQueueGrowth() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.queueHistory) < 5 {
		return 0
	}

	// Simple linear regression to predict growth
	// Calculate average growth rate over prediction window
	windowSize := min(10, len(c.queueHistory))
	recent := c.queueHistory[len(c.queueHistory)-windowSize:]

	var totalGrowth float64
	for i := 1; i < len(recent); i++ {
		totalGrowth += float64(recent[i] - recent[i-1])
	}

	avgGrowth := totalGrowth / float64(len(recent)-1)
	currentQueue := float64(c.queueHistory[len(c.queueHistory)-1])

	// Predict queue depth after prediction window
	predicted := currentQueue + (avgGrowth * 3) // Predict 3 intervals ahead

	return max(0, int(predicted))
}

func (c *Controller) updateRunnerStatusMetrics(runners []*provider.Runner) {
	var provisioning, running, terminating, failed int

	for _, r := range runners {
		switch r.Status {
		case provider.StatusProvisioning, provider.StatusPending:
			provisioning++
		case provider.StatusRunning, provider.StatusIdle, provider.StatusBusy:
			running++
		case provider.StatusTerminating:
			terminating++
		case provider.StatusFailed:
			failed++
		}
	}

	c.metrics.RunnersProvisioning.Set(float64(provisioning))
	c.metrics.RunnersRunning.Set(float64(running))
	c.metrics.RunnersTerminating.Set(float64(terminating))
	c.metrics.RunnersFailed.Set(float64(failed))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
