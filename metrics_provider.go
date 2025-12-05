package httpclient

import "context"

// Константы для имен метрик, унифицированные для всех провайдеров.
const (
	MetricRequestsTotal     = "http_client_requests_total"
	MetricRequestDuration   = "http_client_request_duration_seconds"
	MetricRetriesTotal      = "http_client_retries_total"
	MetricInflightRequests  = "http_client_inflight_requests"
	MetricRequestSizeBytes  = "http_client_request_size_bytes"
	MetricResponseSizeBytes = "http_client_response_size_bytes"
)

// DefaultDurationBuckets содержит бакеты по умолчанию для гистограмм длительности запросов (в секундах).
var DefaultDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
	1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60,
}

// DefaultSizeBuckets содержит бакеты по умолчанию для гистограмм размеров запросов и ответов (в байтах).
var DefaultSizeBuckets = []float64{
	256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
}

// MetricsProvider определяет интерфейс для различных бэкендов метрик.
type MetricsProvider interface {
	// RecordRequest записывает метрику запроса
	RecordRequest(ctx context.Context, method, host, status string, retry, hasError bool)

	// RecordDuration записывает длительность запроса в секундах
	RecordDuration(ctx context.Context, seconds float64, method, host, status string, attempt int)

	// RecordRetry записывает метрику повторной попытки
	RecordRetry(ctx context.Context, reason, method, host string)

	// RecordRequestSize записывает размер запроса в байтах
	RecordRequestSize(ctx context.Context, bytes int64, method, host string)

	// RecordResponseSize записывает размер ответа в байтах
	RecordResponseSize(ctx context.Context, bytes int64, method, host, status string)

	// InflightInc увеличивает счетчик активных запросов
	InflightInc(ctx context.Context, method, host string)

	// InflightDec уменьшает счетчик активных запросов
	InflightDec(ctx context.Context, method, host string)

	// Close освобождает ресурсы провайдера
	Close() error
}

// MetricsBackend определяет тип бэкенда метрик.
type MetricsBackend string

const (
	MetricsBackendPrometheus    MetricsBackend = "prometheus"
	MetricsBackendOpenTelemetry MetricsBackend = "otel"
)
