package httpclient

import (
	"context"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// Константы для метрик.
const (
	// Минимальное значение в бакетах длительности.
	minDurationBucketSeconds = 0.001

	// Минимальное значение в бакетах размера (в байтах).
	minSizeBucketBytes = 256

	// Суффиксы для имен метрик.
	metricsRequestsTotal     = "_http_client_requests_total"
	metricsRequestDuration   = "_http_client_request_duration_seconds"
	metricsRetriesTotal      = "_http_client_retries_total"
	metricsInflightRequests  = "_http_client_inflight_requests"
	metricsRequestSizeBytes  = "_http_client_request_size_bytes"
	metricsResponseSizeBytes = "_http_client_response_size_bytes"
)
// Metrics содержит все метрики HTTP клиента.
type Metrics struct {
	// RequestsTotal счётчик общего количества запросов
	RequestsTotal *prometheus.CounterVec

	// RequestDuration гистограмма длительности запросов
	RequestDuration *prometheus.HistogramVec

	// RetriesTotal счётчик ретраев
	RetriesTotal *prometheus.CounterVec

	// InflightRequests gauge активных запросов
	InflightRequests *prometheus.GaugeVec

	// RequestSize гистограмма размера запросов
	RequestSize *prometheus.HistogramVec

	// ResponseSize гистограмма размера ответов
	ResponseSize *prometheus.HistogramVec

	registry *prometheus.Registry
}

// NewMetrics создаёт новый экземпляр метрик.
func NewMetrics(meterName string) *Metrics {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: meterName + metricsRequestsTotal,
			Help: "Total number of HTTP client requests",
		},
		[]string{"method", "host", "status", "retry", "error"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: meterName + metricsRequestDuration,
			Help: "HTTP client request duration in seconds",
			Buckets: []float64{
				minDurationBucketSeconds, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60,
			},
		},
		[]string{"method", "host", "status", "attempt"},
	)

	retriesTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: meterName + metricsRetriesTotal,
			Help: "Total number of HTTP client retries",
		},
		[]string{"reason", "method", "host"},
	)

	inflightRequests := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: meterName + metricsInflightRequests,
			Help: "Number of HTTP client requests currently in-flight",
		},
		[]string{"method", "host"},
	)

	requestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: meterName + metricsRequestSizeBytes,
			Help: "HTTP client request size in bytes",
			Buckets: []float64{
				minSizeBucketBytes, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
			},
		},
		[]string{"method", "host"},
	)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: meterName + metricsResponseSizeBytes,
			Help: "HTTP client response size in bytes",
			Buckets: []float64{
				minSizeBucketBytes, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
			},
		},
		[]string{"method", "host", "status"},
	)

	// Регистрируем все метрики
	reg.MustRegister(
		requestsTotal,
		requestDuration,
		retriesTotal,
		inflightRequests,
		requestSize,
		responseSize,
	)

	return &Metrics{
		RequestsTotal:    requestsTotal,
		RequestDuration:  requestDuration,
		RetriesTotal:     retriesTotal,
		InflightRequests: inflightRequests,
		RequestSize:      requestSize,
		ResponseSize:     responseSize,
		registry:         reg,
	}
}

// RecordRequest записывает метрики для запроса.
func (m *Metrics) RecordRequest(_ context.Context, method, host, status string, retry, hasError bool) {
	retryStr := "false"
	if retry {
		retryStr = "true"
	}
	errorStr := "false"
	if hasError {
		errorStr = "true"
	}
	m.RequestsTotal.WithLabelValues(method, host, status, retryStr, errorStr).Inc()
}

// RecordDuration записывает длительность запроса.
func (m *Metrics) RecordDuration(_ context.Context, duration float64, method, host, status string, attempt int) {
	attemptStr := strconv.Itoa(attempt)
	m.RequestDuration.WithLabelValues(method, host, status, attemptStr).Observe(duration)
}

// RecordRetry записывает метрику retry.
func (m *Metrics) RecordRetry(_ context.Context, reason, method, host string) {
	m.RetriesTotal.WithLabelValues(reason, method, host).Inc()
}

// RecordRequestSize записывает размер запроса.
func (m *Metrics) RecordRequestSize(_ context.Context, size int64, method, host string) {
	m.RequestSize.WithLabelValues(method, host).Observe(float64(size))
}

// RecordResponseSize записывает размер ответа.
func (m *Metrics) RecordResponseSize(_ context.Context, size int64, method, host, status string) {
	m.ResponseSize.WithLabelValues(method, host, status).Observe(float64(size))
}

// IncrementInflight увеличивает счётчик активных запросов.
func (m *Metrics) IncrementInflight(_ context.Context, method, host string) {
	m.InflightRequests.WithLabelValues(method, host).Inc()
}

// DecrementInflight уменьшает счётчик активных запросов.
func (m *Metrics) DecrementInflight(_ context.Context, method, host string) {
	m.InflightRequests.WithLabelValues(method, host).Dec()
}

// Close освобождает ресурсы метрик.
func (m *Metrics) Close() error {
	// В Prometheus метриках нет ресурсов для освобождения
	return nil
}

// Registry возвращает registry с зарегистрированными метриками.
// Используется для создания HTTP обработчика метрик.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}
