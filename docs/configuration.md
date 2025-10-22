# Конфигурация

HTTP клиент пакет предлагает комплексные возможности конфигурации для различных сценариев использования.

## Структура Config

```go
type Config struct {
    Timeout         time.Duration    // Общий таймаут запроса
    PerTryTimeout   time.Duration    // Таймаут на каждую попытку
    RetryEnabled    bool             // Включает/выключает retry механизм  
    RetryConfig     RetryConfig      // Конфигурация повторов
    TracingEnabled  bool             // Включить OpenTelemetry tracing
    Transport       http.RoundTripper // Пользовательский транспорт
    CircuitBreakerEnable bool        // Включить Circuit Breaker
    CircuitBreaker       httpclient.CircuitBreaker // Экземпляр Circuit Breaker
	RateLimiterEnabled bool         // RateLimiterEnabled включает/выключает rate limiting
	RateLimiterConfig RateLimiterConfig // RateLimiterConfig конфигурация rate limiter
}
```

## Параметры конфигурации

### Timeout (Общий таймаут)
- **Тип:** `time.Duration`
- **По умолчанию:** `5 * time.Second`
- **Описание:** Максимальное время ожидания для всего запроса, включая все retry попытки

```go
config := httpclient.Config{
    Timeout: 30 * time.Second, // Общий лимит 30 секунд
}
```

### PerTryTimeout (Таймаут попытки)
- **Тип:** `time.Duration`
- **По умолчанию:** `2 * time.Second`
- **Описание:** Максимальное время ожидания для одной попытки запроса

```go
config := httpclient.Config{
    PerTryTimeout: 5 * time.Second, // Каждая попытка до 5 секунд
}
```

### RetryEnabled (Включает/выключает retry механизм )
- **Тип:** `bool`
- **По умолчанию:** `false`
- **Описание:** Включает/выключает retry механизм

```go
config := httpclient.Config{
	RetryEnabled: true,
}

client := httpclient.New(config, "httpclient")
```

### TracingEnabled (Включение tracing)
- **Тип:** `bool`
- **По умолчанию:** `false`
- **Описание:** Включает создание OpenTelemetry спанов для каждого запроса

```go
config := httpclient.Config{
    TracingEnabled: true, // Включить tracing
}
```

### Transport (Пользовательский транспорт)
- **Тип:** `http.RoundTripper`
- **По умолчанию:** `http.DefaultTransport`
- **Описание:** Позволяет настроить пользовательский HTTP транспорт

```go
config := httpclient.Config{
    Transport: &http.Transport{
        MaxIdleConns:       100,
        IdleConnTimeout:    90 * time.Second,
        DisableCompression: false,
    },
}
    RateLimiterEnabled bool                // Включить Rate Limiter
    RateLimiterConfig  RateLimiterConfig   // Конфигурация Rate Limiter
```

## Конфигурация Rate Limiter

Rate Limiter реализует алгоритм Token Bucket для ограничения частоты исходящих запросов. Это помогает соблюдать ограничения API внешних сервисов и защищать от перегрузки.

### Интерфейс RateLimiter

```go
type RateLimiter interface {
    // Allow проверяет, можно ли выполнить запрос немедленно
    Allow() bool

    // Wait блокирует выполнение до получения разрешения на запрос
    Wait(ctx context.Context) error
}
```

**Методы:**
- `Allow()` - Неблокирующая проверка доступности токена. Возвращает `true`, если запрос можно выполнить немедленно
- `Wait(ctx)` - Блокирует выполнение до появления токена. Учитывает контекст для отмены

### Реализация TokenBucketLimiter

```go
type TokenBucketLimiter struct {
    rate     float64    // токенов в секунду
    capacity int        // максимальная емкость корзины
    tokens   float64    // текущее количество токенов
    lastTime time.Time  // время последнего обновления
    mu       sync.Mutex // защита от конкурентного доступа
}
```

**Создание:**
```go
limiter := httpclient.NewTokenBucketLimiter(
    5.0,  // rate: 5 запросов в секунду
    10,   // capacity: корзина на 10 токенов
)
```

### Архитектура Rate Limiter

Rate Limiter реализован как middleware в цепочке RoundTripper'ов. Он выполняется перед основным механизмом retry и метрик, но после Circuit Breaker.

```
HTTP Request
    ↓
Circuit Breaker (опционально)
    ↓
Rate Limiter (опционально) ← НОВЫЙ КОМПОНЕНТ
    ↓
RoundTripper (retry + metrics + tracing)
    ↓
Base HTTP Transport
    ↓
Сеть / Внешний сервис
```

#### Особенности архитектуры:

1. **Middleware pattern**: Rate Limiter не изменяет существующую логику, а добавляет новый слой
2. **Опциональность**: Полностью опциональный компонент, включается только при RateLimiterEnabled: true
3. **Позиционирование**: Правильно расположен в цепочке - лимитирует до retry, но после circuit breaker
4. **Независимость**: Не влияет на метрики, tracing или retry логику

### Алгоритм Token Bucket

Rate Limiter использует алгоритм Token Bucket ("Корзина токенов"):

#### Принцип работы:
1. Корзина имеет определенную емкость (BurstCapacity)
2. Токены добавляются с постоянной скоростью (RequestsPerSecond)
3. Каждый запрос потребляет 1 токен
4. Если токенов нет - запрос ожидает их появления

#### Преимущества:
- **Burst traffic**: Позволяет короткие всплески запросов
- **Smooth limiting**: Плавное ограничение без резких отказов
- **Predictable**: Предсказуемое поведение и латентность
- **Wait strategy**: Автоматическое ожидание вместо отклонения

### Структура RateLimiterConfig

```go
type RateLimiterConfig struct {
    RequestsPerSecond float64 // Максимальное количество запросов в секунду
    BurstCapacity     int     // Размер корзины для пиковых запросов
}
```

### RateLimiterEnabled (Включение Rate Limiter)
- **Тип:** `bool`
- **По умолчанию:** `false`
- **Описание:** Включает/выключает rate limiting для всех запросов

```go
config := httpclient.Config{
    RateLimiterEnabled: true, // Включить rate limiting
}
```

### RequestsPerSecond (Запросов в секунду)
- **Тип:** `float64`
- **По умолчанию:** `10.0`
- **Описание:** Максимальная устойчивая скорость запросов. Токены добавляются в корзину с этой скоростью

```go
RateLimiterConfig{
    RequestsPerSecond: 5.0, // 5 запросов в секунду
}
```

### BurstCapacity (Размер корзины)
- **Тип:** `int`
- **По умолчанию:** равен `RequestsPerSecond`
- **Описание:** Максимальное количество токенов в корзине. Позволяет делать пиковые запросы сверх устойчивой скорости

```go
RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     20, // Можно сразу сделать до 20 запросов
}
```


## Конфигурация Retry

### Структура RetryConfig

```go
type RetryConfig struct {
    MaxAttempts int           // Максимальное количество попыток
    BaseDelay   time.Duration // Базовая задержка для backoff
    MaxDelay    time.Duration // Максимальная задержка
    Jitter      float64       // Фактор джиттера (0.0-1.0)
    RetryMethods []string     // список HTTP методов для retry
    RetryStatusCodes []int   // список HTTP статусов для retry
    RespectRetryAfter bool    // учитывать заголовок Retry-After
}
```

### MaxAttempts (Максимум попыток)
- **Тип:** `int`
- **По умолчанию:** `3` (1 основная + 2 повтора)
- **Описание:** Общее количество попыток (включая первоначальную)

```go
RetryConfig{
    MaxAttempts: 3, // 1 основная + 2 повтора
}
```

### BaseDelay (Базовая задержка)
- **Тип:** `time.Duration`
- **По умолчанию:** `100 * time.Millisecond`
- **Описание:** Начальная задержка для экспоненциального backoff

```go
RetryConfig{
    BaseDelay: 200 * time.Millisecond, // Начинать с 200ms
}
```

### MaxDelay (Максимальная задержка)
- **Тип:** `time.Duration`
- **По умолчанию:** `2 * time.Second`
- **Описание:** Максимальная задержка между попытками

```go
RetryConfig{
    MaxDelay: 10 * time.Second, // Не больше 10 секунд
}
```

### Jitter (Джиттер)
- **Тип:** `float64`
- **Диапазон:** `0.0 - 1.0`
- **По умолчанию:** `0.2`
- **Описание:** Случайное отклонение задержки для предотвращения thundering herd

```go
RetryConfig{
    Jitter: 0.3, // ±30% случайного отклонения
}
```

### RetryMethods (HTTP методы для retry)
- **Тип:** `[]string`
- **По умолчанию:** `["GET", "HEAD", "OPTIONS", "PUT", "DELETE"]`
- **Описание:** Список HTTP методов, для которых будет выполняться retry. По умолчанию включены только идемпотентные методы. POST и PATCH будут повторяться только при наличии заголовка `Idempotency-Key`

```go
RetryConfig{
    RetryMethods: []string{"GET", "POST", "PUT"}, // Кастомный список методов
}
```

### RetryStatusCodes (HTTP статус коды для retry)
- **Тип:** `[]int`
- **По умолчанию:** `[429, 500, 502, 503, 504]`
- **Описание:** Список HTTP статус кодов, при получении которых будет выполняться retry. Включает временные серверные ошибки и rate limiting

```go
RetryConfig{
    RetryStatusCodes: []int{429, 500, 502, 503}, // Исключить 504 Gateway Timeout
}
```

### RespectRetryAfter (Учет заголовка Retry-After)
- **Тип:** `bool`
- **По умолчанию:** `true`
- **Описание:** При значении `true` клиент будет учитывать заголовок `Retry-After` в ответах сервера и ждать указанное время перед повторной попыткой. Имеет приоритет над стандартным backoff алгоритмом

```go
RetryConfig{
    RespectRetryAfter: false, // Игнорировать Retry-After, использовать только backoff
}
```

## Примеры использования Rate Limiter

### Ограничение для внешних API

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,  // API позволяет 5 RPS
        BurstCapacity:     10,   // Можно сделать пакет до 10 запросов
    },
}

client := httpclient.New(config, "external-api-client")
```

### Высокочастотные запросы с burst поддержкой

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 100.0, // 100 RPS глобально
        BurstCapacity:     50,    // Консервативный burst
    },
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
    },
}

client := httpclient.New(config, "high-throughput-client")
// Глобальный лимитер для всех запросов клиента
```

### Консервативная конфигурация для надежности

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.0,  // Очень консервативно
        BurstCapacity:     1,    // Никаких пиков
    },
    Timeout:       30 * time.Second,
    PerTryTimeout: 10 * time.Second,
}

client := httpclient.New(config, "conservative-client")
```

### Кастомный Rate Limiter

Можно создать собственную реализацию `RateLimiter` для особых случаев:

```go
// Создание кастомного limiter напрямую
limiter := httpclient.NewTokenBucketLimiter(50.0, 100)

// Использование в клиенте через Config
// ВАЖНО: при использовании кастомного RateLimiter,
// RateLimiterConfig игнорируется
config := httpclient.Config{
    RateLimiterEnabled: true,
    // RateLimiterConfig не используется, если установлен RateLimiter
}

// Кастомный limiter устанавливается через внутренние механизмы
// (см. исходный код для деталей реализации)
client := httpclient.New(config, "custom-limiter-client")
```

**Когда использовать кастомный limiter:**
- Нужна специфическая логика ограничения (например, разные лимиты для разных endpoint'ов)
- Требуется интеграция с внешней системой rate limiting
- Необходим особый алгоритм (не Token Bucket)

**Реализация собственного RateLimiter:**
```go
type MyCustomLimiter struct {
    // ваша логика
}

func (m *MyCustomLimiter) Allow() bool {
    // реализация неблокирующей проверки
    return true
}

func (m *MyCustomLimiter) Wait(ctx context.Context) error {
    // реализация блокирующего ожидания
    return nil
}
```

## Значения по умолчанию

```go
// Автоматически применяется при создании клиента
defaultConfig := Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 2 * time.Second,
    RetryConfig: RetryConfig{
        MaxAttempts: 3,        // 1 основная + 2 повтора
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    2 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled:     false,
    RateLimiterEnabled: false, // Rate limiter выключен по умолчанию
    Transport:          http.DefaultTransport,
}
```

## Примеры конфигураций

### Быстрые внутренние сервисы

```go
config := httpclient.Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 1 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   50 * time.Millisecond,
        MaxDelay:    500 * time.Millisecond,
        Jitter:      0.1,
    },
}

client := httpclient.New(config, "internal-api")
```

### Внешние API (требующие надежности)

```go
config := httpclient.Config{
    Timeout:       30 * time.Second,
    PerTryTimeout: 10 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   200 * time.Millisecond,
        MaxDelay:    10 * time.Second,
        Jitter:      0.3,
    },
    TracingEnabled: true,
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0, // Соблюдаем лимиты API
        BurstCapacity:     15,   // Небольшой burst
    },
}

client := httpclient.New(config, "external-api")
```

### Критичные платежные сервисы

```go
config := httpclient.Config{
    Timeout:       60 * time.Second,
    PerTryTimeout: 15 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 7,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Jitter:      0.25,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        50,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

client := httpclient.New(config, "payment-service")
```

### Высокопроизводительные API Gateway

```go
config := httpclient.Config{
    Timeout:       10 * time.Second,
    PerTryTimeout: 3 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   25 * time.Millisecond,
        MaxDelay:    1 * time.Second,
        Jitter:      0.1,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        200,
        MaxIdleConnsPerHost: 50,
        IdleConnTimeout:     60 * time.Second,
    },
}

client := httpclient.New(config, "api-gateway")
```

## Продвинутые настройки Transport

### Настройка пулов соединений

```go
transport := &http.Transport{
    // Общий пул соединений
    MaxIdleConns:        100,
    
    // Соединения на хост
    MaxIdleConnsPerHost: 10,
    
    // Время жизни idle соединений
    IdleConnTimeout:     90 * time.Second,
    
    // Таймауты TLS
    TLSHandshakeTimeout: 10 * time.Second,
    
    // Таймауты TCP
    DialTimeout:         5 * time.Second,
    
    // Keep-alive
    KeepAlive:           30 * time.Second,
    
    // Отключить сжатие
    DisableCompression:  false,
    
    // Размер буфера чтения
    ReadBufferSize:      4096,
    
    // Размер буфера записи
    WriteBufferSize:     4096,
}

config := httpclient.Config{
    Transport: transport,
}
```

### Настройка TLS

```go
tlsConfig := &tls.Config{
    // Проверка сертификатов
    InsecureSkipVerify: false,
    
    // Минимальная версия TLS
    MinVersion: tls.VersionTLS12,
    
    // Предпочитаемые cipher suites
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

config := httpclient.Config{
    Transport: transport,
}
```

## Валидация конфигурации

Пакет автоматически валидирует конфигурацию:

```go
// Некорректные значения будут исправлены
config := httpclient.Config{
    Timeout:       -1 * time.Second,  // Будет установлен в default
    PerTryTimeout: 0,                 // Будет установлен в default
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: -5,              // Будет установлен в 1
        Jitter:      2.0,             // Будет ограничен до 1.0
    },
}

client := httpclient.New(config, "service") // Работает с исправленными значениями
```

## Получение текущей конфигурации

```go
client := httpclient.New(config, "service")

// Получить активную конфигурацию
currentConfig := client.GetConfig()

fmt.Printf("Timeout: %v\n", currentConfig.Timeout)
fmt.Printf("Max Attempts: %d\n", currentConfig.RetryConfig.MaxAttempts)
```

## Рекомендации по конфигурации

### По типу сервиса

| Тип сервиса | Timeout | PerTryTimeout | MaxAttempts | BaseDelay |
|-------------|---------|---------------|-------------|-----------|
| Внутренний API | 5s | 1s | 2 | 50ms |
| Внешний API | 30s | 10s | 5 | 200ms |
| Базы данных | 10s | 3s | 3 | 100ms |
| Платежи | 60s | 15s | 7 | 500ms |
| File Upload | 300s | 60s | 3 | 1s |

### По SLA требованиям

- **99.9% SLA:** MaxAttempts = 3-5, агрессивные таймауты
- **99.95% SLA:** MaxAttempts = 5-7, умеренные таймауты  
- **99.99% SLA:** MaxAttempts = 7-10, консервативные таймауты

### По сетевой среде

- **Внутренняя сеть:** Низкий jitter (0.1), быстрые таймауты
- **Публичный интернет:** Высокий jitter (0.3), длинные таймауты
- **Мобильные сети:** Очень высокий jitter (0.5), очень длинные таймауты

## Отладка конфигурации

Включите tracing для отладки:

```go
config := httpclient.Config{
    TracingEnabled: true,
    // ... другие настройки
}
```

Это поможет увидеть:
- Реальное время выполнения запросов
- Количество retry попыток
- Причины ошибок
- Эффективность backoff стратегии

## Логика сохранения статус кодов

HTTP клиент корректно сохраняет и возвращает статус коды ответов в зависимости от результата retry логики:

### Успешный retry
Когда retry завершается успехом, возвращается статус код успешного ответа:
```go
// Последовательность: 500 → 503 → 200
// Результат: StatusCode = 200 (успех)
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 200 {
    // Получили успешный ответ после retry
}
```

### Исчерпание retry попыток
Когда все retry попытки исчерпаны, возвращается статус код последней попытки:
```go
// Последовательность: 500 → 503 → 502
// Результат: StatusCode = 502 (последняя ошибка)
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 502 {
    // Все retry исчерпаны, возвращается последний статус
}
```

### Отсутствие retry
Когда retry не применяется, возвращается оригинальный статус код:
```go
// 400 Bad Request (не подлежит retry)
// Результат: StatusCode = 400 (оригинальная ошибка)  
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 400 {
    // Оригинальный статус сохранён
}
```

### Смешанные статус коды
Клиент корректно обрабатывает различные комбинации статус кодов:
```go
// Пример: 502 → 429 → 201
// Результат: StatusCode = 201 (финальный успех)

// Пример: 429 → 429 → 429 
// Результат: StatusCode = 429 (последняя из исчерпанных попыток)
```

**Гарантии:**
- ✅ Статус код последней попытки всегда сохраняется
- ✅ Успешные ответы имеют приоритет над ошибками
- ✅ Клиентские ошибки (4xx) не влияют на retry логику
- ✅ Серверные ошибки (5xx) и 429 корректно обрабатываются