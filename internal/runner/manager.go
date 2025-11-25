package runner

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Manager struct {
	mu      sync.RWMutex
	runners map[string]*Runner
	order   []string // Track insertion order for deterministic removal
}

type Runner struct {
	ID     string
	Status string
}

func NewManager() *Manager {
	return &Manager{
		runners: make(map[string]*Runner),
		order:   make([]string, 0),
	}
}

func (m *Manager) Add() {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	m.runners[id] = &Runner{
		ID:     id,
		Status: "idle",
	}
	m.order = append(m.order, id)
	log.Printf("runner added: %s", id)
}

func (m *Manager) Remove() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove oldest runner (first in order slice) for deterministic behavior
	if len(m.order) == 0 {
		return
	}

	id := m.order[0]
	m.order = m.order[1:]
	delete(m.runners, id)
	log.Printf("runner removed: %s", id)
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.runners)
}

func generateID() string {
	return fmt.Sprintf("runner-%d", time.Now().UnixNano())
}
