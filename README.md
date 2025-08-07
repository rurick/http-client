# HTTP Клиент

Универсальная библиотека HTTP клиента для Go с встроенными механизмами надежности и наблюдаемости.

## Что это такое

HTTP клиент предоставляет готовое к использованию решение для выполнения HTTP запросов в Go приложениях. Библиотека разработана для продакшн систем и включает всё необходимое для надежной работы с внешними API и микросервисами.

## Основные возможности

🚀 **Надежность**
- Автоматические повторы с различными стратегиями (экспоненциальная задержка, фиксированная, адаптивная)
- Circuit Breaker для защиты от каскадных сбоев
- Настраиваемые таймауты и управление соединениями

🔧 **Удобство использования**
- Простое API для JSON и XML запросов
- Поддержка всех стандартных HTTP методов
- Потоковая обработка больших данных
- Готовые middleware для аутентификации, логирования, rate limiting

📊 **Наблюдаемость**
- Полная интеграция с OpenTelemetry и экспорт метрик в Prometheus
- Детальные метрики: запросы, задержки, размеры, повторы, circuit breaker
- Распределенный трейсинг для отслеживания запросов
- Подробное логирование операций

🧪 **Тестирование**
- Mock объекты для unit тестов
- Тестовые утилиты и помощники
- Изоляция внешних зависимостей

## Быстрый старт

### Установка

```bash
go get gitlab.citydrive.tech/back-end/go/pkg/http-client
```

### Простое использование

> **⚠️ ВАЖНО: Всегда используйте контекстные методы (GetCtx, PostCtx и т.д.) для лучшего контроля запросов!**

```go
import (
    "context"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// Создание клиента
client, err := httpclient.NewClient()
if err != nil {
    log.Fatal(err)
}

// Создание контекста с таймаутом (РЕКОМЕНДУЕТСЯ)
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// HTTP запрос с контекстом (ЛУЧШАЯ ПРАКТИКА)
resp, err := client.GetCtx(ctx, "https://api.example.com/data")

// JSON запрос с контекстом (РЕКОМЕНДУЕТСЯ)
var result MyStruct
err = client.GetJSON(ctx, "https://api.example.com/json", &result)
```

### Продакшн конфигурация

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3, 200*time.Millisecond, 5*time.Second)),
    httpclient.WithCircuitBreaker(httpclient.NewSimpleCircuitBreaker()),
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMetrics(true),
)
```

## Когда использовать

✅ **Подходит для:**
- Взаимодействие с внешними API
- Коммуникация между микросервисами
- CLI утилиты с HTTP запросами
- Системы с требованиями к надежности
- Приложения где важен мониторинг HTTP операций

❌ **Не подходит для:**
- Простых однократных HTTP запросов в скриптах
- Случаев где нужен минимальный overhead
- WebSocket соединений (только HTTP/HTTPS)

## Документация

📚 **Полная документация доступна в папке [docs/](docs/index.md)**

### Основные разделы
- [Быстрый старт](docs/quick-start.md) - Первые шаги с библиотекой
- [Настройки по умолчанию](docs/default-settings.md) - Стандартные параметры клиента
- [Пул соединений](docs/connection-pool.md) - Конфигурация и оптимизация соединений
- [Контекстные методы](docs/context-methods.md) - HTTP методы с поддержкой контекста
- [Конфигурация](docs/configuration.md) - Все опции настройки
- [Стратегии повтора](docs/retry-strategies.md) - Настройка механизмов повтора
- [Circuit Breaker](docs/circuit-breaker.md) - Защита от каскадных сбоев
- [Middleware](docs/middleware.md) - Система промежуточного ПО
- [Метрики](docs/metrics.md) - Сбор и экспорт метрик
- [Трейсинг](docs/tracing.md) - Распределенная трассировка

- [Тестирование](docs/testing.md) - Mock объекты и утилиты
- [API Reference](docs/api-reference.md) - Полное описание API
- [Примеры](docs/examples.md) - Практические примеры использования



## Разработка

Библиотека разработана специально для корпоративного использования в CityDrive с учетом требований надежности, производительности и наблюдаемости.

### Репозиторий
```
gitlab.citydrive.tech/back-end/go/pkg/http-client
```

### Участие в разработке
- Создавайте issues для багов и предложений
- Следуйте принципам conventional commits
- Все изменения проходят через merge requests
- CI/CD автоматически запускает тесты и проверки качества

## Лицензия

Внутренний проект CityDrive. Использование ограничено рамками организации.