package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"Zeno/internal/config"
	"Zeno/internal/controller"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctrl, err := controller.New(cfg)
	if err != nil {
		log.Fatalf("failed to create controller: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	if err := ctrl.Run(ctx); err != nil {
		log.Fatalf("controller error: %v", err)
	}
}
