# Метрики и мониторинг

HTTP клиент автоматически собирает комплексные Prometheus метрики через prometheus/client_golang v1.22.0 для полной observability ваших HTTP запросов.

## Доступные метрики

### 1. http_client_requests_total (Счетчик)
Отслеживает общее количество HTTP запросов.

**Метки:**
- `method`: HTTP метод (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
- `host`: Целевой хост (example.com)
- `status`: HTTP статус код (200, 404, 500, и т.д.)
- `retry`: Была ли это попытка повтора (true/false)
- `error`: Привел ли запрос к ошибке (true/false)

```promql
# Общее количество запросов
http_client_requests_total

# Запросы по методам
http_client_requests_total{method="GET"}

# Успешные запросы
http_client_requests_total{error="false"}
```

### 2. http_client_request_duration_seconds (Гистограмма)
Измеряет длительность запросов в секундах.

**Метки:**
- `method`: HTTP метод
- `host`: Целевой хост
- `status`: HTTP статус код
- `attempt`: Номер попытки (1, 2, 3, и т.д.)

**Бакеты:** `0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60`

```promql
# 95-й перцентиль латентности
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le))

# Средняя латентность
rate(http_client_request_duration_seconds_sum[5m]) / rate(http_client_request_duration_seconds_count[5m])
```

### 3. http_client_retries_total (Счетчик)
Подсчитывает попытки повторов с детализацией причин.

**Метки:**
- `reason`: Причина повтора (status_code, network_error, timeout, connection_error)
- `method`: HTTP метод
- `host`: Целевой хост

```promql
# Частота повторов
rate(http_client_retries_total[5m])

# Повторы по причинам
sum(rate(http_client_retries_total[5m])) by (reason)
```

### 4. http_client_inflight_requests (UpDownCounter)
Текущее количество активных запросов.

**Метки:**
- `host`: Целевой хост

```promql
# Текущие активные запросы
http_client_inflight_requests

# Максимум за период
max_over_time(http_client_inflight_requests[5m])
```

### 5. http_client_request_size_bytes (Гистограмма)
Размер тела запроса в байтах.

**Метки:**
- `method`: HTTP метод
- `host`: Целевой хост

**Бакеты:** `256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216`

```promql
# 95-й перцентиль размера запросов
histogram_quantile(0.95, sum(rate(http_client_request_size_bytes_bucket[5m])) by (le))
```

### 6. http_client_response_size_bytes (Гистограмма)
Размер тела ответа в байтах.

**Метки:**
- `method`: HTTP метод
- `host`: Целевой хост
- `status`: HTTP статус код

**Бакеты:** Те же, что и для размера запроса

```promql
# 95-й перцентиль размера ответов
histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le))
```

## PromQL запросы

### Базовые метрики производительности

#### Частота запросов (RPS)
```promql
# Запросов в секунду
sum(rate(http_client_requests_total[5m]))

# RPS по сервисам
sum(rate(http_client_requests_total[5m])) by (host)

# RPS по методам
sum(rate(http_client_requests_total[5m])) by (method)
```

#### Процент ошибок
```promql
# Общий процент ошибок
sum(rate(http_client_requests_total{error="true"}[5m])) / 
sum(rate(http_client_requests_total[5m])) * 100

# Процент ошибок по сервисам
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) / 
sum(rate(http_client_requests_total[5m])) by (host) * 100

# Процент HTTP ошибок (4xx, 5xx)
sum(rate(http_client_requests_total{status=~"[45].."}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host) * 100
```

#### Анализ латентности
```promql
# 50-й, 95-й, 99-й перцентили
histogram_quantile(0.50, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
histogram_quantile(0.99, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Средняя латентность
sum(rate(http_client_request_duration_seconds_sum[5m])) by (host) /
sum(rate(http_client_request_duration_seconds_count[5m])) by (host)

# Латентность по статус кодам
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, status))
```

### Анализ retry поведения

#### Статистика повторов
```promql
# Частота повторов
sum(rate(http_client_retries_total[5m])) by (host, reason)

# Процент запросов с повторами
sum(rate(http_client_requests_total{retry="true"}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host) * 100

# Успешность повторов
sum(rate(http_client_requests_total{retry="true", error="false"}[5m])) by (host) /
sum(rate(http_client_retries_total[5m])) by (host) * 100
```

#### Топ причин повторов
```promql
# Самые частые причины повторов
topk(5, sum(rate(http_client_retries_total[5m])) by (reason))

# Повторы по сервисам
topk(10, sum(rate(http_client_retries_total[5m])) by (host))
```

### Анализ нагрузки

#### Активные соединения
```promql
# Текущие активные запросы
http_client_inflight_requests

# Пиковая нагрузка за час
max_over_time(http_client_inflight_requests[1h])

# Средняя нагрузка
avg_over_time(http_client_inflight_requests[5m])
```

#### Анализ размеров
```promql
# Средний размер запросов
rate(http_client_request_size_bytes_sum[5m]) / rate(http_client_request_size_bytes_count[5m])

# Средний размер ответов
rate(http_client_response_size_bytes_sum[5m]) / rate(http_client_response_size_bytes_count[5m])

# Топ самых "тяжелых" эндпоинтов
topk(10, histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le, host)))
```

### Dashboard запросы

#### SLI метрики
```promql
# Availability (99.9% target)
sum(rate(http_client_requests_total{error="false"}[5m])) /
sum(rate(http_client_requests_total[5m])) * 100

# Latency SLI (95% requests < 500ms)
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le))

# Throughput
sum(rate(http_client_requests_total[5m]))
```

## Правила алертов

### Критичные алерты

#### Высокий процент ошибок
```yaml
groups:
- name: httpclient.critical
  rules:
  - alert: HTTPClientHighErrorRate
    expr: |
      (
        sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
        sum(rate(http_client_requests_total[5m])) by (host)
      ) > 0.05
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Высокий процент ошибок HTTP клиента"
      description: "{{ $labels.host }} имеет {{ $value | humanizePercentage }} ошибок за последние 5 минут"
```

#### Критически высокая латентность
```yaml
  - alert: HTTPClientCriticalLatency
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 5
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Критически высокая латентность HTTP клиента"
      description: "{{ $labels.host }} имеет 95-й перцентиль латентности {{ $value }}с"
```

### Предупреждения

#### Повышенная латентность
```yaml
- name: httpclient.warnings
  rules:
  - alert: HTTPClientHighLatency
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Повышенная латентность HTTP клиента"
      description: "{{ $labels.host }} имеет 95-й перцентиль латентности {{ $value }}с за 5 минут"
```

#### Чрезмерные повторы
```yaml
  - alert: HTTPClientExcessiveRetries
    expr: |
      sum(rate(http_client_retries_total[5m])) by (host) > 1
    for: 3m
    labels:
      severity: warning
    annotations:
      summary: "Высокая частота повторов HTTP клиента"
      description: "{{ $labels.host }} делает {{ $value }} повторов/сек за последние 5 минут"
```

#### Много активных запросов
```yaml
  - alert: HTTPClientHighInflight
    expr: |
      http_client_inflight_requests > 100
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Много одновременных HTTP запросов"
      description: "{{ $labels.host }} имеет {{ $value }} одновременных запросов"
```

### Информационные алерты

#### Необычно большие ответы
```yaml
- name: httpclient.info
  rules:
  - alert: HTTPClientLargeResponses
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le, host)) > 10485760 # 10MB
    for: 10m
    labels:
      severity: info
    annotations:
      summary: "Большие HTTP ответы"
      description: "{{ $labels.host }} возвращает ответы размером {{ $value | humanizeBytes }}"
```

## Grafana Dashboard

### Основные панели

#### Overview Panel
```promql
# Requests per second
sum(rate(http_client_requests_total[5m])) by (host)

# Error rate
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host)

# 95th percentile latency
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Active requests
http_client_inflight_requests
```

#### Detailed Analytics
```promql
# Requests by method
sum(rate(http_client_requests_total[5m])) by (method)

# Status code distribution
sum(rate(http_client_requests_total[5m])) by (status)

# Retry analysis
sum(rate(http_client_retries_total[5m])) by (reason)

# Size distribution
histogram_quantile(0.95, sum(rate(http_client_request_size_bytes_bucket[5m])) by (le))
```

### Recording Rules

Для оптимизации производительности используйте recording rules:

```yaml
groups:
- name: httpclient.recording
  interval: 30s
  rules:
  - record: httpclient:request_rate
    expr: sum(rate(http_client_requests_total[5m])) by (host)
    
  - record: httpclient:error_rate
    expr: |
      sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
      sum(rate(http_client_requests_total[5m])) by (host)
    
  - record: httpclient:latency_p95
    expr: histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
    
  - record: httpclient:retry_rate
    expr: sum(rate(http_client_retries_total[5m])) by (host)
```

## Использование метрик в коде

Метрики собираются автоматически, но вы можете получить к ним доступ:

```go
// Метрики доступны через клиент (internal API)
// Обычно не требуется прямое взаимодействие

client := httpclient.New(config, "my-service")
defer client.Close()

// Все метрики собираются автоматически при выполнении запросов
resp, err := client.Get(ctx, "https://api.example.com/data")
```

## Использование метрик в коде

```go
import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// Метрики создаются автоматически при создании клиента
client := httpclient.New(httpclient.Config{}, "my-service")
defer client.Close()

// Создаём HTTP endpoint для метрик - метрики автоматически регистрируются
http.Handle("/metrics", promhttp.Handler())

// Метрики собираются автоматически при выполнении запросов
resp, err := client.Get(ctx, "https://api.example.com/data")
```

## Troubleshooting метрик

### Метрики не появляются
1. Проверьте настройку OpenTelemetry
2. Убедитесь что exporter настроен корректно
3. Проверьте что клиент выполняет запросы

### Неожиданные значения
1. Проверьте лейблы в PromQL запросах
2. Убедитесь в правильности временных интервалов
3. Проверьте фильтры по host/method

### Производительность
1. Используйте recording rules для часто используемых запросов
2. Оптимизируйте время выполнения PromQL запросов
3. Настройте appropriate retention policy