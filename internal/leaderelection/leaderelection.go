package leaderelection

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"
)

type LeaderElector struct {
	config LeaderElectionConfig
	logger *slog.Logger
	lockFd int
	isLeader bool
}

type LeaderElectionConfig struct {
	Enabled       bool
	LockFilePath  string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

// New creates a new leader elector
func New(cfg LeaderElectionConfig, logger *slog.Logger) *LeaderElector {
	return &LeaderElector{
		config:   cfg,
		logger:   logger.With("component", "leader-election"),
		lockFd:   -1,
		isLeader: false,
	}
}

// Run starts the leader election process
func (le *LeaderElector) Run(ctx context.Context, onStartLeading, onStopLeading func(ctx context.Context)) error {
	if !le.config.Enabled {
		le.logger.Info("leader election disabled, assuming leadership")
		le.isLeader = true
		onStartLeading(ctx)
		<-ctx.Done()
		return nil
	}

	le.logger.Info("starting leader election",
		"lock_file", le.config.LockFilePath,
		"lease_duration", le.config.LeaseDuration,
	)

	ticker := time.NewTicker(le.config.RetryPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if le.isLeader {
				le.release()
				onStopLeading(ctx)
			}
			return nil

		case <-ticker.C:
			acquired, err := le.tryAcquireLock()
			if err != nil {
				le.logger.Error("failed to acquire lock", "error", err)
				continue
			}

			if acquired && !le.isLeader {
				le.logger.Info("acquired leadership")
				le.isLeader = true
				go onStartLeading(ctx)
			} else if !acquired && le.isLeader {
				le.logger.Warn("lost leadership")
				le.isLeader = false
				onStopLeading(ctx)
			}
		}
	}
}

// IsLeader returns whether this instance is the leader
func (le *LeaderElector) IsLeader() bool {
	return le.isLeader || !le.config.Enabled
}

func (le *LeaderElector) tryAcquireLock() (bool, error) {
	// Try to open/create lock file
	fd, err := syscall.Open(le.config.LockFilePath, syscall.O_CREAT|syscall.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		syscall.Close(fd)
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Write our PID to the lock file
	pid := fmt.Sprintf("%d\n", os.Getpid())
	if _, err := syscall.Write(fd, []byte(pid)); err != nil {
		syscall.Close(fd)
		return false, fmt.Errorf("failed to write PID: %w", err)
	}

	// Release old lock if we had one
	if le.lockFd >= 0 {
		syscall.Close(le.lockFd)
	}

	le.lockFd = fd
	return true, nil
}

func (le *LeaderElector) release() {
	if le.lockFd >= 0 {
		syscall.Flock(le.lockFd, syscall.LOCK_UN)
		syscall.Close(le.lockFd)
		le.lockFd = -1
		le.logger.Info("released leadership")
	}
}
