# Метрики и мониторинг

HTTP клиент автоматически собирает детальные метрики через OpenTelemetry для мониторинга производительности и надежности. Все метрики экспортируются в формате Prometheus и готовы для интеграции с системами мониторинга.

> **🔄 МИГРАЦИЯ:** Детальные метрики (StatusCodes, TotalRetries, RequestSize и др.) больше не доступны через ClientMetrics. Используйте OpenTelemetry/Prometheus для полного набора метрик.

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

**Бакеты:** 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10 (явно определены в коде)

**Диапазоны:**
- 0.001s (1мс) - очень быстрые запросы
- 0.005s (5мс) - быстрые запросы  
- 0.01s (10мс) - нормальные локальные запросы
- 0.025s (25мс) - приемлемые запросы
- 0.05s (50мс) - медленные запросы
- 0.1s (100мс) - очень медленные запросы
- 0.25s (250мс) - критически медленные
- 0.5s (500мс) - неприемлемо медленные
- 1.0s (1сек) - таймаут-кандидаты
- 2.5s, 5.0s, 10.0s - сверхмедленные запросы

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
**Описание:** Общее количество попыток повтора запросов (записывается автоматически)  
**Лейблы:**
- `method` - HTTP метод  
- `url` - URL запроса
- `attempt` - Номер попытки (2, 3, 4...)
- `success` - Успешность попытки (true/false)

**🔄 Автоматическая запись:** Эта метрика записывается автоматически при каждой retry попытке без необходимости ручного вызова `RecordRetry`.

```prometheus  
# Примеры значений
http_retries_total{method="GET",url="https://api.example.com/users",attempt="2",success="false"} 15
http_retries_total{method="POST",url="https://api.example.com/orders",attempt="3",success="true"} 8
http_retries_total{method="GET",host="api.example.com",retry_reason="server_error"} 7
```

#### `http_retry_attempts` (Histogram)
**Тип:** Histogram  
**Описание:** Количество попыток для каждого запроса (записывается автоматически)  
**Лейблы:**
- `method` - HTTP метод
- `host` - Хост назначения

**Бакеты:** 1, 2, 3, 4, 5, 10

**🔄 Автоматическая запись:** Количество retry попыток записывается автоматически при каждой retry операции.

```prometheus
# Примеры значений
http_retry_attempts_bucket{method="GET",host="api.example.com",le="2"} 892
http_retry_attempts_bucket{method="GET",host="api.example.com",le="3"} 945
http_retry_attempts_sum{method="GET",host="api.example.com"} 1756
http_retry_attempts_count{method="GET",host="api.example.com"} 856
```

### Метрики соединений (Connection Pool)

#### `http_connections_active` (Gauge)
**Тип:** Gauge  
**Описание:** Количество активных HTTP соединений (записывается автоматически)  
**Лейблы:**
- `host` - Хост назначения

**🔄 Автоматическая запись:** Эта метрика записывается автоматически при каждом запросе без необходимости ручного вызова.

#### `http_connections_idle` (Gauge)
**Тип:** Gauge  
**Описание:** Количество неактивных HTTP соединений в пуле (записывается автоматически)  
**Лейблы:**
- `host` - Хост назначения

#### `http_connection_pool_hits_total` (Counter)
**Тип:** Counter  
**Описание:** Количество повторного использования соединений из пула (записывается автоматически)  
**Лейблы:**
- `host` - Хост назначения

**🔄 Автоматическая запись:** Записывается при успешных запросах (status code < 500).

#### `http_connection_pool_misses_total` (Counter)
**Тип:** Counter  
**Описание:** Количество случаев создания новых соединений (записывается автоматически)  
**Лейблы:**
- `host` - Хост назначения

**🔄 Автоматическая запись:** Записывается при неуспешных запросах (status code >= 500).

### Метрики middleware (Промежуточное ПО)

#### `middleware_duration_seconds` (Histogram)
**Тип:** Histogram  
**Описание:** Время выполнения middleware в секундах (записывается автоматически)  
**Лейблы:**
- `middleware_name` - Имя middleware (logging, auth, rate_limit, etc.)
- `method` - HTTP метод

**Бакеты:** 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1

**🔄 Автоматическая запись:** Время выполнения записывается автоматически для всех middleware в цепочке.

```prometheus
# Примеры значений
middleware_duration_seconds_bucket{middleware_name="logging",method="GET",le="0.001"} 245
middleware_duration_seconds_sum{middleware_name="logging",method="GET"} 2.567
middleware_duration_seconds_count{middleware_name="logging",method="GET"} 1000
```

#### `middleware_errors_total` (Counter)
**Тип:** Counter  
**Описание:** Количество ошибок в middleware (записывается автоматически)  
**Лейблы:**
- `middleware_name` - Имя middleware
- `error_type` - Тип ошибки (request_failed, etc.)

**🔄 Автоматическая запись:** Ошибки записываются автоматически при возникновении ошибок в middleware.

```prometheus
# Примеры значений
middleware_errors_total{middleware_name="logging",error_type="request_failed"} 12
middleware_errors_total{middleware_name="auth",error_type="token_expired"} 5
```

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

### Retry и Connection Pool мониторинг
```promql
# Частота retry попыток
rate(http_retries_total[5m])

# Средние попытки retry
rate(http_retry_attempts_sum[5m]) / rate(http_retry_attempts_count[5m])
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
# Много повторов
- alert: HighRetryRate
  expr: rate(http_retries_total[5m]) > 10
  for: 3m
  labels:
    severity: warning
  annotations:
    summary: "Высокая частота повторов запросов"

# Низкая эффективность connection pool
- alert: LowConnectionPoolEfficiency
  expr: rate(http_connection_pool_hits_total[5m]) / (rate(http_connection_pool_hits_total[5m]) + rate(http_connection_pool_misses_total[5m])) < 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Низкая эффективность пула соединений"

# Медленные middleware
- alert: SlowMiddleware
  expr: histogram_quantile(0.95, rate(middleware_duration_seconds_bucket[5m])) > 0.1
  for: 3m
  labels:
    severity: warning
  annotations:
    summary: "Медленное выполнение middleware"
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
    
    // УДАЛЕНО: Circuit Breaker метрики больше не используются
    // MetricCircuitBreakerState, MetricCircuitBreakerFailures, 
    // MetricCircuitBreakerSuccesses, MetricCircuitBreakerStateChanges
    
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

## 🔄 Миграция с ClientMetrics на OpenTelemetry

### Устаревшие поля ClientMetrics
Следующие поля больше не доступны в `ClientMetrics`:
- `StatusCodes` → используйте метрику `http_requests_total` с лейблом `status_code`
- `TotalRetries` → используйте метрику `http_retries_total`
- `TotalRequestSize` → используйте метрику `http_request_size_bytes`
- `TotalResponseSize` → используйте метрику `http_response_size_bytes`
- `MinLatency`, `MaxLatency` → используйте перцентили в `http_request_duration_seconds`
- `CircuitBreakerState`, `CircuitBreakerTrips` → используйте метрики `circuit_breaker_state`

### Переход на OpenTelemetry
```go
// Получение OpenTelemetry meter от HTTP клиента
meter := client.GetMeter()

// Создание собственных метрик с тем же meter
requestCounter, _ := meter.Int64Counter(
    "my_app_requests_total",
    metric.WithDescription("Метрики моего приложения"),
)
```

Все детальные метрики теперь автоматически экспортируются в Prometheus через OpenTelemetry интеграцию.