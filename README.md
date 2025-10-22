# HTTP Client Package

Комплексный Go HTTP клиент с автоматическими retry механизмами, встроенными Prometheus метриками и distributed tracing через OpenTelemetry.

## Основные возможности

- **Умные повторы** с экспоненциальным backoff и джиттером
- **Встроенные Prometheus метрики** на базе prometheus/client_golang
- **Distributed tracing** через OpenTelemetry
- **Circuit Breaker** для защиты от каскадных сбоев
- **Встроенный Rate Limiter** с Token Bucket алгоритмом
- **Политики идемпотентности** для безопасных повторов POST/PATCH
- **Настраиваемые таймауты** и стратегии backoff
- **Testing utilities** для unit и integration тестов

## Быстрый старт

```go
package main

import (
    "context"
    httpclient "github.com/rurick/http-client"
)

func main() {
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    // Простой GET запрос
    resp, err := client.Get(context.Background(), "https://api.example.com/data")
    if err != nil {
        // обработка ошибки
    }
    defer resp.Body.Close()
    
    // GET с заголовками через новые опции
    resp, err = client.Get(context.Background(), "https://api.example.com/users",
        httpclient.WithHeaders(map[string]string{
            "Authorization": "Bearer your-token",
            "Accept": "application/json",
        }))
    if err != nil {
        return
    }
    defer resp.Body.Close()
    
    // POST с JSON телом
    user := map[string]interface{}{
        "name": "John Doe",
        "email": "john@example.com",
    }
    resp, err = client.Post(context.Background(), "https://api.example.com/users", nil,
        httpclient.WithJSONBody(user),
        httpclient.WithBearerToken("your-token"))
    if err != nil {
        return
    }
    defer resp.Body.Close()

    // POST с JSON телом как строка
	userString := `{"name": "John Doe","email": "john@example.com"}`
	resp, err = client.Post(context.Background(), "https://api.example.com/users", nil,
		httpclient.WithJSONBody(userString),
		httpclient.WithBearerToken("your-token"))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Использование Rate Limiter
	clientWithRateLimit := httpclient.New(httpclient.Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: httpclient.RateLimiterConfig{
			RequestsPerSecond: 5.0, // 5 запросов в секунду
			BurstCapacity:     10,  // до 10 запросов сразу
		},
	}, "rate-limited-service")
	defer clientWithRateLimit.Close()
}
```

## Документация

**Полная документация:** [docs/index.md](docs/index.md)

**Основные разделы:**
- [Быстрый старт](docs/quick-start.md) - Примеры использования  
- [Конфигурация](docs/configuration.md) - Настройки клиента
- [Метрики](docs/metrics.md) - Мониторинг и алерты
- [API справочник](docs/api-reference.md) - Полное описание функций
- [Лучшие практики](docs/best-practices.md) - Рекомендации
- [Тестирование](docs/testing.md) - Утилиты и примеры
- [Troubleshooting](docs/troubleshooting.md) - Решение проблем
- [Примеры](docs/examples.md) - Готовые code snippets

