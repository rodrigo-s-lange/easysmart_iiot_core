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
		telemetryIngestedTotal,
		telemetryRejectedTotal,
		authRateLimitTotal,
	)
}

func ObserveHTTP(method, path, status string) {
	httpRequestsTotal.WithLabelValues(method, path, status).Inc()
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
