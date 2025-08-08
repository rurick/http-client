# Конфигурация

HTTP клиент пакет предлагает комплексные возможности конфигурации для различных сценариев использования.

## Структура Config

```go
type Config struct {
    Timeout         time.Duration    // Общий таймаут запроса
    PerTryTimeout   time.Duration    // Таймаут на каждую попытку
    RetryConfig     RetryConfig      // Конфигурация повторов
    TracingEnabled  bool             // Включить OpenTelemetry tracing
    Transport       http.RoundTripper // Пользовательский транспорт
}
```

## Параметры конфигурации

### Timeout (Общий таймаут)
- **Тип:** `time.Duration`
- **По умолчанию:** `5 * time.Second`
- **Описание:** Максимальное время ожидания для всего запроса, включая все retry попытки

```go
config := httpclient.Config{
    Timeout: 30 * time.Second, // Общий лимит 30 секунд
}
```

### PerTryTimeout (Таймаут попытки)
- **Тип:** `time.Duration`
- **По умолчанию:** `2 * time.Second`
- **Описание:** Максимальное время ожидания для одной попытки запроса

```go
config := httpclient.Config{
    PerTryTimeout: 5 * time.Second, // Каждая попытка до 5 секунд
}
```

### TracingEnabled (Включение tracing)
- **Тип:** `bool`
- **По умолчанию:** `false`
- **Описание:** Включает создание OpenTelemetry спанов для каждого запроса

```go
config := httpclient.Config{
    TracingEnabled: true, // Включить tracing
}
```

### Transport (Пользовательский транспорт)
- **Тип:** `http.RoundTripper`
- **По умолчанию:** `http.DefaultTransport`
- **Описание:** Позволяет настроить пользовательский HTTP транспорт

```go
config := httpclient.Config{
    Transport: &http.Transport{
        MaxIdleConns:       100,
        IdleConnTimeout:    90 * time.Second,
        DisableCompression: false,
    },
}
```

## Конфигурация Retry

### Структура RetryConfig

```go
type RetryConfig struct {
    MaxAttempts int           // Максимальное количество попыток
    BaseDelay   time.Duration // Базовая задержка для backoff
    MaxDelay    time.Duration // Максимальная задержка
    Jitter      float64       // Фактор джиттера (0.0-1.0)
}
```

### MaxAttempts (Максимум попыток)
- **Тип:** `int`
- **По умолчанию:** `1` (без повторов)
- **Описание:** Общее количество попыток (включая первоначальную)

```go
RetryConfig{
    MaxAttempts: 3, // 1 основная + 2 повтора
}
```

### BaseDelay (Базовая задержка)
- **Тип:** `time.Duration`
- **По умолчанию:** `100 * time.Millisecond`
- **Описание:** Начальная задержка для экспоненциального backoff

```go
RetryConfig{
    BaseDelay: 200 * time.Millisecond, // Начинать с 200ms
}
```

### MaxDelay (Максимальная задержка)
- **Тип:** `time.Duration`
- **По умолчанию:** `5 * time.Second`
- **Описание:** Максимальная задержка между попытками

```go
RetryConfig{
    MaxDelay: 10 * time.Second, // Не больше 10 секунд
}
```

### Jitter (Джиттер)
- **Тип:** `float64`
- **Диапазон:** `0.0 - 1.0`
- **По умолчанию:** `0.2`
- **Описание:** Случайное отклонение задержки для предотвращения thundering herd

```go
RetryConfig{
    Jitter: 0.3, // ±30% случайного отклонения
}
```

## Значения по умолчанию

```go
// Автоматически применяется при создании клиента
defaultConfig := Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 2 * time.Second,
    RetryConfig: RetryConfig{
        MaxAttempts: 1,        // Без повторов
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    5 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled: false,
    Transport:      http.DefaultTransport,
}
```

## Примеры конфигураций

### Быстрые внутренние сервисы

```go
config := httpclient.Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 1 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   50 * time.Millisecond,
        MaxDelay:    500 * time.Millisecond,
        Jitter:      0.1,
    },
}

client := httpclient.New(config, "internal-api")
```

### Внешние API (требующие надежности)

```go
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
}

client := httpclient.New(config, "external-api")
```

### Критичные платежные сервисы

```go
config := httpclient.Config{
    Timeout:       60 * time.Second,
    PerTryTimeout: 15 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 7,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Jitter:      0.25,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        50,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

client := httpclient.New(config, "payment-service")
```

### Высокопроизводительные API Gateway

```go
config := httpclient.Config{
    Timeout:       10 * time.Second,
    PerTryTimeout: 3 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   25 * time.Millisecond,
        MaxDelay:    1 * time.Second,
        Jitter:      0.1,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        200,
        MaxIdleConnsPerHost: 50,
        IdleConnTimeout:     60 * time.Second,
    },
}

client := httpclient.New(config, "api-gateway")
```

## Продвинутые настройки Transport

### Настройка пулов соединений

```go
transport := &http.Transport{
    // Общий пул соединений
    MaxIdleConns:        100,
    
    // Соединения на хост
    MaxIdleConnsPerHost: 10,
    
    // Время жизни idle соединений
    IdleConnTimeout:     90 * time.Second,
    
    // Таймауты TLS
    TLSHandshakeTimeout: 10 * time.Second,
    
    // Таймауты TCP
    DialTimeout:         5 * time.Second,
    
    // Keep-alive
    KeepAlive:           30 * time.Second,
    
    // Отключить сжатие
    DisableCompression:  false,
    
    // Размер буфера чтения
    ReadBufferSize:      4096,
    
    // Размер буфера записи
    WriteBufferSize:     4096,
}

config := httpclient.Config{
    Transport: transport,
}
```

### Настройка TLS

```go
tlsConfig := &tls.Config{
    // Проверка сертификатов
    InsecureSkipVerify: false,
    
    // Минимальная версия TLS
    MinVersion: tls.VersionTLS12,
    
    // Предпочитаемые cipher suites
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

config := httpclient.Config{
    Transport: transport,
}
```

## Валидация конфигурации

Пакет автоматически валидирует конфигурацию:

```go
// Некорректные значения будут исправлены
config := httpclient.Config{
    Timeout:       -1 * time.Second,  // Будет установлен в default
    PerTryTimeout: 0,                 // Будет установлен в default
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: -5,              // Будет установлен в 1
        Jitter:      2.0,             // Будет ограничен до 1.0
    },
}

client := httpclient.New(config, "service") // Работает с исправленными значениями
```

## Получение текущей конфигурации

```go
client := httpclient.New(config, "service")

// Получить активную конфигурацию
currentConfig := client.GetConfig()

fmt.Printf("Timeout: %v\n", currentConfig.Timeout)
fmt.Printf("Max Attempts: %d\n", currentConfig.RetryConfig.MaxAttempts)
```

## Рекомендации по конфигурации

### По типу сервиса

| Тип сервиса | Timeout | PerTryTimeout | MaxAttempts | BaseDelay |
|-------------|---------|---------------|-------------|-----------|
| Внутренний API | 5s | 1s | 2 | 50ms |
| Внешний API | 30s | 10s | 5 | 200ms |
| Базы данных | 10s | 3s | 3 | 100ms |
| Платежи | 60s | 15s | 7 | 500ms |
| File Upload | 300s | 60s | 3 | 1s |

### По SLA требованиям

- **99.9% SLA:** MaxAttempts = 3-5, агрессивные таймауты
- **99.95% SLA:** MaxAttempts = 5-7, умеренные таймауты  
- **99.99% SLA:** MaxAttempts = 7-10, консервативные таймауты

### По сетевой среде

- **Внутренняя сеть:** Низкий jitter (0.1), быстрые таймауты
- **Публичный интернет:** Высокий jitter (0.3), длинные таймауты
- **Мобильные сети:** Очень высокий jitter (0.5), очень длинные таймауты

## Отладка конфигурации

Включите tracing для отладки:

```go
config := httpclient.Config{
    TracingEnabled: true,
    // ... другие настройки
}
```

Это поможет увидеть:
- Реальное время выполнения запросов
- Количество retry попыток
- Причины ошибок
- Эффективность backoff стратегии