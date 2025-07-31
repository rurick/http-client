# Метрики и мониторинг

Библиотека предоставляет два типа метрик: встроенные (без внешних зависимостей) и OpenTelemetry метрики для интеграции с системами мониторинга.

## Встроенные метрики

### Включение встроенных метрик

```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true), // Включить сбор встроенных метрик
)
```

### Получение метрик

```go
// Получить все метрики
metrics := client.GetMetrics()

fmt.Printf("Всего запросов: %d\n", metrics.TotalRequests)
fmt.Printf("Успешных запросов: %d\n", metrics.SuccessfulRequests)
fmt.Printf("Неудачных запросов: %d\n", metrics.FailedRequests)
fmt.Printf("Средняя задержка: %v\n", metrics.AverageLatency)
fmt.Printf("Общий размер запросов: %d байт\n", metrics.TotalRequestSize)
fmt.Printf("Общий размер ответов: %d байт\n", metrics.TotalResponseSize)

// Статистика по статус кодам
statusCodes := metrics.GetStatusCodes()
for code, count := range statusCodes {
    fmt.Printf("Статус %d: %d раз\n", code, count)
}
```

### Доступ к коллектору метрик

```go
// Получить прямой доступ к коллектору метрик
collector := client.GetMetricsCollector()
if collector != nil {
    metrics := collector.GetMetrics()
    // Работа с метриками...
}
```

## OpenTelemetry метрики

### Настройка экспорта метрик

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
    "go.opentelemetry.io/otel/metric/global"
    "go.opentelemetry.io/otel/sdk/metric"
)

// Настройка экспортера метрик (example: stdout)
exporter, err := stdoutmetric.New()
if err != nil {
    log.Fatal(err)
}

// Создание провайдера метрик
provider := metric.NewMeterProvider(
    metric.WithReader(metric.NewPeriodicReader(exporter)),
)

// Установка глобального провайдера
global.SetMeterProvider(provider)

// Создание клиента с OpenTelemetry
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),
    httpclient.WithOpenTelemetry(true), // Включить OpenTelemetry
)
```

### Экспорт в Prometheus

```go
import (
    "go.opentelemetry.io/otel/exporters/prometheus"
)

// Prometheus exporter
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}

provider := metric.NewMeterProvider(
    metric.WithReader(exporter),
)
global.SetMeterProvider(provider)
```

## Структура метрик

### ClientMetrics

```go
type ClientMetrics struct {
    TotalRequests       int64         // Общее количество запросов
    SuccessfulRequests  int64         // Успешные запросы (2xx)
    FailedRequests      int64         // Неудачные запросы (4xx, 5xx, errors)
    AverageLatency      time.Duration // Средняя задержка
    TotalRequestSize    int64         // Общий размер всех запросов в байтах
    TotalResponseSize   int64         // Общий размер всех ответов в байтах
    
    // Приватные поля для расчетов
    statusCodes         map[int]int64 // Счетчики по статус кодам
    totalLatency        time.Duration // Сумма всех задержек
    mu                  sync.RWMutex  // Мьютекс для потокебезопасности
}
```

### Методы ClientMetrics

```go
// Получить копию статус кодов (потокобезопасно)
statusCodes := metrics.GetStatusCodes()

// Reset всех метрик
metrics.Reset()
```

## Автоматически отслеживаемые метрики

### HTTP запросы
- Общее количество запросов
- Количество успешных запросов (статус 2xx)
- Количество неудачных запросов (статус 4xx/5xx + network errors)
- Время выполнения каждого запроса
- Размеры запросов и ответов

### Повторы
- Количество попыток повтора для каждого запроса
- Тип ошибки, вызвавшей повтор
- URL и HTTP метод запроса с повтором

### Circuit Breaker
- Изменения состояния circuit breaker
- Количество времени в каждом состоянии

## Практические примеры

### Простой мониторинг

```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),
)

// Выполняем запросы...
resp, err := client.Get("https://api.example.com/data")

// Периодически выводим статистику
ticker := time.NewTicker(10 * time.Second)
go func() {
    for range ticker.C {
        metrics := client.GetMetrics()
        log.Printf("Метрики: запросов=%d, успешных=%d, средняя задержка=%v",
            metrics.TotalRequests,
            metrics.SuccessfulRequests,
            metrics.AverageLatency)
    }
}()
```

### Мониторинг по статус кодам

```go
func printStatusCodeStats(client httpclient.ExtendedHTTPClient) {
    metrics := client.GetMetrics()
    statusCodes := metrics.GetStatusCodes()
    
    fmt.Println("Статистика по статус кодам:")
    for code, count := range statusCodes {
        fmt.Printf("  %d: %d запросов\n", code, count)
    }
    
    // Вычисляем success rate
    total := metrics.TotalRequests
    successful := metrics.SuccessfulRequests
    if total > 0 {
        successRate := float64(successful) / float64(total) * 100
        fmt.Printf("Success Rate: %.2f%%\n", successRate)
    }
}
```

### Алерты на основе метрик

```go
func checkMetricsForAlerts(client httpclient.ExtendedHTTPClient) {
    metrics := client.GetMetrics()
    
    // Алерт при высоком error rate
    if metrics.TotalRequests > 100 {
        errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
        if errorRate > 0.1 { // Более 10% ошибок
            log.Printf("ALERT: High error rate: %.2f%%", errorRate*100)
        }
    }
    
    // Алерт при высокой задержке
    if metrics.AverageLatency > 5*time.Second {
        log.Printf("ALERT: High latency: %v", metrics.AverageLatency)
    }
}
```

### Экспорт метрик в JSON

```go
func exportMetricsToJSON(client httpclient.ExtendedHTTPClient) ([]byte, error) {
    metrics := client.GetMetrics()
    
    data := map[string]interface{}{
        "total_requests":       metrics.TotalRequests,
        "successful_requests":  metrics.SuccessfulRequests,
        "failed_requests":      metrics.FailedRequests,
        "average_latency_ms":   metrics.AverageLatency.Milliseconds(),
        "total_request_size":   metrics.TotalRequestSize,
        "total_response_size":  metrics.TotalResponseSize,
        "status_codes":         metrics.GetStatusCodes(),
        "timestamp":            time.Now().Unix(),
    }
    
    return json.Marshal(data)
}
```

## Когда использовать встроенные метрики

✅ **Используйте встроенные метрики когда:**
- Нужна простая статистика без внешних зависимостей
- Разрабатываете CLI утилиты или простые приложения
- Хотите быстро добавить базовый мониторинг
- Нужны метрики только для логирования или debug

## Когда использовать OpenTelemetry

✅ **Используйте OpenTelemetry когда:**
- У вас есть система мониторинга (Prometheus, Grafana, etc.)
- Разрабатываете микросервисы или распределенные системы
- Нужна интеграция с существующей observability инфраструктурой
- Требуется экспорт метрик в различные системы мониторинга

## Различия между встроенными метриками и OpenTelemetry

| Аспект | Встроенные метрики | OpenTelemetry |
|--------|-------------------|---------------|
| **Зависимости** | Нет внешних зависимостей | Требует OpenTelemetry SDK |
| **Экспорт** | Только программный доступ | Множество экспортеров |
| **Производительность** | Минимальные накладные расходы | Некоторые накладные расходы |
| **Функциональность** | Базовые метрики | Полная observability платформа |
| **Интеграция** | Простая | Требует настройки |

## Лучшие практики

1. **Включайте метрики в продакшне** - для мониторинга и алертов
2. **Регулярно сбрасывайте метрики** - чтобы избежать переполнения памяти
3. **Мониторьте success rate** - ключевой показатель надежности
4. **Следите за latency** - для оптимизации производительности
5. **Настройте алерты** - на критические пороговые значения

## См. также

- [Трейсинг](tracing.md) - Распределенная трассировка запросов
- [Circuit Breaker](circuit-breaker.md) - Метрики состояния circuit breaker
- [Конфигурация](configuration.md) - Включение/выключение метрик