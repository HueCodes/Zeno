package analytics

import (
	"testing"
	"time"

	"Zeno/internal/models"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker()
	if tracker == nil {
		t.Fatal("NewTracker() returned nil")
	}

	if tracker.history == nil {
		t.Error("history should be initialized")
	}
}

func TestUpdateMetrics(t *testing.T) {
	tracker := NewTracker()

	metrics := models.Metrics{
		ActiveRunners: 5,
		IdleRunners:   2,
		QueuedJobs:    10,
		RunningJobs:   3,
	}

	tracker.UpdateMetrics(metrics)

	got := tracker.GetMetrics()
	if got.ActiveRunners != 5 {
		t.Errorf("expected ActiveRunners=5, got %d", got.ActiveRunners)
	}
	if got.QueuedJobs != 10 {
		t.Errorf("expected QueuedJobs=10, got %d", got.QueuedJobs)
	}
}

func TestRecordDecision(t *testing.T) {
	tracker := NewTracker()

	decision := models.ScalingDecision{
		Action:       "scale_up",
		CurrentCount: 1,
		DesiredCount: 3,
		QueuedJobs:   10,
		Reason:       "queue above threshold",
	}

	tracker.RecordDecision(decision)

	history := tracker.GetHistory(10)
	if len(history) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(history))
	}

	if history[0].Action != "scale_up" {
		t.Errorf("expected action=scale_up, got %s", history[0].Action)
	}

	if history[0].Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestGetHistoryLimit(t *testing.T) {
	tracker := NewTracker()

	// Add 10 decisions
	for i := 0; i < 10; i++ {
		tracker.RecordDecision(models.ScalingDecision{
			Action:       "scale_up",
			CurrentCount: i,
			DesiredCount: i + 1,
		})
	}

	// Test limit
	history := tracker.GetHistory(5)
	if len(history) != 5 {
		t.Errorf("expected 5 decisions, got %d", len(history))
	}

	// Test getting all
	history = tracker.GetHistory(0)
	if len(history) != 10 {
		t.Errorf("expected 10 decisions, got %d", len(history))
	}

	// Test getting more than available
	history = tracker.GetHistory(20)
	if len(history) != 10 {
		t.Errorf("expected 10 decisions, got %d", len(history))
	}
}

func TestHistoryCapacity(t *testing.T) {
	tracker := NewTracker()

	// Add 150 decisions (more than the 100 limit)
	for i := 0; i < 150; i++ {
		tracker.RecordDecision(models.ScalingDecision{
			Action:       "scale_up",
			CurrentCount: i,
			DesiredCount: i + 1,
		})
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	history := tracker.GetHistory(0)
	if len(history) != 100 {
		t.Errorf("expected history limited to 100, got %d", len(history))
	}

	// Verify oldest entries were removed (should start at 50, not 0)
	if history[0].CurrentCount != 50 {
		t.Errorf("expected oldest entry CurrentCount=50, got %d", history[0].CurrentCount)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker := NewTracker()

	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 50; i++ {
			tracker.RecordDecision(models.ScalingDecision{
				Action: "scale_up",
			})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 50; i++ {
			tracker.GetHistory(10)
		}
		done <- true
	}()

	// Concurrent metric updates
	go func() {
		for i := 0; i < 50; i++ {
			tracker.UpdateMetrics(models.Metrics{
				ActiveRunners: i,
			})
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Should complete without race conditions
}
