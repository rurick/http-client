# Rate Limiter

Комплексный гид по использованию встроенного Rate Limiter для управления частотой HTTP запросов.

## Обзор

Rate Limiter реализует алгоритм **Token Bucket** для ограничения частоты исходящих HTTP запросов. Основные задачи:
- Соблюдение лимитов внешних API
- Защита от перегрузки собственных сервисов
- Предотвращение превышения квот
- Плавное распределение нагрузки

## Архитектура

Rate Limiter интегрирован как middleware в цепочке обработки запросов:

```
HTTP Request
    ↓
Circuit Breaker (опционально)
    ↓
Rate Limiter (опционально) ← КОМПОНЕНТ
    ↓
RoundTripper (retry + metrics + tracing)
    ↓
Base HTTP Transport
    ↓
Сеть / Внешний сервис
```

**Особенности:**
- Полностью опциональный (включается через `RateLimiterEnabled: true`)
- Не влияет на существующую логику retry, metrics, tracing
- Работает на уровне всего клиента (глобальный лимит)
- Потокобезопасный (thread-safe)

## Алгоритм Token Bucket

### Принцип работы

Token Bucket ("Корзина токенов") - классический алгоритм для rate limiting:

1. **Корзина имеет емкость** (`BurstCapacity`) - максимальное количество токенов
2. **Токены добавляются с постоянной скоростью** (`RequestsPerSecond`)
3. **Каждый запрос потребляет 1 токен**
4. **Если токенов нет - запрос ожидает** их появления

### Визуализация

```
Время: 0s        1s        2s        3s
       ┌─────────────────────────────┐
       │ ████████ (8 токенов)       │ BurstCapacity = 10
       │                             │ Rate = 5 tokens/sec
       └─────────────────────────────┘
           ↓ 3 запроса (3 токена)
       ┌─────────────────────────────┐
       │ █████ (5 токенов)           │
       └─────────────────────────────┘
           ↓ +5 токенов за 1 секунду
       ┌─────────────────────────────┐
       │ ██████████ (10 токенов)     │ (достигнут лимит)
       └─────────────────────────────┘
```

### Преимущества алгоритма

✅ **Burst traffic** - позволяет короткие всплески запросов  
✅ **Smooth limiting** - плавное ограничение без резких отказов  
✅ **Predictable** - предсказуемое поведение и латентность  
✅ **Wait strategy** - автоматическое ожидание вместо отклонения  
✅ **Simple** - простая и понятная математика  

## Интерфейс RateLimiter

```go
type RateLimiter interface {
    // Allow проверяет, можно ли выполнить запрос немедленно
    Allow() bool

    // Wait блокирует выполнение до получения разрешения на запрос
    Wait(ctx context.Context) error
}
```

### Методы

#### Allow()
Неблокирующая проверка доступности токена.

```go
if limiter.Allow() {
    // Токен доступен, можно сделать запрос
    resp, err := client.Get(ctx, url)
} else {
    // Токен недоступен, лимит исчерпан
}
```

**Возвращает:**
- `true` - токен доступен, запрос можно выполнить немедленно
- `false` - токен недоступен, нужно подождать

**Использование:** когда нужен fail-fast подход без ожидания.

#### Wait(ctx)
Блокирующее ожидание доступности токена.

```go
// Ожидаем доступности токена
err := limiter.Wait(ctx)
if err != nil {
    // Контекст отменен или истек таймаут
    return err
}

// Токен получен, можно делать запрос
resp, err := client.Get(ctx, url)
```

**Параметры:**
- `ctx context.Context` - контекст для отмены ожидания

**Возвращает:**
- `nil` - токен получен, можно продолжать
- `error` - контекст отменен или истек таймаут

**Использование:** в HTTP клиенте используется автоматически.

## Реализация TokenBucketLimiter

### Структура

```go
type TokenBucketLimiter struct {
    rate     float64    // токенов в секунду (RequestsPerSecond)
    capacity int        // максимальная емкость корзины (BurstCapacity)
    tokens   float64    // текущее количество токенов
    lastTime time.Time  // время последнего обновления
    mu       sync.Mutex // защита от конкурентного доступа
}
```

### Создание

```go
// Напрямую через конструктор
limiter := httpclient.NewTokenBucketLimiter(
    5.0,  // rate: 5 запросов в секунду
    10,   // capacity: корзина на 10 токенов
)

// Через конфигурацию клиента (рекомендуется)
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,
        BurstCapacity:     10,
    },
}
client := httpclient.New(config, "my-service")
```

### Внутренняя логика

#### Пополнение токенов (refill)

```go
// Псевдокод логики пополнения
elapsed := time.Since(lastTime).Seconds()
tokensToAdd := elapsed * rate
tokens = min(tokens + tokensToAdd, capacity)
```

**Пример:**
- Rate = 5 tokens/sec
- Прошло 2 секунды
- Добавляется: 2 * 5 = 10 токенов
- Но не более capacity

#### Потребление токена

```go
// Псевдокод потребления
refill() // пополняем перед проверкой
if tokens >= 1.0 {
    tokens -= 1.0
    return true // токен получен
}
return false // токен недоступен
```

## Конфигурация

### RateLimiterConfig

```go
type RateLimiterConfig struct {
    RequestsPerSecond float64 // Максимальное количество запросов в секунду
    BurstCapacity     int     // Размер корзины для пиковых запросов
}
```

### RequestsPerSecond (RPS)

Максимальная устойчивая скорость запросов.

**Значения:**
- **По умолчанию:** `10.0`
- **Минимум:** `> 0.0` (положительное число)
- **Рекомендуемые:**
  - Консервативно: `1.0 - 5.0`
  - Умеренно: `10.0 - 50.0`
  - Агрессивно: `100.0+`

**Примеры:**
```go
RateLimiterConfig{
    RequestsPerSecond: 5.0,   // 5 RPS - строгий лимит
    RequestsPerSecond: 10.0,  // 10 RPS - умеренный
    RequestsPerSecond: 100.0, // 100 RPS - высокая нагрузка
}
```

**Выбор значения:**
- Проверьте документацию API (обычно указан лимит)
- Начните консервативно и увеличивайте постепенно
- Учитывайте другие потребители того же API
- Оставляйте запас для burst запросов

### BurstCapacity

Максимальный размер корзины токенов.

**Значения:**
- **По умолчанию:** равен `RequestsPerSecond`
- **Минимум:** `> 0` (положительное целое)
- **Рекомендуемые:**
  - Консервативно: `= RequestsPerSecond` (без burst)
  - Умеренно: `= RequestsPerSecond * 1.5 - 2`
  - Агрессивно: `> RequestsPerSecond * 2`

**Примеры:**
```go
RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     10,   // Без burst запасов
}

RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     20,   // Умеренный burst
}

RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     50,   // Агрессивный burst
}
```

**Выбор значения:**
- `= RPS`: для строгого ограничения без всплесков
- `> RPS`: для обработки коротких пиков нагрузки
- Учитывайте паттерн использования (равномерный vs. пакетный)

## Примеры использования

### Базовое использование

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,  // 5 запросов в секунду
        BurstCapacity:     10,   // до 10 запросов сразу
    },
}

client := httpclient.New(config, "rate-limited-service")
defer client.Close()

// Запросы автоматически ограничиваются
resp, err := client.Get(ctx, "https://api.example.com/data")
```

### Внешние API с жесткими лимитами

```go
// API с лимитом 100 запросов в минуту (≈1.67 RPS)
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.5,  // Консервативно: 90 запросов/мин
        BurstCapacity:     3,    // Минимальный burst
    },
    Timeout: 30 * time.Second,
}

client := httpclient.New(config, "external-api")
defer client.Close()
```

### Высокопроизводительные сервисы

```go
// Внутренний микросервис с высокой пропускной способностью
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 100.0, // 100 RPS
        BurstCapacity:     200,   // Двойной burst
    },
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
    },
}

client := httpclient.New(config, "high-throughput-service")
defer client.Close()
```

### Строгое ограничение без burst

```go
// Критичный сервис с гарантированным лимитом
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.0,  // 1 запрос в секунду
        BurstCapacity:     1,    // Без burst
    },
}

client := httpclient.New(config, "strict-limiter")
defer client.Close()
```

### Пакетная обработка с burst

```go
// Обработка данных пакетами с периодическими всплесками
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0, // Средняя скорость 10 RPS
        BurstCapacity:     50,   // Пакет до 50 запросов сразу
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
