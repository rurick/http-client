package httpclient

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics содержит все метрики HTTP клиента.
type Metrics struct {
	// RequestsTotal счётчик общего количества запросов
	RequestsTotal metric.Int64Counter

	// RequestDuration гистограмма длительности запросов
	RequestDuration metric.Float64Histogram

	// RetriesTotal счётчик ретраев
	RetriesTotal metric.Int64Counter

	// InflightRequests updowncounter активных запросов
	InflightRequests metric.Int64UpDownCounter

	// RequestSize гистограмма размера запросов
	RequestSize metric.Int64Histogram

	// ResponseSize гистограмма размера ответов
	ResponseSize metric.Int64Histogram

	meter metric.Meter
}

// NewMetrics создаёт новый экземпляр метрик.
func NewMetrics(meterName string) *Metrics {
	meter := otel.Meter(meterName)

	requestsTotal, _ := meter.Int64Counter(
		"http_client_requests_total",
		metric.WithDescription("Total number of HTTP client requests"),
		metric.WithUnit("1"),
	)

	requestDuration, _ := meter.Float64Histogram(
		"http_client_request_duration_seconds",
		metric.WithDescription("HTTP client request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries( //nolint:mnd // OpenTelemetry histogram buckets for latency
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, //nolint:mnd // standard latency buckets
			1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60, //nolint:mnd // standard latency buckets
		),
	)

	retriesTotal, _ := meter.Int64Counter(
		"http_client_retries_total",
		metric.WithDescription("Total number of HTTP client retries"),
		metric.WithUnit("1"),
	)

	inflightRequests, _ := meter.Int64UpDownCounter(
		"http_client_inflight_requests",
		metric.WithDescription("Number of HTTP client requests currently in-flight"),
		metric.WithUnit("1"),
	)

	requestSize, _ := meter.Int64Histogram(
		"http_client_request_size_bytes",
		metric.WithDescription("HTTP client request size in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries( //nolint:mnd // OpenTelemetry histogram buckets for byte sizes
			256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, //nolint:mnd // standard byte size buckets
		),
	)

	responseSize, _ := meter.Int64Histogram(
		"http_client_response_size_bytes",
		metric.WithDescription("HTTP client response size in bytes"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries( //nolint:mnd // OpenTelemetry histogram buckets for byte sizes
			256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, //nolint:mnd // standard byte size buckets
		),
	)

	return &Metrics{
		RequestsTotal:    requestsTotal,
		RequestDuration:  requestDuration,
		RetriesTotal:     retriesTotal,
		InflightRequests: inflightRequests,
		RequestSize:      requestSize,
		ResponseSize:     responseSize,
		meter:            meter,
	}
}

// RecordRequest записывает метрики для запроса.
func (m *Metrics) RecordRequest(ctx context.Context, method, host, status string, retry, hasError bool) {
	m.RequestsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
			attribute.String("status", status),
			attribute.Bool("retry", retry),
			attribute.Bool("error", hasError),
		),
	)
}

// RecordDuration записывает длительность запроса.
func (m *Metrics) RecordDuration(ctx context.Context, duration float64, method, host, status string, attempt int) {
	m.RequestDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
			attribute.String("status", status),
			attribute.Int("attempt", attempt),
		),
	)
}

// RecordRetry записывает метрику retry.
func (m *Metrics) RecordRetry(ctx context.Context, reason, method, host string) {
	m.RetriesTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("reason", reason),
			attribute.String("method", method),
			attribute.String("host", host),
		),
	)
}

// RecordInflight записывает количество активных запросов.
func (m *Metrics) RecordInflight(ctx context.Context, delta int64, host string) {
	m.InflightRequests.Add(ctx, delta,
		metric.WithAttributes(
			attribute.String("host", host),
		),
	)
}

// RecordRequestSize записывает размер запроса.
func (m *Metrics) RecordRequestSize(ctx context.Context, size int64, method, host string) {
	m.RequestSize.Record(ctx, size,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
		),
	)
}

// RecordResponseSize записывает размер ответа.
func (m *Metrics) RecordResponseSize(ctx context.Context, size int64, method, host, status string) {
	m.ResponseSize.Record(ctx, size,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
			attribute.String("status", status),
		),
	)
}

// IncrementInflight увеличивает счётчик активных запросов.
func (m *Metrics) IncrementInflight(ctx context.Context, method, host string) {
	m.InflightRequests.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
		),
	)
}

// DecrementInflight уменьшает счётчик активных запросов.
func (m *Metrics) DecrementInflight(ctx context.Context, method, host string) {
	m.InflightRequests.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("host", host),
		),
	)
}

// Close освобождает ресурсы метрик.
func (m *Metrics) Close() error {
	// В текущей реализации нет ресурсов для освобождения
	return nil
}
