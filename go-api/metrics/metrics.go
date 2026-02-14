package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"method", "path", "status"},
	)

	telemetryIngestedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telemetry_ingested_total",
			Help: "Total telemetry records ingested",
		},
		[]string{"slot"},
	)

	telemetryRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telemetry_rejected_total",
			Help: "Total telemetry records rejected",
		},
		[]string{"reason"},
	)

	authRateLimitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_rate_limit_total",
			Help: "Total auth rate limit blocks",
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDurationSeconds,
		telemetryIngestedTotal,
		telemetryRejectedTotal,
		authRateLimitTotal,
	)
}

func ObserveHTTP(method, path, status string) {
	httpRequestsTotal.WithLabelValues(method, path, status).Inc()
}

func ObserveHTTPDuration(method, path, status string, seconds float64) {
	httpRequestDurationSeconds.WithLabelValues(method, path, status).Observe(seconds)
}

func TelemetryIngested(slot string) {
	telemetryIngestedTotal.WithLabelValues(slot).Inc()
}

func TelemetryRejected(reason string) {
	telemetryRejectedTotal.WithLabelValues(reason).Inc()
}

func AuthRateLimited(path string) {
	authRateLimitTotal.WithLabelValues(path).Inc()
}
