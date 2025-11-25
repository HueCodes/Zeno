package controller

import (
	"context"
	"testing"
	"time"

	"Zeno/internal/config"
)

func BenchmarkReconcile(b *testing.B) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			Token:        "test-token",
			Organization: "test-org",
		},
		Runner: config.RunnerConfig{
			MinRunners:         1,
			MaxRunners:         10,
			ScaleUpThreshold:   5,
			ScaleDownThreshold: 0,
			CheckInterval:      30 * time.Second,
		},
	}

	ctrl, err := New(cfg)
	if err != nil {
		b.Fatalf("failed to create controller: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Ignore errors in benchmark (expected due to test environment)
		_ = ctrl.reconcile(ctx)
	}
}
