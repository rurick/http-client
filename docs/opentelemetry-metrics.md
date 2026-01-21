# OpenTelemetry Metrics Support

The HTTP client now supports metrics collection via OpenTelemetry in addition to Prometheus.

## Quick Start

### Prometheus

```go
client := httpclient.New(httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendPrometheus,
}, "my-client")
```

### OpenTelemetry (Default)

```go
import "go.opentelemetry.io/otel/metric"

// Create client with OpenTelemetry metrics (default)
client := httpclient.New(httpclient.Config{
    // MetricsBackend omitted - "otel" used by default
    // OTelMeterProvider optional - defaults to otel.GetMeterProvider()
}, "my-client")
```

### Disabling Metrics

```go
// Completely disable metrics collection
enabled := false
client := httpclient.New(httpclient.Config{
    MetricsEnabled: &enabled,
}, "my-client")
```

## Configuration with Custom MeterProvider

```go
import (
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/exporters/prometheus"
)

// Create custom MeterProvider
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}

meterProvider := sdkmetric.NewMeterProvider(
    sdkmetric.WithReader(exporter),
)
defer meterProvider.Shutdown(context.Background())

// Use custom MeterProvider
client := httpclient.New(httpclient.Config{
    MetricsBackend:    httpclient.MetricsBackendOpenTelemetry,
    OTelMeterProvider: meterProvider,
}, "my-client")
```

## Histogram Buckets

The library automatically sets buckets for all histograms in both providers (Prometheus and OpenTelemetry), ensuring metric consistency.

**Buckets for request duration** (`http_client_request_duration_seconds`):
`0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60` seconds

**Buckets for sizes** (`http_client_request_size_bytes`, `http_client_response_size_bytes`):
`256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216` bytes

## Custom Prometheus Registry

```go
import "github.com/prometheus/client_golang/prometheus"

// Create custom Prometheus registry
customRegistry := prometheus.NewRegistry()

client := httpclient.New(httpclient.Config{
    MetricsBackend:       httpclient.MetricsBackendPrometheus, // explicitly specify prometheus
    PrometheusRegisterer: customRegistry,
}, "my-client")

// Use custom registry for metrics
http.Handle("/custom-metrics", promhttp.HandlerFor(
    customRegistry, 
    promhttp.HandlerOpts{},
))
```

## Available Metrics

All metrics have the same names in both providers:

### Counters
- `http_client_requests_total` - total number of requests
- `http_client_retries_total` - number of retry attempts

### Гистограммы (Histograms)
- `http_client_request_duration_seconds` - длительность запросов
- `http_client_request_size_bytes` - размер запросов  
- `http_client_response_size_bytes` - размер ответов

### Датчики (Gauges/UpDownCounters)
- `http_client_inflight_requests` - количество активных запросов

## Лейблы/Атрибуты

Все метрики включают следующие лейблы (Prometheus) или атрибуты (OpenTelemetry):

- `client_name` - имя клиента, заданное при создании
- `method` - HTTP метод (GET, POST, и т.д.)
- `host` - хост назначения
- `status` - HTTP статус код ответа
- `retry` - флаг повторной попытки (true/false)  
- `error` - флаг ошибки (true/false)
- `attempt` - номер попытки для метрик длительности

## Примеры

Полные рабочие примеры доступны в:
- `examples/otel_metrics/` - использование OpenTelemetry
- `examples/custom_prometheus/` - кастомный Prometheus registry
- `examples/metrics/` - стандартные Prometheus метрики

## Миграция

OpenTelemetry используется по умолчанию. Для переключения на Prometheus укажите:

```go
config.MetricsBackend = httpclient.MetricsBackendPrometheus
```

API клиента остается неизменным независимо от выбранного провайдера метрик.