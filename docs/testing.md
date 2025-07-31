# Тестирование

Комплексные утилиты для тестирования кода, использующего HTTP клиент.

## Mock объекты

### MockHTTPClient

Основной mock объект для имитации HTTP клиента в тестах.

```go
package main

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
    httpmock "gitlab.citydrive.tech/back-end/go/pkg/http-client/mock"
)

func TestUserService(t *testing.T) {
    // Создание mock клиента
    mockClient := httpmock.NewMockHTTPClient()
    
    // Настройка ожидаемых вызовов
    mockClient.On("GetJSON", 
        mock.Anything,                    // context
        "https://api.example.com/user/123", // URL
        mock.AnythingOfType("*main.User"), // результат
    ).Return(nil).Run(func(args mock.Arguments) {
        // Заполняем результат
        user := args.Get(2).(*User)
        user.ID = 123
        user.Name = "Test User"
        user.Email = "test@example.com"
    })
    
    // Создание сервиса с mock клиентом
    service := &UserService{client: mockClient}
    
    // Тестирование
    user, err := service.GetUser(context.Background(), "123")
    
    // Проверки
    assert.NoError(t, err)
    assert.Equal(t, 123, user.ID)
    assert.Equal(t, "Test User", user.Name)
    assert.Equal(t, "test@example.com", user.Email)
    
    // Проверка что все ожидаемые методы были вызваны
    mockClient.AssertExpectations(t)
}
```

### Настройка различных сценариев

#### Успешный ответ

```go
func TestSuccessfulRequest(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    // Настройка успешного ответа
    mockClient.On("GetJSON", mock.Anything, "https://api.example.com/data", mock.Anything).
        Return(nil).
        Run(func(args mock.Arguments) {
            result := args.Get(2).(*APIResponse)
            result.Status = "success"
            result.Data = "test data"
        })
    
    // Тест...
}
```

#### Ошибка сети

```go
func TestNetworkError(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    // Настройка сетевой ошибки
    mockClient.On("GetJSON", mock.Anything, mock.Anything, mock.Anything).
        Return(errors.New("network error"))
    
    service := &APIService{client: mockClient}
    
    _, err := service.FetchData(context.Background())
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "network error")
    mockClient.AssertExpectations(t)
}
```

#### HTTP ошибки

```go
func TestHTTPError(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    // Создаем HTTP ответ с ошибкой
    resp := &http.Response{
        StatusCode: 404,
        Status:     "404 Not Found",
        Body:       io.NopCloser(strings.NewReader(`{"error": "not found"}`)),
    }
    
    mockClient.On("Get", "https://api.example.com/nonexistent").
        Return(resp, nil)
    
    service := &APIService{client: mockClient}
    
    response, err := service.GetResource(context.Background(), "nonexistent")
    
    assert.NoError(t, err) // Нет сетевой ошибки
    assert.Equal(t, 404, response.StatusCode)
    mockClient.AssertExpectations(t)
}
```

## Тестовые помощники

### TestServer

Создание тестового HTTP сервера для интеграционных тестов.

```go
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/stretchr/testify/assert"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func TestRealHTTPClient(t *testing.T) {
    // Создание тестового сервера
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/users/123":
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(200)
            w.Write([]byte(`{"id": 123, "name": "Test User"}`))
        case "/error":
            w.WriteHeader(500)
            w.Write([]byte(`{"error": "internal server error"}`))
        default:
            w.WriteHeader(404)
        }
    }))
    defer server.Close()
    
    // Создание реального клиента
    client, err := httpclient.NewClient(
        httpclient.WithTimeout(5*time.Second),
        httpclient.WithRetryMax(0), // Отключить повторы в тестах
    )
    assert.NoError(t, err)
    
    // Тест успешного запроса
    var user User
    err = client.GetJSON(context.Background(), server.URL+"/users/123", &user)
    
    assert.NoError(t, err)
    assert.Equal(t, 123, user.ID)
    assert.Equal(t, "Test User", user.Name)
    
    // Тест ошибки
    resp, err := client.Get(server.URL + "/error")
    assert.NoError(t, err)
    assert.Equal(t, 500, resp.StatusCode)
    resp.Body.Close()
}
```

### Фабрики тестовых данных

```go
// Фабрика для создания тестовых пользователей
func CreateTestUser(id int, name string) *User {
    return &User{
        ID:    id,
        Name:  name,
        Email: fmt.Sprintf("%s@test.com", strings.ToLower(name)),
    }
}

// Фабрика для создания тестового API ответа
func CreateTestAPIResponse(status string, data interface{}) *APIResponse {
    return &APIResponse{
        Status: status,
        Data:   data,
    }
}

func TestWithFactories(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    expectedUser := CreateTestUser(456, "Factory User")
    
    mockClient.On("GetJSON", mock.Anything, mock.Anything, mock.Anything).
        Return(nil).
        Run(func(args mock.Arguments) {
            user := args.Get(2).(*User)
            *user = *expectedUser
        })
    
    service := &UserService{client: mockClient}
    user, err := service.GetUser(context.Background(), "456")
    
    assert.NoError(t, err)
    assert.Equal(t, expectedUser.ID, user.ID)
    assert.Equal(t, expectedUser.Name, user.Name)
}
```

## Тестирование разных сценариев

### Тестирование retry логики

```go
func TestRetryBehavior(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    // Первые два вызова завершаются ошибкой
    mockClient.On("Get", "https://api.example.com/data").
        Return(nil, errors.New("temporary error")).
        Times(2)
    
    // Третий вызов успешен
    resp := &http.Response{
        StatusCode: 200,
        Status:     "200 OK",
        Body:       io.NopCloser(strings.NewReader(`{"status": "ok"}`)),
    }
    mockClient.On("Get", "https://api.example.com/data").
        Return(resp, nil).
        Once()
    
    // Создание клиента с retry
    client, _ := httpclient.NewClient(
        httpclient.WithRetryMax(3),
        httpclient.WithRetryStrategy(httpclient.NewFixedDelayStrategy(3, 100*time.Millisecond)),
    )
    
    // Подменяем внутренний клиент на mock (для демонстрации)
    // В реальности нужно будет использовать dependency injection
    
    service := &APIService{client: client}
    
    // Тест должен завершиться успешно после 3 попыток
    data, err := service.FetchDataWithRetry(context.Background())
    
    assert.NoError(t, err)
    assert.NotNil(t, data)
    
    // Проверяем что было сделано ровно 3 вызова
    mockClient.AssertExpectations(t)
}
```

### Тестирование Circuit Breaker

```go
func TestCircuitBreakerBehavior(t *testing.T) {
    mockClient := httpmock.NewMockHTTPClient()
    
    // Настраиваем 5 неудачных запросов (для открытия circuit breaker)
    mockClient.On("Get", mock.Anything).
        Return(nil, errors.New("service unavailable")).
        Times(5)
    
    client, _ := httpclient.NewClient(
        httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(3, 1*time.Second, 1)),
    )
    
    service := &APIService{client: client}
    
    // Первые 3 запроса должны пройти и завершиться ошибкой
    for i := 0; i < 3; i++ {
        _, err := service.FetchData(context.Background())
        assert.Error(t, err)
    }
    
    // 4-й запрос должен быть заблокирован circuit breaker
    _, err := service.FetchData(context.Background())
    assert.Error(t, err)
    // Проверяем что это именно ошибка circuit breaker
    assert.Contains(t, err.Error(), "circuit breaker")
    
    mockClient.AssertExpectations(t)
}
```

### Тестирование Middleware

```go
func TestMiddlewareBehavior(t *testing.T) {
    // Создаем middleware который добавляет заголовок
    authMiddleware := httpclient.NewBearerTokenMiddleware("test-token")
    
    // Тестовый сервер который проверяет заголовок
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
        if auth != "Bearer test-token" {
            w.WriteHeader(401)
            return
        }
        w.WriteHeader(200)
        w.Write([]byte(`{"status": "authorized"}`))
    }))
    defer server.Close()
    
    client, _ := httpclient.NewClient(
        httpclient.WithMiddleware(authMiddleware),
    )
    
    // Запрос должен быть авторизован
    resp, err := client.Get(server.URL)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    resp.Body.Close()
}
```

## Тестирование метрик

### Проверка сбора метрик

```go
func TestMetricsCollection(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        w.Write([]byte(`{"data": "test"}`))
    }))
    defer server.Close()
    
    client, _ := httpclient.NewClient(
        httpclient.WithMetrics(true),
    )
    
    // Выполняем несколько запросов
    for i := 0; i < 5; i++ {
        resp, _ := client.Get(server.URL)
        resp.Body.Close()
    }
    
    // Проверяем метрики
    metrics := client.GetMetrics()
    
    assert.Equal(t, int64(5), metrics.TotalRequests)
    assert.Equal(t, int64(5), metrics.SuccessfulRequests)
    assert.Equal(t, int64(0), metrics.FailedRequests)
    assert.True(t, metrics.AverageLatency > 0)
    
    // Проверяем статус коды
    statusCodes := metrics.GetStatusCodes()
    assert.Equal(t, int64(5), statusCodes[200])
}
```

### Тестирование reset метрик

```go
func TestMetricsReset(t *testing.T) {
    client, _ := httpclient.NewClient(httpclient.WithMetrics(true))
    
    // Делаем запрос
    resp, _ := client.Get("https://httpbin.org/status/200")
    resp.Body.Close()
    
    // Проверяем что метрики не пустые
    metrics := client.GetMetrics()
    assert.Equal(t, int64(1), metrics.TotalRequests)
    
    // Сбрасываем метрики
    metrics.Reset()
    
    // Проверяем что метрики сброшены
    assert.Equal(t, int64(0), metrics.TotalRequests)
    assert.Equal(t, int64(0), metrics.SuccessfulRequests)
    assert.Equal(t, int64(0), metrics.FailedRequests)
}
```

## Интеграционные тесты

### Тестирование с реальными API

```go
func TestRealAPIIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Пропуск интеграционного теста в коротком режиме")
    }
    
    client, _ := httpclient.NewClient(
        httpclient.WithTimeout(10*time.Second),
        httpclient.WithRetryMax(3),
    )
    
    // Тест с httpbin.org (публичный тестовый API)
    t.Run("GET request", func(t *testing.T) {
        resp, err := client.Get("https://httpbin.org/status/200")
        assert.NoError(t, err)
        assert.Equal(t, 200, resp.StatusCode)
        resp.Body.Close()
    })
    
    t.Run("JSON request", func(t *testing.T) {
        var result map[string]interface{}
        err := client.GetJSON(context.Background(), "https://httpbin.org/json", &result)
        assert.NoError(t, err)
        assert.NotEmpty(t, result)
    })
    
    t.Run("POST JSON", func(t *testing.T) {
        data := map[string]string{"key": "value"}
        var result map[string]interface{}
        
        err := client.PostJSON(context.Background(), "https://httpbin.org/post", data, &result)
        assert.NoError(t, err)
        assert.NotEmpty(t, result)
    })
}
```

### Тестирование производительности

```go
func BenchmarkHTTPClient(b *testing.B) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        w.Write([]byte(`{"status": "ok"}`))
    }))
    defer server.Close()
    
    client, _ := httpclient.NewClient()
    
    b.ResetTimer()
    
    b.Run("Sequential", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            resp, _ := client.Get(server.URL)
            resp.Body.Close()
        }
    })
    
    b.Run("Parallel", func(b *testing.B) {
        b.RunParallel(func(pb *testing.PB) {
            for pb.Next() {
                resp, _ := client.Get(server.URL)
                resp.Body.Close()
            }
        })
    })
}
```

## Лучшие практики тестирования

### 1. Изоляция тестов

```go
func TestIsolatedBehavior(t *testing.T) {
    // Каждый тест должен создавать свой собственный mock
    mockClient := httpmock.NewMockHTTPClient()
    
    // Четко определенные ожидания для каждого теста
    mockClient.On("GetJSON", mock.Anything, "specific-url", mock.Anything).
        Return(nil).
        Once() // Ограничиваем количество вызовов
    
    // Тест...
    
    mockClient.AssertExpectations(t)
}
```

### 2. Тестирование граничных случаев

```go
func TestEdgeCases(t *testing.T) {
    testCases := []struct {
        name           string
        statusCode     int
        responseBody   string
        expectedError  string
    }{
        {"Success", 200, `{"status": "ok"}`, ""},
        {"Not Found", 404, `{"error": "not found"}`, "not found"},
        {"Server Error", 500, `{"error": "internal error"}`, "internal error"},
        {"Bad Gateway", 502, "", "bad gateway"},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            mockClient := httpmock.NewMockHTTPClient()
            
            resp := &http.Response{
                StatusCode: tc.statusCode,
                Body:       io.NopCloser(strings.NewReader(tc.responseBody)),
            }
            
            mockClient.On("Get", mock.Anything).Return(resp, nil)
            
            service := &APIService{client: mockClient}
            _, err := service.FetchData(context.Background())
            
            if tc.expectedError != "" {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tc.expectedError)
            } else {
                assert.NoError(t, err)
            }
            
            mockClient.AssertExpectations(t)
        })
    }
}
```

### 3. Использование table-driven тестов

```go
func TestRetryStrategies(t *testing.T) {
    tests := []struct {
        name     string
        strategy httpclient.RetryStrategy
        attempts int
        expected time.Duration
    }{
        {
            name:     "Fixed Delay",
            strategy: httpclient.NewFixedDelayStrategy(3, 1*time.Second),
            attempts: 1,
            expected: 1 * time.Second,
        },
        {
            name:     "Exponential Backoff",
            strategy: httpclient.NewExponentialBackoffStrategy(3, 100*time.Millisecond, 5*time.Second),
            attempts: 2,
            expected: 200 * time.Millisecond,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            delay := tt.strategy.NextDelay(tt.attempts, nil)
            assert.Equal(t, tt.expected, delay)
        })
    }
}
```

### 4. Cleanup и ресурсы

```go
func TestWithCleanup(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))
    
    // Используем t.Cleanup для гарантированной очистки
    t.Cleanup(func() {
        server.Close()
    })
    
    client, _ := httpclient.NewClient()
    
    resp, err := client.Get(server.URL)
    assert.NoError(t, err)
    
    t.Cleanup(func() {
        if resp != nil {
            resp.Body.Close()
        }
    })
}
```

## Запуск тестов

### Команды для запуска

```bash
# Все тесты
go test ./...

# Только unit тесты (быстрые)
go test -short ./...

# Тесты с покрытием
go test -cover ./...

# Только интеграционные тесты
go test -run Integration ./...

# Benchmark тесты
go test -bench=. ./...

# Verbose вывод
go test -v ./...
```

### Структура тестов

```
project/
├── client_test.go          # Тесты основного клиента
├── retry_test.go           # Тесты retry логики
├── middleware_test.go      # Тесты middleware
├── metrics_test.go         # Тесты метрик
├── integration_test.go     # Интеграционные тесты
└── mock/
    ├── mock_client.go      # Mock HTTP клиент
    └── test_helpers.go     # Тестовые помощники
```

## См. также

- [Примеры](examples.md) - Практические примеры использования
- [API Reference](api-reference.md) - Полное описание интерфейсов
- [Mock Documentation](../mock/README.md) - Подробная документация по mock объектам