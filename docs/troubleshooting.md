# Troubleshooting

Руководство по решению частых проблем и диагностике работы HTTP клиента.

## Общие проблемы

### Клиент не выполняет запросы
**Симптомы:** Вызовы методов клиента не работают или падают сразу.

**Возможные причины:**
1. Клиент не был создан корректно
2. Конфигурация содержит ошибки
3. Клиент был закрыт

**Решение:**
```go
// ✅ Правильное создание
client := httpclient.New(httpclient.Config{}, "service-name")
defer client.Close() // Важно закрывать

// ❌ Неправильно - клиент уже закрыт
client.Close()
resp, err := client.Get(ctx, url) // Ошибка!
```

### Таймауты происходят слишком быстро
**Симптомы:** Все запросы завершаются с timeout ошибками.

**Диагностика:**
```go
// Проверьте текущую конфигурацию
config := client.GetConfig()
fmt.Printf("Timeout: %v\n", config.Timeout)
fmt.Printf("PerTryTimeout: %v\n", config.PerTryTimeout)
```

**Решение:**
```go
config := httpclient.Config{
    Timeout:       30 * time.Second,  // Увеличьте общий таймаут
    PerTryTimeout: 10 * time.Second,  // Увеличьте таймаут попытки
}
```

### Слишком много retry попыток
**Симптомы:** Запросы выполняются очень долго из-за множественных повторов.

**Диагностика:**
```go
// Проверьте метрики повторов
# PromQL
sum(rate(http_client_retries_total[5m])) by (host, reason)
```

**Решение:**
```go
config := httpclient.Config{
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,               // Уменьшите количество попыток
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    1 * time.Second, // Уменьшите максимальную задержку
    },
}
```

## Проблемы с retry

### Retry не работают для POST запросов
**Симптомы:** GET запросы повторяются, а POST - нет.

**Причина:** POST требует Idempotency-Key для безопасных повторов.

**Решение:**
```go
req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
req.Header.Set("Idempotency-Key", "unique-operation-id") // Обязательно!
req.Header.Set("Content-Type", "application/json")

resp, err := client.Do(req)
```

### Неподходящие ошибки повторяются
**Симптомы:** 4xx ошибки повторяются бесконечно.

**Диагностика:**
```go
// Проверьте классификацию ошибок
classification := httpclient.ClassifyError(err)
isRetryable := httpclient.IsRetryableError(err)
fmt.Printf("Ошибка: %s, Повторяемая: %t\n", classification, isRetryable)
```

**Объяснение:** Только определенные ошибки повторяются:
- 5xx статус коды
- 429 Too Many Requests
- Сетевые ошибки
- Таймауты

4xx ошибки (кроме 429) НЕ повторяются.

### Очень медленные retry
**Симптомы:** Между попытками слишком большие паузы.

**Диагностика:**
```go
// Проверьте настройки backoff
for attempt := 1; attempt <= 5; attempt++ {
    delay := httpclient.CalculateBackoffDelay(
        attempt, 100*time.Millisecond, 5*time.Second, 0.2)
    fmt.Printf("Попытка %d: задержка %v\n", attempt, delay)
}
```

## Проблемы с Circuit Breaker

### CB всегда «открыт», запросы не уходят
**Симптомы:** Клиент быстро возвращает ошибку, не выполняя запросы.

**Проверка:**
```go
cfg := client.GetConfig()
fmt.Printf("CB enabled: %t\n", cfg.CircuitBreakerEnable)
// Если включен — проверьте состояние
if cfg.CircuitBreaker != nil {
    fmt.Printf("CB state: %s\n", cfg.CircuitBreaker.State())
}
```

**Возможные причины и решения:**
1. Порог неудач слишком низкий — увеличьте `FailureThreshold`.
2. Сервис действительно нестабилен — уменьшите трафик/включите fallback, дождитесь восстановления.
3. Таймаут восстановления слишком большой — уменьшите `Timeout`.
4. Неверно классифицируются статусы — настройте `FailStatusCodes` в `CircuitBreakerConfig`.

### Почему при открытом CB нет retry?
**Ответ:** Ошибка `ErrCircuitBreakerOpen` не инициирует retry — это осознанно, чтобы не «молотить» по недоступному сервису.

### Как получить «последний неуспешный ответ» при открытом CB?
**Ответ:** В открытом состоянии возвращается клон последнего неуспешного ответа (если он был). Проверьте тело/заголовки как обычно:
```go
resp, err := client.Get(ctx, url)
if errors.Is(err, httpclient.ErrCircuitBreakerOpen) && resp != nil {
    body, _ := io.ReadAll(resp.Body)
    log.Printf("Последний ответ: %d, body=%s", resp.StatusCode, string(body))
}
```

### Как логировать переходы состояний CB?
**Ответ:** Используйте `OnStateChange` в `CircuitBreakerConfig`.
```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 3,
    SuccessThreshold: 1,
    Timeout:          10 * time.Second,
    OnStateChange: func(from, to httpclient.CircuitBreakerState) { log.Printf("CB: %s -> %s", from, to) },
})
```

### В какой момент применяется CB относительно retry?
**Ответ:** CB применяется на каждую попытку. Если CB «открыт» — попытка завершается сразу и retry не выполняется.

**Решение:**
```go
config := httpclient.Config{
    RetryConfig: httpclient.RetryConfig{
        BaseDelay: 50 * time.Millisecond,  // Уменьшите базовую задержку
        MaxDelay:  1 * time.Second,        // Уменьшите максимум
        Jitter:    0.1,                    // Уменьшите джиттер
    },
}
```

## Проблемы с метриками

### Метрики не появляются в Prometheus
**Симптомы:** Метрики `http_client_*` отсутствуют в Prometheus.

**Возможные причины:**
1. OpenTelemetry не настроен
2. Prometheus exporter не запущен
3. Неправильный meterName

**Диагностика:**
```go
// Проверьте настройку OpenTelemetry
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

func checkMetricsSetup() {
    exporter, err := prometheus.New()
    if err != nil {
        log.Fatal("Ошибка создания Prometheus exporter:", err)
    }
    
    provider := metric.NewMeterProvider(metric.WithReader(exporter))
    otel.SetMeterProvider(provider)
    
    // HTTP клиент теперь должен экспортировать метрики
}
```

### Неожиданные значения метрик
**Симптомы:** Метрики показывают неправильные значения.

**Проверка лейблов:**
```promql
# Убедитесь что фильтруете по правильному host
http_client_requests_total{host="api.example.com"}

# Проверьте все доступные лейблы
{__name__=~"http_client_.*"}
```

**Проверка временных интервалов:**
```promql
# Для rate всегда используйте интервалы >= 2 * scrape_interval
rate(http_client_requests_total[5m])  # Если scrape каждые 15s

# Не используйте слишком короткие интервалы
rate(http_client_requests_total[10s]) # ❌ Плохо
```

### Высокая кардинальность метрик
**Симптомы:** Слишком много уникальных комбинаций лейблов.

**Причина:** Динамические значения в лейблах (например, user_id в host).

**Решение:**
```go
// ❌ Плохо - создает много лейблов
client := httpclient.New(config, fmt.Sprintf("user-%d", userID))

// ✅ Хорошо - статичное имя
client := httpclient.New(config, "user-service")
```

## Проблемы с производительностью

### Высокая латентность
**Симптомы:** Запросы выполняются медленно.

**Диагностика:**
```promql
# Проверьте percentiles латентности
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Сравните с количеством retry
sum(rate(http_client_retries_total[5m])) by (host)
```

**Возможные причины:**
1. Много retry попыток
2. Медленный целевой сервис
3. Сетевые проблемы
4. Неоптимальный Transport

**Решение:**
```go
// Оптимизация Transport
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    DialTimeout:         5 * time.Second,
    KeepAlive:          30 * time.Second,
}

config := httpclient.Config{
    Transport: transport,
    // Уменьшите retry если проблема в целевом сервисе
    RetryConfig: httpclient.RetryConfig{MaxAttempts: 2},
}
```

### Много одновременных соединений
**Симптомы:** Высокие значения `http_client_inflight_requests`.

**Диагностика:**
```promql
# Текущая нагрузка
http_client_inflight_requests

# Пиковая нагрузка
max_over_time(http_client_inflight_requests[1h])
```

**Решение:**
```go
// Ограничьте конкурентность на уровне приложения
semaphore := make(chan struct{}, 10) // Максимум 10 одновременных запросов

func limitedRequest(client *httpclient.Client, url string) error {
    semaphore <- struct{}{}
    defer func() { <-semaphore }()
    
    resp, err := client.Get(context.Background(), url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### Утечки памяти
**Симптомы:** Постоянно растущее потребление памяти.

**Частые причины:**
1. Не закрываются response.Body
2. Не закрываются клиенты
3. Накопление горутин

**Решение:**
```go
// ✅ Всегда закрывайте response body
resp, err := client.Get(ctx, url)
if err != nil {
    return err
}
defer resp.Body.Close() // Обязательно!

// ✅ Всегда закрывайте клиенты
client := httpclient.New(config, "service")
defer client.Close() // Обязательно!

// ✅ Проверяйте горутины
go func() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        fmt.Printf("Горутин: %d\n", runtime.NumGoroutine())
    }
}()
```

## Проблемы с tracing

### Spans не создаются
**Симптомы:** OpenTelemetry traces не содержат HTTP spans.

**Проверка:**
```go
config := httpclient.Config{
    TracingEnabled: true, // Убедитесь что включено!
}

// Также убедитесь что OpenTelemetry настроен глобально
```

### Прерывистые traces
**Симптомы:** Некоторые запросы создают spans, другие - нет.

**Причина:** Контекст без trace информации.

**Решение:**
```go
// ✅ Передавайте контекст с trace
ctx, span := tracer.Start(context.Background(), "business-operation")
defer span.End()

resp, err := client.Get(ctx, url) // Использует контекст с trace
```

### Spans содержат мало информации
**Симптомы:** Spans есть, но без полезных атрибутов.

**Объяснение:** HTTP клиент автоматически добавляет:
- URL и HTTP метод
- Статус код ответа
- Информацию об ошибках
- Номер попытки при retry

Дополнительную информацию добавляйте на уровне приложения.

## Отладка

### Включение debug логирования
```go
import (
    "log"
    "os"
)

func enableDebugLogging() {
    // Для OpenTelemetry
    os.Setenv("OTEL_LOG_LEVEL", "debug")
    
    // Для HTTP транспорта (если нужно)
    os.Setenv("GODEBUG", "http2debug=1")
}
```

### Мониторинг в реальном времени
```go
func monitorClient(client *httpclient.Client) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            config := client.GetConfig()
            log.Printf("Клиент активен. Timeout: %v, MaxAttempts: %d",
                config.Timeout, config.RetryConfig.MaxAttempts)
        }
    }()
}
```

### Проверка сетевой связности
```go
func diagnoseNetworking(host string) {
    // Простая проверка DNS
    addrs, err := net.LookupHost(host)
    if err != nil {
        log.Printf("DNS ошибка для %s: %v", host, err)
    } else {
        log.Printf("DNS для %s: %v", host, addrs)
    }
    
    // Проверка TCP соединения
    conn, err := net.DialTimeout("tcp", host+":443", 5*time.Second)
    if err != nil {
        log.Printf("TCP ошибка для %s: %v", host, err)
    } else {
        conn.Close()
        log.Printf("TCP соединение с %s: ОК", host)
    }
}
```

## Часто задаваемые вопросы

### Q: Как логировать все HTTP запросы?
**A:** Используйте пользовательский Transport:
```go
type LoggingTransport struct {
    base http.RoundTripper
}

func (lt *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    log.Printf("Запрос: %s %s", req.Method, req.URL)
    start := time.Now()
    
    resp, err := lt.base.RoundTrip(req)
    
    duration := time.Since(start)
    if err != nil {
        log.Printf("Ошибка за %v: %v", duration, err)
    } else {
        log.Printf("Ответ за %v: %d", duration, resp.StatusCode)
    }
    
    return resp, err
}

config := httpclient.Config{
    Transport: &LoggingTransport{base: http.DefaultTransport},
}
```

### Q: Как изменить User-Agent?
**A:** Добавьте header к каждому запросу:
```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
req.Header.Set("User-Agent", "MyApp/1.0")
resp, err := client.Do(req)
```

### Q: Как использовать custom SSL сертификаты?
**A:** Настройте TLS config в Transport:
```go
tlsConfig := &tls.Config{
    RootCAs: customCertPool,
}

transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

config := httpclient.Config{
    Transport: transport,
}
```

### Q: Как отключить retry для конкретного запроса?
**A:** Создайте отдельный клиент или используйте MaxAttempts: 1:
```go
noRetryConfig := httpclient.Config{
    RetryConfig: httpclient.RetryConfig{MaxAttempts: 1},
}
noRetryClient := httpclient.New(noRetryConfig, "no-retry")
```

### Q: Как проверить что метрики собираются правильно?
**A:** Используйте встроенный HTTP handler Prometheus:
```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "net/http"
)

// Экспорт метрик на /metrics
http.Handle("/metrics", promhttp.Handler())
log.Fatal(http.ListenAndServe(":2112", nil))
```

Затем проверьте `http://localhost:2112/metrics` для метрик `http_client_*`.

## Поддержка и обратная связь

Если проблема не решена:

1. **Проверьте логи** с включенным debug режимом
2. **Соберите метрики** для анализа поведения  
3. **Создайте минимальный воспроизводящий пример**
4. **Обратитесь к команде Backend разработки** с детальным описанием

**Полезная информация для отчета:**
- Версия Go
- Конфигурация клиента
- Логи ошибок
- Метрики (если доступны)
- Сетевые условия