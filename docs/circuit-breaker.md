# Автоматический выключатель (Circuit Breaker)

Circuit Breaker защищает систему от каскадных сбоев, автоматически «отключая» проблемные сервисы и быстро возвращая ошибку, не дожидаясь таймаутов.

## Состояния

1. **Closed (закрыт)**: все запросы выполняются. При ошибках увеличивается счетчик неудач.
2. **Open (открыт)**: запросы не отправляются. Возвращается последняя неуспешная реакция (клон) и ошибка `ErrCircuitBreakerOpen`.
3. **Half-Open (полуоткрыт)**: единичные «пробные» запросы. Успехи закрывают выключатель, неудачи возвращают его в открытое состояние.

## Включение и базовое использование

```go
config := httpclient.Config{
    CircuitBreakerEnable: true, // включаем CB
    // CircuitBreaker: nil     // не задаем — будет использован SimpleCircuitBreaker с дефолтами
}

client := httpclient.New(config, "my-service")
defer client.Close()

resp, err := client.Get(ctx, "https://api.example.com")
```

Если `CircuitBreakerEnable == true` и `CircuitBreaker == nil`, автоматически создается `SimpleCircuitBreaker` с настройками по умолчанию.

## Кастомная конфигурация

```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 3,              // сколько неудач до открытия
    SuccessThreshold: 1,               // сколько успехов для закрытия из Half-Open
    Timeout:          10 * time.Second, // пауза перед переходом в Half-Open
    FailStatusCodes:  []int{429, 500, 502, 503}, // опционально: что считать неуспехом
    OnStateChange: func(from, to httpclient.CircuitBreakerState) {
        // ваш логгер/метрики
    },
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "my-service")
```

## Что считается успехом/неуспехом

- Неуспех: любая ошибка транспорта, `nil`-ответ, либо HTTP статус из `FailStatusCodes`.
- Если `FailStatusCodes == nil`, неуспехом считаются `429` и любые `5xx`. Остальные статусы (включая `4xx`, кроме `429`) считаются успехом.

## Значения по умолчанию (SimpleCircuitBreaker)

```text
FailureThreshold: 5
SuccessThreshold: 3
Timeout:          60s
FailStatusCodes:  nil   // означает: 429 и >=500 считаются неуспехом
```

## Поведение с ретраями

- Circuit Breaker применяется на каждую попытку.
- Ошибка `ErrCircuitBreakerOpen` не является поводом для retry и завершает попытки.

## Получение состояния и сброс

```go
state := cb.State() // httpclient.CircuitBreakerClosed/Open/HalfOpen
cb.Reset()          // принудительно закрыть выключатель
```

## Наблюдаемость

- Используйте `OnStateChange` для логирования и метрик состояния выключателя.
- Метрики HTTP-клиента продолжают работать как обычно (запросы/длительности/ретраи).

## Пример

```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 2,
    SuccessThreshold: 1,
    Timeout:          5 * time.Second,
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "orders-client")

resp, err := client.Get(ctx, "https://service.internal/orders/123")
// при открытом выключателе вернется клон последнего неуспешного ответа и ErrCircuitBreakerOpen
```

## Лучшие практики

1. Подбирайте пороги под сервис (слишком низкие — ложные срабатывания, слишком высокие — поздняя реакция).
2. Логируйте переходы состояний через `OnStateChange` и мониторьте последствия в метриках клиента.
3. Для UX предусмотрите fallback, если выключатель открыт (кэш/заготовленный ответ).