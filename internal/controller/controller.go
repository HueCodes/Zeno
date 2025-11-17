package controller

import (
	"context"
	"fmt"
	"log"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/github"
	"Zeno/internal/runner"
)

type Controller struct {
	cfg       *config.Config
	ghClient  *github.Client
	runnerMgr *runner.Manager
}

func New(cfg *config.Config) (*Controller, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ghClient := github.NewClient(cfg.GitHub.Token, cfg.GitHub.Organization, cfg.GitHub.Repository)
	runnerMgr := runner.NewManager()

	return &Controller{
		cfg:       cfg,
		ghClient:  ghClient,
		runnerMgr: runnerMgr,
	}, nil
}

func (c *Controller) Run(ctx context.Context) error {
	log.Println("controller starting...")

	ticker := time.NewTicker(c.cfg.Runner.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("controller stopped")
			return nil
		case <-ticker.C:
			if err := c.reconcile(ctx); err != nil {
				log.Printf("reconcile error: %v", err)
			}
		}
	}
}

func (c *Controller) reconcile(ctx context.Context) error {
	queuedJobs, err := c.ghClient.GetQueuedWorkflowJobs(ctx)
	if err != nil {
		return err
	}

	currentRunners := c.runnerMgr.Count()
	log.Printf("queued jobs: %d, current runners: %d", queuedJobs, currentRunners)

	if queuedJobs >= c.cfg.Runner.ScaleUpThreshold && currentRunners < c.cfg.Runner.MaxRunners {
		needed := minInt(queuedJobs-currentRunners, c.cfg.Runner.MaxRunners-currentRunners)
		log.Printf("scaling up: adding %d runners", needed)
		for i := 0; i < needed; i++ {
			c.runnerMgr.Add()
		}
	} else if queuedJobs <= c.cfg.Runner.ScaleDownThreshold && currentRunners > c.cfg.Runner.MinRunners {
		excess := currentRunners - maxInt(c.cfg.Runner.MinRunners, queuedJobs)
		if excess > 0 {
			log.Printf("scaling down: removing %d runners", excess)
			for i := 0; i < excess; i++ {
				c.runnerMgr.Remove()
			}
		}
	}

	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
