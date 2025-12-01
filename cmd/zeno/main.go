package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"Zeno/internal/api"
	"Zeno/internal/config"
	"Zeno/internal/controller"
	"Zeno/internal/github"
	"Zeno/internal/leaderelection"
	"Zeno/internal/metrics"
	"Zeno/internal/provider"
	"Zeno/internal/provider/docker"
	"Zeno/internal/provider/ec2"
	"Zeno/internal/store"

	"github.com/prometheus/client_golang/prometheus"
)

const version = "2.0.0"

func main() {
	configPath := flag.String("config", "", "Path to configuration file (optional)")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup structured logging
	logger := setupLogger(cfg.LogLevel)
	logger.Info("starting Zeno",
		"version", version,
		"provider", cfg.Provider.Type,
		"dry_run", cfg.DryRun,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize metrics
	registry := prometheus.NewRegistry()
	met := metrics.NewMetrics(registry)
	met.ControllerInfo.WithLabelValues(version, cfg.Provider.Type, modeString(cfg.DryRun)).Set(1)

	// Initialize GitHub client
	ghClient := github.NewClient(cfg.GitHub, logger)

	// Initialize provider
	prov, err := createProvider(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}
	defer prov.Close()

	// Initialize store
	st, err := store.New(store.StoreConfig{
		Enabled:   cfg.Store.Enabled,
		Path:      cfg.Store.Path,
		MaxEvents: cfg.Store.MaxEvents,
	})
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Initialize controller
	ctrl := controller.New(cfg, ghClient, prov, st, met, logger)

	// Initialize API server
	apiServer := api.New(cfg, prov, st, met, logger)

	// Start API server
	go func() {
		if err := apiServer.Start(ctx); err != nil {
			logger.Error("API server error", "error", err)
		}
	}()

	// Initialize leader election
	le := leaderelection.New(leaderelection.LeaderElectionConfig{
		Enabled:       cfg.LeaderElection.Enabled,
		LockFilePath:  cfg.LeaderElection.LockFilePath,
		LeaseDuration: cfg.LeaderElection.LeaseDuration,
		RenewDeadline: cfg.LeaderElection.RenewDeadline,
		RetryPeriod:   cfg.LeaderElection.RetryPeriod,
	}, logger)

	// Start controller with leader election
	errCh := make(chan error, 1)
	go func() {
		errCh <- le.Run(ctx,
			func(ctx context.Context) {
				logger.Info("became leader, starting controller")
				met.LeaderElection.Set(1)
				if err := ctrl.Run(ctx); err != nil {
					logger.Error("controller error", "error", err)
				}
			},
			func(ctx context.Context) {
				logger.Info("stopped being leader")
				met.LeaderElection.Set(0)
			},
		)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		logger.Info("received shutdown signal")
		cancel()
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	logger.Info("shutdown complete")
	return nil
}

func createProvider(cfg *config.Config, logger *slog.Logger) (provider.Provider, error) {
	switch cfg.Provider.Type {
	case "docker":
		return docker.New(cfg.Provider.Docker, logger)
	case "ec2":
		return ec2.New(cfg.Provider.AWS, logger)
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Provider.Type)
	}
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

func modeString(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}
	return "production"
}
