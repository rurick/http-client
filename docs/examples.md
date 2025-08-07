# Примеры использования

Практические примеры для разных сценариев использования HTTP клиента.

## Базовые примеры

### Простой GET запрос

```go
package main

import (
  "fmt"
  "log"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
  client, err := httpclient.NewClient()
  if err != nil {
    log.Fatal(err)
  }

  resp, err := client.Get("https://api.github.com/users/octocat")
  if err != nil {
    log.Fatal(err)
  }
  defer resp.Body.Close()

  fmt.Printf("Статус: %s\n", resp.Status)
}
```

### JSON API клиент

```go
package main

import (
  "context"
  "fmt"
  "log"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type User struct {
  ID    int    `json:"id"`
  Name  string `json:"name"`
  Email string `json:"email"`
}

func main() {
  client, err := httpclient.NewClient(
    httpclient.WithTimeout(10*time.Second),
    httpclient.WithRetryMax(3),
  )
  if err != nil {
    log.Fatal(err)
  }

  // GET JSON
  var user User
  err = client.GetJSON(context.Background(), "https://api.example.com/user/1", &user)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Printf("Пользователь: %+v\n", user)

  // POST JSON
  newUser := User{Name: "John Doe", Email: "john@example.com"}
  var createdUser User
  err = client.PostJSON(context.Background(), "https://api.example.com/users", newUser, &createdUser)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Printf("Создан пользователь: %+v\n", createdUser)
}
```

## Продвинутые примеры

### Клиент с полной конфигурацией

```go
package main

import (
  "context"
  "log"
  "time"
  "go.uber.org/zap"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
  logger, _ := zap.NewProduction()

  client, err := httpclient.NewClient(
    // Базовые настройки
    httpclient.WithTimeout(30*time.Second),
    httpclient.WithMaxIdleConns(100),
    httpclient.WithMaxConnsPerHost(20),

    // Стратегия повтора
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
      3, 200*time.Millisecond, 5*time.Second)),

    // Circuit breaker
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(5, 10*time.Second, 3)),

    // Middleware
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware("your-api-token")),
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(100, 150)),

    // Метрики и трейсинг
    httpclient.WithMetrics(true),
    httpclient.WithOpenTelemetry(true),
  )
  if err != nil {
    log.Fatal(err)
  }

  // Использование клиента
  resp, err := client.Get("https://api.example.com/data")
  if err != nil {
    log.Printf("Ошибка запроса: %v", err)
    return
  }
  defer resp.Body.Close()

  // Получение метрик
  metrics := client.GetMetrics()
  log.Printf("Выполнено запросов: %d, успешных: %d",
    metrics.TotalRequests, metrics.SuccessfulRequests)
}
```

### Микросервисный клиент

```go
package main

import (
  "context"
  "fmt"
  "time"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type OrderService struct {
  client httpclient.ExtendedHTTPClient
}

func NewOrderService(baseURL string) *OrderService {
  client, _ := httpclient.NewClient(
    httpclient.WithTimeout(5*time.Second), // Быстрые таймауты для внутренних сервисов
    httpclient.WithRetryMax(5),
    httpclient.WithRetryStrategy(httpclient.NewSmartRetryStrategy(
      5, 50*time.Millisecond, 2*time.Second)),
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(10, 5*time.Second, 5)),
    httpclient.WithMetrics(true),
  )

  return &OrderService{client: client}
}

type Order struct {
  ID       string  `json:"id"`
  UserID   string  `json:"user_id"`
  Amount   float64 `json:"amount"`
  Status   string  `json:"status"`
}

func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
  var order Order
  url := fmt.Sprintf("http://order-service/orders/%s", orderID)

  err := s.client.GetJSON(ctx, url, &order)
  if err != nil {
    return nil, fmt.Errorf("failed to get order %s: %w", orderID, err)
  }

  return &order, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, order *Order) (*Order, error) {
  var createdOrder Order

  err := s.client.PostJSON(ctx, "http://order-service/orders", order, &createdOrder)
  if err != nil {
    return nil, fmt.Errorf("failed to create order: %w", err)
  }

  return &createdOrder, nil
}

func (s *OrderService) GetMetrics() {
  metrics := s.client.GetMetrics()
  fmt.Printf("Order Service Metrics:\n")
  fmt.Printf("  Запросов: %d\n", metrics.TotalRequests)
  fmt.Printf("  Успешных: %d\n", metrics.SuccessfulRequests)
  fmt.Printf("  Средняя задержка: %v\n", metrics.AverageLatency)
}
```

### CLI утилита с прогресс баром

```go
package main

import (
  "context"
  "fmt"
  "log"
  "sync"
  "time"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
  client, err := httpclient.NewClient(
    httpclient.WithTimeout(60*time.Second),
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
      3, 1*time.Second, 30*time.Second)),
    httpclient.WithMetrics(true),
  )
  if err != nil {
    log.Fatal(err)
  }

  urls := []string{
    "https://api.example.com/data/1",
    "https://api.example.com/data/2",
    "https://api.example.com/data/3",
    "https://api.example.com/data/4",
    "https://api.example.com/data/5",
  }

  // Параллельная обработка URL
  var wg sync.WaitGroup
  results := make(chan string, len(urls))

  for i, url := range urls {
    wg.Add(1)
    go func(index int, u string) {
      defer wg.Done()

      resp, err := client.Get(u)
      if err != nil {
        results <- fmt.Sprintf("❌ URL %d failed: %v", index+1, err)
        return
      }
      defer resp.Body.Close()

      results <- fmt.Sprintf("✅ URL %d: %s", index+1, resp.Status)
    }(i, url)
  }

  // Ожидание завершения и вывод результатов
  go func() {
    wg.Wait()
    close(results)
  }()

  for result := range results {
    fmt.Println(result)
  }

  // Финальная статистика
  metrics := client.GetMetrics()
  fmt.Printf("\n📊 Статистика:\n")
  fmt.Printf("  Всего запросов: %d\n", metrics.TotalRequests)
  fmt.Printf("  Успешных: %d\n", metrics.SuccessfulRequests)
  fmt.Printf("  Неудачных: %d\n", metrics.FailedRequests)
  fmt.Printf("  Средняя задержка: %v\n", metrics.AverageLatency)
}
```

## Обработка ошибок

### Типичные паттерны обработки ошибок

```go
package main

import (
  "context"
  "errors"
  "fmt"
  "net/http"
  "time"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func handleAPICall(client httpclient.ExtendedHTTPClient, url string) error {
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()

  resp, err := client.Get(url)
  if err != nil {
    // Проверяем тип ошибки
    if errors.Is(err, context.DeadlineExceeded) {
      return fmt.Errorf("запрос превысил время ожидания: %w", err)
    }

    // Другие network ошибки
    return fmt.Errorf("сетевая ошибка: %w", err)
  }
  defer resp.Body.Close()

  // Обработка HTTP статус кодов
  switch resp.StatusCode {
  case http.StatusOK:
    fmt.Println("Успешный запрос")
    return nil
  case http.StatusTooManyRequests:
    return fmt.Errorf("превышен лимит запросов, повторите позже")
  case http.StatusInternalServerError:
    return fmt.Errorf("внутренняя ошибка сервера")
  case http.StatusServiceUnavailable:
    return fmt.Errorf("сервис временно недоступен")
  default:
    return fmt.Errorf("неожиданный статус код: %d", resp.StatusCode)
  }
}
```

### Graceful degradation

```go
func getDataWithFallback(client httpclient.ExtendedHTTPClient, primaryURL, fallbackURL string) ([]byte, error) {
// Пытаемся основной URL
resp, err := client.Get(primaryURL)
if err == nil && resp.StatusCode == 200 {
defer resp.Body.Close()
return io.ReadAll(resp.Body)
}

if resp != nil {
resp.Body.Close()
}

fmt.Println("Основной сервис недоступен, используем fallback...")

// Fallback URL
resp, err = client.Get(fallbackURL)
if err != nil {
return nil, fmt.Errorf("все сервисы недоступны: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != 200 {
return nil, fmt.Errorf("fallback сервис вернул статус: %d", resp.StatusCode)
}

return io.ReadAll(resp.Body)
}
```

## Тестирование

### Unit тесты с mock клиентом

```go
package main

import (
  "context"
  "net/http"
  "strings"
  "testing"

  "github.com/stretchr/testify/assert"
  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
  "gitlab.citydrive.tech/back-end/go/pkg/http-client/mock"
)

func TestAPIClient(t *testing.T) {
  // Создание mock клиента
  mockClient := mock.NewMockHTTPClient()

  // Настройка ожидаемых вызовов
  mockClient.On("GetJSON",
    mock.Anything,
    "https://api.example.com/user/123",
    mock.Anything).Return(nil).Run(func(args mock.Arguments) {
    user := args.Get(2).(*User)
    user.ID = 123
    user.Name = "Test User"
    user.Email = "test@example.com"
  })

  // Тестирование
  service := &UserService{client: mockClient}
  user, err := service.GetUser(context.Background(), "123")

  assert.NoError(t, err)
  assert.Equal(t, 123, user.ID)
  assert.Equal(t, "Test User", user.Name)

  // Проверка что методы были вызваны
  mockClient.AssertExpectations(t)
}

func TestAPIClientWithRealHTTP(t *testing.T) {
  // Интеграционный тест с реальным HTTP сервером
  client, err := httpclient.NewClient(
    httpclient.WithTimeout(5*time.Second),
    httpclient.WithRetryMax(0), // Отключить повторы в тестах
  )
  assert.NoError(t, err)

  // Тест с httpbin.org
  resp, err := client.Get("https://httpbin.org/status/200")
  assert.NoError(t, err)
  assert.Equal(t, 200, resp.StatusCode)
  resp.Body.Close()
}
```

## Мониторинг и метрики

### Экспорт метрик в JSON

```go
package main

import (
  "encoding/json"
  "fmt"
  "net/http"
  "time"

  httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

type MetricsServer struct {
  client httpclient.ExtendedHTTPClient
}

func (s *MetricsServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
  metrics := s.client.GetMetrics()

  data := map[string]interface{}{
    "timestamp":           time.Now().Unix(),
    "total_requests":      metrics.TotalRequests,
    "successful_requests": metrics.SuccessfulRequests,
    "failed_requests":     metrics.FailedRequests,
    "average_latency_ms":  metrics.AverageLatency.Milliseconds(),
    "total_request_size":  metrics.TotalRequestSize,
    "total_response_size": metrics.TotalResponseSize,
    "status_codes":        metrics.GetStatusCodes(),
  }

  w.Header().Set("Content-Type", "application/json")
  json.NewEncoder(w).Encode(data)
}

func main() {
  client, _ := httpclient.NewClient(httpclient.WithMetrics(true))

  server := &MetricsServer{client: client}

  http.HandleFunc("/metrics", server.metricsHandler)

  fmt.Println("Metrics server running on :8080/metrics")
  log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Периодический мониторинг

```go
func startMetricsMonitoring(client httpclient.ExtendedHTTPClient) {
ticker := time.NewTicker(30 * time.Second)
go func() {
for range ticker.C {
metrics := client.GetMetrics()

// Вычисляем key metrics
var successRate float64
if metrics.TotalRequests > 0 {
successRate = float64(metrics.SuccessfulRequests) / float64(metrics.TotalRequests) * 100
}

log.Printf("📊 HTTP Client Metrics:")
log.Printf("  Requests: %d total, %d successful (%.1f%%)",
metrics.TotalRequests, metrics.SuccessfulRequests, successRate)
log.Printf("  Avg Latency: %v", metrics.AverageLatency)
log.Printf("  Data: %d bytes sent, %d bytes received",
metrics.TotalRequestSize, metrics.TotalResponseSize)

// Алерты
if successRate < 95 && metrics.TotalRequests > 10 {
log.Printf("🚨 ALERT: Success rate below 95%%: %.1f%%", successRate)
}

if metrics.AverageLatency > 5*time.Second {
log.Printf("🚨 ALERT: High latency: %v", metrics.AverageLatency)
}
}
}()
}
```

## Интерактивное тестирование

### 🚀 Test Server - Полнофункциональный тестовый сервер
**Файл:** `examples/test_server/main.go`

Интерактивный HTTP сервер для тестирования всех возможностей клиента:

```bash
# Запуск тестового сервера
cd examples/test_server
go run main.go

# Откройте http://localhost:8080 в браузере
```

**Возможности тестового сервера:**

- **Веб-интерфейс** - HTML страница для отправки GET/POST запросов
- **API Endpoints:**
  - `GET/POST /api/test` - Основные тестовые запросы
  - `GET /api/echo` - Возвращает параметры запроса
  - `GET /api/status` - Статус сервера и метрики клиента
  - `GET /metrics` - Метрики в формате Prometheus
- **OpenTelemetry Prometheus метрики** - Histogram латентности, counter запросов, gauge uptime
- **Graceful Shutdown** - Корректное завершение работы
- **Интерактивное тестирование** - Форма в браузере для отправки запросов

**Пример использования через веб-интерфейс:**
1. Откройте `http://localhost:8080`
2. Выберите HTTP метод (GET/POST)
3. Укажите endpoint (`/api/test`)
4. Введите сообщение и JSON данные
5. Нажмите "Отправить запрос"

**Тестирование через curl:**
```bash
# GET запрос
curl "http://localhost:8080/api/test?message=hello"

# POST запрос
curl -X POST http://localhost:8080/api/test \
  -H "Content-Type: application/json" \
  -d '{"message": "test", "data": {"key": "value"}}'

# Метрики Prometheus
curl http://localhost:8080/metrics
```

**Пример ответа сервера:**
```json
{
  "status": "success",
  "message": "POST запрос получен: test message",
  "timestamp": "2025-08-01T15:30:45Z",
  "echo": {
    "key": "value"
  }
}
```

## См. также

- [Быстрый старт](quick-start.md) - Основы использования
- [Конфигурация](configuration.md) - Настройка клиента
- [Тестирование](testing.md) - Утилиты для тестирования
- [Метрики](metrics.md) - Сбор и мониторинг метрик
- [Test Server README](../examples/test_server/README.md) - Подробное описание тестового сервера