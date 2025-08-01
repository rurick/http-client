# Метрики и мониторинг

HTTP клиент автоматически собирает детальные метрики для мониторинга производительности и надежности. Все метрики экспортируются в формате Prometheus и готовы для интеграции с системами мониторинга.

## 📊 Доступные метрики Prometheus

### Основные HTTP метрики

#### `http_requests_total` (Counter)
**Тип:** Counter  
**Описание:** Общее количество HTTP запросов, выполненных клиентом  
**Лейблы:**
- `method` - HTTP метод (GET, POST, PUT, DELETE, etc.)
- `status_code` - HTTP код ответа (200, 404, 500, etc.)
- `host` - Хост назначения

```prometheus
# Примеры значений
http_requests_total{method="GET",status_code="200",host="api.example.com"} 1245
http_requests_total{method="POST",status_code="201",host="api.example.com"} 89
http_requests_total{method="GET",status_code="404",host="api.example.com"} 12
```

#### `http_request_duration_seconds` (Histogram)
**Тип:** Histogram  
**Описание:** Время выполнения HTTP запросов в секундах  
**Лейблы:**
- `method` - HTTP метод
- `status_code` - HTTP код ответа
- `host` - Хост назначения

**Бакеты:** 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10

```prometheus
# Примеры значений
http_request_duration_seconds_bucket{method="GET",status_code="200",host="api.example.com",le="0.1"} 892
http_request_duration_seconds_bucket{method="GET",status_code="200",host="api.example.com",le="0.5"} 1203
http_request_duration_seconds_sum{method="GET",status_code="200",host="api.example.com"} 156.78
http_request_duration_seconds_count{method="GET",status_code="200",host="api.example.com"} 1245
```

#### `http_request_size_bytes` (Histogram)
**Тип:** Histogram  
**Описание:** Размер тела HTTP запроса в байтах  
**Лейблы:**
- `method` - HTTP метод
- `host` - Хост назначения

**Бакеты:** 64, 256, 1024, 4096, 16384, 65536, 262144, 1048576

#### `http_response_size_bytes` (Histogram)
**Тип:** Histogram  
**Описание:** Размер тела HTTP ответа в байтах  
**Лейблы:**
- `method` - HTTP метод
- `status_code` - HTTP код ответа
- `host` - Хост назначения

**Бакеты:** 64, 256, 1024, 4096, 16384, 65536, 262144, 1048576

### Метрики повторов (Retry)

#### `http_retries_total` (Counter)
**Тип:** Counter  
**Описание:** Общее количество попыток повтора запросов  
**Лейблы:**
- `method` - HTTP метод
- `host` - Хост назначения
- `retry_reason` - Причина повтора (timeout, server_error, network_error)

```prometheus
# Примеры значений
http_retries_total{method="POST",host="api.example.com",retry_reason="timeout"} 23
http_retries_total{method="GET",host="api.example.com",retry_reason="server_error"} 7
```

#### `http_retry_attempts` (Histogram)
**Тип:** Histogram  
**Описание:** Количество попыток для каждого запроса  
**Лейблы:**
- `method` - HTTP метод
- `host` - Хост назначения

**Бакеты:** 1, 2, 3, 4, 5, 10

### Метрики Circuit Breaker

#### `circuit_breaker_state` (Gauge)
**Тип:** Gauge  
**Описание:** Текущее состояние автоматического выключателя  
**Значения:** 0 = Closed, 1 = Open, 2 = Half-Open  
**Лейблы:**
- `circuit_name` - Имя circuit breaker
- `host` - Хост назначения

```prometheus
# Примеры значений
circuit_breaker_state{circuit_name="api_circuit",host="api.example.com"} 0
```

#### `circuit_breaker_failures_total` (Counter)
**Тип:** Counter  
**Описание:** Общее количество неудачных запросов через circuit breaker  
**Лейблы:**
- `circuit_name` - Имя circuit breaker
- `host` - Хост назначения

#### `circuit_breaker_successes_total` (Counter)
**Тип:** Counter  
**Описание:** Общее количество успешных запросов через circuit breaker  
**Лейблы:**
- `circuit_name` - Имя circuit breaker
- `host` - Хост назначения

#### `circuit_breaker_state_changes_total` (Counter)
**Тип:** Counter  
**Описание:** Количество изменений состояния circuit breaker  
**Лейблы:**
- `circuit_name` - Имя circuit breaker
- `from_state` - Предыдущее состояние
- `to_state` - Новое состояние
- `host` - Хост назначения

### Метрики соединений

#### `http_connections_active` (Gauge)
**Тип:** Gauge  
**Описание:** Количество активных HTTP соединений  
**Лейблы:**
- `host` - Хост назначения

#### `http_connections_idle` (Gauge)
**Тип:** Gauge  
**Описание:** Количество неактивных HTTP соединений в пуле  
**Лейблы:**
- `host` - Хост назначения

#### `http_connection_pool_hits_total` (Counter)
**Тип:** Counter  
**Описание:** Количество повторного использования соединений из пула  
**Лейблы:**
- `host` - Хост назначения

#### `http_connection_pool_misses_total` (Counter)
**Тип:** Counter  
**Описание:** Количество случаев создания новых соединений (промах пула)  
**Лейблы:**
- `host` - Хост назначения

### Метрики middleware

#### `middleware_duration_seconds` (Histogram)
**Тип:** Histogram  
**Описание:** Время выполнения middleware в секундах  
**Лейблы:**
- `middleware_name` - Имя middleware (auth, logging, rate_limit, etc.)
- `method` - HTTP метод

**Бакеты:** 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1

#### `middleware_errors_total` (Counter)
**Тип:** Counter  
**Описание:** Количество ошибок в middleware  
**Лейблы:**
- `middleware_name` - Имя middleware
- `error_type` - Тип ошибки

## 🔧 Конфигурация метрик

### Включение метрик
```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true), // Включить сбор метрик
    httpclient.WithTracing(true), // Включить трейсинг
)
```

### Настройка имени для OpenTelemetry инструментов
```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),
    httpclient.WithMetricsMeterName("myapp"), // Установить имя для OpenTelemetry meter/tracer
)
```

При установке имени "myapp" оно будет использоваться для идентификации OpenTelemetry meter и tracer, а имена метрик остаются стандартными:
- `http_requests_total` 
- `http_request_duration_seconds`
- `circuit_breaker_state`
- `http_retries_total`
- И так далее

**Важно:** Префикс передается в `otel.Meter(metricsName)` и `otel.Tracer(metricsName)` для идентификации источника метрик.

**По умолчанию:** имя "httpclient"

### Примеры настройки имен для разных сервисов
```go
// API Gateway
apiClient, _ := httpclient.NewClient(
    httpclient.WithMetricsMeterName("api_gateway"), // Имя для OpenTelemetry инструментов
)

// User Service
userClient, _ := httpclient.NewClient(
    httpclient.WithMetricsMeterName("user_service"), // Имя для OpenTelemetry инструментов
)

// Payment Service
paymentClient, _ := httpclient.NewClient(
    httpclient.WithMetricsMeterName("payment_svc"), // Имя для OpenTelemetry инструментов
)
```

### Отключение метрик
```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(false), // Отключить сбор метрик
)
```

## 📈 Полезные Prometheus запросы

### Общая производительность
```promql
# Средняя латентность запросов
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# QPS (запросов в секунду)
rate(http_requests_total[5m])

# Процент ошибок
rate(http_requests_total{status_code!~"2.."}[5m]) / rate(http_requests_total[5m]) * 100
```

### Circuit Breaker мониторинг
```promql
# Количество открытых circuit breaker
sum(circuit_breaker_state == 1) by (circuit_name)

# Частота изменения состояний
rate(circuit_breaker_state_changes_total[5m])
```

### Производительность соединений
```promql
# Использование пула соединений
http_connections_active / (http_connections_active + http_connections_idle) * 100

# Эффективность пула соединений
rate(http_connection_pool_hits_total[5m]) / (rate(http_connection_pool_hits_total[5m]) + rate(http_connection_pool_misses_total[5m])) * 100
```

## 🎯 Рекомендуемые алерты

### Критические алерты
```yaml
# Высокий процент ошибок
- alert: HighErrorRate
  expr: rate(http_requests_total{status_code!~"2.."}[5m]) / rate(http_requests_total[5m]) > 0.05
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "Высокий процент ошибок HTTP запросов"

# Высокая латентность
- alert: HighLatency
  expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 2
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Высокая латентность HTTP запросов"
```

### Предупреждающие алерты
```yaml
# Circuit breaker открыт
- alert: CircuitBreakerOpen
  expr: circuit_breaker_state == 1
  for: 1m
  labels:
    severity: warning
  annotations:
    summary: "Circuit breaker открыт для {{ $labels.host }}"

# Много повторов
- alert: HighRetryRate
  expr: rate(http_retries_total[5m]) > 10
  for: 3m
  labels:
    severity: warning
  annotations:
    summary: "Высокая частота повторов запросов"
```

## 📊 Дашборд Grafana

Для визуализации метрик рекомендуется создать дашборд в Grafana со следующими панелями:

1. **Обзор** - QPS, латентность, процент ошибок
2. **Детали запросов** - распределение по методам и кодам ответов
3. **Circuit Breaker** - состояния и переключения
4. **Повторы** - статистика retry попыток
5. **Соединения** - использование пула соединений
6. **Middleware** - производительность промежуточного ПО

## 🔍 Отладка производительности

### Типичные проблемы и их диагностика

1. **Высокая латентность**
   - Проверить `http_request_duration_seconds` percentiles
   - Анализировать `middleware_duration_seconds`

2. **Проблемы с соединениями**
   - Мониторить `http_connections_active` vs `http_connections_idle`
   - Проверить `http_connection_pool_misses_total`

3. **Частые повторы**
   - Анализировать `http_retries_total` по `retry_reason`
   - Проверить состояние `circuit_breaker_state`

Все метрики автоматически экспортируются через OpenTelemetry и готовы для интеграции с Prometheus без дополнительной настройки.

## 📝 Константы метрик в коде

Все имена метрик вынесены в константы в файле `metrics.go` для удобства использования:

```go
// Основные HTTP метрики
const (
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
```

Эти константы используются внутри библиотеки и обеспечивают единообразие имен метрик.