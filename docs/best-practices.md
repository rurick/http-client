# Лучшие практики

Рекомендации по эффективному и безопасному использованию HTTP клиент пакета в продакшене.

## Именование и организация

### Имена клиентов
Используйте описательные имена, которые помогают идентифицировать сервис в метриках:

```go
// ✅ Хорошо - ясно идентифицирует сервис
client := httpclient.New(config, "user-service-v2")
client := httpclient.New(config, "payment-processor")
client := httpclient.New(config, "external-analytics-api")

// ❌ Плохо - слишком общие или неинформативные
client := httpclient.New(config, "client")
client := httpclient.New(config, "api")
client := httpclient.New(config, "service")
```

### Организация клиентов в коде
```go
type ServiceClients struct {
    Users    *httpclient.Client
    Payments *httpclient.Client
    Analytics *httpclient.Client
}

func NewServiceClients() *ServiceClients {
    return &ServiceClients{
        Users:     httpclient.New(userServiceConfig(), "user-service"),
        Payments:  httpclient.New(paymentServiceConfig(), "payment-service"),
        Analytics: httpclient.New(analyticsServiceConfig(), "analytics-api"),
    }
}

func (sc *ServiceClients) Close() error {
    var errs []error
    if err := sc.Users.Close(); err != nil {
        errs = append(errs, err)
    }
    if err := sc.Payments.Close(); err != nil {
        errs = append(errs, err)
    }
    if err := sc.Analytics.Close(); err != nil {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("ошибки при закрытии клиентов: %v", errs)
    }
    return nil
}
```

## Конфигурация таймаутов

### По типу сервиса

#### Внутренние микросервисы
```go
func internalServiceConfig() httpclient.Config {
    return httpclient.Config{
        Timeout:       5 * time.Second,   // Быстрая внутренняя сеть
        PerTryTimeout: 1 * time.Second,   // Короткие попытки
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,               // Минимальные повторы
            BaseDelay:   50 * time.Millisecond,
            MaxDelay:    500 * time.Millisecond,
            Jitter:      0.1,
        },
        TracingEnabled: true,             // Всегда включать для микросервисов
    }
}
```

#### Внешние API
```go
func externalAPIConfig() httpclient.Config {
    return httpclient.Config{
        Timeout:       30 * time.Second,  // Учитываем сетевые задержки
        PerTryTimeout: 10 * time.Second,  // Более длинные попытки
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 5,               // Агрессивные повторы
            BaseDelay:   200 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Jitter:      0.3,             // Высокий джиттер
        },
        TracingEnabled: true,
    }
}
```

#### Критичные операции (платежи, заказы)
```go
func criticalServiceConfig() httpclient.Config {
    return httpclient.Config{
        Timeout:       60 * time.Second,  // Достаточно времени
        PerTryTimeout: 15 * time.Second,  // Терпеливые попытки
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 7,               // Максимальная надежность
            BaseDelay:   500 * time.Millisecond,
            MaxDelay:    30 * time.Second,
            Jitter:      0.25,
        },
        TracingEnabled: true,
    }
}
```

### По SLA требованиям

| SLA | Timeout | PerTryTimeout | MaxAttempts | BaseDelay | Описание |
|-----|---------|---------------|-------------|-----------|----------|
| 99% | 3s | 1s | 2 | 25ms | Быстро, но не надежно |
| 99.9% | 10s | 3s | 3 | 100ms | Баланс скорости и надежности |
| 99.95% | 20s | 5s | 5 | 200ms | Высокая надежность |
| 99.99% | 60s | 15s | 7 | 500ms | Максимальная надежность |

## Стратегии повторов

### Выбор стратегии по сценарию

#### Быстрые внутренние вызовы
```go
RetryConfig{
    MaxAttempts: 2,                    // Быстро отказываться
    BaseDelay:   25 * time.Millisecond, // Минимальная задержка
    MaxDelay:    200 * time.Millisecond,
    Jitter:      0.1,                  // Низкий джиттер
}
```

#### Идемпотентные операции
```go
RetryConfig{
    MaxAttempts: 5,                    // Можно агрессивно повторять
    BaseDelay:   100 * time.Millisecond,
    MaxDelay:    5 * time.Second,
    Jitter:      0.2,
}
```

#### Неидемпотентные операции
```go
// Полагаемся на Idempotency-Key
req.Header.Set("Idempotency-Key", generateIdempotencyKey())

RetryConfig{
    MaxAttempts: 3,                    // Осторожные повторы
    BaseDelay:   200 * time.Millisecond,
    MaxDelay:    2 * time.Second,
    Jitter:      0.2,
}
```

### Адаптивные стратегии

```go
func adaptiveRetryConfig(errorRate float64) httpclient.RetryConfig {
    if errorRate > 0.1 { // Высокий процент ошибок
        return httpclient.RetryConfig{
            MaxAttempts: 2,               // Меньше попыток
            BaseDelay:   500 * time.Millisecond, // Больше задержка
            MaxDelay:    10 * time.Second,
            Jitter:      0.5,             // Высокий джиттер
        }
    }
    
    // Обычные условия
    return httpclient.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    3 * time.Second,
        Jitter:      0.2,
    }
}
```

## Обработка ошибок

### Комплексная обработка
```go
func handleHTTPRequest(client *httpclient.Client, url string) error {
    resp, err := client.Get(context.Background(), url)
    if err != nil {
        // Логирование с контекстом
        switch e := err.(type) {
        case *httpclient.RetryableError:
            log.WithFields(log.Fields{
                "url":      url,
                "attempts": e.Attempts,
                "error":    e.Err.Error(),
            }).Error("Запрос не удался после всех попыток")
            
            // Метрики бизнес-логики
            metrics.HTTPRequestsFailed.WithLabelValues("retries_exhausted").Inc()
            
        case *httpclient.NonRetryableError:
            log.WithFields(log.Fields{
                "url":   url,
                "error": e.Err.Error(),
            }).Error("Неповторяемая ошибка запроса")
            
            metrics.HTTPRequestsFailed.WithLabelValues("non_retryable").Inc()
            
        default:
            log.WithFields(log.Fields{
                "url":   url,
                "error": err.Error(),
            }).Error("Неожиданная ошибка запроса")
            
            metrics.HTTPRequestsFailed.WithLabelValues("unexpected").Inc()
        }
        
        return err
    }
    defer resp.Body.Close()
    
    // Проверка статус кода
    if resp.StatusCode >= 400 {
        body, _ := io.ReadAll(resp.Body)
        log.WithFields(log.Fields{
            "url":         url,
            "status_code": resp.StatusCode,
            "response":    string(body),
        }).Error("HTTP ошибка")
        
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }
    
    return nil
}
```

### Graceful degradation
```go
func getDataWithFallback(client *httpclient.Client) (*Data, error) {
    // Попытка получить данные
    resp, err := client.Get(context.Background(), "https://api.primary.com/data")
    if err == nil {
        defer resp.Body.Close()
        if resp.StatusCode == 200 {
            var data Data
            if json.NewDecoder(resp.Body).Decode(&data) == nil {
                return &data, nil
            }
        }
    }
    
    // Fallback к кешу
    if cachedData := getCachedData(); cachedData != nil {
        log.Info("Используем кешированные данные из-за ошибки API")
        return cachedData, nil
    }
    
    // Fallback к резервному API
    resp, err = client.Get(context.Background(), "https://api.backup.com/data")
    if err == nil {
        defer resp.Body.Close()
        var data Data
        if json.NewDecoder(resp.Body).Decode(&data) == nil {
            log.Info("Используем данные из резервного API")
            return &data, nil
        }
    }
    
    return nil, fmt.Errorf("все источники данных недоступны")
}
```

## Управление ресурсами

### Lifecycle management
```go
type APIClient struct {
    client *httpclient.Client
    closed bool
    mu     sync.Mutex
}

func NewAPIClient(config httpclient.Config) *APIClient {
    return &APIClient{
        client: httpclient.New(config, "api-client"),
    }
}

func (ac *APIClient) Get(ctx context.Context, url string) (*http.Response, error) {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    if ac.closed {
        return nil, fmt.Errorf("клиент закрыт")
    }
    
    return ac.client.Get(ctx, url)
}

func (ac *APIClient) Close() error {
    ac.mu.Lock()
    defer ac.mu.Unlock()
    
    if ac.closed {
        return nil
    }
    
    ac.closed = true
    return ac.client.Close()
}
```

### Пулы клиентов
```go
type ClientPool struct {
    clients chan *httpclient.Client
    config  httpclient.Config
    name    string
}

func NewClientPool(size int, config httpclient.Config, name string) *ClientPool {
    pool := &ClientPool{
        clients: make(chan *httpclient.Client, size),
        config:  config,
        name:    name,
    }
    
    // Предварительное создание клиентов
    for i := 0; i < size; i++ {
        pool.clients <- httpclient.New(config, fmt.Sprintf("%s-%d", name, i))
    }
    
    return pool
}

func (cp *ClientPool) Get() *httpclient.Client {
    return <-cp.clients
}

func (cp *ClientPool) Put(client *httpclient.Client) {
    select {
    case cp.clients <- client:
    default:
        // Пул переполнен, закрываем клиент
        client.Close()
    }
}

func (cp *ClientPool) Close() error {
    close(cp.clients)
    var errs []error
    for client := range cp.clients {
        if err := client.Close(); err != nil {
            errs = append(errs, err)
        }
    }
    return nil
}
```

## Безопасность

### Идемпотентность
```go
func generateIdempotencyKey(operation, userID string) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s:%s:%d", operation, userID, time.Now().Unix()/300))) // 5-минутные окна
    return hex.EncodeToString(h.Sum(nil))[:16]
}

func createPayment(client *httpclient.Client, paymentData PaymentData) error {
    reqBody, _ := json.Marshal(paymentData)
    
    req, err := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://api.payments.com/payments",
        bytes.NewReader(reqBody),
    )
    if err != nil {
        return err
    }
    
    // Критично для POST запросов
    req.Header.Set("Idempotency-Key", generateIdempotencyKey("payment", paymentData.UserID))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### Защита от утечек данных
```go
func secureRequest(client *httpclient.Client, sensitiveData string) error {
    // НЕ логируем sensitive данные
    log.Info("Отправка secure запроса")
    
    resp, err := client.Post(
        context.Background(),
        "https://secure-api.com/data",
        "application/json",
        strings.NewReader(sensitiveData),
    )
    if err != nil {
        // НЕ включаем в лог тело запроса
        log.WithField("error", err.Error()).Error("Ошибка secure запроса")
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        // НЕ логируем потенциально sensitive ответы
        log.WithField("status", resp.StatusCode).Error("HTTP ошибка в secure запросе")
        return fmt.Errorf("HTTP ошибка: %d", resp.StatusCode)
    }
    
    return nil
}
```

## Мониторинг и алерты

### Настройка алертов по сервисам

```yaml
groups:
- name: user-service.rules
  rules:
  - alert: UserServiceHighErrorRate
    expr: |
      (
        sum(rate(http_client_requests_total{host=~".*user.*", error="true"}[5m])) /
        sum(rate(http_client_requests_total{host=~".*user.*"}[5m]))
      ) > 0.05
    for: 2m
    labels:
      severity: critical
      service: user-service
    annotations:
      summary: "Высокий процент ошибок User Service"
      runbook_url: "https://wiki.company.com/runbooks/user-service"

- name: payment-service.rules  
  rules:
  - alert: PaymentServiceHighLatency
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket{host=~".*payment.*"}[5m])) by (le)) > 5
    for: 1m
    labels:
      severity: critical
      service: payment-service
    annotations:
      summary: "Критическая латентность Payment Service"
      runbook_url: "https://wiki.company.com/runbooks/payment-service"
```

### Дашборды в коде
```go
func setupMonitoring() {
    // Бизнес-метрики поверх технических
    userServiceLatency := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "user_service_business_latency_seconds",
            Help: "Бизнес-латентность User Service операций",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"operation", "result"},
    )
    
    prometheus.MustRegister(userServiceLatency)
    
    // Можно корреляцию с техническими метриками HTTP клиента
}
```

## Тестирование

### Тестирование retry логики
```go
func TestRetryBehavior(t *testing.T) {
    // Сервер, который падает первые N раз
    failureCount := 0
    server := httpclient.NewTestServer()
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 500,
        Body:       "Internal Server Error",
    })
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 500,
        Body:       "Internal Server Error", 
    })
    server.AddResponse(httpclient.TestResponse{
        StatusCode: 200,
        Body:       "Success",
    })
    defer server.Close()
    
    config := httpclient.Config{
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   10 * time.Millisecond,
            MaxDelay:    100 * time.Millisecond,
        },
    }
    
    client := httpclient.New(config, "test-client")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), server.URL)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    
    // Проверяем что было сделано 3 попытки
    assert.Equal(t, 3, server.GetRequestCount())
}
```

### Интеграционные тесты
```go
func TestServiceIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Пропускаем интеграционные тесты в коротком режиме")
    }
    
    config := httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{MaxAttempts: 2},
    }
    
    client := httpclient.New(config, "integration-test")
    defer client.Close()
    
    // Тест реального API
    resp, err := client.Get(context.Background(), "https://httpbin.org/status/200")
    require.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    resp.Body.Close()
}
```

## Производительность

### Оптимизация Transport
```go
func optimizedTransport() *http.Transport {
    return &http.Transport{
        // Пулы соединений
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        
        // Таймауты соединений  
        DialTimeout:         5 * time.Second,
        TLSHandshakeTimeout: 5 * time.Second,
        
        // Keep-alive
        KeepAlive:           30 * time.Second,
        
        // Буферы
        ReadBufferSize:      8192,
        WriteBufferSize:     8192,
        
        // Сжатие
        DisableCompression:  false,
        
        // HTTP/2
        ForceAttemptHTTP2:   true,
    }
}
```

### Батчинг запросов
```go
func batchRequests(client *httpclient.Client, urls []string) ([]Response, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    responses := make([]Response, len(urls))
    errChan := make(chan error, len(urls))
    
    // Ограничиваем конкурентность
    semaphore := make(chan struct{}, 10)
    
    var wg sync.WaitGroup
    for i, url := range urls {
        wg.Add(1)
        go func(index int, u string) {
            defer wg.Done()
            
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            resp, err := client.Get(ctx, u)
            if err != nil {
                errChan <- err
                return
            }
            defer resp.Body.Close()
            
            var response Response
            if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
                errChan <- err
                return
            }
            
            responses[index] = response
        }(i, url)
    }
    
    wg.Wait()
    close(errChan)
    
    // Собираем ошибки
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return nil, fmt.Errorf("ошибки в %d из %d запросов", len(errors), len(urls))
    }
    
    return responses, nil
}
```

## Миграция с других клиентов

### С стандартного http.Client
```go
// Было
httpClient := &http.Client{
    Timeout: 10 * time.Second,
}

// Стало
config := httpclient.Config{
    Timeout: 10 * time.Second,
    RetryConfig: httpclient.RetryConfig{MaxAttempts: 3},
    TracingEnabled: true,
}
client := httpclient.New(config, "migrated-service")
defer client.Close()
```

### С других HTTP библиотек
```go
// Resty -> httpclient
func migrateFromResty() {
    // Было (Resty)
    // resp, err := resty.New().R().Get("https://api.example.com")
    
    // Стало
    client := httpclient.New(httpclient.Config{
        Timeout: 30 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    5 * time.Second,
        },
    }, "migrated-from-resty")
    defer client.Close()
    
    resp, err := client.Get(context.Background(), "https://api.example.com")
}
```

Следуйте этим практикам для максимальной надежности, производительности и maintainability вашего HTTP клиента в продакшене.