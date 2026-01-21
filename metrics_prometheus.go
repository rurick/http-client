package httpclient

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// prometheusGlobalMetrics contains global Prometheus metric vectors.
type prometheusGlobalMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RetriesTotal     *prometheus.CounterVec
	InflightRequests *prometheus.GaugeVec
	RequestSize      *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec
}

// globalPrometheusMetrics caches registered metrics by registerer.
var globalPrometheusMetrics sync.Map // map[string]*prometheusGlobalMetrics

// PrometheusMetricsProvider is a provider for collecting metrics via Prometheus.
type PrometheusMetricsProvider struct {
	clientName string
	metrics    *prometheusGlobalMetrics
}

// NewPrometheusMetricsProvider creates a new Prometheus metrics provider.
func NewPrometheusMetricsProvider(clientName string, reg prometheus.Registerer) *PrometheusMetricsProvider {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	// Use registerer address as cache key
	registryKey := fmt.Sprintf("%p", reg)

	metrics, exists := globalPrometheusMetrics.Load(registryKey)
	if !exists {
		// Create and register metrics
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

		// Register all metrics
		reg.MustRegister(
			newMetrics.RequestsTotal,
			newMetrics.RequestDuration,
			newMetrics.RetriesTotal,
			newMetrics.InflightRequests,
			newMetrics.RequestSize,
			newMetrics.ResponseSize,
		)

		// Store in cache
		globalPrometheusMetrics.Store(registryKey, newMetrics)
		metrics = newMetrics
	}

	return &PrometheusMetricsProvider{
		clientName: clientName,
		metrics:    metrics.(*prometheusGlobalMetrics),
	}
}

// RecordRequest records a request metric.
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

// RecordDuration records request duration.
func (p *PrometheusMetricsProvider) RecordDuration(_ context.Context, seconds float64, method, host, status string, attempt int) {
	attemptStr := strconv.Itoa(attempt)
	p.metrics.RequestDuration.WithLabelValues(p.clientName, method, host, status, attemptStr).Observe(seconds)
}

// RecordRetry records a retry attempt metric.
func (p *PrometheusMetricsProvider) RecordRetry(_ context.Context, reason, method, host string) {
	p.metrics.RetriesTotal.WithLabelValues(p.clientName, reason, method, host).Inc()
}

// RecordRequestSize records request size.
func (p *PrometheusMetricsProvider) RecordRequestSize(_ context.Context, bytes int64, method, host string) {
	p.metrics.RequestSize.WithLabelValues(p.clientName, method, host).Observe(float64(bytes))
}

// RecordResponseSize records response size.
func (p *PrometheusMetricsProvider) RecordResponseSize(_ context.Context, bytes int64, method, host, status string) {
	p.metrics.ResponseSize.WithLabelValues(p.clientName, method, host, status).Observe(float64(bytes))
}

// InflightInc increments the active requests counter.
func (p *PrometheusMetricsProvider) InflightInc(_ context.Context, method, host string) {
	p.metrics.InflightRequests.WithLabelValues(p.clientName, method, host).Inc()
}

// InflightDec decrements the active requests counter.
func (p *PrometheusMetricsProvider) InflightDec(_ context.Context, method, host string) {
	p.metrics.InflightRequests.WithLabelValues(p.clientName, method, host).Dec()
}

// Close releases resources.
func (p *PrometheusMetricsProvider) Close() error {
	return nil
}