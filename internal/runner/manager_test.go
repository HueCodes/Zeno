package runner

import (
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	if m.Count() != 0 {
		t.Errorf("expected 0 runners, got %d", m.Count())
	}
}

func TestManagerAdd(t *testing.T) {
	m := NewManager()

	m.Add()
	if m.Count() != 1 {
		t.Errorf("expected 1 runner after Add(), got %d", m.Count())
	}

	m.Add()
	m.Add()
	if m.Count() != 3 {
		t.Errorf("expected 3 runners after 3 Add() calls, got %d", m.Count())
	}
}

func TestManagerRemove(t *testing.T) {
	m := NewManager()

	// Add some runners first
	m.Add()
	m.Add()
	m.Add()

	if m.Count() != 3 {
		t.Fatalf("setup failed: expected 3 runners, got %d", m.Count())
	}

	m.Remove()
	if m.Count() != 2 {
		t.Errorf("expected 2 runners after Remove(), got %d", m.Count())
	}

	m.Remove()
	m.Remove()
	if m.Count() != 0 {
		t.Errorf("expected 0 runners after removing all, got %d", m.Count())
	}

	// Removing from empty should not panic
	m.Remove()
	if m.Count() != 0 {
		t.Errorf("expected 0 runners after Remove() on empty, got %d", m.Count())
	}
}

func TestManagerConcurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	numAdds := 50
	numRemoves := 25

	// Concurrent adds
	for i := 0; i < numAdds; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Add()
		}()
	}

	wg.Wait()

	// Verify all adds completed before removes
	if m.Count() != numAdds {
		t.Errorf("expected %d runners after adds, got %d", numAdds, m.Count())
	}

	// Concurrent removes
	for i := 0; i < numRemoves; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Remove()
		}()
	}

	// Concurrent counts (should not affect count)
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Count()
		}()
	}

	wg.Wait()

	// Should have 25 runners (50 adds - 25 removes)
	expected := numAdds - numRemoves
	if m.Count() != expected {
		t.Errorf("expected %d runners after concurrent operations, got %d", expected, m.Count())
	}
}

func TestRunnerID(t *testing.T) {
	m := NewManager()

	m.Add()
	m.Add()

	// Verify that runners have unique IDs
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.runners) != 2 {
		t.Fatalf("expected 2 runners, got %d", len(m.runners))
	}

	ids := make(map[string]bool)
	for id := range m.runners {
		if ids[id] {
			t.Errorf("duplicate runner ID: %s", id)
		}
		ids[id] = true
	}
}

func TestRunnerStatus(t *testing.T) {
	m := NewManager()

	m.Add()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, runner := range m.runners {
		if runner.Status != "idle" {
			t.Errorf("expected runner status 'idle', got '%s'", runner.Status)
		}
	}
}
