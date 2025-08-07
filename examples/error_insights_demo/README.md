# 🤖 AI-Powered Error Insights - Демонстрация

Этот пример демонстрирует возможности контекстного анализа ошибок с AI-powered рекомендациями.

## Запуск демонстрации

```bash
cd examples/error_insights_demo
go mod init error_insights_demo
go mod edit -replace gitlab.citydrive.tech/back-end/go/pkg/http-client=../..
go mod tidy
go run main.go
```

## Возможности Error Insights

### 🔍 Автоматический анализ ошибок

Error Insights автоматически анализирует все типы ошибок:

- **Сетевые ошибки** - connection refused, no such host, DNS errors
- **Таймауты** - deadline exceeded, request timeout
- **HTTP ошибки** - 401, 403, 429, 4xx, 5xx codes
- **Аутентификация** - unauthorized, forbidden access
- **Rate Limiting** - слишком много запросов

### 🧠 AI-Powered рекомендации

Для каждой ошибки предоставляются:
- **Категоризация** - автоматическое определение типа ошибки
- **Контекстные советы** - что делать для решения проблемы
- **Retry рекомендации** - безопасно ли повторять запрос
- **Техническая информация** - детали для отладки

### ⚙️ Пользовательские правила

```go
// Добавление пользовательского правила анализа
customRule := httpclient.ErrorAnalysisRule{
    Name: "custom_api_error",
    Condition: func(req *http.Request, resp *http.Response, err error) bool {
        return resp != nil && resp.StatusCode == 422
    },
    Insight: func(req *http.Request, resp *http.Response, err error) *httpclient.ErrorInsight {
        return &httpclient.ErrorInsight{
            Category:    httpclient.ErrorCategoryClientError,
            Title:       "Ошибка валидации данных",
            Description: "Переданные данные не прошли валидацию",
            // ... остальные поля
        }
    },
}

analyzer.AddCustomRule(customRule)
```

### 📊 Управление категориями

```go
analyzer := client.GetErrorInsightsAnalyzer()

// Отключить анализ таймаутов
analyzer.DisableCategory(httpclient.ErrorCategoryTimeout)

// Включить обратно
analyzer.EnableCategory(httpclient.ErrorCategoryTimeout)

// Проверить статус
enabled := analyzer.IsCategoryEnabled(httpclient.ErrorCategoryTimeout)
```

## Поддерживаемые категории

- `ErrorCategoryNetwork` - сетевые ошибки
- `ErrorCategoryTimeout` - ошибки таймаута
- `ErrorCategoryServerError` - ошибки сервера (5xx)
- `ErrorCategoryClientError` - ошибки клиента (4xx)
- `ErrorCategoryAuthentication` - ошибки аутентификации
- `ErrorCategoryRateLimit` - ошибки лимитов запросов
- `ErrorCategoryCircuitBreaker` - ошибки circuit breaker
- `ErrorCategoryUnknown` - неизвестные ошибки

## Интеграция с клиентом

```go
// После выполнения запроса получить анализ последней ошибки
insight := client.AnalyzeLastError(context.Background())
if insight != nil {
    fmt.Printf("Категория: %s\n", insight.Category)
    fmt.Printf("Рекомендации: %v\n", insight.Suggestions)
    fmt.Printf("Retry совет: %s\n", insight.RetryAdvice)
}
```

Error Insights работает автоматически для всех запросов и предоставляет ценную информацию для отладки и улучшения надежности приложений.