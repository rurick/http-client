package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrorCategory represents different types of HTTP errors
type ErrorCategory string

const (
	ErrorCategoryNetwork        ErrorCategory = "network"
	ErrorCategoryTimeout        ErrorCategory = "timeout"
	ErrorCategoryServerError    ErrorCategory = "server_error"
	ErrorCategoryClientError    ErrorCategory = "client_error"
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	ErrorCategoryRateLimit      ErrorCategory = "rate_limit"
	ErrorCategoryCircuitBreaker ErrorCategory = "circuit_breaker"
	ErrorCategoryUnknown        ErrorCategory = "unknown"
)

// ErrorInsight contains contextual information about an error
type ErrorInsight struct {
	Category         ErrorCategory          `json:"category"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	Severity         string                 `json:"severity"` // low, medium, high, critical
	Suggestions      []string               `json:"suggestions"`
	RetryAdvice      string                 `json:"retry_advice"`
	TechnicalDetails map[string]interface{} `json:"technical_details"`
	Timestamp        time.Time              `json:"timestamp"`
}

// ErrorInsightsAnalyzer provides AI-powered error analysis and recommendations
type ErrorInsightsAnalyzer struct {
	enabledCategories map[ErrorCategory]bool
	customRules       []ErrorAnalysisRule
}

// ErrorAnalysisRule defines custom error analysis logic
type ErrorAnalysisRule struct {
	Name      string
	Condition func(*http.Request, *http.Response, error) bool
	Insight   func(*http.Request, *http.Response, error) *ErrorInsight
}

// NewErrorInsightsAnalyzer creates a new error insights analyzer
func NewErrorInsightsAnalyzer() *ErrorInsightsAnalyzer {
	return &ErrorInsightsAnalyzer{
		enabledCategories: map[ErrorCategory]bool{
			ErrorCategoryNetwork:        true,
			ErrorCategoryTimeout:        true,
			ErrorCategoryServerError:    true,
			ErrorCategoryClientError:    true,
			ErrorCategoryAuthentication: true,
			ErrorCategoryRateLimit:      true,
			ErrorCategoryCircuitBreaker: true,
			ErrorCategoryUnknown:        true,
		},
		customRules: []ErrorAnalysisRule{},
	}
}

// AnalyzeError analyzes an HTTP error and provides contextual insights
func (eia *ErrorInsightsAnalyzer) AnalyzeError(ctx context.Context, req *http.Request, resp *http.Response, err error) *ErrorInsight {
	// Check custom rules first
	for _, rule := range eia.customRules {
		if rule.Condition(req, resp, err) {
			return rule.Insight(req, resp, err)
		}
	}

	// Built-in analysis
	return eia.analyzeBuiltinError(ctx, req, resp, err)
}

// analyzeBuiltinError provides built-in error analysis
func (eia *ErrorInsightsAnalyzer) analyzeBuiltinError(ctx context.Context, req *http.Request, resp *http.Response, err error) *ErrorInsight {
	insight := &ErrorInsight{
		Timestamp: time.Now(),
		TechnicalDetails: map[string]interface{}{
			"method": req.Method,
			"url":    req.URL.String(),
		},
	}

	// Network errors
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			insight.Category = ErrorCategoryNetwork
			insight.Title = "Соединение отклонено"
			insight.Description = "Сервер недоступен или не слушает на указанном порту"
			insight.Severity = "high"
			insight.Suggestions = []string{
				"Проверьте что сервер запущен",
				"Убедитесь что порт правильный",
				"Проверьте сетевую доступность",
			}
			insight.RetryAdvice = "Повторите через некоторое время или проверьте статус сервера"
			insight.TechnicalDetails["error"] = err.Error()
			return insight
		}

		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			insight.Category = ErrorCategoryTimeout
			insight.Title = "Таймаут запроса"
			insight.Description = "Запрос превысил максимальное время ожидания"
			insight.Severity = "medium"
			insight.Suggestions = []string{
				"Увеличьте таймаут клиента",
				"Проверьте производительность сервера",
				"Рассмотрите асинхронную обработку",
			}
			insight.RetryAdvice = "Безопасно повторить с увеличенным таймаутом"
			insight.TechnicalDetails["error"] = err.Error()
			return insight
		}

		if strings.Contains(err.Error(), "no such host") {
			insight.Category = ErrorCategoryNetwork
			insight.Title = "Хост не найден"
			insight.Description = "DNS не может разрешить имя хоста"
			insight.Severity = "high"
			insight.Suggestions = []string{
				"Проверьте правильность URL",
				"Убедитесь в работе DNS",
				"Проверьте сетевое подключение",
			}
			insight.RetryAdvice = "Не повторяйте до исправления URL или DNS"
			insight.TechnicalDetails["error"] = err.Error()
			return insight
		}
	}

	// HTTP status code analysis
	if resp != nil {
		switch {
		case resp.StatusCode == 401:
			insight.Category = ErrorCategoryAuthentication
			insight.Title = "Ошибка аутентификации"
			insight.Description = "Отсутствуют или неверные учетные данные"
			insight.Severity = "high"
			insight.Suggestions = []string{
				"Проверьте API ключи или токены",
				"Убедитесь что учетные данные актуальны",
				"Проверьте формат заголовков авторизации",
			}
			insight.RetryAdvice = "Не повторяйте до исправления аутентификации"

		case resp.StatusCode == 403:
			insight.Category = ErrorCategoryAuthentication
			insight.Title = "Доступ запрещен"
			insight.Description = "У пользователя нет прав для выполнения этого запроса"
			insight.Severity = "high"
			insight.Suggestions = []string{
				"Проверьте права пользователя",
				"Убедитесь что ресурс доступен",
				"Свяжитесь с администратором API",
			}
			insight.RetryAdvice = "Не повторяйте до получения необходимых прав"

		case resp.StatusCode == 429:
			insight.Category = ErrorCategoryRateLimit
			insight.Title = "Превышен лимит запросов"
			insight.Description = "Слишком много запросов за короткий период"
			insight.Severity = "medium"
			insight.Suggestions = []string{
				"Реализуйте экспоненциальный откат",
				"Добавьте задержки между запросами",
				"Рассмотрите увеличение лимитов",
			}
			insight.RetryAdvice = "Повторите через время, указанное в Retry-After заголовке"

			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				insight.TechnicalDetails["retry_after"] = retryAfter
			}

		case resp.StatusCode >= 500 && resp.StatusCode < 600:
			insight.Category = ErrorCategoryServerError
			insight.Title = "Ошибка сервера"
			insight.Description = fmt.Sprintf("Внутренняя ошибка сервера (HTTP %d)", resp.StatusCode)
			insight.Severity = "medium"
			insight.Suggestions = []string{
				"Повторите запрос через некоторое время",
				"Проверьте статус сервера",
				"Реализуйте circuit breaker паттерн",
			}
			insight.RetryAdvice = "Безопасно повторить с экспоненциальным откатом"

		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			insight.Category = ErrorCategoryClientError
			insight.Title = "Ошибка клиента"
			insight.Description = fmt.Sprintf("Неверный запрос клиента (HTTP %d)", resp.StatusCode)
			insight.Severity = "high"
			insight.Suggestions = []string{
				"Проверьте корректность запроса",
				"Убедитесь в правильности параметров",
				"Проверьте документацию API",
			}
			insight.RetryAdvice = "Не повторяйте до исправления запроса"
		}

		insight.TechnicalDetails["status_code"] = resp.StatusCode
		insight.TechnicalDetails["status"] = resp.Status

		// Add response headers for debugging
		if len(resp.Header) > 0 {
			headers := make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			insight.TechnicalDetails["response_headers"] = headers
		}

		return insight
	}

	// If we have response with valid status code analysis, don't override
	if resp != nil && insight.Category != "" {
		return insight
	}

	// Unknown error
	insight.Category = ErrorCategoryUnknown
	insight.Title = "Неизвестная ошибка"
	insight.Description = "Произошла неопознанная ошибка"
	insight.Severity = "medium"
	insight.Suggestions = []string{
		"Проверьте логи для дополнительной информации",
		"Убедитесь в корректности запроса",
		"Свяжитесь с поддержкой если проблема повторяется",
	}
	insight.RetryAdvice = "Осторожно повторите после анализа причин"

	if err != nil {
		insight.TechnicalDetails["error"] = err.Error()
	}

	return insight
}

// AddCustomRule adds a custom error analysis rule
func (eia *ErrorInsightsAnalyzer) AddCustomRule(rule ErrorAnalysisRule) {
	eia.customRules = append(eia.customRules, rule)
}

// EnableCategory enables analysis for a specific error category
func (eia *ErrorInsightsAnalyzer) EnableCategory(category ErrorCategory) {
	eia.enabledCategories[category] = true
}

// DisableCategory disables analysis for a specific error category
func (eia *ErrorInsightsAnalyzer) DisableCategory(category ErrorCategory) {
	eia.enabledCategories[category] = false
}

// IsCategoryEnabled checks if a category is enabled
func (eia *ErrorInsightsAnalyzer) IsCategoryEnabled(category ErrorCategory) bool {
	return eia.enabledCategories[category]
}

// GetSupportedCategories returns all supported error categories
func (eia *ErrorInsightsAnalyzer) GetSupportedCategories() []ErrorCategory {
	return []ErrorCategory{
		ErrorCategoryNetwork,
		ErrorCategoryTimeout,
		ErrorCategoryServerError,
		ErrorCategoryClientError,
		ErrorCategoryAuthentication,
		ErrorCategoryRateLimit,
		ErrorCategoryCircuitBreaker,
		ErrorCategoryUnknown,
	}
}
