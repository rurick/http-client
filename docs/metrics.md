# Метрики Prometheus

HTTP-клиент автоматически собирает детальные метрики через OpenTelemetry и экспортирует их в Prometheus.

## Доступные метрики

### http_client_requests_total

**Тип**: Counter  
**Единица**: количество запросов  
**Описание**: Общее количество HTTP запросов клиента

**Лейблы**:
- `method` — HTTP метод (GET, POST, PUT, DELETE, etc.)
- `host` — хост назначения
- `status` — HTTP статус код ответа
- `retry` — флаг повторного запроса (true/false)
- `error` — флаг наличия ошибки (true/false)

**Примечания**:
- Каждая попытка (включая ретраи) учитывается отдельно
- При сетевых ошибках status может быть "0"

### http_client_request_duration_seconds

**Тип**: Histogram  
**Единица**: секунды  
**Описание**: Длительность HTTP запросов

**Лейблы**:
- `method` — HTTP метод
- `host` — хост назначения  
- `status` — HTTP статус код
- `attempt` — номер попытки

**Buckets по умолчанию**: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60]

### http_client_retries_total

**Тип**: Counter  
**Единица**: количество ретраев  
**Описание**: Общее количество повторных попыток

**Лейблы**:
- `reason` — причина ретрая (status|net|timeout)
- `method` — HTTP метод
- `host` — хост назначения

### http_client_inflight_requests

**Тип**: UpDownCounter/Gauge  
**Единица**: количество запросов  
**Описание**: Количество активных HTTP запросов

**Лейблы**:
- `host` — хост назначения

### http_client_request_size_bytes

**Тип**: Histogram  
**Единица**: байты  
**Описание**: Размер HTTP запросов в байтах

**Лейблы**:
- `method` — HTTP метод
- `host` — хост назначения

**Buckets по умолчанию**: [256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216]

### http_client_response_size_bytes

**Тип**: Histogram  
**Единица**: байты  
**Описание**: Размер HTTP ответов в байтах

**Лейблы**:
- `method` — HTTP метод
- `host` — хост назначения  
- `status` — HTTP статус код

**Buckets по умолчанию**: [256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216]

## Примеры PromQL запросов

### Мониторинг ошибок

```promql
# Общая доля ошибок
sum by (status) (rate(http_client_requests_total{error="true"}[5m])) / sum(rate(http_client_requests_total[5m]))

# Доля ошибок по сервису
sum by (host) (rate(http_client_requests_total{error="true"}[5m])) / sum by (host) (rate(http_client_requests_total[5m]))

# Rate 5xx ошибок
sum(rate(http_client_requests_total{status=~"5.."}[5m]))
