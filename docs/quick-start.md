# Быстрый старт

## Установка

```bash
go get gitlab.citydrive.tech/back-end/go/pkg/http-client
```

## Базовое использование

### Создание простого клиента (РЕКОМЕНДУЕМЫЙ способ)

**Всегда используйте контекстные методы для лучшего контроля!**

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    // Создание нового клиента с настройками по умолчанию
    client, err := httpclient.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    
    // Создание контекста с таймаутом (ОБЯЗАТЕЛЬНО!)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Выполнение GET запроса с контекстом (РЕКОМЕНДУЕТСЯ)
    resp, err := client.GetCtx(ctx, "https://api.example.com/data")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    fmt.Printf("Статус: %s\n", resp.Status)
}
```

### Устаревший способ (НЕ рекомендуется)

```go
// Старый способ без контекста - используйте только для legacy кода
resp, err := client.Get("https://api.example.com/data")
```

### Клиент с пользовательской конфигурацией

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(10*time.Second),
    httpclient.WithRetryMax(5),
    httpclient.WithMetrics(true),
    httpclient.WithCircuitBreaker(httpclient.NewSimpleCircuitBreaker()),
)
```

### JSON запросы (с контекстом)

```go
// Создание контекста с таймаутом
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

// GET JSON
var result map[string]any
err := client.GetJSON(ctx, "https://api.example.com/data", &result)

// POST JSON
data := map[string]string{"key": "value"}
var response map[string]any
err := client.PostJSON(ctx, "https://api.example.com/submit", data, &response)
```

## Основные HTTP методы (с контекстом - РЕКОМЕНДУЕТСЯ)

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// GET запрос
resp, err := client.GetCtx(ctx, "https://api.example.com/users")

// POST запрос
resp, err := client.PostCtx(ctx, "https://api.example.com/users", "application/json", body)

// POST форма
formData := map[string][]string{
    "name": {"John Doe"},
    "email": {"john@example.com"},
}
resp, err := client.PostFormCtx(ctx, "https://api.example.com/users", formData)

// HEAD запрос
resp, err := client.HeadCtx(ctx, "https://api.example.com/users/1")
```

### Устаревшие методы (используйте только для legacy кода)

```go
// Старые методы без контекста - НЕ рекомендуются
resp, err := client.Get("https://api.example.com/users")
resp, err := client.Post("https://api.example.com/users", "application/json", body)

// DELETE запрос
resp, err := client.Delete("https://api.example.com/users/1")
```

## Обработка ошибок

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // Включить повторы
)
if err != nil {
    log.Fatal("Ошибка создания клиента:", err)
}

resp, err := client.Get("https://api.example.com/data")
if err != nil {
    log.Printf("Ошибка запроса: %v", err)
    return
}
defer resp.Body.Close()

if resp.StatusCode >= 400 {
    log.Printf("HTTP ошибка: %s", resp.Status)
    return
}
```

## Следующие шаги

- [Настройка стратегий повтора](retry-strategies.md)
- [Конфигурация клиента](configuration.md)
- [Примеры использования](examples.md)
- [Middleware система](middleware.md)