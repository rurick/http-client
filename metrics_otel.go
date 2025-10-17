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

// otelInstruments содержит набор инструментов OpenTelemetry.
type otelInstruments struct {
	requests  metric.Int64Counter
	retries   metric.Int64Counter
	duration  metric.Float64Histogram
	reqSize   metric.Float64Histogram
	respSize  metric.Float64Histogram
	inflight  metric.Int64UpDownCounter
}

// globalOtelInstruments кеширует инструменты по MeterProvider.
var globalOtelInstruments sync.Map // map[string]*otelInstruments

// OpenTelemetryMetricsProvider - провайдер для сбора метрик через OpenTelemetry.
type OpenTelemetryMetricsProvider struct {
	clientName string
	inst       *otelInstruments
}

// NewOpenTelemetryMetricsProvider создает новый провайдер метрик OpenTelemetry.
func NewOpenTelemetryMetricsProvider(clientName string, mp metric.MeterProvider) *OpenTelemetryMetricsProvider {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	// Используем адрес MeterProvider как ключ кеша
	providerKey := fmt.Sprintf("%p", mp)

	inst, exists := globalOtelInstruments.Load(providerKey)
	if !exists {
		meter := mp.Meter("github.com/rurick/http-client")

		// Создаем инструменты
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
		)

		reqSize, _ := meter.Float64Histogram(
			MetricRequestSizeBytes,
			metric.WithDescription("HTTP client request size in bytes"),
			metric.WithUnit("By"),
		)

		respSize, _ := meter.Float64Histogram(
			MetricResponseSizeBytes,
			metric.WithDescription("HTTP client response size in bytes"),
			metric.WithUnit("By"),
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

		// Сохраняем в кеше
		globalOtelInstruments.Store(providerKey, newInst)
		inst = newInst
	}

	return &OpenTelemetryMetricsProvider{
		clientName: clientName,
		inst:       inst.(*otelInstruments),
	}
}

// RecordRequest записывает метрику запроса.
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

// RecordDuration записывает длительность запроса.
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

// RecordRetry записывает метрику повторной попытки.
func (o *OpenTelemetryMetricsProvider) RecordRetry(ctx context.Context, reason, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("reason", reason),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.retries.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRequestSize записывает размер запроса.
func (o *OpenTelemetryMetricsProvider) RecordRequestSize(ctx context.Context, bytes int64, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.reqSize.Record(ctx, float64(bytes), metric.WithAttributes(attrs...))
}

// RecordResponseSize записывает размер ответа.
func (o *OpenTelemetryMetricsProvider) RecordResponseSize(ctx context.Context, bytes int64, method, host, status string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
		attribute.String("status", status),
	}
	o.inst.respSize.Record(ctx, float64(bytes), metric.WithAttributes(attrs...))
}

// InflightInc увеличивает счетчик активных запросов.
func (o *OpenTelemetryMetricsProvider) InflightInc(ctx context.Context, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.inflight.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// InflightDec уменьшает счетчик активных запросов.
func (o *OpenTelemetryMetricsProvider) InflightDec(ctx context.Context, method, host string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_name", o.clientName),
		attribute.String("method", method),
		attribute.String("host", host),
	}
	o.inst.inflight.Add(ctx, -1, metric.WithAttributes(attrs...))
}

// Close освобождает ресурсы.
func (o *OpenTelemetryMetricsProvider) Close() error {
	return nil
}