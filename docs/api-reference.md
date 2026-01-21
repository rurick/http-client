# API Reference

Complete reference of all functions, types, and constants of the HTTP client package.

## Main Types

### Client
```go
type Client struct {
    // contains unexported fields
}
```

Main HTTP client with automatic metrics collection and retry capabilities.

#### Client Methods

##### HTTP Methods
```go
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
func (c *Client) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error)
func (c *Client) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error)
func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
func (c *Client) Head(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
func (c *Client) Patch(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error)
func (c *Client) Do(req *http.Request) (*http.Response, error)
func (c *Client) PostForm(ctx context.Context, url string, data url.Values) (*http.Response, error)
```

##### Utility Methods
```go
func (c *Client) Close() error
func (c *Client) GetConfig() Config
```

**Examples:**
```go
// GET request
resp, err := client.Get(ctx, "https://api.example.com/users")

// GET with headers
resp, err := client.Get(ctx, url, WithHeaders(map[string]string{
    "Authorization": "Bearer token",
    "Accept": "application/json",
}))

// POST with JSON body via option
type User struct {
    Name string `json:"name"`
    Email string `json:"email"`
}
user := User{Name: "John", Email: "john@example.com"}
resp, err := client.Post(ctx, url, nil, WithJSONBody(user))

// POST with form data
formData := url.Values{}
formData.Set("username", "john")
formData.Set("password", "secret")
resp, err := client.Post(ctx, url, nil, WithFormBody(formData))

// PATCH with idempotency
resp, err := client.Patch(ctx, url, strings.NewReader(data), 
    WithContentType("application/json"),
    WithIdempotencyKey("unique-key-123"))

// Arbitrary request
req, _ := http.NewRequestWithContext(ctx, "PATCH", url, body)
resp, err := client.Do(req)

// Close client
client.Close()
```

### Config
```go
type Config struct {
    Timeout         time.Duration    // Overall request timeout
    PerTryTimeout   time.Duration    // Timeout per attempt
    RetryConfig     RetryConfig      // Retry configuration
    TracingEnabled  bool             // Enable OpenTelemetry tracing
    Transport       http.RoundTripper // Custom transport
    CircuitBreakerEnable bool        // Enable Circuit Breaker
    CircuitBreaker       httpclient.CircuitBreaker // Circuit Breaker instance
}
```

HTTP client behavior configuration.

**Example:**
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

**Constructors:**
```go
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker
func NewCircuitBreakerWithConfig(CircuitBreakerConfig) *SimpleCircuitBreaker
```
```go
type RetryConfig struct {
    MaxAttempts int           // Maximum number of attempts
    BaseDelay   time.Duration // Base delay for backoff
    MaxDelay    time.Duration // Maximum delay
    Jitter      float64       // Jitter factor (0.0-1.0)
}
```

Configuration for retry behavior and exponential backoff.

**Example:**
```go
retryConfig := httpclient.RetryConfig{
    MaxAttempts: 5,
    BaseDelay:   200 * time.Millisecond,
    MaxDelay:    10 * time.Second,
    Jitter:      0.3,
}
```

## Error Types

### RetryableError
```go
type RetryableError struct {
    Err      error // Original error
    Attempts int   // Number of attempts
}

func (e *RetryableError) Error() string
func (e *RetryableError) Unwrap() error
```

Error that occurred after all retry attempts were exhausted.

**Handling Example:**
```go
resp, err := client.Get(ctx, url)
if err != nil {
    if retryableErr, ok := err.(*httpclient.RetryableError); ok {
        log.Printf("Request failed after %d attempts: %v", 
            retryableErr.Attempts, retryableErr.Err)
    }
}
```

### NonRetryableError
```go
type NonRetryableError struct {
    Err error // Original error
}

func (e *NonRetryableError) Error() string
func (e *NonRetryableError) Unwrap() error
```

Error that should not be retried (e.g., 400 Bad Request).

**Handling Example:**
```go
resp, err := client.Get(ctx, url)
if err != nil {
    if nonRetryableErr, ok := err.(*httpclient.NonRetryableError); ok {
        log.Printf("Non-retryable error: %v", nonRetryableErr.Err)
    }
}
```

## Constructor Functions

### New
```go
func New(config Config, meterName string) *Client
```

Creates a new HTTP client with the specified configuration.

**Parameters:**
- `config`: Client configuration (passed by value)
- `meterName`: Name for OpenTelemetry meter (if empty, "http-client" is used)

**Returns:** Configured HTTP client

**Examples:**
```go
// Basic client
client := httpclient.New(httpclient.Config{}, "my-service")

// With configuration
config := httpclient.Config{Timeout: 10 * time.Second}
client := httpclient.New(config, "api-client")

// Default meter name
client := httpclient.New(httpclient.Config{}, "")
```

## Backoff Functions

### CalculateBackoffDelay
```go
func CalculateBackoffDelay(attempt int, baseDelay, maxDelay time.Duration, jitter float64) time.Duration
```

Calculates exponential backoff delay with jitter.

**Parameters:**
- `attempt`: Current attempt number (starting from 1)
- `baseDelay`: Base delay for exponential backoff
- `maxDelay`: Maximum allowed delay
- `jitter`: Jitter factor (0.0-1.0)

**Returns:** Calculated delay

**Example:**
```go
// For 3rd attempt
delay := httpclient.CalculateBackoffDelay(3, 100*time.Millisecond, 5*time.Second, 0.2)
// Result: ~400ms ± 20% jitter
```

### CalculateExponentialBackoff
```go
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration
```

Calculates exponential backoff delay without jitter.

**Parameters:**
- `attempt`: Current attempt number (starting from 1)
- `baseDelay`: Base delay
- `maxDelay`: Maximum allowed delay

**Returns:** Calculated delay

**Example:**
```go
delay := httpclient.CalculateExponentialBackoff(2, 100*time.Millisecond, 5*time.Second)
// Result: 200ms (100ms * 2^(2-1))
```

### CalculateLinearBackoff
```go
func CalculateLinearBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration
```

Calculates linear backoff delay.

**Parameters:**
- `attempt`: Current attempt number (starting from 1)
- `baseDelay`: Base delay increment
- `maxDelay`: Maximum allowed delay

**Returns:** Calculated delay

**Example:**
```go
delay := httpclient.CalculateLinearBackoff(3, 100*time.Millisecond, 5*time.Second)
// Result: 300ms (100ms * 3)
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

## RequestOption - функциональные опции

### RequestOption тип
```go
type RequestOption func(*http.Request)
```

Функциональная опция для настройки HTTP запросов. Позволяет модифицировать запросы перед их выполнением.

### Опции заголовков

#### WithHeader
```go
func WithHeader(key, value string) RequestOption
```
Устанавливает один заголовок в запросе.

**Пример:**
```go
resp, err := client.Get(ctx, url, WithHeader("Accept", "application/json"))
```

#### WithHeaders
```go
func WithHeaders(headers map[string]string) RequestOption
```
Устанавливает несколько заголовков в запросе.

**Пример:**
```go
headers := map[string]string{
    "Authorization": "Bearer token123",
    "Accept": "application/json",
    "User-Agent": "MyApp/1.0",
}
resp, err := client.Get(ctx, url, WithHeaders(headers))
```

#### WithContentType
```go
func WithContentType(contentType string) RequestOption
```
Устанавливает заголовок Content-Type.

**Пример:**
```go
resp, err := client.Post(ctx, url, body, WithContentType("application/xml"))
```

#### WithAuthorization
```go
func WithAuthorization(auth string) RequestOption
```
Устанавливает заголовок Authorization.

**Пример:**
```go
resp, err := client.Get(ctx, url, WithAuthorization("Bearer token123"))
```

#### WithBearerToken
```go
func WithBearerToken(token string) RequestOption
```
Устанавливает заголовок Authorization с Bearer токеном.

**Пример:**
```go
resp, err := client.Get(ctx, url, WithBearerToken("token123"))
```

#### WithIdempotencyKey
```go
func WithIdempotencyKey(key string) RequestOption
```
Устанавливает заголовок Idempotency-Key для безопасных повторов POST/PATCH запросов.

**Пример:**
```go
import "github.com/google/uuid"

idempotencyKey := uuid.New().String()
resp, err := client.Post(ctx, url, body, 
    WithContentType("application/json"),
    WithIdempotencyKey(idempotencyKey))
```

#### WithUserAgent
```go
func WithUserAgent(userAgent string) RequestOption
```
Устанавливает заголовок User-Agent.

**Пример:**
```go
resp, err := client.Get(ctx, url, WithUserAgent("MyService/2.1.0"))
```

#### WithAccept
```go
func WithAccept(accept string) RequestOption
```
Устанавливает заголовок Accept.

**Пример:**
```go
resp, err := client.Get(ctx, url, WithAccept("application/json"))
```

### Опции тела запроса

#### WithJSONBody
```go
func WithJSONBody(v interface{}) RequestOption
```
Устанавливает тело запроса как JSON представление объекта и устанавливает Content-Type в application/json.

**Пример:**
```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

user := CreateUserRequest{
    Name:  "John Doe",
    Email: "john@example.com",
    Age:   30,
}

// POST с JSON телом (body параметр nil)
resp, err := client.Post(ctx, url, nil, WithJSONBody(user))
```

#### WithFormBody
```go
func WithFormBody(values url.Values) RequestOption
```
Устанавливает тело запроса как URL-encoded form данные и устанавливает Content-Type в application/x-www-form-urlencoded.

**Пример:**
```go
formData := url.Values{}
formData.Set("username", "john")
formData.Set("password", "secret123")
formData.Set("remember_me", "true")

// POST с form данными
resp, err := client.Post(ctx, url, nil, WithFormBody(formData))
```

#### WithXMLBody
```go
func WithXMLBody(v interface{}) RequestOption
```
Устанавливает тело запроса как XML представление объекта и устанавливает Content-Type в application/xml.

**Пример:**
```go
type User struct {
    XMLName xml.Name `xml:"user"`
    Name    string   `xml:"name"`
    Email   string   `xml:"email"`
}

user := User{
    Name:  "John Doe",
    Email: "john@example.com",
}

resp, err := client.Post(ctx, url, nil, WithXMLBody(user))
```

#### WithTextBody
```go
func WithTextBody(text string) RequestOption
```
Устанавливает тело запроса как простой текст и устанавливает Content-Type в text/plain; charset=utf-8.

**Пример:**
```go
resp, err := client.Post(ctx, url, nil, WithTextBody("Hello, World!"))
```

#### WithRawBody
```go
func WithRawBody(body io.Reader) RequestOption
```
Устанавливает тело запроса из io.Reader без изменения Content-Type. Полезно для полного контроля над телом запроса.

**Пример:**
```go
// Бинарные данные
data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
resp, err := client.Post(ctx, url, nil, 
    WithRawBody(bytes.NewReader(data)),
    WithContentType("image/png"))

// Файл
file, err := os.Open("document.pdf")
if err != nil {
    return err
}
defer file.Close()

resp, err := client.Post(ctx, url, nil,
    WithRawBody(file),
    WithContentType("application/pdf"))
```

#### WithMultipartFormData
```go
func WithMultipartFormData(fields map[string]string, boundary string) RequestOption
```
Создает multipart/form-data тело для загрузки форм. Упрощенная версия для текстовых полей.

**Пример:**
```go
import "crypto/rand"
import "fmt"

// Генерация случайной границы
buf := make([]byte, 16)
rand.Read(buf)
boundary := fmt.Sprintf("----formdata-%x", buf)

fields := map[string]string{
    "title":       "My Document",
    "description": "Important file",
    "category":    "documents",
}

resp, err := client.Post(ctx, url, nil, 
    WithMultipartFormData(fields, boundary))
```

### Комбинирование опций

Опции можно комбинировать для создания сложных запросов:

```go
// Полный POST запрос с аутентификацией и идемпотентностью
type Order struct {
    ProductID int     `json:"product_id"`
    Quantity  int     `json:"quantity"`
    Price     float64 `json:"price"`
}

order := Order{
    ProductID: 12345,
    Quantity:  2,
    Price:     99.99,
}

resp, err := client.Post(ctx, "https://api.shop.com/orders", nil,
    WithJSONBody(order),
    WithBearerToken("your-auth-token"),
    WithIdempotencyKey(uuid.New().String()),
    WithHeader("X-Request-ID", requestID),
    WithUserAgent("ShopApp/1.0"))

// GET с кастомными заголовками
resp, err := client.Get(ctx, "https://api.example.com/data",
    WithHeaders(map[string]string{
        "Accept":        "application/json",
        "Accept-Language": "en-US,en;q=0.9",
    }),
    WithBearerToken(token),
    WithHeader("X-API-Version", "v2"))

// PUT для обновления с XML
resp, err := client.Put(ctx, "https://api.example.com/config", nil,
    WithXMLBody(configData),
    WithAuthorization("Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass"))),
    WithHeader("X-Update-Source", "admin-panel"))
```

### Обработка ошибок сериализации

При ошибке сериализации в WithJSONBody или WithXMLBody, информация об ошибке добавляется в специальные заголовки:

- `X-JSON-Marshal-Error`: Ошибка JSON маршалинга
- `X-XML-Marshal-Error`: Ошибка XML маршалинга

```go
// Проверка ошибок сериализации
resp, err := client.Post(ctx, url, nil, WithJSONBody(data))
if err == nil && resp != nil {
    if marshalErr := resp.Request.Header.Get("X-JSON-Marshal-Error"); marshalErr != "" {
        log.Printf("Ошибка JSON маршалинга: %s", marshalErr)
        // Обработать ошибку...
    }
}
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