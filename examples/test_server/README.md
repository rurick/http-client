# HTTP Client Test Server

Этот пример демонстрирует полнофункциональный тестовый HTTP сервер для проверки возможностей HTTP клиента. Сервер включает веб-интерфейс для интерактивного тестирования запросов и экспорт метрик Prometheus.

## Возможности

### 🌐 Веб-интерфейс
- Простая HTML страница для отправки GET/POST запросов
- Интерактивная форма с выбором метода, endpoint и данных
- Отображение ответов в реальном времени
- Валидация JSON данных

### 🔧 API Endpoints
- `GET/POST /api/test` - Основные тестовые запросы
- `GET /api/echo` - Возвращает параметры запроса
- `GET /api/status` - Статус сервера и метрики клиента
- `GET /metrics` - Метрики в формате Prometheus

### 📊 OpenTelemetry Prometheus метрики
- **Counter метрики** - `test_server_requests_total_total` (счетчик запросов)
- **Histogram латентности** - `test_server_request_duration_seconds_seconds` с полными buckets
- **Gauge метрики** - `test_server_uptime_seconds` (время работы сервера)
- **HTTP клиент метрики** - полная статистика внутреннего HTTP клиента через OpenTelemetry
- **Автоматические Go метрики** - memory, garbage collector, goroutines
- **Нативная OpenTelemetry интеграция** - совместимость с Jaeger, Zipkin, Prometheus

## Запуск

```bash
# Из корневой директории проекта
cd examples/test_server
go run main.go
```

Сервер запустится на `http://localhost:8080`

## Конфигурация

Настройки сервера в структуре `Config`:

```go
type Config struct {
    Port            int    // Порт сервера (по умолчанию 8080)
    Host            string // Хост (по умолчанию localhost)
    MetricsEndpoint string // Путь для метрик (по умолчанию /metrics)
}
```

## Использование

### 1. Веб-интерфейс
Откройте браузер и перейдите на `http://localhost:8080`:
- Выберите HTTP метод (GET/POST)
- Укажите endpoint (например, `/api/test`)
- Введите сообщение и JSON данные
- Нажмите "Отправить запрос"

### 2. API тестирование
```bash
# GET запрос
curl "http://localhost:8080/api/test?message=hello"

# POST запрос
curl -X POST http://localhost:8080/api/test \
  -H "Content-Type: application/json" \
  -d '{"message": "test", "data": {"key": "value"}}'

# Эхо параметров
curl "http://localhost:8080/api/echo?param1=value1&param2=value2"

# Статус сервера
curl http://localhost:8080/api/status
```

### 3. Метрики Prometheus
```bash
# Получить метрики
curl http://localhost:8080/metrics
```

Пример OpenTelemetry метрик:
```
# HELP test_server_requests_total_total Общее количество запросов к тестовому серверу
# TYPE test_server_requests_total_total counter
test_server_requests_total_total{otel_scope_name="test_server"} 15

# HELP test_server_request_duration_seconds_seconds Время обработки запросов в секундах
# TYPE test_server_request_duration_seconds_seconds histogram
test_server_request_duration_seconds_seconds_bucket{le="0.005"} 12
test_server_request_duration_seconds_seconds_bucket{le="0.01"} 14
test_server_request_duration_seconds_seconds_bucket{le="+Inf"} 15
test_server_request_duration_seconds_seconds_sum 0.015420
test_server_request_duration_seconds_seconds_count 15

# HELP test_server_uptime_seconds Время работы сервера в секундах
# TYPE test_server_uptime_seconds gauge
test_server_uptime_seconds{otel_scope_name="test_server"} 125.34

# HELP http_client_requests_total Общее количество запросов HTTP клиента
# TYPE http_client_requests_total gauge
http_client_requests_total{otel_scope_name="test_server"} 8
```

## Интеграция с HTTP клиентом

Сервер использует HTTP клиент из этого пакета для демонстрации:

```go
// Создание клиента с настройками
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithRetryMax(3),
    httpclient.WithLogger(logger),
    httpclient.WithMetricsEnabled(true),
)

// Получение метрик для экспорта
metrics := client.GetMetrics()
```

## Graceful Shutdown

Сервер поддерживает корректное завершение работы:
- Обработка сигналов SIGINT/SIGTERM
- Ожидание завершения активных запросов (до 30 секунд)
- Логирование процесса остановки

## Мониторинг

### Prometheus конфигурация
```yaml
- job_name: 'http-client-test-server'
  static_configs:
    - targets: ['localhost:8080']
  metrics_path: '/metrics'
  scrape_interval: 15s
```

### Grafana дашборд
Рекомендуемые панели для визуализации:

**Основные метрики:**
- `rate(test_server_requests_total_total[5m])` - RPS сервера
- `histogram_quantile(0.95, test_server_request_duration_seconds_seconds)` - P95 латентность
- `histogram_quantile(0.99, test_server_request_duration_seconds_seconds)` - P99 латентность
- `rate(test_server_request_duration_seconds_seconds_sum[5m]) / rate(test_server_request_duration_seconds_seconds_count[5m])` - Средняя латентность

**HTTP клиент:**
- `http_client_requests_total` - Общее количество запросов клиента
- `http_client_successful_requests_total` - Успешные запросы
- `http_client_failed_requests_total` - Неудачные запросы
- `http_client_average_latency_seconds` - Средняя задержка клиента

**Система:**
- `test_server_uptime_seconds` - Время работы сервера

## Возможные расширения

1. **Аутентификация** - добавить Basic Auth или JWT
2. **Rate Limiting** - ограничение запросов
3. **Логирование запросов** - детальное логирование в файл
4. **WebSocket поддержка** - для real-time тестирования
5. **Генерация нагрузки** - встроенный load tester

Этот пример демонстрирует практическое использование HTTP клиента в реальном веб-сервере с полным циклом observability.