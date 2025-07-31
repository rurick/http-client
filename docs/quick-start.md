# Быстрый старт

## Установка

```bash
go get gitlab.citydrive.tech/back-end/go/pkg/http-client
```

## Базовое использование

### Создание простого клиента

```go
package main

import (
    "fmt"
    "log"
    
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    // Создание нового клиента с настройками по умолчанию
    client, err := httpclient.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    
    // Выполнение простого GET запроса
    resp, err := client.Get("https://api.example.com/data")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    
    fmt.Printf("Статус: %s\n", resp.Status)
}
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

### JSON запросы

```go
// GET JSON
var result map[string]interface{}
err := client.GetJSON(context.Background(), "https://api.example.com/data", &result)

// POST JSON
data := map[string]string{"key": "value"}
var response map[string]interface{}
err := client.PostJSON(context.Background(), "https://api.example.com/submit", data, &response)
```

## Основные HTTP методы

```go
// GET запрос
resp, err := client.Get("https://api.example.com/users")

// POST запрос
resp, err := client.Post("https://api.example.com/users", "application/json", body)

// PUT запрос
resp, err := client.Put("https://api.example.com/users/1", "application/json", body)

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