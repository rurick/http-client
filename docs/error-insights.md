# 🤖 Contextual Error Insights with AI-Powered Recommendations

HTTP Клиент предоставляет мощную систему анализа ошибок с AI-powered рекомендациями, которая автоматически анализирует каждую ошибку и предоставляет контекстную информацию для ее устранения.

## Обзор функциональности

### 🔍 Автоматический анализ ошибок

Error Insights автоматически анализирует все типы ошибок HTTP запросов:

- **Сетевые ошибки** - connection refused, no such host, DNS проблемы
- **Ошибки таймаута** - deadline exceeded, request timeout  
- **HTTP статус коды** - 4xx и 5xx ошибки с детальным анализом
- **Ошибки аутентификации** - 401, 403 с рекомендациями по исправлению
- **Rate limiting** - 429 ошибки с советами по retry стратегии

### 🧠 AI-Powered рекомендации

Для каждой ошибки система предоставляет:

- **Категоризацию** - автоматическое определение типа ошибки
- **Контекстное описание** - понятное объяснение проблемы  
- **Список рекомендаций** - конкретные шаги для устранения
- **Retry советы** - безопасно ли повторять запрос и как
- **Техническую информацию** - детали для отладки

## Поддерживаемые категории ошибок

```go
const (
    ErrorCategoryNetwork       = "network"        // Сетевые проблемы
    ErrorCategoryTimeout       = "timeout"        // Ошибки таймаута
    ErrorCategoryServerError   = "server_error"   // 5xx ошибки сервера
    ErrorCategoryClientError   = "client_error"   // 4xx ошибки клиента
    ErrorCategoryAuthentication = "authentication" // Проблемы аутентификации
    ErrorCategoryRateLimit     = "rate_limit"     // Превышение лимитов
    ErrorCategoryCircuitBreaker = "circuit_breaker" // Circuit breaker ошибки
    ErrorCategoryUnknown       = "unknown"        // Неизвестные ошибки
)
```

## Основное использование

### Анализ ошибок в клиенте

```go
client, err := httpclient.NewClient()
if err != nil {
    log.Fatal(err)
}

// Выполняем запрос
resp, err := client.Get("https://api.example.com/users")
if err != nil || (resp != nil && resp.StatusCode >= 400) {
    // Получаем анализ последней ошибки
    insight := client.AnalyzeLastError(context.Background())
    if insight != nil {
        fmt.Printf("Категория: %s\n", insight.Category)
        fmt.Printf("Проблема: %s\n", insight.Title)
        fmt.Printf("Описание: %s\n", insight.Description)
        fmt.Printf("Серьезность: %s\n", insight.Severity)
        
        fmt.Println("Рекомендации:")
        for i, suggestion := range insight.Suggestions {
            fmt.Printf("  %d. %s\n", i+1, suggestion)
        }
        
        fmt.Printf("Retry совет: %s\n", insight.RetryAdvice)
    }
}
```

### Структура ErrorInsight

```go
type ErrorInsight struct {
    Category     ErrorCategory              `json:"category"`
    Title        string                     `json:"title"`
    Description  string                     `json:"description"`
    Severity     string                     `json:"severity"`    // low, medium, high, critical
    Suggestions  []string                   `json:"suggestions"`
    RetryAdvice  string                     `json:"retry_advice"`
    TechnicalDetails map[string]interface{} `json:"technical_details"`
    Timestamp    time.Time                  `json:"timestamp"`
}
```

## Пользовательские правила анализа

### Добавление собственных правил

```go
// Получаем анализатор
analyzer := client.GetErrorInsightsAnalyzer()

// Добавляем пользовательское правило
customRule := httpclient.ErrorAnalysisRule{
    Name: "custom_api_validation",
    Condition: func(req *http.Request, resp *http.Response, err error) bool {
        return resp != nil && resp.StatusCode == 422 && 
               strings.Contains(req.URL.String(), "/api/")
    },
    Insight: func(req *http.Request, resp *http.Response, err error) *httpclient.ErrorInsight {
        return &httpclient.ErrorInsight{
            Category:    httpclient.ErrorCategoryClientError,
            Title:       "API валидация данных",
            Description: "Переданные данные не прошли валидацию на сервере",
            Severity:    "high",
            Suggestions: []string{
                "Проверьте схему данных согласно API документации",
                "Убедитесь что все обязательные поля присутствуют",
                "Проверьте типы и форматы данных",
            },
            RetryAdvice: "Исправьте данные перед повтором запроса",
            TechnicalDetails: map[string]interface{}{
                "rule_name": "custom_api_validation",
                "endpoint":  req.URL.String(),
                "method":    req.Method,
            },
            Timestamp: time.Now(),
        }
    },
}

analyzer.AddCustomRule(customRule)
```

## Управление категориями

### Включение/отключение категорий

```go
analyzer := client.GetErrorInsightsAnalyzer()

// Отключить анализ определенного типа ошибок
analyzer.DisableCategory(httpclient.ErrorCategoryTimeout)

// Включить обратно
analyzer.EnableCategory(httpclient.ErrorCategoryTimeout)

// Проверить статус категории
enabled := analyzer.IsCategoryEnabled(httpclient.ErrorCategoryTimeout)

// Получить все поддерживаемые категории
categories := analyzer.GetSupportedCategories()
for _, category := range categories {
    enabled := analyzer.IsCategoryEnabled(category)
    fmt.Printf("Категория %s: %v\n", category, enabled)
}
```

## Примеры анализа ошибок

### Сетевые ошибки

```go
// Connection refused
insight := analyzer.AnalyzeError(ctx, req, nil, 
    fmt.Errorf("dial tcp: connection refused"))

// Результат:
// Category: network
// Title: "Соединение отклонено"
// Suggestions: ["Проверьте что сервер запущен", "Убедитесь что порт правильный"]
// RetryAdvice: "Повторите через некоторое время или проверьте статус сервера"
```

### HTTP ошибки аутентификации

```go
resp := &http.Response{StatusCode: 401}
insight := analyzer.AnalyzeError(ctx, req, resp, nil)

// Результат:
// Category: authentication  
// Title: "Ошибка аутентификации"
// Suggestions: ["Проверьте API ключи или токены", "Убедитесь что учетные данные актуальны"]
// RetryAdvice: "Не повторяйте до исправления аутентификации"
```

### Rate Limiting

```go
resp := &http.Response{
    StatusCode: 429,
    Header: http.Header{"Retry-After": []string{"60"}},
}
insight := analyzer.AnalyzeError(ctx, req, resp, nil)

// Результат:
// Category: rate_limit
// Title: "Превышен лимит запросов"  
// Suggestions: ["Реализуйте экспоненциальный откат", "Добавьте задержки между запросами"]
// RetryAdvice: "Повторите через время, указанное в Retry-After заголовке"
// TechnicalDetails: {"retry_after": "60"}
```

## Интеграция с middleware

Error Insights автоматически работает с существующей middleware архитектурой:

```go
// Error insight добавляется в контекст запроса
func customMiddleware(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
    resp, err := next(req)
    
    // Получаем анализ ошибки из контекста если он есть
    if insight, ok := req.Context().Value("error_insight").(*httpclient.ErrorInsight); ok {
        // Логируем детальную информацию об ошибке
        log.Printf("Error insight: %s - %s", insight.Category, insight.Title)
    }
    
    return resp, err
}
```

## Лучшие практики

### 1. Обработка ошибок в приложении

```go
func handleAPIError(client httpclient.ExtendedHTTPClient, req *http.Request) error {
    resp, err := client.DoWithContext(context.Background(), req)
    
    if err != nil || (resp != nil && resp.StatusCode >= 400) {
        insight := client.AnalyzeLastError(context.Background())
        if insight != nil {
            // Логируем структурированную информацию
            log.WithFields(log.Fields{
                "category":    insight.Category,
                "severity":    insight.Severity,
                "retry_safe":  insight.RetryAdvice,
                "suggestions": insight.Suggestions,
            }).Error(insight.Title)
            
            // Принимаем решение на основе категории
            switch insight.Category {
            case httpclient.ErrorCategoryRateLimit:
                return &RetryableError{Delay: parseRetryAfter(insight)}
            case httpclient.ErrorCategoryAuthentication:
                return &AuthError{Message: insight.Description}
            case httpclient.ErrorCategoryServerError:
                return &RetryableError{Delay: exponentialBackoff()}
            default:
                return &PermanentError{Message: insight.Description}
            }
        }
    }
    
    return nil
}
```

### 2. Мониторинг качества API

```go
func monitorAPIHealth(client httpclient.ExtendedHTTPClient) {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        resp, _ := client.Get("https://api.example.com/health")
        
        if insight := client.AnalyzeLastError(context.Background()); insight != nil {
            // Отправляем метрики о проблемах API
            metrics.Counter("api_errors").
                WithTag("category", string(insight.Category)).
                WithTag("severity", insight.Severity).
                Increment()
                
            // Алерты для критических проблем
            if insight.Severity == "critical" {
                alerting.Send("API Critical Error", insight.Description)
            }
        }
    }
}
```

### 3. Автоматическое логирование ошибок

```go
type ErrorInsightLogger struct {
    client httpclient.ExtendedHTTPClient
    logger *zap.Logger
}

func (l *ErrorInsightLogger) logRequest(method, url string) {
    switch method {
    case "GET":
        resp, err := l.client.Get(url)
    case "POST":
        resp, err := l.client.Post(url, "application/json", nil)
    }
    
    // Автоматически логируем детали любых ошибок
    if insight := l.client.AnalyzeLastError(context.Background()); insight != nil {
        l.logger.Error("HTTP request failed",
            zap.String("category", string(insight.Category)),
            zap.String("title", insight.Title),
            zap.String("severity", insight.Severity),
            zap.Strings("suggestions", insight.Suggestions),
            zap.String("retry_advice", insight.RetryAdvice),
            zap.Any("technical_details", insight.TechnicalDetails),
        )
    }
}
```

## Заключение

Contextual Error Insights с AI-Powered рекомендациями значительно упрощает диагностику и устранение проблем HTTP запросов, предоставляя:

- **Автоматический анализ** всех типов ошибок
- **Контекстные рекомендации** для быстрого решения проблем  
- **Гибкую настройку** через пользовательские правила
- **Интеграцию с существующим кодом** без изменений архитектуры
- **Улучшенную надежность** приложений через умные retry стратегии

Эта функциональность работает "из коробки" и автоматически анализирует ошибки для всех HTTP запросов, делая отладку более эффективной и приложения более надежными.