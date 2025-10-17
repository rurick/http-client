# Документация HTTP Client Package

Добро пожаловать в документацию HTTP клиент пакета - комплексного решения для HTTP запросов с автоматическими retry механизмами, встроенными Prometheus метриками и политиками идемпотентности.

## Содержание

- [Быстрый старт](quick-start.md) - Примеры использования и первые шаги
- [Конфигурация](configuration.md) - Полная документация по настройке
- [Circuit Breaker](circuit-breaker.md) - Автоматический выключатель и защита от каскадных сбоев
- [Метрики](metrics.md) - Описание метрик и PromQL запросы
- [OpenTelemetry метрики](opentelemetry-metrics.md) - Интеграция с OpenTelemetry
- [Тестирование](testing.md) - Утилиты и примеры тестов
- [API справочник](api-reference.md) - Полное описание всех функций
- [Лучшие практики](best-practices.md) - Рекомендации по использованию
- [Troubleshooting](troubleshooting.md) - Решение частых проблем
- [Примеры](examples.md) - Готовые code snippets

## Основные особенности

### 🔄 Умные Retry Механизмы
- Экспоненциальный backoff с jitter
- Автоматическое определение идемпотентных методов
- Поддержка Idempotency-Key для POST/PATCH запросов
- Настраиваемые таймауты и количество попыток

### 📊 Автоматические Метрики
- Поддержка Prometheus (prometheus/client_golang v1.22.0) и OpenTelemetry
- 6 типов метрик: запросы, длительности, retry, размеры, inflight
- Настраиваемые провайдеры метрик (Prometheus/OpenTelemetry/Noop)
- Готовые PromQL запросы и алерты

### 🔍 Observability
### 🛡️ Circuit Breaker
- Встроенная поддержка автоматического выключателя
- Настраиваемые пороги ошибок и таймаут восстановления
- Возвращает последнюю неуспешную реакцию при открытом состоянии
- Открытый Circuit Breaker не инициирует retry
- Полная интеграция с OpenTelemetry tracing
- Автоматическое создание спанов для каждого запроса
- Передача контекста между сервисами
- Детальное логирование ошибок

### 🧪 Testing Utilities
- TestServer для интеграционных тестов
- MockRoundTripper для unit тестов
- Helpers для проверки условий с timeout
- Collectors для тестирования метрик

## Быстрый старт

```go
package main

import (
    "context"
    "log"
    httpclient "github.com/rurick/http-client"
)

func main() {
    // Создание клиента с настройками по умолчанию
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    // GET запрос
    resp, err := client.Get(context.Background(), "https://api.example.com/users")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
}
```

## Конфигурация с retry

```go
config := httpclient.Config{
    Timeout:       30 * time.Second,
    PerTryTimeout: 5 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    10 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled: true,
}

client := httpclient.New(config, "payment-service")
```

## Конфигурация с OpenTelemetry метриками

```go
// Основная конфигурация OpenTelemetry
config := httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendOpenTelemetry,
    // Можно указать кастомный MeterProvider
    // OTelMeterProvider: customMeterProvider,
}

client := httpclient.New(config, "otel-service")

// Отключение метрик
config = httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendNone,
}
client = httpclient.New(config, "no-metrics-service")
```

## Доступные метрики

1. **http_client_requests_total** - Общее количество запросов
2. **http_client_request_duration_seconds** - Длительность запросов
3. **http_client_retries_total** - Количество retry попыток
4. **http_client_inflight_requests** - Текущие активные запросы
5. **http_client_request_size_bytes** - Размер запроса
6. **http_client_response_size_bytes** - Размер ответа

## Готовые алерты

```yaml
# Высокий процент ошибок
- alert: HTTPClientHighErrorRate
  expr: |
    (sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
     sum(rate(http_client_requests_total[5m])) by (host)) > 0.05
  for: 2m

# Высокая задержка
- alert: HTTPClientHighLatency  
  expr: |
    histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 2
  for: 5m
```

## Статус пакета

✅ **Готов к продакшену**
- Все компоненты реализованы и протестированы
- Код компилируется без ошибок
- Покрытие тестами 61.7%+
- Полная документация и примеры
- Integration тесты для метрик
- Mock utilities для unit тестов

## Файлы документации

- [`quick-start.md`](quick-start.md) - Быстрый старт с примерами
- [`configuration.md`](configuration.md) - Детальная документация по конфигурации
- [`circuit-breaker.md`](circuit-breaker.md) - Подробная документация по Circuit Breaker
- [`metrics.md`](metrics.md) - Метрики, PromQL запросы и алерты
- [`opentelemetry-metrics.md`](opentelemetry-metrics.md) - Интеграция с OpenTelemetry метриками
- [`api-reference.md`](api-reference.md) - Полный справочник API
- [`best-practices.md`](best-practices.md) - Лучшие практики использования
- [`testing.md`](testing.md) - Руководство по тестированию
- [`examples.md`](examples.md) - Практические примеры кода
- [`troubleshooting.md`](troubleshooting.md) - Решение проблем

## Использование в проектах

```go
// Для внутренних API
client := httpclient.New(httpclient.Config{
    Timeout: 5 * time.Second,
    RetryConfig: httpclient.RetryConfig{MaxAttempts: 2},
}, "internal-service")

// Для внешних API
client := httpclient.New(httpclient.Config{
    Timeout: 30 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   200 * time.Millisecond,
        MaxDelay:    10 * time.Second,
    },
    TracingEnabled: true,
}, "external-api")
```

Подробные примеры и полную документацию смотрите в соответствующих разделах выше.

## Дополнительные ресурсы

### PromQL примеры для мониторинга

```promql
# Частота запросов
rate(http_client_requests_total[5m])

# Процент ошибок
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) / 
sum(rate(http_client_requests_total[5m])) by (host) * 100

# 95-й перцентиль латентности
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Частота повторов
sum(rate(http_client_retries_total[5m])) by (host, reason)
```

### Рекомендуемые настройки алертов

- **Процент ошибок** > 5% в течение 2 минут
- **95-й перцентиль латентности** > 2 секунд в течение 5 минут  
- **Частота повторов** > 1 запрос/сек в течение 2 минут
- **Активные запросы** > 100 в течение 1 минуты

### Troubleshooting

Частые проблемы и решения:

1. **Высокий процент ошибок**
   - Проверьте доступность целевого сервиса
   - Увеличьте таймауты если нужно
   - Проверьте сетевую связность

2. **Высокая латентность**
   - Проверьте производительность целевого сервиса
   - Рассмотрите увеличение PerTryTimeout
   - Проверьте сетевые задержки

3. **Много повторов**
   - Проверьте стабильность целевого сервиса
   - Рассмотрите уменьшение MaxAttempts
   - Проверьте причины повторов в метриках

### Поддержка и обратная связь

Пакет готов к production использованию. Для вопросов и предложений обращайтесь к команде разработки.