# HTTP Client Package

Этот пакет предоставляет готовый к использованию HTTP клиент с автоматической коллекцией Prometheus метрик через OpenTelemetry, механизмом повторных попыток, обработкой идемпотентности и интеграцией трассировки.

## Основные возможности

- **Автоматические метрики**: Интеграция с Prometheus через OpenTelemetry для всех HTTP запросов
- **Умные повторы**: Exponential backoff с full jitter для повторных попыток
- **Идемпотентность**: Автоматическое определение идемпотентных запросов и поддержка Idempotency-Key
- **Трассировка**: Интеграция OpenTelemetry для распределенной трассировки
- **Гибкая конфигурация**: Настраиваемые тайм-ауты, количество попыток и стратегии backoff
- **Покрытие тестами**: 72%+ покрытие тестами с обширными unit тестами

## Быстрый старт

```go
package main

import (
    "context"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    // Создание клиента с базовой конфигурацией
    client := httpclient.New(httpclient.Config{})
    defer client.Close()
    
    ctx := context.Background()
    
    // Простой GET запрос
    resp, err := client.Get(ctx, "https://api.example.com/users")
    if err != nil {
        // Обработка ошибки
    }
    defer resp.Body.Close()
}
```

## Конфигурация

```go
config := httpclient.Config{
    // Общий тайм-аут для запросов
    Timeout: 30 * time.Second,
    
    // Конфигурация повторных попыток
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    5 * time.Second,
        Jitter:      0.2, // 20% jitter
    },
    
    // Включение трассировки
    TracingEnabled: true,
    
    // Кастомный Transport (опционально)
    Transport: http.DefaultTransport,
}

client := httpclient.New(config)
```

## Метрики

Пакет автоматически собирает следующие Prometheus метрики:

- `http_client_requests_total` - Общее количество HTTP запросов
- `http_client_request_duration_seconds` - Длительность HTTP запросов
- `http_client_retries_total` - Количество повторных попыток
- `http_client_inflight_requests` - Количество активных запросов
- `http_client_request_size_bytes` - Размер HTTP запросов
- `http_client_response_size_bytes` - Размер HTTP ответов

## Повторные попытки

Пакет автоматически повторяет запросы в следующих случаях:

- **GET, PUT, DELETE, HEAD, OPTIONS**: Всегда повторяются при временных ошибках
- **POST, PATCH**: Повторяются только при наличии заголовка `Idempotency-Key`
- **Статус коды**: 500, 502, 503, 504, 429
- **Network ошибки**: Timeout, connection reset, DNS ошибки

### Стратегии Backoff

- **Exponential Backoff**: Задержка растёт экспоненциально (по умолчанию)
- **Linear Backoff**: Задержка растёт линейно
- **Constant Backoff**: Фиксированная задержка
- **Full Jitter**: Добавляет случайность для избежания thundering herd

## Обработка ошибок

```go
resp, err := client.Get(ctx, url)
if err != nil {
    // Проверяем тип ошибки
    if retryableErr, ok := err.(*httpclient.RetryableError); ok {
        // Это ошибка, которую можно повторить
        fmt.Printf("Retryable error after %d attempts: %v", 
                   retryableErr.Attempts, retryableErr.Err)
    } else if nonRetryableErr, ok := err.(*httpclient.NonRetryableError); ok {
        // Это ошибка, которую нельзя повторить
        fmt.Printf("Non-retryable error: %v", nonRetryableErr.Err)
    }
}
```

## Трассировка

Для включения трассировки установите `TracingEnabled: true` в конфигурации. Пакет создаст spans для каждого HTTP запроса с подробной информацией о методе, URL, статусе ответа и количестве попыток.

## Идемпотентность

POST и PATCH запросы повторяются только при наличии заголовка `Idempotency-Key`:

```go
req, _ := http.NewRequest("POST", url, body)
req.Header.Set("Idempotency-Key", "unique-operation-id")

resp, err := client.Do(req)
```

## Примеры использования

Смотрите директорию `examples/` для детальных примеров:

- `example/main.go` - Базовое использование
- `examples/retry.go` - Настройка повторных попыток  
- `examples/metrics.go` - Работа с метриками
- `examples/idempotency.go` - Использование идемпотентности

## Требования

- Go 1.23+
- OpenTelemetry SDK для метрик и трассировки

## Лицензия

MIT License