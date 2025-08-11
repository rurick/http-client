# HTTP Client Package

Комплексный Go HTTP клиент с автоматическими retry механизмами, Prometheus метриками через OpenTelemetry и политиками идемпотентности.

## Основные возможности

- **Умные повторы** с экспоненциальным backoff и джиттером
- **Автоматические Prometheus метрики** через OpenTelemetry  
- **Политики идемпотентности** для безопасных повторов POST/PATCH
- **Distributed tracing** с полной поддержкой OpenTelemetry
- **Настраиваемые таймауты** и стратегии backoff
- **Testing utilities** для unit и integration тестов

## Статус

- ✅ Готов к продакшену
- ✅ Покрытие тестами 76%+
- ✅ Полная русская документация
- ✅ Поддержка Go 1.23+

## Быстрый старт

```go
package main

import (
    "context"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), "https://api.example.com/data")
    if err != nil {
        // обработка ошибки
    }
    defer resp.Body.Close()
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

## Поддержка

Для вопросов обращайтесь к команде Backend разработки CityDrive Tech.