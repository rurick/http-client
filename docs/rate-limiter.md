# Rate Limiter

Comprehensive guide to using the built-in Rate Limiter for managing HTTP request frequency.

## Overview

Rate Limiter implements the **Token Bucket** algorithm to limit outgoing HTTP request frequency. Main tasks:
- Complying with external API limits
- Protecting own services from overload
- Preventing quota overruns
- Smooth load distribution

## Architecture

Rate Limiter is integrated as middleware in the request processing chain:

```
HTTP Request
    ↓
Circuit Breaker (optional)
    ↓
Rate Limiter (optional) ← COMPONENT
    ↓
RoundTripper (retry + metrics + tracing)
    ↓
Base HTTP Transport
    ↓
Network / External Service
```

**Features:**
- Fully optional (enabled via `RateLimiterEnabled: true`)
- Doesn't affect existing retry, metrics, tracing logic
- Works at the entire client level (global limit)
- Thread-safe

## Token Bucket Algorithm

### Working Principle

Token Bucket ("Token Bucket") - classic algorithm for rate limiting:

1. **Bucket has capacity** (`BurstCapacity`) - maximum number of tokens
2. **Tokens are added at constant rate** (`RequestsPerSecond`)
3. **Each request consumes 1 token**
4. **If no tokens available - request waits** for them to appear

### Visualization

```
Time: 0s        1s        2s        3s
       ┌─────────────────────────────┐
       │ ████████ (8 tokens)        │ BurstCapacity = 10
       │                             │ Rate = 5 tokens/sec
       └─────────────────────────────┘
           ↓ 3 requests (3 tokens)
       ┌─────────────────────────────┐
       │ █████ (5 tokens)             │
       └─────────────────────────────┘
           ↓ +5 tokens per 1 second
       ┌─────────────────────────────┐
       │ ██████████ (10 tokens)      │ (limit reached)
       └─────────────────────────────┘
```

### Algorithm Advantages

✅ **Burst traffic** - allows short bursts of requests  
✅ **Smooth limiting** - smooth limiting without abrupt rejections  
✅ **Predictable** - predictable behavior and latency  
✅ **Wait strategy** - automatic waiting instead of rejection  
✅ **Simple** - simple and clear mathematics  

## RateLimiter Interface

```go
type RateLimiter interface {
    // Allow checks if a request can be executed immediately
    Allow() bool

    // Wait blocks execution until permission for a request is received
    Wait(ctx context.Context) error
}
```

### Methods

#### Allow()
Non-blocking token availability check.

```go
if limiter.Allow() {
    // Token available, can make request
    resp, err := client.Get(ctx, url)
} else {
    // Token unavailable, limit exhausted
}
```

**Returns:**
- `true` - token available, request can be executed immediately
- `false` - token unavailable, need to wait

**Usage:** when fail-fast approach without waiting is needed.

#### Wait(ctx)
Blocking wait for token availability.

```go
// Wait for token availability
err := limiter.Wait(ctx)
if err != nil {
    // Context cancelled or timeout expired
    return err
}

// Token received, can make request
resp, err := client.Get(ctx, url)
```

**Parameters:**
- `ctx context.Context` - context for cancellation of wait

**Returns:**
- `nil` - token received, can continue
- `error` - context cancelled or timeout expired

**Usage:** used automatically in HTTP client.

## TokenBucketLimiter Implementation

### Structure

```go
type TokenBucketLimiter struct {
    rate     float64    // tokens per second (RequestsPerSecond)
    capacity int        // maximum bucket capacity (BurstCapacity)
    tokens   float64    // current number of tokens
    lastTime time.Time  // last update time
    mu       sync.Mutex // concurrent access protection
}
```

### Creation

```go
// Directly via constructor
limiter := httpclient.NewTokenBucketLimiter(
    5.0,  // rate: 5 requests per second
    10,   // capacity: bucket for 10 tokens
)

// Via client configuration (recommended)
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,
        BurstCapacity:     10,
    },
}
client := httpclient.New(config, "my-service")
```

### Internal Logic

#### Token Refill

```go
// Pseudocode for refill logic
elapsed := time.Since(lastTime).Seconds()
tokensToAdd := elapsed * rate
tokens = min(tokens + tokensToAdd, capacity)
```

**Example:**
- Rate = 5 tokens/sec
- 2 seconds elapsed
- Add: 2 * 5 = 10 tokens
- But not more than capacity

#### Token Consumption

```go
// Pseudocode for consumption
refill() // refill before check
if tokens >= 1.0 {
    tokens -= 1.0
    return true // token obtained
}
return false // token unavailable
```

## Configuration

### RateLimiterConfig

```go
type RateLimiterConfig struct {
    RequestsPerSecond float64 // Maximum number of requests per second
    BurstCapacity     int     // Bucket size for peak requests
}
```

### RequestsPerSecond (RPS)

Maximum sustainable request rate.

**Values:**
- **Default:** `10.0`
- **Minimum:** `> 0.0` (positive number)
- **Recommended:**
  - Conservative: `1.0 - 5.0`
  - Moderate: `10.0 - 50.0`
  - Aggressive: `100.0+`

**Examples:**
```go
RateLimiterConfig{
    RequestsPerSecond: 5.0,   // 5 RPS - strict limit
    RequestsPerSecond: 10.0,  // 10 RPS - moderate
    RequestsPerSecond: 100.0, // 100 RPS - high load
}
```

**Value Selection:**
- Check API documentation (usually limit is specified)
- Start conservatively and increase gradually
- Consider other consumers of the same API
- Leave margin for burst requests

### BurstCapacity

Maximum token bucket size.

**Values:**
- **Default:** equals `RequestsPerSecond`
- **Minimum:** `> 0` (positive integer)
- **Recommended:**
  - Conservative: `= RequestsPerSecond` (no burst)
  - Moderate: `= RequestsPerSecond * 1.5 - 2`
  - Aggressive: `> RequestsPerSecond * 2`

**Examples:**
```go
RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     10,   // No burst reserves
}

RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     20,   // Moderate burst
}

RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     50,   // Aggressive burst
}
```

**Value Selection:**
- `= RPS`: for strict limiting without spikes
- `> RPS`: for handling short load peaks
- Consider usage pattern (uniform vs. batch)

## Usage Examples

### Basic Usage

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,  // 5 requests per second
        BurstCapacity:     10,   // up to 10 requests at once
    },
}

client := httpclient.New(config, "rate-limited-service")
defer client.Close()

// Requests are automatically limited
resp, err := client.Get(ctx, "https://api.example.com/data")
```

### External APIs with Strict Limits

```go
// API with limit of 100 requests per minute (≈1.67 RPS)
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.5,  // Conservative: 90 requests/min
        BurstCapacity:     3,    // Minimal burst
    },
    Timeout: 30 * time.Second,
}

client := httpclient.New(config, "external-api")
defer client.Close()
```

### High-Performance Services

```go
// Internal microservice with high throughput
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 100.0, // 100 RPS
        BurstCapacity:     200,   // Double burst
    },
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
    },
}

client := httpclient.New(config, "high-throughput-service")
defer client.Close()
```

### Strict Limiting Without Burst

```go
// Critical service with guaranteed limit
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.0,  // 1 request per second
        BurstCapacity:     1,    // No burst
    },
}

client := httpclient.New(config, "strict-limiter")
defer client.Close()
```

### Batch Processing with Burst

```go
// Batch data processing with periodic spikes
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0, // Average rate 10 RPS
        BurstCapacity:     50,   // Batch up to 50 requests at once
    },
}

client := httpclient.New(config, "batch-processor")
defer client.Close()

// Можно сделать 50 запросов сразу, затем 10 RPS
for i := 0; i < 100; i++ {
    resp, err := client.Get(ctx, fmt.Sprintf("https://api.example.com/item/%d", i))
    // Первые 50 - быстро, остальные - по 10 RPS
}
```

## Кастомный Rate Limiter

### Создание собственной реализации

```go
type MyCustomLimiter struct {
    // ваши поля
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

### Когда использовать кастомный limiter

- **Разные лимиты для разных endpoint'ов**
- **Интеграция с внешней системой** (Redis, Consul)
- **Особый алгоритм** (не Token Bucket)
- **Динамические лимиты** на основе метрик

## Взаимодействие с другими компонентами

### Rate Limiter + Retry

Rate Limiter применяется **перед** retry логикой:

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,
        BurstCapacity:     10,
    },
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
    },
}
```

**Поведение:**
1. Rate Limiter ограничивает частоту попыток
2. Каждая retry попытка также проходит через Rate Limiter
3. Учитывайте это при расчете общего времени выполнения

**Пример:**
- RPS = 5.0
- MaxAttempts = 3
- Если все попытки неудачны, это займет ≈0.4 секунды (3 запроса / 5 RPS)

### Rate Limiter + Circuit Breaker

Circuit Breaker проверяется **до** Rate Limiter:

```go
config := httpclient.Config{
    CircuitBreakerEnable: true,
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0,
        BurstCapacity:     20,
    },
}
```

**Порядок:**
1. Circuit Breaker проверяет состояние
2. Если OPEN - запрос отклоняется немедленно
3. Если CLOSED - Rate Limiter ограничивает частоту
4. Токен не тратится, если Circuit Breaker открыт

### Rate Limiter + Timeout

Rate Limiter учитывает контекст и таймауты:

```go
config := httpclient.Config{
    Timeout: 5 * time.Second,  // Общий таймаут
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.0,
    },
}

ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

// Если ожидание токена > 3 секунды, вернется context.DeadlineExceeded
resp, err := client.Get(ctx, url)
```

## Метрики и мониторинг

Rate Limiter интегрирован с системой метрик:

### Prometheus метрики

```
# Общее количество запросов
http_client_requests_total{service="my-service"}

# Задержка от rate limiting (включена в общую латентность)
http_client_request_duration_seconds{service="my-service"}
```

### Отладка

Для отладки Rate Limiter включите tracing:

```go
config := httpclient.Config{
    TracingEnabled: true,
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,
        BurstCapacity:     10,
    },
}
```

Это покажет:
- Время ожидания токена
- Фактическую частоту запросов
- Влияние rate limiting на общую латентность

## Best Practices

### Выбор параметров

1. **Начинайте консервативно**
   ```go
   RateLimiterConfig{
       RequestsPerSecond: API_LIMIT * 0.8,  // 80% от лимита API
       BurstCapacity:     API_LIMIT,        // Полный лимит для burst
   }
   ```

2. **Учитывайте другие потребители**
   ```go
   // Если API используется 3 сервисами
   myRPS := API_LIMIT / 3
   ```

3. **Мониторьте и адаптируйте**
   - Смотрите на метрики latency
   - Отслеживайте 429 ошибки от API
   - Корректируйте параметры на основе реальных данных

### Обработка ошибок

```go
resp, err := client.Get(ctx, url)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Таймаут ожидания токена
        log.Warn("Rate limiter timeout")
    }
    return err
}

if resp.StatusCode == 429 {
    // API вернул 429 несмотря на rate limiting
    // Нужно уменьшить RequestsPerSecond
    log.Warn("API rate limit exceeded despite rate limiting")
}
```

### Тестирование

```go
func TestRateLimiter(t *testing.T) {
    config := httpclient.Config{
        RateLimiterEnabled: true,
        RateLimiterConfig: httpclient.RateLimiterConfig{
            RequestsPerSecond: 10.0,
            BurstCapacity:     20,
        },
    }
    
    client := httpclient.New(config, "test")
    defer client.Close()
    
    start := time.Now()
    
    // Делаем 25 запросов
    for i := 0; i < 25; i++ {
        _, err := client.Get(context.Background(), testURL)
        require.NoError(t, err)
    }
    
    elapsed := time.Since(start)
    
    // Первые 20 - burst, оставшиеся 5 - по 10 RPS
    // Ожидаемое время: ~0.5 секунды
    assert.InDelta(t, 0.5, elapsed.Seconds(), 0.1)
}
```

## Troubleshooting

### Проблема: Запросы слишком медленные

**Симптомы:**
- Высокая latency
- Таймауты запросов
- Медленная обработка пакетов данных

**Решение:**
1. Увеличьте `RequestsPerSecond`
2. Увеличьте `BurstCapacity` для пиковых нагрузок
3. Проверьте, не слишком ли строгий лимит

### Проблема: API возвращает 429

**Симптомы:**
- HTTP 429 Too Many Requests
- Ошибки "rate limit exceeded"

**Решение:**
1. Уменьшите `RequestsPerSecond`
2. Уменьшите `BurstCapacity`
3. Добавьте retry с backoff для 429 ошибок
4. Проверьте лимиты API в документации

### Проблема: Неравномерная нагрузка

**Симптомы:**
- Пакеты быстрых запросов, затем долгое ожидание
- Непредсказуемая latency

**Решение:**
1. Настройте `BurstCapacity` ближе к `RequestsPerSecond`
2. Распределяйте запросы более равномерно в коде
3. Рассмотрите использование очередей

## Заключение

Rate Limiter - мощный инструмент для управления частотой запросов:

✅ **Простота** - минимальная конфигурация для старта  
✅ **Гибкость** - настройка под любые требования  
✅ **Надежность** - защита от превышения лимитов API  
✅ **Прозрачность** - работает автоматически без изменений кода  

**Рекомендуемая конфигурация для старта:**

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0, // Умеренное значение
        BurstCapacity:     15,   // Небольшой запас для burst
    },
    RetryEnabled: true,
    TracingEnabled: true, // Для мониторинга
}

client := httpclient.New(config, "my-service")
defer client.Close()
```

Адаптируйте параметры на основе реальных требований вашего API и мониторинга метрик.
