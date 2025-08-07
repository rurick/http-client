package httpclient

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Константы имен метрик для Prometheus
const (
	// Основные HTTP метрики
	MetricHTTPRequestsTotal   = "http_requests_total"
	MetricHTTPRequestDuration = "http_request_duration_seconds"
	MetricHTTPRequestSize     = "http_request_size_bytes"
	MetricHTTPResponseSize    = "http_response_size_bytes"

	// Метрики повторов (Retry)
	MetricHTTPRetriesTotal  = "http_retries_total"
	MetricHTTPRetryAttempts = "http_retry_attempts"

	// Метрики Circuit Breaker
	MetricCircuitBreakerState        = "circuit_breaker_state"
	MetricCircuitBreakerFailures     = "circuit_breaker_failures_total"
	MetricCircuitBreakerSuccesses    = "circuit_breaker_successes_total"
	MetricCircuitBreakerStateChanges = "circuit_breaker_state_changes_total"

	// Метрики соединений
	MetricHTTPConnectionsActive    = "http_connections_active"
	MetricHTTPConnectionsIdle      = "http_connections_idle"
	MetricHTTPConnectionPoolHits   = "http_connection_pool_hits_total"
	MetricHTTPConnectionPoolMisses = "http_connection_pool_misses_total"

	// Метрики middleware
	MetricMiddlewareDuration = "middleware_duration_seconds"
	MetricMiddlewareErrors   = "middleware_errors_total"
)

// ClientMetrics holds basic metrics interface
// All detailed metrics are collected via OpenTelemetry
type ClientMetrics struct {
	TotalRequests  int64
	SuccessfulReqs int64
	FailedRequests int64
	AverageLatency time.Duration
}

// NewClientMetrics creates a new ClientMetrics instance
func NewClientMetrics() *ClientMetrics {
	return &ClientMetrics{}
}

// OTelMetricsCollector implements MetricsCollector using OpenTelemetry
type OTelMetricsCollector struct {
	meter   metric.Meter
	tracer  trace.Tracer
	metrics *ClientMetrics

	// OpenTelemetry instruments
	requestCounter      metric.Int64Counter
	requestDuration     metric.Float64Histogram
	requestSizeCounter  metric.Int64Counter
	responseSizeCounter metric.Int64Counter
	retryCounter        metric.Int64Counter
}

// NewOTelMetricsCollector creates a new OpenTelemetry metrics collector
func NewOTelMetricsCollector(meterName string) (*OTelMetricsCollector, error) {
	meter := otel.Meter(meterName)
	tracer := otel.Tracer(meterName)

	// Create instruments using constants
	requestCounter, err := meter.Int64Counter(
		MetricHTTPRequestsTotal,
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		MetricHTTPRequestDuration,
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60),
	)
	if err != nil {
		return nil, err
	}

	requestSizeCounter, err := meter.Int64Counter(
		MetricHTTPRequestSize,
		metric.WithDescription("Size of HTTP requests in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	responseSizeCounter, err := meter.Int64Counter(
		MetricHTTPResponseSize,
		metric.WithDescription("Size of HTTP responses in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	retryCounter, err := meter.Int64Counter(
		MetricHTTPRetriesTotal,
		metric.WithDescription("Total number of HTTP request retries"),
	)
	if err != nil {
		return nil, err
	}

	return &OTelMetricsCollector{
		meter:               meter,
		tracer:              tracer,
		metrics:             NewClientMetrics(),
		requestCounter:      requestCounter,
		requestDuration:     requestDuration,
		requestSizeCounter:  requestSizeCounter,
		responseSizeCounter: responseSizeCounter,
		retryCounter:        retryCounter,
	}, nil
}

// GetMeter returns the OpenTelemetry meter for external use
func (omc *OTelMetricsCollector) GetMeter() metric.Meter {
	return omc.meter
}

// RecordRequest records metrics for an HTTP request
func (omc *OTelMetricsCollector) RecordRequest(method, url string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	ctx := context.Background()

	// Update basic internal metrics
	omc.metrics.TotalRequests++
	if statusCode >= 200 && statusCode < 400 {
		omc.metrics.SuccessfulReqs++
	} else {
		omc.metrics.FailedRequests++
	}

	// Simple average calculation
	if omc.metrics.TotalRequests > 0 {
		omc.metrics.AverageLatency = (omc.metrics.AverageLatency*time.Duration(omc.metrics.TotalRequests-1) + duration) / time.Duration(omc.metrics.TotalRequests)
	}

	// Record OpenTelemetry metrics
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("url", url),
		attribute.Int("status_code", statusCode),
	}

	omc.requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	omc.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if requestSize > 0 {
		omc.requestSizeCounter.Add(ctx, requestSize, metric.WithAttributes(attrs...))
	}

	if responseSize > 0 {
		omc.responseSizeCounter.Add(ctx, responseSize, metric.WithAttributes(attrs...))
	}
}

// RecordRetry records metrics for retry attempts
func (omc *OTelMetricsCollector) RecordRetry(method, url string, attempt int, err error) {
	ctx := context.Background()

	// Record OpenTelemetry metrics only
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("url", url),
		attribute.Int("attempt", attempt),
		attribute.Bool("success", err == nil),
	}

	omc.retryCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordCircuitBreakerState records circuit breaker state changes
func (omc *OTelMetricsCollector) RecordCircuitBreakerState(state CircuitBreakerState) {
	// OpenTelemetry only - no internal state tracking
}

// GetMetrics returns a copy of the current metrics
func (omc *OTelMetricsCollector) GetMetrics() *ClientMetrics {
	return &ClientMetrics{
		TotalRequests:  omc.metrics.TotalRequests,
		SuccessfulReqs: omc.metrics.SuccessfulReqs,
		FailedRequests: omc.metrics.FailedRequests,
		AverageLatency: omc.metrics.AverageLatency,
	}
}

// StartSpan starts a new trace span for HTTP request
func (omc *OTelMetricsCollector) StartSpan(ctx context.Context, method, url string) (context.Context, trace.Span) {
	return omc.tracer.Start(ctx, "http_client_request",
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", url),
		),
	)
}

// FinishSpan finishes a trace span with response information
func (omc *OTelMetricsCollector) FinishSpan(span trace.Span, statusCode int, err error) {
	span.SetAttributes(attribute.Int("http.status_code", statusCode))

	if err != nil {
		span.RecordError(err)
	}

	span.End()
}
