# Конфигурация клиента

Полное руководство по всем опциям конфигурации HTTP клиента.

> 📚 **См. также**: [Настройки по умолчанию](default-settings.md) | [Пул соединений](connection-pool.md)

## Базовые опции

### Таймауты

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),           // Общий таймаут запроса
)
```

### Пул соединений

```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(100),                 // Максимум неактивных соединений
    httpclient.WithMaxConnsPerHost(10),               // Максимум соединений на хост
    httpclient.WithIdleConnTimeout(90*time.Second),   // Таймаут неактивного соединения
)
```

> 📚 **Подробнее**: [Пул соединений](connection-pool.md) - полное руководство по настройке и оптимизации

### Пользовательский HTTP клиент

```go
customTransport := &http.Transport{
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

customClient := &http.Client{
    Transport: customTransport,
    Timeout:   30 * time.Second,
}

client, err := httpclient.NewClient(
    httpclient.WithHTTPClient(customClient),          // Использовать кастомный клиент
)
```

## Стратегии повтора

### Без повторов (по умолчанию)

```go
// По умолчанию клиент работает БЕЗ повторов
client, err := httpclient.NewClient()
```

### Включение повторов

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(5),                       // Максимум 5 попыток
    httpclient.WithRetryWait(1*time.Second, 10*time.Second), // Мин/макс время ожидания
)
```

### Экспоненциальная задержка

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3,                          // максимальное количество попыток
        100*time.Millisecond,       // базовая задержка
        5*time.Second,              // максимальная задержка
    )),
)
```

### Фиксированная задержка

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewFixedDelayStrategy(
        3,                          // максимальное количество попыток
        1*time.Second,              // задержка между попытками
    )),
)
```

### Умная стратегия

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

## Circuit Breaker

### Простой circuit breaker

```go
client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(httpclient.NewSimpleCircuitBreaker()),
)
```

### Настраиваемый circuit breaker

```go
circuitBreaker := httpclient.NewCircuitBreaker(
    5,                    // failureThreshold - количество ошибок для открытия
    10*time.Second,       // timeout - время ожидания перед переходом в half-open
    3,                    // maxRequests - максимум запросов в half-open состоянии
)

client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(circuitBreaker),
)
```

## Middleware

### Добавление одного middleware

```go
client, err := httpclient.NewClient(
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
)
```

### Добавление нескольких middleware

```go
client, err := httpclient.NewClient(
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware("token")),
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(10, 20)),
)
```

## Метрики и трейсинг

### Встроенные метрики

```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),                     // Включить встроенные метрики
)
```

### OpenTelemetry

```go
client, err := httpclient.NewClient(
    httpclient.WithOpenTelemetry(true),               // Включить OpenTelemetry
    httpclient.WithMetrics(true),                     // Можно комбинировать с метриками
)
```

## Управление функциями

### Отключение функций

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryDisabled(),                   // Отключить повторы полностью
    httpclient.WithMetricsDisabled(),                 // Отключить сбор метрик
    httpclient.WithTracingDisabled(),                 // Отключить трейсинг
)
```

### Включение только необходимых функций

```go
// Минимальная конфигурация - только HTTP клиент
client, err := httpclient.NewClient(
    httpclient.WithTimeout(10*time.Second),
)

// Клиент с повторами но без метрик
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithMetricsDisabled(),
)
```

## Комплексная конфигурация

### Продакшн конфигурация

```go
logger, _ := zap.NewProduction()

client, err := httpclient.NewClient(
    // Базовые настройки
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithMaxIdleConns(100),
    httpclient.WithMaxConnsPerHost(20),
    
    // Повторы с экспоненциальной задержкой
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3, 200*time.Millisecond, 5*time.Second)),
    
    // Circuit breaker для защиты от каскадных сбоев
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(5, 10*time.Second, 3)),
    
    // Middleware
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(100, 150)),
    
    // Observability
    httpclient.WithMetrics(true),
    httpclient.WithOpenTelemetry(true),
)
```

### Разработческая конфигурация

```go
logger, _ := zap.NewDevelopment()

client, err := httpclient.NewClient(
    // Более короткие таймауты для быстрой разработки
    httpclient.WithTimeout(10*time.Second),
    
    // Агрессивные повторы для нестабильной среды
    httpclient.WithRetryMax(5),
    httpclient.WithRetryStrategy(httpclient.NewSmartRetryStrategy(
        5, 100*time.Millisecond, 3*time.Second)),
    
    // Подробное логирование
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    
    // Метрики для debug
    httpclient.WithMetrics(true),
)
```

### Тестовая конфигурация

```go
client, err := httpclient.NewClient(
    // Быстрые таймауты для тестов
    httpclient.WithTimeout(5*time.Second),
    
    // Без повторов в тестах
    httpclient.WithRetryMax(0),
    
    // Отключить метрики и трейсинг
    httpclient.WithMetricsDisabled(),
    httpclient.WithTracingDisabled(),
)
```

## Конфигурация для разных сценариев

### Клиент для внешних API

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3, 500*time.Millisecond, 10*time.Second)),
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(3, 30*time.Second, 2)),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(10, 15)), // Консервативный rate limit
)
```

### Клиент для внутренних микросервисов

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(5*time.Second),  // Быстрые таймауты
    httpclient.WithRetryMax(5),
    httpclient.WithRetryStrategy(httpclient.NewSmartRetryStrategy(
        5, 50*time.Millisecond, 2*time.Second)),
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(10, 5*time.Second, 5)),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(1000, 1500)), // Высокий rate limit
)
```

### CLI утилита

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(60*time.Second), // Длинные операции
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3, 1*time.Second, 30*time.Second)),
    httpclient.WithMetrics(true), // Встроенные метрики без экспорта
)
```

## Валидация конфигурации

```go
func validateClient(client httpclient.ExtendedHTTPClient) error {
    // Тестовый запрос для проверки конфигурации
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err := client.Get("https://httpbin.org/status/200")
    return err
}
```

## Лучшие практики

### 1. Начинайте с простого
```go
// Сначала базовая конфигурация
client, err := httpclient.NewClient(
    httpclient.WithTimeout(10*time.Second),
)

// Добавляйте функции по мере необходимости
```

### 2. Настраивайте под среду
```go
var client httpclient.ExtendedHTTPClient

switch os.Getenv("ENV") {
case "production":
    client = createProductionClient()
case "development":
    client = createDevelopmentClient()
default:
    client = createTestClient()
}
```

### 3. Используйте фабричные функции
```go
func NewAPIClient(apiToken string) (httpclient.ExtendedHTTPClient, error) {
    return httpclient.NewClient(
        httpclient.WithTimeout(30*time.Second),
        httpclient.WithRetryMax(3),
        httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware(apiToken)),
        httpclient.WithMetrics(true),
    )
}
```

### 4. Документируйте конфигурацию
```go
// ProductionHTTPClient создает HTTP клиент для продакшн среды
// с консервативными настройками повторов и circuit breaker
func ProductionHTTPClient() httpclient.ExtendedHTTPClient {
    // ...
}
```

## См. также

- [Стратегии повтора](retry-strategies.md) - Подробнее о настройке повторов
- [Circuit Breaker](circuit-breaker.md) - Настройка автоматического выключателя
- [Middleware](middleware.md) - Система промежуточного ПО
- [Метрики](metrics.md) - Настройка сбора метрик