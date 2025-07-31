# Система Middleware

Middleware система позволяет добавлять промежуточные обработчики для всех HTTP запросов и ответов.

## Встроенные Middleware

### Аутентификация

Автоматическое добавление заголовков аутентификации к каждому запросу.

```go
// Basic Auth
authMiddleware := httpclient.NewBasicAuthMiddleware("username", "password")

// Bearer Token
tokenMiddleware := httpclient.NewBearerTokenMiddleware("your-token-here")

// API Key
apiKeyMiddleware := httpclient.NewAPIKeyMiddleware("X-API-Key", "your-api-key")

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(authMiddleware),
)
```

### Логирование

Подробное логирование всех HTTP операций.

```go
logger := zap.NewDevelopment() // или ваш zap logger

logMiddleware := httpclient.NewLoggingMiddleware(logger)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(logMiddleware),
)
```

**Что логируется:**
- URL и HTTP метод запроса
- Размер тела запроса
- Время выполнения
- Статус код ответа
- Размер ответа
- Ошибки (если есть)

### Ограничение скорости (Rate Limiting)

Встроенный Rate Limiter использует алгоритм Token Bucket для контроля частоты запросов.

```go
// 10 запросов в секунду с burst до 20 запросов
rateLimitMiddleware := httpclient.NewRateLimitMiddleware(10, 20)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(rateLimitMiddleware),
)
```

#### Принцип работы Token Bucket

1. **Корзина токенов**: Имеет максимальную емкость (capacity)
2. **Пополнение**: Токены добавляются с постоянной скоростью (rate)
3. **Потребление**: Каждый запрос забирает 1 токен
4. **Блокировка**: Если токенов нет - запрос ждет или отклоняется

**Параметры:**
- `rate` - токенов в секунду (средняя скорость)
- `capacity` - размер корзины (burst трафик)

### Таймауты

Middleware для установки таймаутов на уровне запросов.

```go
timeoutMiddleware := httpclient.NewTimeoutMiddleware(5 * time.Second)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(timeoutMiddleware),
)
```

### Пользовательский User-Agent

Установка кастомного User-Agent для всех запросов.

```go
userAgentMiddleware := httpclient.NewUserAgentMiddleware("MyApp/1.0")

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(userAgentMiddleware),
)
```

## Комбинирование Middleware

### Добавление по одному

```go
client, err := httpclient.NewClient(
    httpclient.WithMiddleware(authMiddleware),
    httpclient.WithMiddleware(logMiddleware),
    httpclient.WithMiddleware(rateLimitMiddleware),
)
```

### Добавление всех сразу

```go
middlewares := []httpclient.Middleware{
    httpclient.NewBearerTokenMiddleware("token"),
    httpclient.NewLoggingMiddleware(logger),
    httpclient.NewRateLimitMiddleware(10, 20),
    httpclient.NewTimeoutMiddleware(30*time.Second),
}

client, err := httpclient.NewClient()
if err != nil {
    return err
}

// Добавляем все middleware сразу
client.AddMiddleware(middlewares...)
```

### Порядок выполнения

Middleware выполняются в порядке добавления:

1. **Запрос**: Первый добавленный → Последний добавленный → HTTP запрос
2. **Ответ**: HTTP ответ → Последний добавленный → Первый добавленный

```go
// Порядок выполнения:
client, err := httpclient.NewClient(
    httpclient.WithMiddleware(authMiddleware),    // 1-й в запросе, 3-й в ответе
    httpclient.WithMiddleware(logMiddleware),     // 2-й в запросе, 2-й в ответе  
    httpclient.WithMiddleware(timeoutMiddleware), // 3-й в запросе, 1-й в ответе
)
```

## Создание пользовательского Middleware

```go
// Пример middleware для добавления кастомных заголовков
type CustomHeaderMiddleware struct {
    headers map[string]string
}

func NewCustomHeaderMiddleware(headers map[string]string) *CustomHeaderMiddleware {
    return &CustomHeaderMiddleware{headers: headers}
}

func (m *CustomHeaderMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
    // Обработка запроса - добавляем заголовки
    for key, value := range m.headers {
        req.Header.Set(key, value)
    }
    
    // Выполняем следующий middleware или HTTP запрос
    resp, err := next(req)
    
    // Обработка ответа (если нужно)
    if resp != nil {
        // Можно добавить логику обработки ответа
        fmt.Printf("Получен ответ со статусом: %d\n", resp.StatusCode)
    }
    
    return resp, err
}

// Использование
customHeaders := map[string]string{
    "X-Custom-Header": "MyValue",
    "X-Client-Version": "1.0.0",
}

customMiddleware := NewCustomHeaderMiddleware(customHeaders)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(customMiddleware),
)
```

## Интерфейс Middleware

Все middleware должны реализовывать интерфейс:

```go
type Middleware interface {
    Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
}
```

### Параметры Process

- `req` - HTTP запрос для обработки
- `next` - функция для вызова следующего middleware или выполнения HTTP запроса
- Возвращает ответ и ошибку

## Примеры практического использования

### API клиент с полной настройкой

```go
logger, _ := zap.NewProduction()

client, err := httpclient.NewClient(
    // Аутентификация
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware(apiToken)),
    
    // Логирование всех операций
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    
    // Ограничение: 100 запросов в секунду
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(100, 150)),
    
    // Таймауты для медленных API
    httpclient.WithMiddleware(httpclient.NewTimeoutMiddleware(30*time.Second)),
    
    // Кастомный User-Agent
    httpclient.WithMiddleware(httpclient.NewUserAgentMiddleware("CityDrive-API-Client/1.0")),
)
```

### Микросервисный клиент

```go
client, err := httpclient.NewClient(
    // Логирование для debug
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    
    // Быстрые таймауты для внутренних сервисов
    httpclient.WithMiddleware(httpclient.NewTimeoutMiddleware(5*time.Second)),
    
    // Service-to-service токен
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware(serviceToken)),
)
```

## Лучшие практики

1. **Порядок имеет значение**: Аутентификация должна быть перед логированием
2. **Производительность**: Middleware выполняются для каждого запроса
3. **Обработка ошибок**: Всегда проверяйте ошибки в middleware
4. **Таймауты**: Используйте разумные таймауты чтобы не блокировать систему

## См. также

- [Конфигурация](configuration.md) - Другие опции настройки клиента
- [Метрики](metrics.md) - Мониторинг работы middleware
- [Примеры](examples.md) - Практические примеры использования