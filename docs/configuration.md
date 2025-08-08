# Конфигурация HTTP-клиента

Детальное описание всех параметров конфигурации HTTP-клиента.

## Структура Config

```go
type Config struct {
    Timeout          time.Duration
    PerTryTimeout    time.Duration
    Transport        http.RoundTripper
    RetryEnabled     bool
    RetryConfig      RetryConfig
    TracingEnabled   bool
    MaxResponseBytes *int64
}
