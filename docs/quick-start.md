# Быстрый старт

Этот раздел поможет вам быстро начать использовать HTTP клиент пакет.

## Установка

Пакет является частью внутренней экосистемы CityDrive и доступен через внутренний GitLab:

```bash
go get gitlab.citydrive.tech/back-end/go/pkg/http-client
```

## Базовое использование

### Простой HTTP клиент

```go
package main

import (
    "context"
    "log"
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    // Создание клиента с настройками по умолчанию
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    // GET запрос
    resp, err := client.Get(context.Background(), "https://api.example.com/users")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    // Чтение ответа
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response: %s\n", body)
}
```

### POST запрос с JSON

```go
func createUser(client *httpclient.Client) error {
    userData := `{
        "name": "John Doe",
        "email": "john@example.com"
    }`
    
    resp, err := client.Post(
        context.Background(),
        "https://api.example.com/users",
        "application/json",
        strings.NewReader(userData),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 201 {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    
    return nil
}
```

### Использование с контекстом и таймаутом

```go
func fetchWithTimeout() error {
    client := httpclient.New(httpclient.Config{
        Timeout: 10 * time.Second,
    }, "api-client")
    defer client.Close()
    
    // Создание контекста с таймаутом
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    resp, err := client.Get(ctx, "https://slow-api.example.com/data")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Конфигурация с retry

### Базовые retry настройки

```go
func createRetryClient() *httpclient.Client {
    config := httpclient.Config{
        Timeout:       30 * time.Second,
        PerTryTimeout: 5 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    5 * time.Second,
            Jitter:      0.2,
        },
    }
    
    return httpclient.New(config, "retry-client")
}
```

### Идемпотентные операции

```go
func updateResource(client *httpclient.Client, id string, data string) error {
    // PUT запросы автоматически повторяются
    resp, err := client.Put(
        context.Background(),
        fmt.Sprintf("https://api.example.com/resources/%s", id),
        "application/json",
        strings.NewReader(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### POST с Idempotency-Key

```go
func createPayment(client *httpclient.Client, paymentData string) error {
    req, err := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://api.payment.com/payments",
        strings.NewReader(paymentData),
    )
    if err != nil {
        return err
    }
    
    // Добавление Idempotency-Key позволяет повторять POST запросы
    req.Header.Set("Idempotency-Key", "payment-12345")
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Circuit Breaker (коротко)

### Включить по умолчанию
```go
client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true, // если экземпляр не задан, используется SimpleCircuitBreaker
}, "my-service")
```

### Кастомные пороги и обработчик
```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 2,
    SuccessThreshold: 1,
    Timeout:          5 * time.Second,
    OnStateChange: func(from, to httpclient.CircuitBreakerState) { log.Printf("CB: %s -> %s", from, to) },
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "orders-client")
```

## Обработка ошибок

### Проверка типов ошибок

```go
func handleErrors(client *httpclient.Client) {
    resp, err := client.Get(context.Background(), "https://api.example.com/data")
    if err != nil {
        // Проверка на ошибки после исчерпания retry попыток
        if retryableErr, ok := err.(*httpclient.RetryableError); ok {
            log.Printf("Запрос не удался после %d попыток: %v", 
                retryableErr.Attempts, retryableErr.Err)
            return
        }
        
        // Ошибки, которые не подлежат повтору
        if nonRetryableErr, ok := err.(*httpclient.NonRetryableError); ok {
            log.Printf("Неповторяемая ошибка: %v", nonRetryableErr.Err)
            return
        }
        
        // Другие ошибки
        log.Printf("Общая ошибка: %v", err)
        return
    }
    defer resp.Body.Close()
    
    // Проверка статус кода
    if resp.StatusCode >= 400 {
        log.Printf("HTTP ошибка: %d", resp.StatusCode)
        return
    }
}
```

## Monitoring и Observability

### Включение tracing

```go
func createTracedClient() *httpclient.Client {
    config := httpclient.Config{
        TracingEnabled: true,
        Timeout:        15 * time.Second,
    }
    
    return httpclient.New(config, "traced-service")
}
```

### Метрики автоматически собираются

Пакет автоматически собирает метрики Prometheus:
- Количество запросов
- Латентность
- Ошибки и retry попытки
- Размеры запросов/ответов
- Активные соединения

Никаких дополнительных настроек не требуется!

## Распространенные паттерны

### Клиент для микросервиса

```go
type UserService struct {
    client *httpclient.Client
}

func NewUserService() *UserService {
    config := httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,
            BaseDelay:   50 * time.Millisecond,
            MaxDelay:    1 * time.Second,
        },
        TracingEnabled: true,
    }
    
    return &UserService{
        client: httpclient.New(config, "user-service"),
    }
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    resp, err := s.client.Get(ctx, fmt.Sprintf("/users/%s", id))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }
    
    return &user, nil
}

func (s *UserService) Close() error {
    return s.client.Close()
}
```

### Внешний API клиент

```go
func createExternalAPIClient() *httpclient.Client {
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
        
        // Пользовательский transport при необходимости
        Transport: &http.Transport{
            MaxIdleConns:       100,
            IdleConnTimeout:    90 * time.Second,
            DisableCompression: false,
        },
    }
    
    return httpclient.New(config, "external-api")
}
```

## Следующие шаги

После освоения базового использования изучите:

- [Конфигурация](configuration.md) - Детальные настройки клиента
- [Метрики](metrics.md) - Monitoring и alerting
- [Тестирование](testing.md) - Mock utilities и тестовые сервера
- [Лучшие практики](best-practices.md) - Рекомендации для продакшена

## Частые вопросы

**Q: Как настроить custom headers для всех запросов?**

A: Используйте пользовательский Transport или добавляйте headers к каждому запросу через http.Request.

**Q: Можно ли отключить retry для конкретного запроса?**

A: Установите MaxAttempts = 1 в конфигурации или создайте отдельный клиент.

**Q: Как логировать все HTTP запросы?**

A: Включите TracingEnabled: true и настройте OpenTelemetry logging экспорт.

Больше ответов в разделе [Troubleshooting](troubleshooting.md).