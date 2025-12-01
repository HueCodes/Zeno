package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "zeno"
)

// Metrics holds all Prometheus metrics for the controller
type Metrics struct {
	// Reconciliation metrics
	ReconcileTotal       *prometheus.CounterVec
	ReconcileDuration    *prometheus.HistogramVec
	ReconcileErrors      *prometheus.CounterVec

	// Runner metrics
	RunnersDesired       prometheus.Gauge
	RunnersCurrent       prometheus.Gauge
	RunnersProvisioning  prometheus.Gauge
	RunnersRunning       prometheus.Gauge
	RunnersTerminating   prometheus.Gauge
	RunnersFailed        prometheus.Gauge

	// Scaling metrics
	ScaleUpEvents        *prometheus.CounterVec
	ScaleDownEvents      *prometheus.CounterVec
	ScaleUpDuration      prometheus.Histogram
	ScaleDownDuration    prometheus.Histogram

	// Queue metrics
	QueueDepth           prometheus.Gauge
	QueueDepthSamples    prometheus.Histogram
	WaitingJobs          prometheus.Gauge

	// GitHub API metrics
	GitHubAPIRequests    *prometheus.CounterVec
	GitHubAPIDuration    prometheus.Histogram
	GitHubAPIRateLimit   prometheus.Gauge
	GitHubAPIRateLimitReset prometheus.Gauge

	// Provider metrics
	ProviderOperations   *prometheus.CounterVec
	ProviderDuration     *prometheus.HistogramVec
	ProviderErrors       *prometheus.CounterVec

	// System metrics
	ControllerInfo       *prometheus.GaugeVec
	LeaderElection       prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(registry *prometheus.Registry) *Metrics {
	factory := promauto.With(registry)

	m := &Metrics{
		// Reconciliation metrics
		ReconcileTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reconcile_total",
				Help:      "Total number of reconciliation loops",
			},
			[]string{"status"},
		),
		ReconcileDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "reconcile_duration_seconds",
				Help:      "Duration of reconciliation loops",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status"},
		),
		ReconcileErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reconcile_errors_total",
				Help:      "Total number of reconciliation errors",
			},
			[]string{"error_type"},
		),

		// Runner metrics
		RunnersDesired: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_desired",
				Help:      "Desired number of runners",
			},
		),
		RunnersCurrent: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_current",
				Help:      "Current number of runners",
			},
		),
		RunnersProvisioning: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_provisioning",
				Help:      "Number of runners currently provisioning",
			},
		),
		RunnersRunning: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_running",
				Help:      "Number of runners currently running",
			},
		),
		RunnersTerminating: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_terminating",
				Help:      "Number of runners currently terminating",
			},
		),
		RunnersFailed: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "runners_failed",
				Help:      "Number of failed runners",
			},
		),

		// Scaling metrics
		ScaleUpEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "scale_up_events_total",
				Help:      "Total number of scale up events",
			},
			[]string{"reason"},
		),
		ScaleDownEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "scale_down_events_total",
				Help:      "Total number of scale down events",
			},
			[]string{"reason"},
		),
		ScaleUpDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "scale_up_duration_seconds",
				Help:      "Duration of scale up operations",
				Buckets:   []float64{1, 5, 10, 30, 60, 120, 300},
			},
		),
		ScaleDownDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "scale_down_duration_seconds",
				Help:      "Duration of scale down operations",
				Buckets:   []float64{1, 5, 10, 30, 60, 120, 300},
			},
		),

		// Queue metrics
		QueueDepth: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "queue_depth",
				Help:      "Current queue depth (queued workflow jobs)",
			},
		),
		QueueDepthSamples: factory.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "queue_depth_samples",
				Help:      "Distribution of queue depth samples",
				Buckets:   []float64{0, 1, 5, 10, 25, 50, 100, 250, 500},
			},
		),
		WaitingJobs: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "waiting_jobs",
				Help:      "Number of jobs waiting for runners",
			},
		),

		// GitHub API metrics
		GitHubAPIRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "github_api_requests_total",
				Help:      "Total number of GitHub API requests",
			},
			[]string{"endpoint", "status"},
		),
		GitHubAPIDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "github_api_duration_seconds",
				Help:      "Duration of GitHub API requests",
				Buckets:   prometheus.DefBuckets,
			},
		),
		GitHubAPIRateLimit: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "github_api_rate_limit_remaining",
				Help:      "Remaining GitHub API rate limit",
			},
		),
		GitHubAPIRateLimitReset: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "github_api_rate_limit_reset_timestamp",
				Help:      "GitHub API rate limit reset time (Unix timestamp)",
			},
		),

		// Provider metrics
		ProviderOperations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "provider_operations_total",
				Help:      "Total number of provider operations",
			},
			[]string{"provider", "operation", "status"},
		),
		ProviderDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "provider_operation_duration_seconds",
				Help:      "Duration of provider operations",
				Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
			},
			[]string{"provider", "operation"},
		),
		ProviderErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "provider_errors_total",
				Help:      "Total number of provider errors",
			},
			[]string{"provider", "operation", "error_type"},
		),

		// System metrics
		ControllerInfo: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "controller_info",
				Help:      "Information about the controller",
			},
			[]string{"version", "provider", "mode"},
		),
		LeaderElection: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "leader_election_status",
				Help:      "Leader election status (1 if leader, 0 otherwise)",
			},
		),
	}

	return m
}
