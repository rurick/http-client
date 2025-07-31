# Стратегии повтора

По умолчанию клиент работает **без ретраев**. Чтобы включить повторы, используйте соответствующие опции конфигурации.

## Экспоненциальная задержка

Самая популярная стратегия для продакшн систем - задержка увеличивается экспоненциально с каждой попыткой.

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // Включить повторы (максимум 3 попытки)
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3,                          // максимальное количество попыток
        100*time.Millisecond,       // базовая задержка
        5*time.Second,              // максимальная задержка
    )),
)
```

### Как работает

- **1-я попытка**: сразу
- **2-я попытка**: через 100ms
- **3-я попытка**: через 200ms
- **4-я попытка**: через 400ms (но не более 5s)

## Фиксированная задержка

Простая стратегия с одинаковой задержкой между попытками.

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // Включить повторы
    httpclient.WithRetryStrategy(httpclient.NewFixedDelayStrategy(
        3,                          // максимальное количество попыток
        1*time.Second,              // задержка между попытками
    )),
)
```

### Как работает

- **1-я попытка**: сразу
- **2-я попытка**: через 1s
- **3-я попытка**: через 1s
- **4-я попытка**: через 1s

## Умная стратегия (SmartRetryStrategy)

Адаптивная стратегия, которая анализирует историю ошибок и оптимизирует задержки.

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(5),
    httpclient.WithRetryStrategy(httpclient.NewSmartRetryStrategy(
        5,                          // максимальное количество попыток
        100*time.Millisecond,       // базовая задержка
        10*time.Second,             // максимальная задержка
    )),
)
```

### Особенности

- Анализирует типы ошибок
- Адаптирует задержки на основе истории
- Оптимизирует повторы для конкретных сценариев

## Пользовательская стратегия

Создание собственной логики повторов для специфических случаев.

```go
customStrategy := httpclient.NewCustomRetryStrategy(
    3, // максимальное количество попыток
    
    // Функция определения необходимости повтора
    func(resp *http.Response, err error) bool {
        if err != nil {
            return true // Повторить при любой ошибке
        }
        // Повторить только для определенных статус кодов
        return resp.StatusCode == 429 || resp.StatusCode >= 500
    },
    
    // Функция вычисления задержки
    func(attempt int, lastErr error) time.Duration {
        // Пользовательская логика задержки
        switch attempt {
        case 1:
            return 500 * time.Millisecond
        case 2:
            return 2 * time.Second
        default:
            return 5 * time.Second
        }
    },
)

client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(customStrategy),
)
```

## Настройка времени ожидания

Дополнительные опции для тонкой настройки повторов.

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),                       // Максимум попыток
    httpclient.WithRetryWait(1*time.Second, 10*time.Second), // Мин/макс время ожидания
)
```

## HTTP коды состояния для повтора

По умолчанию повторы выполняются для следующих HTTP кодов:

- `429` - Too Many Requests
- `500` - Internal Server Error  
- `502` - Bad Gateway
- `503` - Service Unavailable
- `504` - Gateway Timeout

### Проверка кода на возможность повтора

```go
if httpclient.IsRetryableStatusCode(resp.StatusCode) {
    fmt.Println("Этот статус код подходит для повтора")
}
```

## Лучшие практики

### Для внешних API
```go
// Умеренные повторы для стабильности
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(3, 200*time.Millisecond, 5*time.Second)),
    httpclient.WithTimeout(10*time.Second),
)
```

### Для внутренних сервисов
```go
// Более агрессивные повторы для временных сбоев
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(5),
    httpclient.WithRetryStrategy(httpclient.NewSmartRetryStrategy(5, 100*time.Millisecond, 3*time.Second)),
    httpclient.WithTimeout(30*time.Second),
)
```

### Для критических операций
```go
// Отключение повторов для идемпотентных операций
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(0), // Отключить повторы
    httpclient.WithTimeout(5*time.Second),
)
```

## См. также

- [Автоматический выключатель](circuit-breaker.md) - Дополнительная защита от сбоев
- [Конфигурация](configuration.md) - Все опции настройки
- [Метрики](metrics.md) - Мониторинг повторов