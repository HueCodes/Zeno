package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"Zeno/internal/config"
	"Zeno/internal/metrics"
	"Zeno/internal/provider"
	"Zeno/internal/store"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config      *config.Config
	provider    provider.Provider
	store       *store.Store
	metrics     *metrics.Metrics
	logger      *slog.Logger
	httpServer  *http.Server
}

// New creates a new API server
func New(
	cfg *config.Config,
	prov provider.Provider,
	st *store.Store,
	met *metrics.Metrics,
	logger *slog.Logger,
) *Server {
	return &Server{
		config:   cfg,
		provider: prov,
		store:    st,
		metrics:  met,
		logger:   logger.With("component", "api-server"),
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Health and readiness endpoints
	mux.HandleFunc(s.config.Observability.HealthCheckPath, s.handleHealth)
	mux.HandleFunc(s.config.Observability.ReadinessPath, s.handleReadiness)

	// Metrics endpoint
	if s.config.Observability.EnableMetrics {
		mux.Handle(s.config.Observability.MetricsPath, promhttp.Handler())
	}

	// API v1 endpoints
	mux.HandleFunc("/api/v1/status", s.authMiddleware(s.handleStatus))
	mux.HandleFunc("/api/v1/runners", s.authMiddleware(s.handleRunners))
	mux.HandleFunc("/api/v1/events", s.authMiddleware(s.handleEvents))

	addr := fmt.Sprintf("%s:%d", s.config.Server.Address, s.config.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	s.logger.Info("starting API server", "address", addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server shutdown error", "error", err)
		}
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Basic health check
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Check provider health
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.provider.HealthCheck(ctx); err != nil {
		s.logger.Error("readiness check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runners, err := s.provider.ListRunners(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to list runners", err)
		return
	}

	response := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"runner_count":  len(runners),
		"min_runners":   s.config.Scaling.MinRunners,
		"max_runners":   s.config.Scaling.MaxRunners,
		"provider":      s.provider.Name(),
		"dry_run":       s.config.DryRun,
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRunners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runners, err := s.provider.ListRunners(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to list runners", err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(runners),
		"runners":   runners,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || !s.config.Store.Enabled {
		s.writeError(w, http.StatusNotFound, "store not enabled", nil)
		return
	}

	events := s.store.GetRecentEvents(100)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(events),
		"events":    events,
	})
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Server.EnableAuth {
			next(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.Header.Get("Authorization")
			if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
				apiKey = apiKey[7:]
			}
		}

		if apiKey != s.config.Server.APIKey {
			s.writeError(w, http.StatusUnauthorized, "unauthorized", nil)
			return
		}

		next(w, r)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		s.logger.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string, err error) {
	response := map[string]string{
		"error": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	s.writeJSON(w, statusCode, response)
}
