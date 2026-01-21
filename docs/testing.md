# Testing

The HTTP client package provides powerful testing utilities, including mock servers, mock transports, and helpers for various testing scenarios.

## Test Utilities

### TestServer - Mock HTTP Server
```go
func TestBasicHTTPRequests(t *testing.T) {
    // Create test server with predefined responses
    server := httpclient.NewTestServer(
        httpclient.TestResponse{
            StatusCode: 200,
            Body:       `{"message": "success"}`,
            Headers:    map[string]string{"Content-Type": "application/json"},
        },
    )
    defer server.Close()
    
    client := httpclient.New(httpclient.Config{}, "test-client")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), server.URL)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    _ = resp.Body.Close()
    
    // Check request count
    assert.Equal(t, 1, server.GetRequestCount())
}
```

### MockRoundTripper - Unit Tests
```go
func TestClientWithMockTransport(t *testing.T) {
    mock := httpclient.NewMockRoundTripper()
    
    // Predefined responses
    mock.AddResponse(&http.Response{
        StatusCode: 200,
        Body:       io.NopCloser(strings.NewReader(`{"data": "test"}`)),
        Header:     http.Header{"Content-Type": []string{"application/json"}},
    })
    
    config := httpclient.Config{Transport: mock}
    client := httpclient.New(config, "mock-test")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), "https://example.com/api")
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    _ = resp.Body.Close()
    
    // Check that request was made
    assert.Equal(t, 1, mock.GetCallCount())
    
    requests := mock.GetRequests()
    assert.Equal(t, "GET", requests[0].Method)
    assert.Equal(t, "https://example.com/api", requests[0].URL.String())
}
```

## Testing Retry Logic

### Successful Retry Test
```go
func TestRetrySuccess(t *testing.T) {
    server := httpclient.NewTestServer()
    
    // First two requests fail, third succeeds
    server.AddResponse(httpclient.TestResponse{StatusCode: 500})
    server.AddResponse(httpclient.TestResponse{StatusCode: 502})
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 200,
        Body:       "success",
    })
    defer server.Close()
    
    config := httpclient.Config{
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   10 * time.Millisecond,
            MaxDelay:    100 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "retry-test")
    defer client.Close()
    
    start := time.Now()
    resp, err := client.Get(context.Background(), server.URL)
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    _ = resp.Body.Close()
    
    // Check that there were 3 attempts
    assert.Equal(t, 3, server.GetRequestCount())
    
    // Check that there were delays between attempts
    assert.Greater(t, duration, 20*time.Millisecond)
}
```

### Retry Exhaustion Test
```go
func TestRetryExhaustion(t *testing.T) {
    server := httpclient.NewTestServer()
    
    // All requests fail
    for i := 0; i < 5; i++ {
        server.AddResponse(httpclient.TestResponse{StatusCode: 500})
    }
    defer server.Close()
    
    config := httpclient.Config{
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   10 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "retry-exhaustion-test")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), server.URL)
    
    // Should be RetryableError
    assert.Error(t, err)
    assert.Nil(t, resp)
    
    var retryableErr *httpclient.RetryableError
    assert.True(t, errors.As(err, &retryableErr))
    assert.Equal(t, 3, retryableErr.Attempts)
    
    // Check attempt count
    assert.Equal(t, 3, server.GetRequestCount())
}
```

### Idempotency Test
```go
func TestIdempotentRetry(t *testing.T) {
    mock := httpclient.NewMockRoundTripper()
    
    // First request fails, second succeeds
    mock.AddError(errors.New("network error"))
    mock.AddResponse(&http.Response{
        StatusCode: 201,
        Body:       io.NopCloser(strings.NewReader(`{"id": 123}`)),
    })
    
    config := httpclient.Config{
        Transport: mock,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,
            BaseDelay:   10 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "idempotent-test")
    defer client.Close()
    
    // POST with Idempotency-Key
    req, _ := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://api.example.com/payments",
        strings.NewReader(`{"amount": 100}`),
    )
    req.Header.Set("Idempotency-Key", "payment-12345")
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    assert.NoError(t, err)
    assert.Equal(t, 201, resp.StatusCode)
    _ = resp.Body.Close()
    
    // Check that there were 2 attempts
    assert.Equal(t, 2, mock.GetCallCount())
    
    // Check that Idempotency-Key was passed in both attempts
    requests := mock.GetRequests()
    for _, req := range requests {
        assert.Equal(t, "payment-12345", req.Header.Get("Idempotency-Key"))
    }
}
```

## Testing Metrics

### Metrics Collection Check
```go
func TestMetricsCollection(t *testing.T) {
    // Setup test Prometheus registry
    registry := prometheus.NewRegistry()
    
    // Create client (metrics will be collected automatically)
    client := httpclient.New(httpclient.Config{}, "metrics-test")
    defer client.Close()
    
    server := httpclient.NewTestServer(
        httpclient.TestResponse{StatusCode: 200, Body: "OK"},
    )
    defer server.Close()
    
    // Execute requests
    for i := 0; i < 5; i++ {
        resp, err := client.Get(context.Background(), server.URL)
        assert.NoError(t, err)
        _ = resp.Body.Close()
    }
    
    // Check that metrics were collected
    // (actual check depends on OpenTelemetry setup in tests)
    assert.Equal(t, 5, server.GetRequestCount())
}
```

### Helper for Metrics Checking
```go
type MetricsCollector struct {
    metrics map[string]interface{}
    mu      sync.RWMutex
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        metrics: make(map[string]interface{}),
    }
}

func (mc *MetricsCollector) Record(name string, value interface{}) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    mc.metrics[name] = value
}

func (mc *MetricsCollector) Get(name string) interface{} {
    mc.mu.RLock()
    defer mc.mu.RUnlock()
    return mc.metrics[name]
}

func TestMetricsWithCollector(t *testing.T) {
    collector := NewMetricsCollector()
    
    // Simulate metrics collection
    collector.Record("requests_total", 10)
    collector.Record("error_rate", 0.05)
    
    assert.Equal(t, 10, collector.Get("requests_total"))
    assert.Equal(t, 0.05, collector.Get("error_rate"))
}
```

## Testing Timeouts

### Overall Timeout Test
```go
func TestOverallTimeout(t *testing.T) {
    server := httpclient.NewTestServer(
        httpclient.TestResponse{
            StatusCode: 200,
            Body:       "OK",
            Delay:      2 * time.Second, // Server responds slowly
        },
    )
    defer server.Close()
    
    config := httpclient.Config{
        Timeout: 1 * time.Second, // Timeout less than server delay
    }
    
    client := httpclient.New(config, "timeout-test")
    defer client.Close()
    
    start := time.Now()
    resp, err := client.Get(context.Background(), server.URL)
    duration := time.Since(start)
    
    assert.Error(t, err)
    assert.Nil(t, resp)
    assert.Less(t, duration, 1500*time.Millisecond) // Completed due to timeout
    
    // Check that this is a timeout error
    assert.Contains(t, err.Error(), "timeout")
}
```

### Per-Try Timeout Test
```go
func TestPerTryTimeout(t *testing.T) {
    server := httpclient.NewTestServer()
    
    // Slow responses for all attempts
    for i := 0; i < 3; i++ {
        server.AddResponse(httpclient.TestResponse{
            StatusCode: 200,
            Body:       "OK",
            Delay:      500 * time.Millisecond,
        })
    }
    defer server.Close()
    
    config := httpclient.Config{
        Timeout:       5 * time.Second,   // Общий таймаут большой
        PerTryTimeout: 200 * time.Millisecond, // Таймаут попытки маленький
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   50 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "per-try-timeout-test")
    defer client.Close()
    
    start := time.Now()
    resp, err := client.Get(context.Background(), server.URL)
    duration := time.Since(start)
    
    assert.Error(t, err)
    assert.Nil(t, resp)
    
    // Должно завершиться после 3 попыток с таймаутами
    expectedMinDuration := 3*200*time.Millisecond + 2*50*time.Millisecond
    assert.Greater(t, duration, expectedMinDuration)
    
    // Но значительно быстрее чем 5 секунд
    assert.Less(t, duration, 2*time.Second)
}
```

## Интеграционные тесты

### Тест с реальным API
```go
func TestRealAPIIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Пропускаем интеграционные тесты в коротком режиме")
    }
    
    client := httpclient.New(httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{MaxAttempts: 2},
    }, "integration-test")
    defer client.Close()
    
    // Тест с httpbin.org
    resp, err := client.Get(context.Background(), "https://httpbin.org/get")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
    
    var response map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&response)
    require.NoError(t, err)
    
    assert.Contains(t, response, "url")
    assert.Equal(t, "https://httpbin.org/get", response["url"])
}
```

### Benchmarks
```go
func BenchmarkHTTPClient(b *testing.B) {
    server := httpclient.NewTestServer(
        httpclient.TestResponse{StatusCode: 200, Body: "OK"},
    )
    defer server.Close()
    
    client := httpclient.New(httpclient.Config{}, "benchmark-test")
    defer client.Close()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            resp, err := client.Get(context.Background(), server.URL)
            if err != nil {
                b.Fatal(err)
            }
            _ = resp.Body.Close()
        }
    })
}

func BenchmarkHTTPClientWithRetry(b *testing.B) {
    server := httpclient.NewTestServer(
        httpclient.TestResponse{StatusCode: 200, Body: "OK"},
    )
    defer server.Close()
    
    config := httpclient.Config{
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   1 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "benchmark-retry-test")
    defer client.Close()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resp, err := client.Get(context.Background(), server.URL)
        if err != nil {
            b.Fatal(err)
        }
        _ = resp.Body.Close()
    }
}
```

## Test Helpers

### Ожидание условий
```go
func WaitForCondition(timeout time.Duration, condition func() bool) bool {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if condition() {
            return true
        }
        time.Sleep(10 * time.Millisecond)
    }
    return false
}

func TestWaitForCondition(t *testing.T) {
    server := httpclient.NewTestServer(
        httpclient.TestResponse{StatusCode: 200, Body: "OK"},
    )
    defer server.Close()
    
    client := httpclient.New(httpclient.Config{}, "wait-test")
    defer client.Close()
    
    // Запускаем запрос в горутине
    go func() {
        resp, _ := client.Get(context.Background(), server.URL)
        if resp != nil {
            _ = resp.Body.Close()
        }
    }()
    
    // Ждем что сервер получит запрос
    success := WaitForCondition(5*time.Second, func() bool {
        return server.GetRequestCount() > 0
    })
    
    assert.True(t, success, "Сервер должен был получить запрос")
}
```

### Утверждения для eventual consistency
```go
func AssertEventuallyTrue(t testing.TB, timeout time.Duration, condition func() bool, message string) {
    t.Helper()
    
    if WaitForCondition(timeout, condition) {
        return
    }
    
    t.Fatalf("Условие не выполнилось за %v: %s", timeout, message)
}

func TestEventuallyTrue(t *testing.T) {
    counter := 0
    
    go func() {
        time.Sleep(100 * time.Millisecond)
        counter = 5
    }()
    
    AssertEventuallyTrue(t, 1*time.Second, func() bool {
        return counter == 5
    }, "counter должен стать равным 5")
}
```

## Примеры тестовых сценариев

### Полный integration тест сервиса
```go
func TestUserServiceIntegration(t *testing.T) {
    // Настройка mock сервера
    server := httpclient.NewTestServer()
    
    // Mock ответы для разных эндпоинтов
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 201,
        Body:       `{"id": 123, "name": "Test User"}`,
        Headers:    map[string]string{"Content-Type": "application/json"},
    })
    
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 200,
        Body:       `{"id": 123, "name": "Test User", "email": "test@example.com"}`,
        Headers:    map[string]string{"Content-Type": "application/json"},
    })
    defer server.Close()
    
    // Настройка клиента
    config := httpclient.Config{
        Timeout: 5 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,
            BaseDelay:   10 * time.Millisecond,
        },
        TracingEnabled: true,
    }
    
    client := httpclient.New(config, "user-service-test")
    defer client.Close()
    
    // Тест создания пользователя
    userData := `{"name": "Test User"}`
    resp, err := client.Post(
        context.Background(),
        server.URL+"/users",
        "application/json",
        strings.NewReader(userData),
    )
    
    assert.NoError(t, err)
    assert.Equal(t, 201, resp.StatusCode)
    _ = resp.Body.Close()
    
    // Тест получения пользователя
    resp, err = client.Get(context.Background(), server.URL+"/users/123")
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    
    var user map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&user)
    assert.NoError(t, err)
    assert.Equal(t, float64(123), user["id"])
    assert.Equal(t, "Test User", user["name"])
    _ = resp.Body.Close()
    
    // Проверка количества запросов
    assert.Equal(t, 2, server.GetRequestCount())
    
    // Проверка последнего запроса
    lastRequest := server.GetLastRequest()
    assert.Equal(t, "GET", lastRequest.Method)
    assert.Contains(t, lastRequest.URL, "/users/123")
}
```

Эти тестовые утилиты и примеры помогают создать комплексные тесты для проверки всех аспектов работы HTTP клиента - от базовой функциональности до сложных сценариев с retry, метриками и реальными интеграциями.