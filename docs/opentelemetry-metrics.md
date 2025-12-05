# OpenTelemetry Metrics Support

HTTP-клиент теперь поддерживает сбор метрик через OpenTelemetry в дополнение к Prometheus.

## Быстрый старт

### Prometheus

```go
client := httpclient.New(httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendPrometheus,
}, "my-client")
```

### OpenTelemetry (по умолчанию)

```go
import "go.opentelemetry.io/otel/metric"

// Создание клиента с OpenTelemetry метриками (по умолчанию)
client := httpclient.New(httpclient.Config{
    // MetricsBackend опускается - используется "otel" по умолчанию
    // OTelMeterProvider опционально - по умолчанию otel.GetMeterProvider()
}, "my-client")
```

### Отключение метрик

```go
// Полное отключение сбора метрик
enabled := false
client := httpclient.New(httpclient.Config{
    MetricsEnabled: &enabled,
}, "my-client")
```

## Настройка с кастомным MeterProvider

```go
import (
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/exporters/prometheus"
)

// Создание кастомного MeterProvider
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}

meterProvider := sdkmetric.NewMeterProvider(
    sdkmetric.WithReader(exporter),
)
defer meterProvider.Shutdown(context.Background())

// Использование кастомного MeterProvider
client := httpclient.New(httpclient.Config{
    MetricsBackend:    httpclient.MetricsBackendOpenTelemetry,
    OTelMeterProvider: meterProvider,
}, "my-client")
```

## Бакеты гистограмм

Библиотека автоматически задает бакеты для всех гистограмм в обоих провайдерах (Prometheus и OpenTelemetry), обеспечивая согласованность метрик.

**Бакеты для длительности запросов** (`http_client_request_duration_seconds`):
`0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60` секунд

**Бакеты для размеров** (`http_client_request_size_bytes`, `http_client_response_size_bytes`):
`256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216` байт

## Кастомный Prometheus Registry

```go
import "github.com/prometheus/client_golang/prometheus"

// Создание кастомного Prometheus registry
customRegistry := prometheus.NewRegistry()

client := httpclient.New(httpclient.Config{
    MetricsBackend:       httpclient.MetricsBackendPrometheus, // явно указываем prometheus
    PrometheusRegisterer: customRegistry,
}, "my-client")

// Использование кастомного registry для метрик
http.Handle("/custom-metrics", promhttp.HandlerFor(
    customRegistry, 
    promhttp.HandlerOpts{},
))
```

## Доступные метрики

Все метрики имеют одинаковые имена в обоих провайдерах:

### Счетчики (Counters)
- `http_client_requests_total` - общее количество запросов
- `http_client_retries_total` - количество повторных попыток

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