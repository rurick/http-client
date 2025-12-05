package httpclient

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// prometheusGlobalMetrics содержит глобальные векторы метрик Prometheus.
type prometheusGlobalMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RetriesTotal     *prometheus.CounterVec
	InflightRequests *prometheus.GaugeVec
	RequestSize      *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec
}

// globalPrometheusMetrics кеширует зарегистрированные метрики по регистратору.
var globalPrometheusMetrics sync.Map // map[string]*prometheusGlobalMetrics

// PrometheusMetricsProvider - провайдер для сбора метрик через Prometheus.
type PrometheusMetricsProvider struct {
	clientName string
	metrics    *prometheusGlobalMetrics
}

// NewPrometheusMetricsProvider создает новый провайдер метрик Prometheus.
func NewPrometheusMetricsProvider(clientName string, reg prometheus.Registerer) *PrometheusMetricsProvider {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	// Используем адрес регистратора как ключ кеша
	registryKey := fmt.Sprintf("%p", reg)

	metrics, exists := globalPrometheusMetrics.Load(registryKey)
	if !exists {
		// Создаем и регистрируем метрики
		newMetrics := &prometheusGlobalMetrics{
			RequestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: MetricRequestsTotal,
					Help: "Total number of HTTP client requests",
				},
				[]string{"client_name", "method", "host", "status", "retry", "error"},
			),
			RequestDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    MetricRequestDuration,
					Help:    "HTTP client request duration in seconds",
					Buckets: DefaultDurationBuckets,
				},
				[]string{"client_name", "method", "host", "status", "attempt"},
			),
			RetriesTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: MetricRetriesTotal,
					Help: "Total number of HTTP client retries",
				},
				[]string{"client_name", "reason", "method", "host"},
			),
			InflightRequests: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: MetricInflightRequests,
					Help: "Number of HTTP client requests currently in-flight",
				},
				[]string{"client_name", "method", "host"},
			),
			RequestSize: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    MetricRequestSizeBytes,
					Help:    "HTTP client request size in bytes",
					Buckets: DefaultSizeBuckets,
				},
				[]string{"client_name", "method", "host"},
			),
			ResponseSize: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    MetricResponseSizeBytes,
					Help:    "HTTP client response size in bytes",
					Buckets: DefaultSizeBuckets,
				},
				[]string{"client_name", "method", "host", "status"},
			),
		}

		// Регистрируем все метрики
		reg.MustRegister(
			newMetrics.RequestsTotal,
			newMetrics.RequestDuration,
			newMetrics.RetriesTotal,
			newMetrics.InflightRequests,
			newMetrics.RequestSize,
			newMetrics.ResponseSize,
		)

		// Сохраняем в кеше
		globalPrometheusMetrics.Store(registryKey, newMetrics)
		metrics = newMetrics
	}

	return &PrometheusMetricsProvider{
		clientName: clientName,
		metrics:    metrics.(*prometheusGlobalMetrics),
	}
}

// RecordRequest записывает метрику запроса.
func (p *PrometheusMetricsProvider) RecordRequest(_ context.Context, method, host, status string, retry, hasError bool) {
	retryStr := "false"
	if retry {
		retryStr = "true"
	}
	errorStr := "false"
	if hasError {
		errorStr = "true"
	}
	p.metrics.RequestsTotal.WithLabelValues(p.clientName, method, host, status, retryStr, errorStr).Inc()
}

// RecordDuration записывает длительность запроса.
func (p *PrometheusMetricsProvider) RecordDuration(_ context.Context, seconds float64, method, host, status string, attempt int) {
	attemptStr := strconv.Itoa(attempt)
	p.metrics.RequestDuration.WithLabelValues(p.clientName, method, host, status, attemptStr).Observe(seconds)
}

// RecordRetry записывает метрику повторной попытки.
func (p *PrometheusMetricsProvider) RecordRetry(_ context.Context, reason, method, host string) {
	p.metrics.RetriesTotal.WithLabelValues(p.clientName, reason, method, host).Inc()
}

// RecordRequestSize записывает размер запроса.
func (p *PrometheusMetricsProvider) RecordRequestSize(_ context.Context, bytes int64, method, host string) {
	p.metrics.RequestSize.WithLabelValues(p.clientName, method, host).Observe(float64(bytes))
}

// RecordResponseSize записывает размер ответа.
func (p *PrometheusMetricsProvider) RecordResponseSize(_ context.Context, bytes int64, method, host, status string) {
	p.metrics.ResponseSize.WithLabelValues(p.clientName, method, host, status).Observe(float64(bytes))
}

// InflightInc увеличивает счетчик активных запросов.
func (p *PrometheusMetricsProvider) InflightInc(_ context.Context, method, host string) {
	p.metrics.InflightRequests.WithLabelValues(p.clientName, method, host).Inc()
}

// InflightDec уменьшает счетчик активных запросов.
func (p *PrometheusMetricsProvider) InflightDec(_ context.Context, method, host string) {
	p.metrics.InflightRequests.WithLabelValues(p.clientName, method, host).Dec()
}

// Close освобождает ресурсы.
func (p *PrometheusMetricsProvider) Close() error {
	return nil
}