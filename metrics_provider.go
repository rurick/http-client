package httpclient

import "context"

// Constants for metric names, unified for all providers.
const (
	MetricRequestsTotal     = "http_client_requests_total"
	MetricRequestDuration   = "http_client_request_duration_seconds"
	MetricRetriesTotal      = "http_client_retries_total"
	MetricInflightRequests  = "http_client_inflight_requests"
	MetricRequestSizeBytes  = "http_client_request_size_bytes"
	MetricResponseSizeBytes = "http_client_response_size_bytes"
)

// DefaultDurationBuckets contains default buckets for request duration histograms (in seconds).
var DefaultDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
	1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60,
}

// DefaultSizeBuckets contains default buckets for request and response size histograms (in bytes).
var DefaultSizeBuckets = []float64{
	256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
}

// MetricsProvider defines the interface for various metrics backends.
type MetricsProvider interface {
	// RecordRequest records a request metric
	RecordRequest(ctx context.Context, method, host, status string, retry, hasError bool)

	// RecordDuration records request duration in seconds
	RecordDuration(ctx context.Context, seconds float64, method, host, status string, attempt int)

	// RecordRetry records a retry attempt metric
	RecordRetry(ctx context.Context, reason, method, host string)

	// RecordRequestSize records request size in bytes
	RecordRequestSize(ctx context.Context, bytes int64, method, host string)

	// RecordResponseSize records response size in bytes
	RecordResponseSize(ctx context.Context, bytes int64, method, host, status string)

	// InflightInc increments the active requests counter
	InflightInc(ctx context.Context, method, host string)

	// InflightDec decrements the active requests counter
	InflightDec(ctx context.Context, method, host string)

	// Close releases provider resources
	Close() error
}

// MetricsBackend defines the type of metrics backend.
type MetricsBackend string

const (
	MetricsBackendPrometheus    MetricsBackend = "prometheus"
	MetricsBackendOpenTelemetry MetricsBackend = "otel"
)
