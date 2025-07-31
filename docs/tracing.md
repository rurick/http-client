# Трейсинг и распределенная трассировка

Библиотека автоматически создает OpenTelemetry spans для всех HTTP запросов, обеспечивая полную видимость в распределенных системах.

## Включение трейсинга

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/propagation"
)

// Настройка трейсинга
func setupTracing() {
    // Создание экспортера (пример: stdout)
    exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
    if err != nil {
        log.Fatal(err)
    }
    
    // Создание провайдера трейсов
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("my-service"),
            semconv.ServiceVersionKey.String("1.0.0"),
        )),
    )
    
    // Установка глобального провайдера
    otel.SetTracerProvider(tp)
    
    // Настройка propagation для distributed tracing
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))
}

func main() {
    setupTracing()
    
    // Создание клиента
    client, err := httpclient.NewClient(
        httpclient.WithOpenTelemetry(true), // Включить трейсинг
    )
}
```

## Автоматическое создание spans

Библиотека автоматически создает spans для:

### HTTP запросов
```go
// Автоматически создается span "HTTP GET"
resp, err := client.Get("https://api.example.com/users")
```

**Span содержит:**
- Имя: `"HTTP {METHOD}"`
- Атрибуты: URL, метод, статус код, размеры запроса/ответа
- Время выполнения
- Статус (success/error)

### Повторов
```go
// Создается дополнительный span для каждого повтора
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithOpenTelemetry(true),
)

// Создаст spans: "HTTP GET", "HTTP GET (retry 1)", "HTTP GET (retry 2)"
resp, err := client.Get("https://unreliable-api.com/data")
```

## Что отслеживается в трейсах

### Атрибуты HTTP запросов
- `http.method` - HTTP метод (GET, POST, etc.)
- `http.url` - Полный URL запроса
- `http.status_code` - Статус код ответа
- `http.request.size` - Размер тела запроса в байтах
- `http.response.size` - Размер тела ответа в байтах
- `http.user_agent` - User-Agent заголовок

### Атрибуты повторов
- `retry.attempt` - Номер попытки повтора
- `retry.strategy` - Тип стратегии повтора
- `retry.delay` - Задержка перед повтором

### Атрибуты Circuit Breaker
- `circuit_breaker.state` - Состояние circuit breaker
- `circuit_breaker.failures` - Количество неудач

## Интеграция с существующими трейсами

Библиотека автоматически интегрируется с существующим trace context:

```go
func handleRequest(ctx context.Context) {
    // Создаем span для всей операции
    ctx, span := otel.Tracer("my-service").Start(ctx, "handle_user_request")
    defer span.End()
    
    // HTTP запрос автоматически станет child span
    resp, err := client.GetJSON(ctx, "https://api.example.com/user/123", &user)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return
    }
    
    // Дополнительная обработка...
    span.SetStatus(codes.Ok, "success")
}
```

## Распределенная трассировка

Библиотека автоматически пропагирует trace context в исходящие запросы:

```go
// Service A
func callServiceB(ctx context.Context) {
    // Trace context автоматически добавляется в заголовки
    resp, err := client.Get("http://service-b/api/data")
}

// Service B получит заголовки:
// traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
// tracestate: rojo=00f067aa0ba902b7,congo=t61rcWkgMzE
```

## Настройка экспорта в Jaeger

```go
import (
    "go.opentelemetry.io/otel/exporters/jaeger"
)

func setupJaegerTracing() {
    // Jaeger exporter
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
    ))
    if err != nil {
        log.Fatal(err)
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("http-client"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})
}
```

## Настройка экспорта в Zipkin

```go
import (
    "go.opentelemetry.io/otel/exporters/zipkin"
)

func setupZipkinTracing() {
    exporter, err := zipkin.New("http://localhost:9411/api/v2/spans")
    if err != nil {
        log.Fatal(err)
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("http-client"),
        )),
    )
    
    otel.SetTracerProvider(tp)
}
```

## Добавление пользовательских атрибутов

```go
func makeAPICall(ctx context.Context, userID string) {
    // Добавляем пользовательские атрибуты в текущий span
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String("user.id", userID),
        attribute.String("operation", "fetch_user_data"),
    )
    
    // HTTP запрос наследует эти атрибуты
    resp, err := client.GetJSON(ctx, fmt.Sprintf("/users/%s", userID), &user)
}
```

## Пример интеграции в микросервисах

### Service A (API Gateway)

```go
func (s *APIGateway) HandleUserRequest(w http.ResponseWriter, r *http.Request) {
    ctx, span := otel.Tracer("api-gateway").Start(r.Context(), "handle_user_request")
    defer span.End()
    
    userID := r.URL.Query().Get("user_id")
    span.SetAttributes(attribute.String("user.id", userID))
    
    // Вызов User Service - trace пропагируется автоматически
    user, err := s.userClient.GetUser(ctx, userID)
    if err != nil {
        span.RecordError(err)
        http.Error(w, err.Error(), 500)
        return
    }
    
    // Вызов Order Service
    orders, err := s.orderClient.GetUserOrders(ctx, userID)
    if err != nil {
        span.RecordError(err)
        // Не критическая ошибка, продолжаем
    }
    
    response := UserResponse{User: user, Orders: orders}
    json.NewEncoder(w).Encode(response)
    span.SetStatus(codes.Ok, "success")
}
```

### Service B (User Service)

```go
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    ctx, span := otel.Tracer("user-service").Start(ctx, "get_user")
    defer span.End()
    
    span.SetAttributes(attribute.String("user.id", userID))
    
    // Вызов внешнего API - trace пропагируется
    resp, err := s.httpClient.GetJSON(ctx, 
        fmt.Sprintf("https://external-api.com/users/%s", userID), 
        &user)
    
    return &user, err
}
```

## Преимущества трейсинга

### Визуализация запросов
- Полная карта вызовов между сервисами
- Время выполнения каждого компонента
- Выявление узких мест в производительности

### Debugging распределенных систем
- Трассировка ошибок через все сервисы
- Понимание последовательности операций
- Корреляция логов с trace ID

### Мониторинг производительности
- Латентность каждого сервиса
- Самые медленные операции
- Зависимости между сервисами

## Лучшие практики

### 1. Meaningful span names
```go
// Хорошо
span := tracer.Start(ctx, "fetch_user_profile")

// Плохо
span := tracer.Start(ctx, "http_request")
```

### 2. Добавляйте контекстные атрибуты
```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.String("operation.type", "read"),
    attribute.Int("batch.size", len(items)),
)
```

### 3. Обрабатывайте ошибки правильно
```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    return err
}
span.SetStatus(codes.Ok, "success")
```

### 4. Не забывайте закрывать spans
```go
defer span.End() // Всегда используйте defer
```

## Производительность

### Sampling
Настройте sampling для снижения накладных расходов:

```go
tp := trace.NewTracerProvider(
    trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
    trace.WithBatcher(exporter),
)
```

### Batch экспорт
Используйте batch экспорт для лучшей производительности:

```go
tp := trace.NewTracerProvider(
    trace.WithBatcher(exporter,
        trace.WithBatchTimeout(time.Second),
        trace.WithMaxExportBatchSize(512),
    ),
)
```

## См. также

- [Метрики](metrics.md) - Сбор метрик вместе с трейсингом
- [Middleware](middleware.md) - Логирование совместно с трейсингом
- [Примеры](examples.md) - Практические примеры с трейсингом
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)