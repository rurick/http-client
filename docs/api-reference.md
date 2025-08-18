# API справочник

Полный справочник всех функций, типов и констант HTTP клиент пакета.

## Основные типы

### Client
```go
type Client struct {
    // содержит неэкспортируемые поля
}
```

Основной HTTP клиент с автоматическим сбором метрик и возможностями повторов.

#### Методы Client

##### HTTP методы
```go
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error)
func (c *Client) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
func (c *Client) Put(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
func (c *Client) Delete(ctx context.Context, url string) (*http.Response, error)
func (c *Client) Do(req *http.Request) (*http.Response, error)
```

##### Утилитарные методы
```go
func (c *Client) Close() error
func (c *Client) GetConfig() Config
```

**Примеры:**
```go
// GET запрос
resp, err := client.Get(ctx, "https://api.example.com/users")

// POST с JSON
resp, err := client.Post(ctx, url, "application/json", strings.NewReader(data))

// Произвольный запрос
req, _ := http.NewRequestWithContext(ctx, "PATCH", url, body)
resp, err := client.Do(req)

// Закрытие клиента
client.Close()
```

### Config
```go
type Config struct {
    Timeout         time.Duration    // Общий таймаут запроса
    PerTryTimeout   time.Duration    // Таймаут на попытку
    RetryConfig     RetryConfig      // Конфигурация повторов
    TracingEnabled  bool             // Включить OpenTelemetry tracing
    Transport       http.RoundTripper // Пользовательский транспорт
    CircuitBreakerEnable bool        // Включить Circuit Breaker
    CircuitBreaker       httpclient.CircuitBreaker // Экземпляр Circuit Breaker
}
```

Конфигурация поведения HTTP клиента.

**Пример:**
```go
config := httpclient.Config{
    Timeout:       30 * time.Second,
    PerTryTimeout: 5 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    5 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled: true,
}
```

### RetryConfig
### CircuitBreaker
```go
type CircuitBreaker interface {
    Execute(fn func() (*http.Response, error)) (*http.Response, error)
    State() CircuitBreakerState
    Reset()
}
```

```go
type CircuitBreakerState int

const (
    CircuitBreakerClosed CircuitBreakerState = iota
    CircuitBreakerOpen
    CircuitBreakerHalfOpen
)
```

```go
type CircuitBreakerConfig struct {
    FailStatusCodes  []int
    FailureThreshold int
    SuccessThreshold int
    Timeout          time.Duration
    OnStateChange    func(from, to CircuitBreakerState)
}
```

**Конструкторы:**
```go
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker
func NewCircuitBreakerWithConfig(CircuitBreakerConfig) *SimpleCircuitBreaker
```
```go
type RetryConfig struct {
    MaxAttempts int           // Максимальное количество попыток
    BaseDelay   time.Duration // Базовая задержка для backoff
    MaxDelay    time.Duration // Максимальная задержка
    Jitter      float64       // Фактор джиттера (0.0-1.0)
}
```

Конфигурация для поведения повторов и экспоненциального backoff.

**Пример:**
```go
retryConfig := httpclient.RetryConfig{
    MaxAttempts: 5,
    BaseDelay:   200 * time.Millisecond,
    MaxDelay:    10 * time.Second,
    Jitter:      0.3,
}
```

## Типы ошибок

### RetryableError
```go
type RetryableError struct {
    Err      error // Исходная ошибка
    Attempts int   // Количество попыток
}

func (e *RetryableError) Error() string
func (e *RetryableError) Unwrap() error
```

Ошибка, которая произошла после исчерпания всех попыток повтора.

**Пример обработки:**
```go
resp, err := client.Get(ctx, url)
if err != nil {
    if retryableErr, ok := err.(*httpclient.RetryableError); ok {
        log.Printf("Запрос не удался после %d попыток: %v", 
            retryableErr.Attempts, retryableErr.Err)
    }
}
```

### NonRetryableError
```go
type NonRetryableError struct {
    Err error // Исходная ошибка
}

func (e *NonRetryableError) Error() string
func (e *NonRetryableError) Unwrap() error
```

Ошибка, которую не следует повторять (например, 400 Bad Request).

**Пример обработки:**
```go
resp, err := client.Get(ctx, url)
if err != nil {
    if nonRetryableErr, ok := err.(*httpclient.NonRetryableError); ok {
        log.Printf("Неповторяемая ошибка: %v", nonRetryableErr.Err)
    }
}
```

## Функции-конструкторы

### New
```go
func New(config Config, meterName string) *Client
```

Создает новый HTTP клиент с указанной конфигурацией.

**Параметры:**
- `config`: Конфигурация клиента (передается по значению)
- `meterName`: Имя для OpenTelemetry метера (если пустой, используется "http-client")

**Возвращает:** Настроенный HTTP клиент

**Примеры:**
```go
// Базовый клиент
client := httpclient.New(httpclient.Config{}, "my-service")

// С конфигурацией
config := httpclient.Config{Timeout: 10 * time.Second}
client := httpclient.New(config, "api-client")

// Имя метера по умолчанию
client := httpclient.New(httpclient.Config{}, "")
```

## Функции backoff

### CalculateBackoffDelay
```go
func CalculateBackoffDelay(attempt int, baseDelay, maxDelay time.Duration, jitter float64) time.Duration
```

Вычисляет задержку экспоненциального backoff с джиттером.

**Параметры:**
- `attempt`: Номер текущей попытки (начиная с 1)
- `baseDelay`: Базовая задержка для экспоненциального backoff
- `maxDelay`: Максимально разрешенная задержка
- `jitter`: Фактор джиттера (0.0-1.0)

**Возвращает:** Вычисленная задержка

**Пример:**
```go
// Для 3-й попытки
delay := httpclient.CalculateBackoffDelay(3, 100*time.Millisecond, 5*time.Second, 0.2)
// Результат: ~400ms ± 20% джиттер
```

### CalculateExponentialBackoff
```go
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration
```

Вычисляет задержку экспоненциального backoff без джиттера.

**Параметры:**
- `attempt`: Номер текущей попытки (начиная с 1)
- `baseDelay`: Базовая задержка
- `maxDelay`: Максимально разрешенная задержка

**Возвращает:** Вычисленная задержка

**Пример:**
```go
delay := httpclient.CalculateExponentialBackoff(2, 100*time.Millisecond, 5*time.Second)
// Результат: 200ms (100ms * 2^(2-1))
```

### CalculateLinearBackoff
```go
func CalculateLinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration
```

Вычисляет задержку линейного backoff.

**Параметры:**
- `attempt`: Номер текущей попытки (начиная с 1)
- `baseDelay`: Базовый инкремент задержки
- `maxDelay`: Максимально разрешенная задержка

**Возвращает:** Вычисленная задержка

**Пример:**
```go
delay := httpclient.CalculateLinearBackoff(3, 100*time.Millisecond, 5*time.Second)
// Результат: 300ms (100ms * 3)
```

### CalculateConstantBackoff
```go
func CalculateConstantBackoff(baseDelay time.Duration) time.Duration
```

Возвращает постоянную задержку для всех попыток.

**Параметры:**
- `baseDelay`: Фиксированная задержка для возврата

**Возвращает:** Базовая задержка (без изменений)

**Пример:**
```go
delay := httpclient.CalculateConstantBackoff(500*time.Millisecond)
// Результат: всегда 500ms
```

## Функции обработки ошибок

### IsRetryableError
```go
func IsRetryableError(err error) bool
```

Определяет, должна ли ошибка вызвать повтор.

**Параметры:**
- `err`: Ошибка для оценки

**Возвращает:** True, если ошибка подлежит повтору

**Логика определения:**
- Сетевые ошибки: да
- Таймауты: да
- HTTP 5xx: да
- HTTP 429: да
- HTTP 4xx: нет
- Context cancelled: нет

**Пример:**
```go
if httpclient.IsRetryableError(err) {
    log.Println("Ошибка подлежит повтору")
}
```

### ClassifyError
```go
func ClassifyError(err error) string
```

Классифицирует ошибку для метрик и логирования.

**Параметры:**
- `err`: Ошибка для классификации

**Возвращает:** Строка классификации ошибки

**Возможные значения:**
- `"network_error"`: Сетевая ошибка
- `"timeout"`: Таймаут
- `"connection_error"`: Ошибка соединения  
- `"status_code"`: HTTP статус ошибка
- `"unknown"`: Неизвестная ошибка

**Пример:**
```go
classification := httpclient.ClassifyError(err)
log.Printf("Тип ошибки: %s", classification)
```

### NewRetryableError
```go
func NewRetryableError(err error, attempts int) *RetryableError
```

Создает новую ошибку, подлежащую повтору.

**Параметры:**
- `err`: Базовая ошибка
- `attempts`: Количество сделанных попыток

**Возвращает:** Экземпляр RetryableError

### NewNonRetryableError
```go
func NewNonRetryableError(err error) *NonRetryableError
```

Создает новую ошибку, не подлежащую повтору.

**Параметры:**
- `err`: Базовая ошибка

**Возвращает:** Экземпляр NonRetryableError

## Константы

### Значения по умолчанию
```go
const (
    DefaultTimeout       = 5 * time.Second
    DefaultPerTryTimeout = 2 * time.Second
    DefaultMaxAttempts   = 1
    DefaultBaseDelay     = 100 * time.Millisecond
    DefaultMaxDelay      = 5 * time.Second
    DefaultJitter        = 0.2
    DefaultMeterName     = "http-client"
)
```

### HTTP методы
```go
const (
    MethodGet     = "GET"
    MethodPost    = "POST"
    MethodPut     = "PUT"
    MethodDelete  = "DELETE"
    MethodPatch   = "PATCH"
    MethodHead    = "HEAD"
    MethodOptions = "OPTIONS"
)
```

### Имена метрик
```go
const (
    MetricRequestsTotal      = "http_client_requests_total"
    MetricRequestDuration    = "http_client_request_duration_seconds"
    MetricRetriesTotal       = "http_client_retries_total"
    MetricInflightRequests   = "http_client_inflight_requests"
    MetricRequestSize        = "http_client_request_size_bytes"
    MetricResponseSize       = "http_client_response_size_bytes"
)
```

## Переменные пакета

### Методы повторов
```go
var (
    // HTTP методы, которые всегда можно повторять
    IdempotentMethods = []string{
        "GET", "PUT", "DELETE", "HEAD", "OPTIONS",
    }
    
    // HTTP методы, которые требуют Idempotency-Key для повтора
    ConditionalRetryMethods = []string{
        "POST", "PATCH",
    }
    
    // HTTP статус коды, которые вызывают повторы
    RetryableStatusCodes = []int{
        429, 500, 502, 503, 504,
    }
)
```

## Внутренние типы (Advanced)

Эти типы доступны для продвинутого использования, но обычно не требуются.

### Metrics
```go
type Metrics struct {
    RequestsTotal    metric.Int64Counter
    RequestDuration  metric.Float64Histogram
    RetriesTotal     metric.Int64Counter
    InflightRequests metric.Int64UpDownCounter
    RequestSize      metric.Int64Histogram
    ResponseSize     metric.Int64Histogram
    // содержит неэкспортируемые поля
}
```

### RoundTripper
```go
type RoundTripper struct {
    // содержит неэкспортируемые поля
}

func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error)
```

### Tracer
```go
type Tracer struct {
    tracer trace.Tracer
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, trace.Span)
```

## Примеры комплексного использования

### Создание клиента для микросервиса
```go
func createMicroserviceClient(serviceName string) *httpclient.Client {
    config := httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    2 * time.Second,
            Jitter:      0.2,
        },
        TracingEnabled: true,
    }
    
    return httpclient.New(config, serviceName)
}
```

### Обработка всех типов ошибок
```go
func handleAllErrors(client *httpclient.Client, url string) {
    resp, err := client.Get(context.Background(), url)
    if err != nil {
        switch e := err.(type) {
        case *httpclient.RetryableError:
            log.Printf("Не удалось после %d попыток: %v", e.Attempts, e.Err)
        case *httpclient.NonRetryableError:
            log.Printf("Неповторяемая ошибка: %v", e.Err)
        default:
            log.Printf("Общая ошибка: %v", err)
        }
        return
    }
    defer resp.Body.Close()
    
    // Обработка успешного ответа
}
```

### Использование пользовательского backoff
```go
func customBackoffExample() {
    for attempt := 1; attempt <= 5; attempt++ {
        // Различные стратегии backoff
        expDelay := httpclient.CalculateExponentialBackoff(attempt, 100*time.Millisecond, 5*time.Second)
        linDelay := httpclient.CalculateLinearBackoff(attempt, 100*time.Millisecond, 5*time.Second)
        constDelay := httpclient.CalculateConstantBackoff(500*time.Millisecond)
        jitterDelay := httpclient.CalculateBackoffDelay(attempt, 100*time.Millisecond, 5*time.Second, 0.3)
        
        fmt.Printf("Попытка %d: exp=%v, lin=%v, const=%v, jitter=%v\n", 
            attempt, expDelay, linDelay, constDelay, jitterDelay)
    }
}
```