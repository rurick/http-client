# Примеры использования

Практические примеры использования HTTP клиент пакета для различных сценариев.

## RequestOption - новые возможности

Начиная с новой версии, все HTTP методы поддерживают функциональные опции RequestOption.

### Краткий обзор RequestOption
```go
// Основные опции заголовков
resp, err := client.Get(ctx, url,
    httpclient.WithBearerToken("token123"),
    httpclient.WithAccept("application/json"),
    httpclient.WithHeader("X-Request-ID", "unique-id"),
)

// Опции тела запроса
resp, err := client.Post(ctx, url, nil,
    httpclient.WithJSONBody(data),                    // Автоматическая JSON сериализация
    httpclient.WithIdempotencyKey("operation-123"),  // Безопасные повторы
    httpclient.WithBearerToken("token123"),
)

// Комбинирование опций
resp, err := client.Put(ctx, url, nil,
    httpclient.WithFormBody(formData),
    httpclient.WithHeaders(map[string]string{
        "X-API-Version": "v2",
        "X-Client-Type": "web",
    }),
    httpclient.WithUserAgent("MyApp/1.0"),
)
```

### Полные примеры с RequestOption

#### JSON запросы
```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// POST с автоматической JSON сериализацией
user := User{Name: "John", Email: "john@example.com"}
resp, err := client.Post(ctx, "/api/users", nil,
    httpclient.WithJSONBody(user),
    httpclient.WithBearerToken("your-token"),
    httpclient.WithIdempotencyKey("create-user-123"),
)

// PATCH для обновления
updates := map[string]interface{}{"status": "active"}
resp, err := client.Patch(ctx, "/api/users/123", nil,
    httpclient.WithJSONBody(updates),
    httpclient.WithIdempotencyKey("update-user-123"),
)
```

#### Form данные
```go
// POST с form данными
formData := url.Values{}
formData.Set("username", "john")
formData.Set("password", "secret")
formData.Set("remember_me", "true")

resp, err := client.Post(ctx, "/login", nil,
    httpclient.WithFormBody(formData),
    httpclient.WithHeader("X-CSRF-Token", csrfToken),
)
```

#### XML и текст
```go
type Config struct {
    XMLName xml.Name `xml:"config"`
    Setting string   `xml:"setting"`
    Value   int      `xml:"value"`
}

// XML запрос
config := Config{Setting: "timeout", Value: 30}
resp, err := client.Put(ctx, "/api/config", nil,
    httpclient.WithXMLBody(config),
    httpclient.WithAuthorization("Basic "+basicAuth),
)

// Простой текст
resp, err := client.Post(ctx, "/api/webhook", nil,
    httpclient.WithTextBody("webhook payload data"),
    httpclient.WithContentType("text/plain"),
)
```

#### Полный контроль над телом
```go
// Бинарные данные
imageData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
resp, err := client.Post(ctx, "/api/upload", nil,
    httpclient.WithRawBody(bytes.NewReader(imageData)),
    httpclient.WithContentType("image/png"),
    httpclient.WithHeader("X-Upload-Type", "avatar"),
)

// Файл
file, _ := os.Open("document.pdf")
defer file.Close()
resp, err := client.Post(ctx, "/api/documents", nil,
    httpclient.WithRawBody(file),
    httpclient.WithContentType("application/pdf"),
)
```

## Базовые примеры

### Простой GET запрос
```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    client := httpclient.New(httpclient.Config{}, "example-service")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), "https://jsonplaceholder.typicode.com/posts/1")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response: %s\n", body)
}
```

### POST запрос с JSON (новый API с RequestOption)
```go
package main

import (
    "context"
    "fmt"
    "log"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type Post struct {
    Title  string `json:"title"`
    Body   string `json:"body"`
    UserID int    `json:"userId"`
}

func main() {
    client := httpclient.New(httpclient.Config{}, "json-example")
    defer client.Close()
    
    post := Post{
        Title:  "Тестовый пост",
        Body:   "Содержимое поста",
        UserID: 1,
    }
    
    // Новый подход с автоматической JSON сериализацией
    resp, err := client.Post(
        context.Background(),
        "https://jsonplaceholder.typicode.com/posts",
        nil, // body = nil, используем WithJSONBody
        httpclient.WithJSONBody(post),
        httpclient.WithHeader("X-Request-ID", "example-123"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    fmt.Printf("Status: %d\n", resp.StatusCode)
}
```

## Примеры с retry

### Отказоустойчивый клиент
```go
package main

import (
    "context"
    "log"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    config := httpclient.Config{
        Timeout:       30 * time.Second,
        PerTryTimeout: 5 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 5,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Jitter:      0.3,
        },
        TracingEnabled: true,
    }
    
    client := httpclient.New(config, "resilient-client")
    defer client.Close()
    
    // Этот запрос будет повторяться при ошибках
    resp, err := client.Get(context.Background(), "https://httpbin.org/status/500")
    if err != nil {
        if retryableErr, ok := err.(*httpclient.RetryableError); ok {
            log.Printf("Запрос не удался после %d попыток: %v", 
                retryableErr.Attempts, retryableErr.Err)
        } else {
            log.Printf("Неповторяемая ошибка: %v", err)
        }
        return
    }
    defer resp.Body.Close()
    
    log.Printf("Успешный ответ: %d", resp.StatusCode)
}
```

### Идемпотентные операции
```go
package main

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func generateIdempotencyKey(operation, userID string) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s:%s:%d", operation, userID, time.Now().Unix()/300)))
    return hex.EncodeToString(h.Sum(nil))[:16]
}

func main() {
    config := httpclient.Config{
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   200 * time.Millisecond,
            MaxDelay:    2 * time.Second,
        },
    }
    
    client := httpclient.New(config, "idempotent-example")
    defer client.Close()
    
    paymentData := `{"amount": 100, "currency": "USD", "user_id": "123"}`
    
    req, err := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://httpbin.org/post",
        bytes.NewReader([]byte(paymentData)),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Idempotency-Key позволяет безопасно повторять POST
    req.Header.Set("Idempotency-Key", generateIdempotencyKey("payment", "123"))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    fmt.Printf("Платеж выполнен: %d\n", resp.StatusCode)
}
```

## Микросервисы

### Клиент для User Service
```go
package userservice

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserService struct {
    client  *httpclient.Client
    baseURL string
}

func NewUserService(baseURL string) *UserService {
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
    
    return &UserService{
        client:  httpclient.New(config, "user-service"),
        baseURL: baseURL,
    }
}

func (us *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    url := fmt.Sprintf("%s/users/%d", us.baseURL, userID)
    
    resp, err := us.client.Get(ctx, url)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 404 {
        return nil, fmt.Errorf("пользователь не найден")
    }
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("неожиданный статус: %d", resp.StatusCode)
    }
    
    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
    }
    
    return &user, nil
}

func (us *UserService) CreateUser(ctx context.Context, user User) (*User, error) {
    url := fmt.Sprintf("%s/users", us.baseURL)
    
    // Новый подход с WithJSONBody - без ручной сериализации
    resp, err := us.client.Post(ctx, url, nil,
        httpclient.WithJSONBody(user),
        httpclient.WithHeader("X-Service", "user-service"),
    )
    if err != nil {
        return nil, fmt.Errorf("ошибка создания пользователя: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 201 {
        return nil, fmt.Errorf("ошибка создания, статус: %d", resp.StatusCode)
    }
    
    var createdUser User
    if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
        return nil, fmt.Errorf("ошибка декодирования созданного пользователя: %w", err)
    }
    
    return &createdUser, nil
}

func (us *UserService) Close() error {
    return us.client.Close()
}
```

### Использование User Service
```go
package main

import (
    "context"
    "log"
    "userservice" // ваш пакет выше
)

func main() {
    service := userservice.NewUserService("https://api.example.com")
    defer service.Close()
    
    // Получение пользователя
    user, err := service.GetUser(context.Background(), 1)
    if err != nil {
        log.Printf("Ошибка получения пользователя: %v", err)
    } else {
        log.Printf("Пользователь: %+v", user)
    }
    
    // Создание пользователя
    newUser := userservice.User{
        Name:  "Иван Иванов",
        Email: "ivan@example.com",
    }
    
    created, err := service.CreateUser(context.Background(), newUser)
    if err != nil {
        log.Printf("Ошибка создания пользователя: %v", err)
    } else {
        log.Printf("Создан пользователь: %+v", created)
    }
}
```

## Внешние API

### Клиент для погодного API
```go
package weather

import (
    "context"
    "encoding/json"
    "fmt"
    "net/url"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type WeatherData struct {
    Location    string  `json:"location"`
    Temperature float64 `json:"temperature"`
    Humidity    int     `json:"humidity"`
    Description string  `json:"description"`
}

type WeatherClient struct {
    client *httpclient.Client
    apiKey string
    baseURL string
}

func NewWeatherClient(apiKey string) *WeatherClient {
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
    }
    
    return &WeatherClient{
        client:  httpclient.New(config, "weather-api"),
        apiKey:  apiKey,
        baseURL: "https://api.openweathermap.org/data/2.5",
    }
}

func (wc *WeatherClient) GetWeather(ctx context.Context, city string) (*WeatherData, error) {
    params := url.Values{}
    params.Add("q", city)
    params.Add("appid", wc.apiKey)
    params.Add("units", "metric")
    
    requestURL := fmt.Sprintf("%s/weather?%s", wc.baseURL, params.Encode())
    
    resp, err := wc.client.Get(ctx, requestURL)
    if err != nil {
        return nil, fmt.Errorf("ошибка запроса погоды: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 401 {
        return nil, fmt.Errorf("неверный API ключ")
    }
    
    if resp.StatusCode == 404 {
        return nil, fmt.Errorf("город не найден: %s", city)
    }
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("API ошибка: %d", resp.StatusCode)
    }
    
    var response struct {
        Name string `json:"name"`
        Main struct {
            Temp     float64 `json:"temp"`
            Humidity int     `json:"humidity"`
        } `json:"main"`
        Weather []struct {
            Description string `json:"description"`
        } `json:"weather"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
    }
    
    weather := &WeatherData{
        Location:    response.Name,
        Temperature: response.Main.Temp,
        Humidity:    response.Main.Humidity,
    }
    
    if len(response.Weather) > 0 {
        weather.Description = response.Weather[0].Description
    }
    
    return weather, nil
}

func (wc *WeatherClient) Close() error {
    return wc.client.Close()
}
```

## Обработка файлов

### Загрузка файлов
```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func uploadFile(client *httpclient.Client, filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return fmt.Errorf("ошибка открытия файла: %w", err)
    }
    defer file.Close()
    
    var buffer bytes.Buffer
    writer := multipart.NewWriter(&buffer)
    
    part, err := writer.CreateFormFile("file", filename)
    if err != nil {
        return fmt.Errorf("ошибка создания form field: %w", err)
    }
    
    if _, err := io.Copy(part, file); err != nil {
        return fmt.Errorf("ошибка копирования файла: %w", err)
    }
    
    writer.Close()
    
    req, err := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://httpbin.org/post",
        &buffer,
    )
    if err != nil {
        return fmt.Errorf("ошибка создания запроса: %w", err)
    }
    
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("ошибка загрузки файла: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("ошибка загрузки, статус: %d", resp.StatusCode)
    }
    
    fmt.Printf("Файл успешно загружен: %s\n", filename)
    return nil
}

func main() {
    config := httpclient.Config{
        Timeout:       300 * time.Second, // Длинный таймаут для файлов
        PerTryTimeout: 60 * time.Second,  // Таймаут на попытку
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   1 * time.Second,
            MaxDelay:    10 * time.Second,
        },
    }
    
    client := httpclient.New(config, "file-upload")
    defer client.Close()
    
    if err := uploadFile(client, "example.txt"); err != nil {
        log.Fatal(err)
    }
}
```

### Скачивание файлов
```go
package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func downloadFile(client *httpclient.Client, url, filename string) error {
    resp, err := client.Get(context.Background(), url)
    if err != nil {
        return fmt.Errorf("ошибка запроса файла: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("ошибка скачивания, статус: %d", resp.StatusCode)
    }
    
    file, err := os.Create(filename)
    if err != nil {
        return fmt.Errorf("ошибка создания файла: %w", err)
    }
    defer file.Close()
    
    _, err = io.Copy(file, resp.Body)
    if err != nil {
        return fmt.Errorf("ошибка записи файла: %w", err)
    }
    
    fmt.Printf("Файл скачан: %s\n", filename)
    return nil
}

func main() {
    config := httpclient.Config{
        Timeout:       300 * time.Second,
        PerTryTimeout: 60 * time.Second,
    }
    
    client := httpclient.New(config, "file-download")
    defer client.Close()
    
    if err := downloadFile(client, "https://httpbin.org/json", "data.json"); err != nil {
        log.Fatal(err)
    }
}
```

## Batch операции

### Параллельные запросы
```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type Result struct {
    URL    string
    Status int
    Error  error
}

func batchRequests(client *httpclient.Client, urls []string, concurrency int) []Result {
    results := make([]Result, len(urls))
    semaphore := make(chan struct{}, concurrency)
    
    var wg sync.WaitGroup
    for i, url := range urls {
        wg.Add(1)
        go func(index int, u string) {
            defer wg.Done()
            
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            resp, err := client.Get(context.Background(), u)
            if err != nil {
                results[index] = Result{URL: u, Error: err}
                return
            }
            defer resp.Body.Close()
            
            results[index] = Result{URL: u, Status: resp.StatusCode}
        }(i, url)
    }
    
    wg.Wait()
    return results
}

func main() {
    config := httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    1 * time.Second,
        },
    }
    
    client := httpclient.New(config, "batch-client")
    defer client.Close()
    
    urls := []string{
        "https://httpbin.org/status/200",
        "https://httpbin.org/status/404",
        "https://httpbin.org/status/500",
        "https://httpbin.org/delay/2",
    }
    
    results := batchRequests(client, urls, 3) // Максимум 3 одновременных запроса
    
    for _, result := range results {
        if result.Error != nil {
            fmt.Printf("❌ %s: %v\n", result.URL, result.Error)
        } else {
            fmt.Printf("✅ %s: %d\n", result.URL, result.Status)
        }
    }
}
```

## Circuit Breaker

### Встроенный Circuit Breaker
```go
package main

import (
    "context"
    "fmt"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    // Включаем встроенный CB по умолчанию
    client := httpclient.New(httpclient.Config{
        CircuitBreakerEnable: true,
    }, "cb-example")
    defer client.Close()

    for i := 0; i < 5; i++ {
        resp, err := client.Get(context.Background(), "https://httpbin.org/status/500")
        if err != nil {
            fmt.Printf("%d) err: %v\n", i+1, err)
        } else {
            fmt.Printf("%d) status: %d\n", i+1, resp.StatusCode)
            _ = resp.Body.Close()
        }
        time.Sleep(200 * time.Millisecond)
    }
}
```

### Кастомные пороги и обработчик состояний
```go
package main

import (
    "context"
    "fmt"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
        FailureThreshold: 2,
        SuccessThreshold: 1,
        Timeout:          2 * time.Second,
        OnStateChange: func(from, to httpclient.CircuitBreakerState) { fmt.Printf("state: %s -> %s\n", from, to) },
    })

    client := httpclient.New(httpclient.Config{
        CircuitBreakerEnable: true,
        CircuitBreaker:       cb,
    }, "cb-custom")
    defer client.Close()

    for i := 0; i < 6; i++ {
        resp, err := client.Get(context.Background(), "https://httpbin.org/status/500")
        if err != nil {
            fmt.Printf("%d) err: %v\n", i+1, err)
        } else {
            fmt.Printf("%d) status: %d\n", i+1, resp.StatusCode)
            _ = resp.Body.Close()
        }
        time.Sleep(300 * time.Millisecond)
    }
}
```

## Webhooks

### Обработка webhook событий
```go
package main

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type WebhookEvent struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}

type WebhookSender struct {
    client *httpclient.Client
    secret string
}

func NewWebhookSender(secret string) *WebhookSender {
    config := httpclient.Config{
        Timeout: 30 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 5,
            BaseDelay:   200 * time.Millisecond,
            MaxDelay:    30 * time.Second,
            Jitter:      0.2,
        },
    }
    
    return &WebhookSender{
        client: httpclient.New(config, "webhook-sender"),
        secret: secret,
    }
}

func (ws *WebhookSender) generateSignature(payload []byte) string {
    h := hmac.New(sha256.New, []byte(ws.secret))
    h.Write(payload)
    return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func (ws *WebhookSender) SendWebhook(ctx context.Context, url string, event WebhookEvent) error {
    payload, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("ошибка сериализации события: %w", err)
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("ошибка создания запроса: %w", err)
    }
    
    // Webhook headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "MyApp-Webhook/1.0")
    req.Header.Set("X-Webhook-Signature", ws.generateSignature(payload))
    req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
    
    // Idempotency для webhook
    req.Header.Set("Idempotency-Key", event.ID)
    
    resp, err := ws.client.Do(req)
    if err != nil {
        return fmt.Errorf("ошибка отправки webhook: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("webhook не доставлен, статус: %d", resp.StatusCode)
    }
    
    fmt.Printf("Webhook доставлен: %s -> %s\n", event.Type, url)
    return nil
}

func (ws *WebhookSender) Close() error {
    return ws.client.Close()
}

func main() {
    sender := NewWebhookSender("my-webhook-secret")
    defer sender.Close()
    
    event := WebhookEvent{
        ID:        "evt_123456",
        Type:      "user.created",
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "user_id": 12345,
            "email":   "user@example.com",
        },
    }
    
    urls := []string{
        "https://httpbin.org/post",
        "https://webhook.site/unique-id", // замените на реальный
    }
    
    for _, url := range urls {
        if err := sender.SendWebhook(context.Background(), url, event); err != nil {
            fmt.Printf("Ошибка отправки webhook на %s: %v\n", url, err)
        }
    }
}
```

Эти примеры показывают практическое использование HTTP клиент пакета в различных реальных сценариях - от простых запросов до сложных паттернов микросервисной архитектуры.