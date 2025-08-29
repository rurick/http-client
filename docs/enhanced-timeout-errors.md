# Детализированные ошибки тайм-аута

## Обзор

Мы добавили улучшенную обработку ошибок тайм-аута в HTTP клиент. Теперь вместо стандартной ошибки `context deadline exceeded` с минимальной информацией, клиент предоставляет детализированные сообщения об ошибках с контекстом и практическими рекомендациями.

## Проблема

### До улучшения
```
"level": "ERROR",
"message": "request failed", 
"error": "Post \"https://openapi.nalog.ru:8090/open-api/AuthService/0.1\": context deadline exceeded"
```

Из этой ошибки невозможно понять:
- Какие тайм-ауты были настроены
- На какой попытке произошёл сбой
- Включён ли retry
- Что именно нужно исправить

### После улучшения
```
"level": "ERROR",
"message": "request failed",
"error": "timeout error: POST https://openapi.nalog.ru:8090/open-api/AuthService/0.1 (host: openapi.nalog.ru) failed after 5s on attempt 1/1. Timeout config: overall=5s, per-try=2s, retry=false. Type: overall. Предложения: [увеличьте общий тайм-аут (текущий: 5s) включите retry для устойчивости к временным сбоям]"
```

## Реализация

### Новый тип ошибки `TimeoutError`

```go
type TimeoutError struct {
    // Основная информация о запросе
    Method   string
    URL      string  
    Host     string
    
    // Информация о тайм-аутах
    Timeout       time.Duration // Общий тайм-аут
    PerTryTimeout time.Duration // Тайм-аут на попытку
    Elapsed       time.Duration // Время выполнения до ошибки
    
    // Контекст retry
    Attempt     int  // Номер попытки на которой произошёл тайм-аут
    MaxAttempts int  // Максимальное количество попыток  
    RetryEnabled bool // Был ли включён retry
    
    // Дополнительный контекст
    TimeoutType string // Тип тайм-аута: "overall", "per-try", "context"
    OriginalErr error  // Оригинальная ошибка
    
    // Предложения по решению
    Suggestions []string
}
```

### Типы тайм-аутов

1. **"overall"** - превышен общий тайм-аут (`Config.Timeout`)
2. **"per-try"** - превышен тайм-аут на попытку (`Config.PerTryTimeout`) 
3. **"context"** - тайм-аут был задан во внешнем контексте
4. **"network"** - сетевой тайм-аут (не связанный с настройками клиента)

### Автоматические предложения

Система анализирует конфигурацию и условия ошибки, генерируя практические рекомендации:

- **Для overall timeout**: "увеличьте общий тайм-аут (текущий: 5s)"
- **Для per-try timeout**: "увеличьте per-try тайм-аут (текущий: 2s)" 
- **Для исчерпанных попыток**: "увеличьте количество попыток (текущий: 3)"
- **Если retry отключён**: "включите retry для устойчивости к временным сбоям"
- **Для медленных сервисов**: "проверьте доступность и производительность удалённого сервиса"

## Использование

### Программная обработка

```go
resp, err := client.Post(ctx, url, body)
if err != nil {
    // Проверяем, является ли это детализированной ошибкой тайм-аута
    var timeoutErr *httpclient.TimeoutError
    if errors.As(err, &timeoutErr) {
        log.Printf("Тайм-аут при %s:", operation)
        log.Printf("  URL: %s", timeoutErr.URL)
        log.Printf("  Попытка: %d/%d", timeoutErr.Attempt, timeoutErr.MaxAttempts)
        log.Printf("  Время выполнения: %v", timeoutErr.Elapsed)
        log.Printf("  Тип: %s", timeoutErr.TimeoutType)
        
        // Программно обрабатываем разные типы тайм-аутов
        switch timeoutErr.TimeoutType {
        case "overall":
            log.Printf("  → Рекомендация: увеличьте общий тайм-аут с %v", timeoutErr.Timeout)
        case "per-try":
            log.Printf("  → Рекомендация: увеличьте per-try тайм-аут с %v", timeoutErr.PerTryTimeout)
        case "context":
            log.Printf("  → Рекомендация: проверьте настройки контекста вызывающего кода")
        }
        return
    }
    
    // Обрабатываем другие типы ошибок как обычно
    log.Printf("Ошибка: %v", err)
}
```

### Рекомендуемая конфигурация для медленных API

Для работы с медленными внешними API (например, API ФНС) рекомендуется:

```go
config := httpclient.Config{
    // Увеличенные тайм-ауты
    Timeout:       60 * time.Second, // 1 минута общий
    PerTryTimeout: 20 * time.Second, // 20 секунд на попытку
    
    // Агрессивный retry для стабильности
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts:       4,    // 4 попытки
        BaseDelay:        500 * time.Millisecond,
        MaxDelay:         15 * time.Second,
        Jitter:           0.3,   // 30% jitter
        RespectRetryAfter: true,
        
        // Дополнительные статусы для retry
        RetryStatusCodes: []int{408, 429, 500, 502, 503, 504, 520, 521, 522, 524},
    },
    
    TracingEnabled: true,
}

client := httpclient.New(config, "external-api-client")
```

## Важные особенности

### Обратная совместимость

- Не-тайм-аут ошибки остаются неизменными
- Существующий код продолжает работать без изменений
- Новая функциональность доступна только при явной проверке типа ошибки

### Тестирование

Добавлены comprehensive тесты, покрывающие:

- ✅ Детализированные сообщения об ошибках
- ✅ Автоматические предложения по исправлению
- ✅ Различные типы тайм-аутов
- ✅ Обработка не-тайм-аут ошибок (остаются неизменными)
- ✅ Реальные сценарии использования с API ФНС
- ✅ Интеграционные тесты с RoundTripper

### Производительность

- Минимальное влияние на производительность
- Детализированные ошибки создаются только при тайм-аутах
- Никакого дополнительного overhead для успешных запросов

## Примеры использования

См. файл `examples/enhanced_timeout_errors/main.go` для полных примеров демонстрации новой функциональности.

## Заключение

Данная реализация значительно улучшает диагностику проблем с тайм-аутами, предоставляя разработчикам всю необходимую информацию для быстрого решения проблем и оптимизации настроек HTTP клиента.
