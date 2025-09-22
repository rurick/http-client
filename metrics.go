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

	// Имена метрик.
	metricsRequestsTotal     = "http_client_requests_total"
	metricsRequestDuration   = "http_client_request_duration_seconds"
	metricsRetriesTotal      = "http_client_retries_total"
	metricsInflightRequests  = "http_client_inflight_requests"
	metricsRequestSizeBytes  = "http_client_request_size_bytes"
	metricsResponseSizeBytes = "http_client_response_size_bytes"
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

	clientName string
	registry   *prometheus.Registry
}

// NewMetrics создаёт новый экземпляр метрик.
func NewMetrics(meterName string) *Metrics {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricsRequestsTotal,
			Help: "Total number of HTTP client requests",
		},
		[]string{"client_name", "method", "host", "status", "retry", "error"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: metricsRequestDuration,
			Help: "HTTP client request duration in seconds",
			Buckets: []float64{
				minDurationBucketSeconds, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60,
			},
		},
		[]string{"client_name", "method", "host", "status", "attempt"},
	)

	retriesTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricsRetriesTotal,
			Help: "Total number of HTTP client retries",
		},
		[]string{"client_name", "reason", "method", "host"},
	)

	inflightRequests := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricsInflightRequests,
			Help: "Number of HTTP client requests currently in-flight",
		},
		[]string{"client_name", "method", "host"},
	)

	requestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: metricsRequestSizeBytes,
			Help: "HTTP client request size in bytes",
			Buckets: []float64{
				minSizeBucketBytes, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
			},
		},
		[]string{"client_name", "method", "host"},
	)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: metricsResponseSizeBytes,
			Help: "HTTP client response size in bytes",
			Buckets: []float64{
				minSizeBucketBytes, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
			},
		},
		[]string{"client_name", "method", "host", "status"},
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
		clientName:       meterName,
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
	m.RequestsTotal.WithLabelValues(m.clientName, method, host, status, retryStr, errorStr).Inc()
}

// RecordDuration записывает длительность запроса.
func (m *Metrics) RecordDuration(_ context.Context, duration float64, method, host, status string, attempt int) {
	attemptStr := strconv.Itoa(attempt)
	m.RequestDuration.WithLabelValues(m.clientName, method, host, status, attemptStr).Observe(duration)
}

// RecordRetry записывает метрику retry.
func (m *Metrics) RecordRetry(_ context.Context, reason, method, host string) {
	m.RetriesTotal.WithLabelValues(m.clientName, reason, method, host).Inc()
}

// RecordRequestSize записывает размер запроса.
func (m *Metrics) RecordRequestSize(_ context.Context, size int64, method, host string) {
	m.RequestSize.WithLabelValues(m.clientName, method, host).Observe(float64(size))
}

// RecordResponseSize записывает размер ответа.
func (m *Metrics) RecordResponseSize(_ context.Context, size int64, method, host, status string) {
	m.ResponseSize.WithLabelValues(m.clientName, method, host, status).Observe(float64(size))
}

// IncrementInflight увеличивает счётчик активных запросов.
func (m *Metrics) IncrementInflight(_ context.Context, method, host string) {
	m.InflightRequests.WithLabelValues(m.clientName, method, host).Inc()
}

// DecrementInflight уменьшает счётчик активных запросов.
func (m *Metrics) DecrementInflight(_ context.Context, method, host string) {
	m.InflightRequests.WithLabelValues(m.clientName, method, host).Dec()
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
