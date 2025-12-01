package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"Zeno/internal/config"
)

type Client struct {
	config     config.GitHubConfig
	httpClient *http.Client
	logger     *slog.Logger

	// Cache
	cache      *queueCache
	cacheMu    sync.RWMutex

	// Rate limit tracking
	rateLimitRemaining int
	rateLimitReset     time.Time
	rateLimitMu        sync.RWMutex
}

type queueCache struct {
	queuedJobs int
	timestamp  time.Time
}

type WorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

type WorkflowRun struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Name   string `json:"name"`
}

type RateLimitInfo struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

// NewClient creates a new GitHub API client with retry and caching capabilities
func NewClient(cfg config.GitHubConfig, logger *slog.Logger) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		logger: logger.With("component", "github-client"),
		cache: &queueCache{
			timestamp: time.Time{},
		},
	}
}

// GetQueuedWorkflowJobs returns the number of queued workflow jobs
func (c *Client) GetQueuedWorkflowJobs(ctx context.Context) (int, error) {
	// Check cache first
	if cached, ok := c.getCachedQueue(); ok {
		c.logger.Debug("using cached queue depth", "queued_jobs", cached)
		return cached, nil
	}

	// Fetch from API with retries
	queuedJobs, err := c.fetchQueuedJobsWithRetry(ctx)
	if err != nil {
		return 0, err
	}

	// Update cache
	c.updateCache(queuedJobs)

	return queuedJobs, nil
}

// GetRateLimitInfo returns current rate limit information
func (c *Client) GetRateLimitInfo() RateLimitInfo {
	c.rateLimitMu.RLock()
	defer c.rateLimitMu.RUnlock()

	return RateLimitInfo{
		Remaining: c.rateLimitRemaining,
		Reset:     c.rateLimitReset,
	}
}

func (c *Client) fetchQueuedJobsWithRetry(ctx context.Context) (int, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			c.logger.Info("retrying GitHub API request",
				"attempt", attempt,
				"backoff", backoff,
			)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}

		queuedJobs, err := c.fetchQueuedJobs(ctx)
		if err == nil {
			return queuedJobs, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if !c.shouldRetry(err) {
			return 0, err
		}
	}

	return 0, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) fetchQueuedJobs(ctx context.Context) (int, error) {
	var url string
	if c.config.Organization != "" {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/actions/runs?status=queued&per_page=100", c.config.Organization)
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/%s/actions/runs?status=queued&per_page=100", c.config.Repository)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)
	c.logger.Debug("GitHub API request completed",
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	// Update rate limit info
	c.updateRateLimitInfo(resp.Header)

	// Handle rate limiting
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		resetTime := c.getRateLimitResetTime(resp.Header)
		waitDuration := time.Until(resetTime)

		c.logger.Warn("rate limited by GitHub API",
			"reset_time", resetTime,
			"wait_duration", waitDuration,
		)

		return 0, &RateLimitError{
			ResetTime: resetTime,
			RetryAfter: waitDuration,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result WorkflowRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("fetched queued jobs", "count", result.TotalCount)
	return result.TotalCount, nil
}

func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff with jitter
	base := c.config.RetryBackoffBase
	max := c.config.RetryBackoffMax

	// Calculate exponential backoff: base * 2^attempt
	backoff := float64(base) * math.Pow(2, float64(attempt-1))

	// Cap at max
	if backoff > float64(max) {
		backoff = float64(max)
	}

	// Add jitter (±25%)
	jitter := backoff * 0.25 * (2*rand.Float64() - 1)
	backoff += jitter

	return time.Duration(backoff)
}

func (c *Client) shouldRetry(err error) bool {
	// Retry on rate limit errors
	if _, ok := err.(*RateLimitError); ok {
		return true
	}

	// Retry on network errors
	// In production, you'd check for specific error types
	return true
}

func (c *Client) getCachedQueue() (int, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	if c.cache == nil || time.Since(c.cache.timestamp) > c.config.CacheTTL {
		return 0, false
	}

	return c.cache.queuedJobs, true
}

func (c *Client) updateCache(queuedJobs int) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache = &queueCache{
		queuedJobs: queuedJobs,
		timestamp:  time.Now(),
	}
}

func (c *Client) updateRateLimitInfo(headers http.Header) {
	c.rateLimitMu.Lock()
	defer c.rateLimitMu.Unlock()

	if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			c.rateLimitRemaining = val
		}
	}

	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			c.rateLimitReset = time.Unix(val, 0)
		}
	}

	// Log warning if approaching rate limit
	if c.rateLimitRemaining < c.config.RateLimitBuffer {
		c.logger.Warn("approaching GitHub API rate limit",
			"remaining", c.rateLimitRemaining,
			"reset_time", c.rateLimitReset,
		)
	}
}

func (c *Client) getRateLimitResetTime(headers http.Header) time.Time {
	// Check Retry-After header first (for 429 responses)
	if retryAfter := headers.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}

	// Fall back to X-RateLimit-Reset
	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			return time.Unix(val, 0)
		}
	}

	// Default: retry in 60 seconds
	return time.Now().Add(60 * time.Second)
}

// RateLimitError represents a rate limiting error
type RateLimitError struct {
	ResetTime  time.Time
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v (reset at %v)", e.RetryAfter, e.ResetTime)
}
