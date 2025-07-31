package httpclient

import (
        "context"
        "sync"
        "time"

        "go.opentelemetry.io/otel"
        "go.opentelemetry.io/otel/attribute"
        "go.opentelemetry.io/otel/metric"
        "go.opentelemetry.io/otel/trace"
)

// Константы имен метрик для Prometheus
const (
        // Основные HTTP метрики
        MetricHTTPRequestsTotal        = "http_requests_total"
        MetricHTTPRequestDuration      = "http_request_duration_seconds"
        MetricHTTPRequestSize          = "http_request_size_bytes"
        MetricHTTPResponseSize         = "http_response_size_bytes"
        
        // Метрики повторов (Retry)
        MetricHTTPRetriesTotal         = "http_retries_total"
        MetricHTTPRetryAttempts        = "http_retry_attempts"
        
        // Метрики Circuit Breaker
        MetricCircuitBreakerState      = "circuit_breaker_state"
        MetricCircuitBreakerFailures   = "circuit_breaker_failures_total"
        MetricCircuitBreakerSuccesses  = "circuit_breaker_successes_total"
        MetricCircuitBreakerStateChanges = "circuit_breaker_state_changes_total"
        
        // Метрики соединений
        MetricHTTPConnectionsActive    = "http_connections_active"
        MetricHTTPConnectionsIdle      = "http_connections_idle"
        MetricHTTPConnectionPoolHits   = "http_connection_pool_hits_total"
        MetricHTTPConnectionPoolMisses = "http_connection_pool_misses_total"
        
        // Метрики middleware
        MetricMiddlewareDuration       = "middleware_duration_seconds"
        MetricMiddlewareErrors         = "middleware_errors_total"
)

// ClientMetrics holds all metrics for the HTTP client
type ClientMetrics struct {
        // Request metrics
        TotalRequests  int64
        SuccessfulReqs int64
        FailedRequests int64

        // Retry metrics
        TotalRetries   int64
        RetrySuccesses int64
        RetryFailures  int64

        // Timing metrics
        TotalLatency   time.Duration
        AverageLatency time.Duration
        MinLatency     time.Duration
        MaxLatency     time.Duration

        // Size metrics
        TotalRequestSize  int64
        TotalResponseSize int64

        // Status code distribution
        StatusCodes map[int]int64

        // Circuit breaker metrics
        CircuitBreakerState string
        CircuitBreakerTrips int64

        mu sync.RWMutex
}

// NewClientMetrics creates a new ClientMetrics instance
func NewClientMetrics() *ClientMetrics {
        return &ClientMetrics{
                StatusCodes: make(map[int]int64),
                MinLatency:  time.Duration(^uint64(0) >> 1), // Max duration as initial min
        }
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

// RecordRequest records metrics for an HTTP request
func (omc *OTelMetricsCollector) RecordRequest(method, url string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
        ctx := context.Background()

        // Update internal metrics
        omc.metrics.mu.Lock()
        omc.metrics.TotalRequests++
        if statusCode >= 200 && statusCode < 400 {
                omc.metrics.SuccessfulReqs++
        } else {
                omc.metrics.FailedRequests++
        }

        omc.metrics.TotalLatency += duration
        omc.metrics.AverageLatency = time.Duration(int64(omc.metrics.TotalLatency) / omc.metrics.TotalRequests)

        if duration < omc.metrics.MinLatency {
                omc.metrics.MinLatency = duration
        }
        if duration > omc.metrics.MaxLatency {
                omc.metrics.MaxLatency = duration
        }

        omc.metrics.TotalRequestSize += requestSize
        omc.metrics.TotalResponseSize += responseSize
        omc.metrics.StatusCodes[statusCode]++
        omc.metrics.mu.Unlock()

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

        // Update internal metrics
        omc.metrics.mu.Lock()
        omc.metrics.TotalRetries++
        if err == nil {
                omc.metrics.RetrySuccesses++
        } else {
                omc.metrics.RetryFailures++
        }
        omc.metrics.mu.Unlock()

        // Record OpenTelemetry metrics
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
        omc.metrics.mu.Lock()
        omc.metrics.CircuitBreakerState = state.String()
        if state == CircuitBreakerOpen {
                omc.metrics.CircuitBreakerTrips++
        }
        omc.metrics.mu.Unlock()
}

// GetMetrics returns a copy of the current metrics
func (omc *OTelMetricsCollector) GetMetrics() *ClientMetrics {
        omc.metrics.mu.RLock()
        defer omc.metrics.mu.RUnlock()

        // Create a deep copy
        metrics := &ClientMetrics{
                TotalRequests:       omc.metrics.TotalRequests,
                SuccessfulReqs:      omc.metrics.SuccessfulReqs,
                FailedRequests:      omc.metrics.FailedRequests,
                TotalRetries:        omc.metrics.TotalRetries,
                RetrySuccesses:      omc.metrics.RetrySuccesses,
                RetryFailures:       omc.metrics.RetryFailures,
                TotalLatency:        omc.metrics.TotalLatency,
                AverageLatency:      omc.metrics.AverageLatency,
                MinLatency:          omc.metrics.MinLatency,
                MaxLatency:          omc.metrics.MaxLatency,
                TotalRequestSize:    omc.metrics.TotalRequestSize,
                TotalResponseSize:   omc.metrics.TotalResponseSize,
                CircuitBreakerState: omc.metrics.CircuitBreakerState,
                CircuitBreakerTrips: omc.metrics.CircuitBreakerTrips,
                StatusCodes:         make(map[int]int64),
        }

        // Копируем status codes
        for code, count := range omc.metrics.StatusCodes {
                metrics.StatusCodes[code] = count
        }

        return metrics
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

// GetStatusCodes возвращает копию статус кодов
func (cm *ClientMetrics) GetStatusCodes() map[int]int64 {
        cm.mu.RLock()
        defer cm.mu.RUnlock()

        result := make(map[int]int64)
        for code, count := range cm.StatusCodes {
                result[code] = count
        }
        return result
}
