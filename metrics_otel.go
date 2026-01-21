package httpclient

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// otelInstruments contains a set of OpenTelemetry instruments.
type otelInstruments struct {
	requests  metric.Int64Counter
	retries   metric.Int64Counter
	duration  metric.Float64Histogram
	reqSize   metric.Float64Histogram
	respSize  metric.Float64Histogram
	inflight  metric.Int64UpDownCounter
}

// globalOtelInstruments caches instruments by MeterProvider.
var globalOtelInstruments sync.Map // map[string]*otelInstruments

// OpenTelemetryMetricsProvider is a provider for collecting metrics via OpenTelemetry.
type OpenTelemetryMetricsProvider struct {
	clientName string
	inst       *otelInstruments
}

// NewOpenTelemetryMetricsProvider creates a new OpenTelemetry metrics provider.
func NewOpenTelemetryMetricsProvider(clientName string, mp metric.MeterProvider) *OpenTelemetryMetricsProvider {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	// Use MeterProvider address as cache key
	providerKey := fmt.Sprintf("%p", mp)

	inst, exists := globalOtelInstruments.Load(providerKey)
	if !exists {
		meter := mp.Meter("github.com/rurick/http-client")

		// Create instruments
		requests, _ := meter.Int64Counter(
			MetricRequestsTotal,
			metric.WithDescription("Total number of HTTP client requests"),
		)

		retries, _ := meter.Int64Counter(
			MetricRetriesTotal,
			metric.WithDescription("Total number of HTTP client retries"),
		)

		duration, _ := meter.Float64Histogram(
			MetricRequestDuration,
			metric.WithDescription("HTTP client request duration in seconds"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(DefaultDurationBuckets...),
		)

		reqSize, _ := meter.Float64Histogram(
			MetricRequestSizeBytes,
			metric.WithDescription("HTTP client request size in bytes"),
			metric.WithUnit("By"),
			metric.WithExplicitBucketBoundaries(DefaultSizeBuckets...),
		)

		respSize, _ := meter.Float64Histogram(
			MetricResponseSizeBytes,
			metric.WithDescription("HTTP client response size in bytes"),
			metric.WithUnit("By"),
			metric.WithExplicitBucketBoundaries(DefaultSizeBuckets...),
		)

		inflight, _ := meter.Int64UpDownCounter(
			MetricInflightRequests,
			metric.WithDescription("Number of HTTP client requests currently in-flight"),
		)

		newInst := &otelInstruments{
			requests: requests,
			retries:  retries,
			duration: duration,
			reqSize:  reqSize,
			respSize: respSize,
			inflight: inflight,
		}

		// Store in cache
		globalOtelInstruments.Store(providerKey, newInst)
		inst = newInst
	}

	return &OpenTelemetryMetricsProvider{
		clientName: clientName,
		inst:       inst.(*otelInstruments),
	}
}

// RecordRequest records a request metric.
func (o *OpenTelemetryMetricsProvider) RecordRequest(ctx context.Context, method, host, status string, retry, hasError bool) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
		attribute.String("status", status),
		attribute.Bool("retry", retry),
		attribute.Bool("error", hasError),
	}
	o.inst.requests.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordDuration records request duration.
func (o *OpenTelemetryMetricsProvider) RecordDuration(ctx context.Context, seconds float64, method, host, status string, attempt int) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
		attribute.String("status", status),
		attribute.String("attempt", strconv.Itoa(attempt)),
	}
	o.inst.duration.Record(ctx, seconds, metric.WithAttributes(attrs...))
}

// RecordRetry records a retry attempt metric.
func (o *OpenTelemetryMetricsProvider) RecordRetry(ctx context.Context, reason, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("reason", reason),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.retries.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRequestSize records request size.
func (o *OpenTelemetryMetricsProvider) RecordRequestSize(ctx context.Context, bytes int64, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.reqSize.Record(ctx, float64(bytes), metric.WithAttributes(attrs...))
}

// RecordResponseSize records response size.
func (o *OpenTelemetryMetricsProvider) RecordResponseSize(ctx context.Context, bytes int64, method, host, status string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
		attribute.String("status", status),
	}
	o.inst.respSize.Record(ctx, float64(bytes), metric.WithAttributes(attrs...))
}

// InflightInc increments the active requests counter.
func (o *OpenTelemetryMetricsProvider) InflightInc(ctx context.Context, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.inflight.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// InflightDec decrements the active requests counter.
func (o *OpenTelemetryMetricsProvider) InflightDec(ctx context.Context, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.inflight.Add(ctx, -1, metric.WithAttributes(attrs...))
}

// Close releases resources.
func (o *OpenTelemetryMetricsProvider) Close() error {
	return nil
}