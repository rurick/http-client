# API Reference

Полное описание всех интерфейсов, методов и структур HTTP клиента.

## Основные интерфейсы

### HTTPClient

Базовый интерфейс HTTP клиента с стандартными методами.

```go
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
    Get(url string) (*http.Response, error)
    Head(url string) (*http.Response, error)
    Post(url, contentType string, body io.Reader) (*http.Response, error)
    PostForm(url string, data map[string][]string) (*http.Response, error)
}
```

### CtxHTTPClient (РЕКОМЕНДУЕТСЯ)

**Основной интерфейс для всех HTTP операций!** Всегда используйте эти методы вместо обычных для лучшего контроля запросов.

```go
type CtxHTTPClient interface {
    DoCtx(context.Context, *http.Request) (*http.Response, error)
    GetCtx(ctx context.Context, url string) (*http.Response, error)
    PostCtx(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
    PostFormCtx(ctx context.Context, url string, data map[string][]string) (*http.Response, error)
    HeadCtx(ctx context.Context, url string) (*http.Response, error)
}
```

**Преимущества контекстных методов:**
- Управление таймаутами запросов
- Возможность отмены запросов
- Распространение трейсинг информации
- Лучшая интеграция с Go ecosystem

### ExtendedHTTPClient

Расширенный интерфейс с дополнительными методами для JSON, XML и потоков.

```go
type ExtendedHTTPClient interface {
    HTTPClient

    // JSON методы
    GetJSON(ctx context.Context, url string, result interface{}) error
    PostJSON(ctx context.Context, url string, body interface{}, result interface{}) error
    PutJSON(ctx context.Context, url string, body interface{}, result interface{}) error
    PatchJSON(ctx context.Context, url string, body interface{}, result interface{}) error
    DeleteJSON(ctx context.Context, url string, result interface{}) error

    // XML методы
    GetXML(ctx context.Context, url string, result interface{}) error
    PostXML(ctx context.Context, url string, body interface{}, result interface{}) error




    // Поддержка контекстных методов
    CtxHTTPClient

    // Доступ к метрикам
    GetMetrics() *ClientMetrics
}
```

## Создание клиента

### NewClient

```go
func NewClient(options ...Option) (ExtendedHTTPClient, error)
```

Создает новый HTTP клиент с указанными опциями.

**Параметры:**
- `options ...Option` - список опций конфигурации

**Возвращает:**
- `ExtendedHTTPClient` - настроенный HTTP клиент
- `error` - ошибка создания клиента

**Пример:**
```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithRetryMax(3),
)
```

## Опции конфигурации

### Базовые опции

#### WithTimeout
```go
func WithTimeout(timeout time.Duration) Option
```
Устанавливает общий таймаут для HTTP запросов.

#### WithHTTPClient
```go
func WithHTTPClient(client *http.Client) Option
```
Использует пользовательский HTTP клиент.

#### WithMaxIdleConns
```go
func WithMaxIdleConns(maxIdleConns int) Option
```
Устанавливает максимальное количество неактивных соединений в пуле.

**Значение по умолчанию**: 100
**Рекомендации**: 20-500 в зависимости от нагрузки

См. подробнее: [Пул соединений](connection-pool.md)

#### WithMaxConnsPerHost
```go
func WithMaxConnsPerHost(maxConnsPerHost int) Option
```
Устанавливает максимальное количество соединений на хост.

**Значение по умолчанию**: 10
**Рекомендации**: 5-50 в зависимости от нагрузки

См. подробнее: [Пул соединений](connection-pool.md)

### Опции повторов

#### WithRetryMax
```go
func WithRetryMax(maxRetries int) Option
```
Устанавливает максимальное количество попыток повтора.

#### WithRetryWait
```go
func WithRetryWait(min, max time.Duration) Option
```
Устанавливает минимальное и максимальное время ожидания между повторами.

#### WithRetryStrategy
```go
func WithRetryStrategy(strategy RetryStrategy) Option
```
Устанавливает стратегию повтора.

#### WithRetryDisabled
```go
func WithRetryDisabled() Option
```
Полностью отключает механизм повторов.

### Опции Circuit Breaker

#### WithCircuitBreaker
```go
func WithCircuitBreaker(cb CircuitBreaker) Option
```
Устанавливает circuit breaker.

### Опции Middleware

#### WithMiddleware
```go
func WithMiddleware(middleware Middleware) Option
```
Добавляет middleware в цепочку обработки.

### Опции метрик и трейсинга

#### WithMetrics
```go
func WithMetrics(enabled bool) Option
```
Включает/выключает сбор встроенных метрик.

#### WithMetricsDisabled
```go
func WithMetricsDisabled() Option
```
Отключает сбор метрик.

#### WithOpenTelemetry
```go
func WithOpenTelemetry(enabled bool) Option
```
Включает/выключает интеграцию с OpenTelemetry.

#### WithTracingDisabled
```go
func WithTracingDisabled() Option
```
Отключает трейсинг.

## Стратегии повтора

### RetryStrategy интерфейс

```go
type RetryStrategy interface {
    NextDelay(attempt int, lastErr error) time.Duration
    ShouldRetry(resp *http.Response, err error) bool
    MaxAttempts() int
}
```

### ExponentialBackoffStrategy

```go
func NewExponentialBackoffStrategy(maxAttempts int, baseDelay, maxDelay time.Duration) *ExponentialBackoffStrategy
```

Стратегия с экспоненциальным увеличением задержки.

**Параметры:**
- `maxAttempts` - максимальное количество попыток
- `baseDelay` - базовая задержка
- `maxDelay` - максимальная задержка

### FixedDelayStrategy

```go
func NewFixedDelayStrategy(maxAttempts int, delay time.Duration) *FixedDelayStrategy
```

Стратегия с фиксированной задержкой.

**Параметры:**
- `maxAttempts` - максимальное количество попыток
- `delay` - задержка между попытками

### SmartRetryStrategy

```go
func NewSmartRetryStrategy(maxAttempts int, baseDelay, maxDelay time.Duration) *SmartRetryStrategy
```

Адаптивная стратегия повтора с анализом ошибок.

**Параметры:**
- `maxAttempts` - максимальное количество попыток
- `baseDelay` - базовая задержка
- `maxDelay` - максимальная задержка

### CustomRetryStrategy

```go
func NewCustomRetryStrategy(
    maxAttempts int, 
    shouldRetry func(resp *http.Response, err error) bool, 
    nextDelay func(attempt int, lastErr error) time.Duration
) *CustomRetryStrategy
```

Пользовательская стратегия повтора.

**Параметры:**
- `maxAttempts` - максимальное количество попыток
- `shouldRetry` - функция определения необходимости повтора
- `nextDelay` - функция вычисления задержки

## Circuit Breaker

### CircuitBreaker интерфейс

```go
type CircuitBreaker interface {
    Execute(fn func() (*http.Response, error)) (*http.Response, error)
    State() CircuitBreakerState
    Reset()
}
```

### CircuitBreakerState

```go
type CircuitBreakerState int

const (
    CircuitBreakerClosed CircuitBreakerState = iota
    CircuitBreakerOpen
    CircuitBreakerHalfOpen
)
```

### NewCircuitBreaker

```go
func NewCircuitBreaker(failureThreshold int, timeout time.Duration, maxRequests int) CircuitBreaker
```

Создает настраиваемый circuit breaker.

**Параметры:**
- `failureThreshold` - количество ошибок для открытия
- `timeout` - время ожидания в открытом состоянии
- `maxRequests` - максимум запросов в полуоткрытом состоянии

### NewSimpleCircuitBreaker

```go
func NewSimpleCircuitBreaker() CircuitBreaker
```

Создает circuit breaker с настройками по умолчанию.

## Middleware

### Middleware интерфейс

```go
type Middleware interface {
    Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
}
```

### Встроенные Middleware

#### NewBasicAuthMiddleware
```go
func NewBasicAuthMiddleware(username, password string) Middleware
```

#### NewBearerTokenMiddleware
```go
func NewBearerTokenMiddleware(token string) Middleware
```

#### NewAPIKeyMiddleware
```go
func NewAPIKeyMiddleware(headerName, apiKey string) Middleware
```

#### NewLoggingMiddleware
```go
func NewLoggingMiddleware(logger *zap.Logger) Middleware
```

#### NewRateLimitMiddleware
```go
func NewRateLimitMiddleware(rate int, capacity int) Middleware
```

#### NewTimeoutMiddleware
```go
func NewTimeoutMiddleware(timeout time.Duration) Middleware
```

#### NewUserAgentMiddleware
```go
func NewUserAgentMiddleware(userAgent string) Middleware
```



## Метрики

### ClientMetrics структура

```go
type ClientMetrics struct {
    TotalRequests       int64         // Общее количество запросов
    SuccessfulRequests  int64         // Успешные запросы (2xx)
    FailedRequests      int64         // Неудачные запросы (4xx, 5xx, errors)
    AverageLatency      time.Duration // Средняя задержка
    TotalRequestSize    int64         // Общий размер всех запросов
    TotalResponseSize   int64         // Общий размер всех ответов
}
```

### Методы ClientMetrics

#### GetStatusCodes
```go
func (m *ClientMetrics) GetStatusCodes() map[int]int64
```
Возвращает копию статистики по статус кодам.

#### Reset
```go
func (m *ClientMetrics) Reset()
```
Сбрасывает все метрики.

### MetricsCollector интерфейс

```go
type MetricsCollector interface {
    RecordRequest(method, url string, statusCode int, duration time.Duration, requestSize, responseSize int64)
    RecordRetry(method, url string, attempt int, err error)
    RecordCircuitBreakerState(state CircuitBreakerState)
    GetMetrics() *ClientMetrics
}
```

## Утилитарные функции

### IsRetryableStatusCode

```go
func IsRetryableStatusCode(statusCode int) bool
```

Проверяет, является ли HTTP статус код подходящим для повтора.

**Параметры:**
- `statusCode` - HTTP статус код

**Возвращает:**
- `bool` - true если код подходит для повтора

**Подходящие коды:**
- 429 (Too Many Requests)
- 500 (Internal Server Error)
- 502 (Bad Gateway)
- 503 (Service Unavailable)
- 504 (Gateway Timeout)

## JSON методы

### GetJSON
```go
func (c *Client) GetJSON(ctx context.Context, url string, result interface{}) error
```

### PostJSON
```go
func (c *Client) PostJSON(ctx context.Context, url string, body interface{}, result interface{}) error
```

### PutJSON
```go
func (c *Client) PutJSON(ctx context.Context, url string, body interface{}, result interface{}) error
```

### PatchJSON
```go
func (c *Client) PatchJSON(ctx context.Context, url string, body interface{}, result interface{}) error
```

### DeleteJSON
```go
func (c *Client) DeleteJSON(ctx context.Context, url string, result interface{}) error
```

## XML методы

### GetXML
```go
func (c *Client) GetXML(ctx context.Context, url string, result interface{}) error
```

### PostXML
```go
func (c *Client) PostXML(ctx context.Context, url string, body interface{}, result interface{}) error
```

## Константы и переменные

### RetryableHTTPCodes

```go
var RetryableHTTPCodes = []int{
    http.StatusTooManyRequests,     // 429
    http.StatusInternalServerError, // 500
    http.StatusBadGateway,          // 502
    http.StatusServiceUnavailable,  // 503
    http.StatusGatewayTimeout,      // 504
}
```

HTTP статус коды, для которых выполняются повторы по умолчанию.

## Ошибки

### Типичные ошибки

- `context.DeadlineExceeded` - превышен таймаут
- `net.Error` - сетевые ошибки
- `json.SyntaxError` - ошибки парсинга JSON
- `xml.SyntaxError` - ошибки парсинга XML

### Обработка ошибок

```go
resp, err := client.Get("https://api.example.com")
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Обработка таймаута
    } else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
        // Обработка network timeout
    } else {
        // Другие ошибки
    }
    return err
}
```

## Примеры использования

### Базовый клиент

```go
client, err := httpclient.NewClient()
if err != nil {
    log.Fatal(err)
}

resp, err := client.Get("https://api.example.com/data")
```

### Клиент с полной конфигурацией

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(3, 100*time.Millisecond, 5*time.Second)),
    httpclient.WithCircuitBreaker(httpclient.NewSimpleCircuitBreaker()),
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMetrics(true),
)
```

### JSON API

```go
var user User
err := client.GetJSON(context.Background(), "https://api.example.com/user/123", &user)

newUser := User{Name: "John", Email: "john@example.com"}
var createdUser User
err := client.PostJSON(context.Background(), "https://api.example.com/users", newUser, &createdUser)
```

### Потоковая обработка

```go
req, _ := http.NewRequest("GET", "https://api.example.com/stream", nil)
stream, err := client.Stream(context.Background(), req)
if err == nil {
    defer stream.Close()
    scanner := bufio.NewScanner(stream.Body())
    for scanner.Scan() {
        fmt.Println(scanner.Text())
    }
}
```

## См. также

- [Быстрый старт](quick-start.md) - Основы использования
- [Конфигурация](configuration.md) - Настройка клиента
- [Примеры](examples.md) - Практические примеры
- [Тестирование](testing.md) - Mock объекты и утилиты