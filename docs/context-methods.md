# Контекстные HTTP методы

HTTP клиент предоставляет два набора методов для выполнения HTTP запросов:

- **Обычные методы** (`Get`, `Post`, `Put`, `Delete`, `Head`) - совместимы со стандартным интерфейсом `http.Client`
- **Контекстные методы** (`GetCtx`, `PostCtx`, `PutCtx`, `DeleteCtx`, `HeadCtx`) - **рекомендуемые** с поддержкой контекста

## Рекомендация по использованию

⭐ **Всегда используйте контекстные методы** для новой разработки. Они предоставляют лучший контроль над запросами и соответствуют современным Go практикам.

## Основные интерфейсы

### HTTPClient (обычные методы)
Базовый интерфейс, совместимый со стандартным `http.Client`:

```go
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
    Get(url string) (*http.Response, error)
    Head(url string) (*http.Response, error)
    Post(url, contentType string, body io.Reader) (*http.Response, error)
    PostForm(url string, data map[string][]string) (*http.Response, error)
}
```

### CtxHTTPClient (контекстные методы) ⭐
Расширенный интерфейс с поддержкой контекста:

```go
type CtxHTTPClient interface {
    DoCtx(ctx context.Context, req *http.Request) (*http.Response, error)
    GetCtx(ctx context.Context, url string) (*http.Response, error)
    PostCtx(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
    PostFormCtx(ctx context.Context, url string, data map[string][]string) (*http.Response, error)
    HeadCtx(ctx context.Context, url string) (*http.Response, error)
}
```

## Преимущества контекстных методов

### 1. Управление таймаутами
```go
// Установка таймаута для конкретного запроса
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.GetCtx(ctx, "https://api.example.com/data")
```

### 2. Отмена запросов
```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(2 * time.Second)
    cancel() // Отменяем запрос через 2 секунды
}()

resp, err := client.GetCtx(ctx, "https://slow-api.example.com")
if err != nil && ctx.Err() == context.Canceled {
    fmt.Println("Запрос был отменен")
}
```

### 3. Передача метаданных
```go
ctx := context.WithValue(context.Background(), "user-id", "12345")
ctx = context.WithValue(ctx, "request-id", "req-67890")

resp, err := client.PostCtx(ctx, url, "application/json", body)
```

### 4. Распределенная трассировка
```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("my-service")
ctx, span := tracer.Start(context.Background(), "api-call")
defer span.End()

// Трейсинг информация автоматически передается с запросом  
resp, err := client.GetCtx(ctx, "https://api.example.com/users")
```

## Примеры использования

### Базовый GET запрос
```go
// Контекстный метод (рекомендуется)
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.GetCtx(ctx, "https://jsonplaceholder.typicode.com/posts/1")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

// Обычный метод (для совместимости)
resp, err := client.Get("https://jsonplaceholder.typicode.com/posts/1")
```

### POST запрос с JSON
```go
data := map[string]any{
    "title":  "Новый пост",
    "body":   "Содержимое поста", 
    "userId": 1,
}

jsonData, _ := json.Marshal(data)
body := bytes.NewReader(jsonData)

// Контекстный метод
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

resp, err := client.PostCtx(ctx, "https://jsonplaceholder.typicode.com/posts", 
    "application/json", body)
```

### Отправка формы
```go
formData := map[string][]string{
    "name":  {"Иван Иванов"},
    "email": {"ivan@example.com"},
    "age":   {"30"},
}

ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

resp, err := client.PostFormCtx(ctx, "https://example.com/submit", formData)
```

## Обработка ошибок контекста

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.GetCtx(ctx, url)
if err != nil {
    switch {
    case ctx.Err() == context.DeadlineExceeded:
        log.Println("Запрос превысил таймаут")
    case ctx.Err() == context.Canceled:
        log.Println("Запрос был отменен")
    default:
        log.Printf("Другая ошибка: %v", err)
    }
    return
}
```

## Интеграция с middleware

Все middleware полностью совместимы с контекстными методами:

```go
logger, _ := zap.NewProduction()

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware("token123")),
    httpclient.WithRetryMax(3),
)

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Middleware применяется автоматически
resp, err := client.GetCtx(ctx, "https://api.example.com/protected")
```

## Совместимость

### Использование как стандартный http.Client
```go
// Для библиотек, ожидающих стандартный интерфейс
var httpClient httpclient.HTTPClient = client

// Можно использовать в существующем коде
resp, err := httpClient.Get("https://example.com")
```

### Постепенный переход
```go
// Можно смешивать обычные и контекстные методы
resp1, err1 := client.Get(url1)        // Старый код
resp2, err2 := client.GetCtx(ctx, url2) // Новый код
```

## Рекомендации по производительности

### 1. Переиспользование контекста
```go
// Создайте контекст один раз для связанных запросов
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
defer cancel()

user, err := client.GetCtx(ctx, "/api/user/123")
orders, err := client.GetCtx(ctx, "/api/user/123/orders")  
profile, err := client.GetCtx(ctx, "/api/user/123/profile")
```

### 2. Правильная настройка таймаутов
```go
// Для быстрых API
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

// Для медленных операций  
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

// Для критически важных запросов
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
```

### 3. Использование контекста приложения
```go
// В веб-приложениях используйте контекст запроса
func handleAPI(w http.ResponseWriter, r *http.Request) {
    // Используем контекст HTTP запроса
    resp, err := client.GetCtx(r.Context(), "https://api.example.com/data")
    
    // Запрос автоматически отменится если клиент отключится
}
```

## Миграция на контекстные методы

### Простая замена
```go
// Было
resp, err := client.Get(url)

// Стало  
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
resp, err := client.GetCtx(ctx, url)
```

### Использование хелпера
```go
// Создайте хелпер для упрощения миграции
func withTimeout(d time.Duration) context.Context {
    ctx, _ := context.WithTimeout(context.Background(), d)
    return ctx
}

// Использование
resp, err := client.GetCtx(withTimeout(30*time.Second), url)
```

## Лучшие практики

1. **Всегда используйте контекстные методы** для новой разработки
2. **Устанавливайте разумные таймауты** для всех запросов
3. **Не забывайте вызывать cancel()** чтобы освободить ресурсы
4. **Проверяйте ошибки контекста** для правильной обработки таймаутов и отмен
5. **Переиспользуйте контекст** для связанных запросов
6. **Используйте контекст приложения** в веб-приложениях

Контекстные методы обеспечивают лучший контроль, производительность и соответствуют современным стандартам разработки на Go.