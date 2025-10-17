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
	MetricsBackendPrometheus     MetricsBackend = "prometheus"
	MetricsBackendOpenTelemetry  MetricsBackend = "otel"
)