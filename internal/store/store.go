package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Store struct {
	config   StoreConfig
	events   []ScaleEvent
	mu       sync.RWMutex
}

type StoreConfig struct {
	Enabled   bool
	Path      string
	MaxEvents int
}

type ScaleEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	Action        string    `json:"action"`
	Reason        string    `json:"reason"`
	QueueDepth    int       `json:"queue_depth"`
	RunnersBefore int       `json:"runners_before"`
	RunnersAfter  int       `json:"runners_after"`
}

// New creates a new store instance
func New(cfg StoreConfig) (*Store, error) {
	s := &Store{
		config: cfg,
		events: make([]ScaleEvent, 0),
	}

	// Load existing events if file exists
	if cfg.Enabled && cfg.Path != "" {
		if err := s.load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load store: %w", err)
		}
	}

	return s, nil
}

// RecordScaleEvent records a scaling event
func (s *Store) RecordScaleEvent(event ScaleEvent) error {
	if !s.config.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)

	// Trim old events if we exceed max
	if len(s.events) > s.config.MaxEvents {
		s.events = s.events[len(s.events)-s.config.MaxEvents:]
	}

	// Persist to disk
	return s.persist()
}

// GetRecentEvents returns recent scaling events
func (s *Store) GetRecentEvents(count int) []ScaleEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count > len(s.events) {
		count = len(s.events)
	}

	return s.events[len(s.events)-count:]
}

// GetAllEvents returns all scaling events
func (s *Store) GetAllEvents() []ScaleEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]ScaleEvent(nil), s.events...)
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.config.Path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.events)
}

func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.events, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	return os.WriteFile(s.config.Path, data, 0644)
}
