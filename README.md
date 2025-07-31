# HTTP Клиент

Комплексная библиотека HTTP клиента для Go с встроенными механизмами повтора, сбором метрик, паттерном автоматического выключателя, поддержкой промежуточного ПО и интеграцией с OpenTelemetry.

## Оглавление

- [Возможности](#возможности)
- [Установка](#установка)
- [Быстрый старт](#быстрый-старт)
- [Опции конфигурации](#опции-конфигурации)
  - [Стратегии повтора](#стратегии-повтора)
  - [Дополнительные опции конфигурации](#дополнительные-опции-конфигурации)
    - [Настройка соединений](#настройка-соединений)
    - [Настройка времени ожидания повторов](#настройка-времени-ожидания-повторов)
    - [Пользовательские HTTP клиенты](#пользовательские-http-клиенты)
    - [Управление функциями](#управление-функциями)
  - [Автоматический выключатель (Circuit Breaker)](#автоматический-выключатель-circuit-breaker)
    - [Принцип работы](#принцип-работы)
    - [Что считается неудачей](#что-считается-неудачей)  
    - [Настройки по умолчанию](#настройки-по-умолчанию)
    - [Примеры использования](#примеры-использования-1)
    - [Преимущества](#преимущества)
  - [Промежуточное ПО](#промежуточное-по)
    - [Аутентификация](#аутентификация)
    - [Ограничение скорости (Rate Limiting)](#ограничение-скорости-rate-limiting)
    - [Логирование](#логирование)
    - [Таймауты](#таймауты)
    - [Пользовательский User-Agent](#пользовательский-user-agent)
    - [Комбинирование middleware](#комбинирование-middleware)
  - [Трейсинг и распределенная трассировка](#трейсинг-и-распределенная-трассировка)
    - [Автоматическое создание spans](#автоматическое-создание-spans)
    - [Что отслеживается в трейсах](#что-отслеживается-в-трейсах)
    - [Интеграция с существующими трейсами](#интеграция-с-существующими-трейсами)
    - [Распределенная трассировка](#распределенная-трассировка)
    - [Преимущества трейсинга](#преимущества-трейсинга)
  - [Метрики и мониторинг](#метрики-и-мониторинг)
    - [Встроенные метрики](#встроенные-метрики)
    - [OpenTelemetry метрики](#opentelemetry-метрики)
    - [Когда использовать встроенные метрики](#когда-использовать-встроенные-метрики)
    - [Когда использовать OpenTelemetry](#когда-использовать-opentelemetry)
- [Потоковые запросы](#потоковые-запросы)
- [API Reference](#api-reference)
  - [HTTP методы](#http-методы)
  - [JSON методы](#json-методы)
  - [XML методы](#xml-методы)
- [Примеры](#примеры)
- [Тестирование](#тестирование)
- [GitLab CI/CD](#gitlab-cicd)
- [Лицензия](#лицензия)

## Возможности

- **Механизмы повтора**: Экспоненциальная задержка, фиксированная задержка и пользовательские стратегии повтора
- **Автоматический выключатель**: Предотвращает каскадные сбои с настраиваемыми порогами
- **Система промежуточного ПО**: Расширяемая цепочка middleware для обработки запросов/ответов
- **Сбор метрик**: Встроенные метрики с интеграцией OpenTelemetry
- **Распределенное трассирование**: Поддержка трассирования OpenTelemetry
- **HTTP методы**: Стандартные HTTP методы плюс специализированные JSON/XML методы
- **Поддержка потоков**: Потоковая передача больших запросов и ответов
- **Пул соединений**: Настраиваемый пул соединений
- **Поддержка заглушек**: Комплексные утилиты для тестирования и объекты-заглушки

## Установка

```bash
go get gitlab.citydrive.tech/back-end/go/pkg/http-client
```

## Быстрый старт

### Базовое использование

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

## Опции конфигурации

### Стратегии повтора

По умолчанию клиент работает **без ретраев**. Чтобы включить повторы, используйте соответствующие опции:

```go
// Включение экспоненциальной задержки
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // Включить повторы (максимум 3 попытки)
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(
        3,                          // максимальное количество попыток
        100*time.Millisecond,       // базовая задержка
        5*time.Second,              // максимальная задержка
    )),
)

// Фиксированная задержка
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // Включить повторы
    httpclient.WithRetryStrategy(httpclient.NewFixedDelayStrategy(
        3,                          // максимальное количество попыток
        1*time.Second,              // задержка между попытками
    )),
)
```

### Дополнительные опции конфигурации

#### Настройка соединений

```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),           // Таймаут запросов
    httpclient.WithMaxIdleConns(100),                 // Максимум неактивных соединений
    httpclient.WithMaxConnsPerHost(10),               // Максимум соединений на хост
)
```

#### Настройка времени ожидания повторов

```go
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),                       // Максимум попыток
    httpclient.WithRetryWait(1*time.Second, 10*time.Second), // Мин/макс время ожидания
)
```

#### Пользовательские HTTP клиенты

```go
// Использование собственного http.Client
customHTTPClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:       50,
        IdleConnTimeout:    90 * time.Second,
    },
}

client, err := httpclient.NewClient(
    httpclient.WithHTTPClient(customHTTPClient),
)

// Использование собственного retryablehttp.Client
retryClient := retryablehttp.NewClient()
retryClient.RetryMax = 5

client, err := httpclient.NewClient(
    httpclient.WithRetryClient(retryClient),
)
```

#### Управление функциями

```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),                     // Включить метрики
    httpclient.WithTracing(false),                    // Отключить трейсинг
    httpclient.WithLogger(customLogger),              // Пользовательский логгер
)
```

### Автоматический выключатель (Circuit Breaker)

Circuit Breaker защищает ваше приложение от каскадных сбоев, автоматически блокируя запросы к неисправным сервисам.

#### Принцип работы

Circuit Breaker имеет **три состояния**:

1. **Closed (Закрыто)** - нормальная работа
   - Все запросы проходят через circuit breaker
   - Ведется подсчет неудачных запросов
   - При достижении порога неудач переходит в состояние Open

2. **Open (Открыто)** - защитный режим
   - **Блокирует ВСЕ запросы** без их отправки
   - Возвращает ошибку `ErrCircuitBreakerOpen`
   - Экономит ресурсы и предотвращает нагрузку на неисправный сервис
   - После истечения таймаута переходит в Half-Open

3. **Half-Open (Полуоткрыто)** - тестовый режим
   - Пропускает ограниченное количество тестовых запросов
   - При успехе возвращается в Closed
   - При неудаче немедленно переходит обратно в Open

#### Что считается неудачей

- Любая сетевая ошибка (таймаут, DNS, соединение)
- HTTP статус коды 5xx (500, 502, 503, 504)
- Пустой ответ (nil response)

#### Настройки по умолчанию

- **FailureThreshold: 5** - открывается после 5 неудачных запросов
- **SuccessThreshold: 3** - закрывается после 3 успешных запросов в полуоткрытом режиме
- **Timeout: 60 секунд** - время ожидания перед переходом в полуоткрытое состояние

#### Примеры использования

```go
// Простой автоматический выключатель с настройками по умолчанию
circuitBreaker := httpclient.NewSimpleCircuitBreaker()

// Автоматический выключатель с пользовательской конфигурацией
config := httpclient.CircuitBreakerConfig{
    FailureThreshold: 3,                // открыть после 3 сбоев
    SuccessThreshold: 2,                // закрыть после 2 успехов
    Timeout:          30*time.Second,   // тестировать через 30 секунд
    OnStateChange: func(from, to httpclient.CircuitBreakerState) {
        fmt.Printf("Circuit Breaker: %s -> %s\n", from, to)
    },
}
circuitBreaker := httpclient.NewCircuitBreakerWithConfig(config)

client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(circuitBreaker),
)

// Проверка состояния
fmt.Printf("Circuit Breaker состояние: %s\n", circuitBreaker.State())

// Сброс в закрытое состояние (для экстренных случаев)
circuitBreaker.Reset()
```

#### Преимущества

- **Быстрый отказ**: немедленно возвращает ошибку вместо ожидания таймаута
- **Защита ресурсов**: не тратит время и память на бесполезные запросы  
- **Автоматическое восстановление**: самостоятельно тестирует сервис и восстанавливает соединение
- **Мониторинг**: callback функции для отслеживания изменений состояния

### Промежуточное ПО

Библиотека предоставляет несколько встроенных middleware для расширения функциональности HTTP клиента.

#### Аутентификация

```go
// Bearer токен аутентификация
authMiddleware := httpclient.NewAuthMiddleware("Bearer", "ваш-токен")

// Basic аутентификация
basicAuthMiddleware := httpclient.NewBasicAuthMiddleware("username", "password")

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(authMiddleware),
)
```

#### Ограничение скорости (Rate Limiting)

Rate Limiter использует алгоритм **Token Bucket** для контроля частоты запросов:

```go
// Ограничить до 10 запросов в секунду
rateLimiter := httpclient.NewRateLimitMiddleware(10)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(rateLimiter),
)
```

**Принцип работы Token Bucket:**
- **Токены** генерируются с заданной скоростью (запросов в секунду)
- Каждый запрос "потребляет" один токен
- Если токенов нет - запрос ждет до появления токена
- Позволяет "всплески" трафика в пределах лимита

#### Логирование

```go
logger, _ := zap.NewDevelopment()
loggingMiddleware := httpclient.NewLoggingMiddleware(logger)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(loggingMiddleware),
)
```

#### Таймауты

```go
// Установить таймаут 5 секунд для каждого запроса
timeoutMiddleware := httpclient.NewTimeoutMiddleware(5 * time.Second)

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(timeoutMiddleware),
)
```

#### Пользовательский User-Agent

```go
userAgentMiddleware := httpclient.NewUserAgentMiddleware("MyApp/1.0")

client, err := httpclient.NewClient(
    httpclient.WithMiddleware(userAgentMiddleware),
)
```

#### Комбинирование middleware

```go
client, err := httpclient.NewClient(
    httpclient.WithMiddleware(
        httpclient.NewLoggingMiddleware(logger),
        httpclient.NewAuthMiddleware("Bearer", "token"),
        httpclient.NewRateLimitMiddleware(10), // 10 RPS
        httpclient.NewTimeoutMiddleware(30*time.Second),
    ),
)
```

### Трейсинг и распределенная трассировка

Библиотека автоматически создает **OpenTelemetry trace spans** для каждого HTTP запроса, обеспечивая полную видимость в распределенных системах.

#### Автоматическое создание spans

```go
import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// Создание клиента с трейсингом
otelCollector, err := httpclient.NewOTelMetricsCollector("my-http-client")
if err != nil {
    log.Fatal(err)
}

client, err := httpclient.NewClient(
    httpclient.WithMetricsCollector(otelCollector),
)
```

#### Что отслеживается в трейсах

Каждый HTTP запрос автоматически создает span с атрибутами:

- **http.method** - HTTP метод (GET, POST, PUT, DELETE)
- **http.url** - полный URL запроса
- **http.status_code** - код ответа (200, 404, 500)
- **Ошибки** - автоматическая запись ошибок в span
- **Время выполнения** - полная длительность запроса

#### Интеграция с существующими трейсами

```go
// Трейсинг работает с родительским контекстом
ctx, parentSpan := tracer.Start(context.Background(), "business_logic")
defer parentSpan.End()

// HTTP клиент создаст дочерний span
resp, err := client.GetWithContext(ctx, "https://api.example.com/users")
// Span автоматически связывается с родительским
```

#### Распределенная трассировка

```go
// Пример полной цепочки трейсов
func processUserData(ctx context.Context) error {
    // 1. Родительский span (business logic)
    ctx, span := tracer.Start(ctx, "process_user_data")
    defer span.End()
    
    // 2. HTTP client создаст дочерний span для каждого запроса
    userData, err := client.GetWithContext(ctx, "https://users-api.com/user/123")
    if err != nil {
        return err
    }
    
    profile, err := client.GetWithContext(ctx, "https://profiles-api.com/profile/123")
    if err != nil {
        return err
    }
    
    // 3. Все spans связаны в единую цепочку трассировки
    return processData(userData, profile)
}
```

#### Преимущества трейсинга

- **Полная видимость** - отслеживание всех HTTP запросов в системе
- **Производительность** - измерение времени каждого запроса
- **Отладка ошибок** - автоматическая запись ошибок и исключений
- **Dependency mapping** - визуализация связей между сервисами
- **Интеграция** - работа с Jaeger, Zipkin, и другими системами трассировки

### Метрики и мониторинг

Библиотека предоставляет два типа метрик: встроенные (для быстрого доступа) и OpenTelemetry (для внешних систем мониторинга).

#### Встроенные метрики

Встроенные метрики - это простые счетчики в памяти, которые позволяют быстро получить статистику использования клиента:

```go
// Включение встроенных метрик
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),
)

// Выполняем несколько запросов
client.Get("https://api.example.com/users")
client.Post("https://api.example.com/users", "application/json", bytes.NewReader(data))

// Получаем текущие метрики
metrics := client.GetMetrics()

// Основные метрики запросов
fmt.Printf("Всего запросов: %d\n", metrics.TotalRequests)
fmt.Printf("Успешные запросы: %d\n", metrics.SuccessfulReqs)  
fmt.Printf("Неудачные запросы: %d\n", metrics.FailedRequests)

// Метрики производительности
fmt.Printf("Средняя задержка: %v\n", metrics.AverageLatency)
fmt.Printf("Минимальная задержка: %v\n", metrics.MinLatency)
fmt.Printf("Максимальная задержка: %v\n", metrics.MaxLatency)

// Метрики повторов
fmt.Printf("Всего повторов: %d\n", metrics.TotalRetries)
fmt.Printf("Успешные повторы: %d\n", metrics.RetrySuccesses)
fmt.Printf("Неудачные повторы: %d\n", metrics.RetryFailures)

// Метрики размера данных
fmt.Printf("Общий размер запросов: %d байт\n", metrics.TotalRequestSize)
fmt.Printf("Общий размер ответов: %d байт\n", metrics.TotalResponseSize)

// Распределение по HTTP кодам
fmt.Println("Распределение по статус кодам:")
for code, count := range metrics.StatusCodes {
    fmt.Printf("  HTTP %d: %d запросов\n", code, count)
}

// Состояние автоматического выключателя
fmt.Printf("Состояние Circuit Breaker: %s\n", metrics.CircuitBreakerState)
fmt.Printf("Количество срабатываний: %d\n", metrics.CircuitBreakerTrips)
```

#### Когда использовать встроенные метрики

**✅ Используйте встроенные метрики когда:**

1. **Отладка во время разработки**
```go
if metrics.FailedRequests > 0 {
    fmt.Printf("Обнаружены ошибки: %d из %d запросов\n", 
        metrics.FailedRequests, metrics.TotalRequests)
}
```

2. **Принятие решений внутри приложения**
```go
// Переключение на резервный сервис при высокой частоте ошибок
errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
if errorRate > 0.5 {
    log.Warn("Высокая частота ошибок, переключаемся на backup сервис")
    switchToBackupService()
}
```

3. **Мониторинг производительности в реальном времени**
```go
if metrics.AverageLatency > 5*time.Second {
    log.Printf("ВНИМАНИЕ: Медленные запросы! Средняя задержка: %v", 
        metrics.AverageLatency)
}
```

4. **Простое логирование статистики**
```go
// Периодический вывод статистики
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        m := client.GetMetrics()
        log.Printf("Статистика: %d запросов, успешность: %.1f%%", 
            m.TotalRequests, 
            float64(m.SuccessfulReqs)/float64(m.TotalRequests)*100)
    }
}()
```

5. **Тестирование и бенчмарки**
```go
func TestClientPerformance(t *testing.T) {
    client, _ := httpclient.NewClient(
        httpclient.WithMetrics(true),
    )
    
    // Выполняем тесты...
    
    metrics := client.GetMetrics()
    if metrics.AverageLatency > 1*time.Second {
        t.Errorf("Слишком медленные запросы: %v", metrics.AverageLatency)
    }
}
```

**❌ НЕ используйте встроенные метрики когда:**

- Нужен долгосрочный мониторинг (метрики сбрасываются при перезапуске)
- Требуется интеграция с внешними системами мониторинга
- Необходимы сложные запросы и агрегации по метрикам
- Нужна персистентность метрик

#### OpenTelemetry метрики

Для продакшн-мониторинга используйте OpenTelemetry интеграцию:

```go
// OpenTelemetry метрики автоматически экспортируются в:
// - Prometheus
// - Grafana  
// - Jaeger
// - Другие OTEL-совместимые системы

// Метрики включаются автоматически при создании клиента
client, err := httpclient.NewClient()
```

OpenTelemetry предоставляет следующие метрики:
- `http_client_requests_total` - счетчик запросов с labels (method, url, status_code)
- `http_client_request_duration_seconds` - гистограмма времени выполнения
- `http_client_request_size_bytes` - размер запросов  
- `http_client_response_size_bytes` - размер ответов
- `http_client_retries_total` - счетчик повторов

## Потоковая передача

```go
req, err := http.NewRequest("GET", "https://api.example.com/stream", nil)
if err != nil {
    log.Fatal(err)
}

streamResp, err := client.Stream(context.Background(), req)
if err != nil {
    log.Fatal(err)
}
defer streamResp.Close()

// Чтение из потока
buffer := make([]byte, 4096)
for {
    n, err := streamResp.Body().Read(buffer)
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    
    // Обработка потоковых данных
    fmt.Print(string(buffer[:n]))
}
```

## Тестирование

Библиотека предоставляет комплексные утилиты для тестирования:

```go
import "gitlab.citydrive.tech/back-end/go/pkg/http-client/mock"

// Создание тестового сервера-заглушки
server := mock.NewTestServer()
defer server.Close()

// Настройка ответов-заглушек
server.On("GET", "/api/data").Return(200, map[string]string{"status": "ok"})

// Использование в тестах
client, _ := httpclient.NewClient()
resp, err := client.Get(server.URL + "/api/data")
// ... проверки в тестах
```

## Примеры

Смотрите директорию `examples/` для полных рабочих примеров:

- `basic_usage.go` - Базовое использование и конфигурация клиента
- `circuit_breaker_example.go` - Паттерны автоматического выключателя и мониторинг
- `middleware_example.go` - Реализация пользовательского middleware
- `metrics_example.go` - Практические примеры использования встроенных метрик для мониторинга, отладки и принятия решений

## Производительность

Клиент разработан для высокой производительности с:

- Пулом соединений и их повторным использованием
- Эффективными механизмами повтора
- Низкоуровневым сбором метрик
- Минимальными выделениями памяти

## Лицензия

Этот проект лицензирован под лицензией MIT.