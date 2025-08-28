package httpclient

import (
	"fmt"
	"net/http"
	"time"
)

// HTTPError представляет HTTP ошибку с дополнительной информацией
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
	Method     string
	Body       []byte
	Headers    http.Header
}

// Error реализует интерфейс error
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d %s: %s %s", e.StatusCode, e.Status, e.Method, e.URL)
}

// IsHTTPError проверяет, является ли ошибка HTTP ошибкой
func IsHTTPError(err error) bool {
	_, ok := err.(*HTTPError)
	return ok
}

// NewHTTPError создаёт новую HTTP ошибку
func NewHTTPError(resp *http.Response, req *http.Request) *HTTPError {
	return &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		URL:        req.URL.String(),
		Method:     req.Method,
		Headers:    resp.Header,
	}
}

// MaxAttemptsExceededError представляет ошибку превышения максимального количества попыток
type MaxAttemptsExceededError struct {
	MaxAttempts int
	LastError   error
	LastStatus  int
}

// Error реализует интерфейс error
func (e *MaxAttemptsExceededError) Error() string {
	if e.LastError != nil {
		return fmt.Sprintf("max attempts (%d) exceeded, last error: %v", e.MaxAttempts, e.LastError)
	}
	return fmt.Sprintf("max attempts (%d) exceeded, last status: %d", e.MaxAttempts, e.LastStatus)
}

// Unwrap возвращает последнюю ошибку для поддержки errors.Unwrap
func (e *MaxAttemptsExceededError) Unwrap() error {
	return e.LastError
}

// TimeoutExceededError представляет ошибку превышения таймаута
type TimeoutExceededError struct {
	Timeout time.Duration
	Elapsed time.Duration
}

// Error реализует интерфейс error
func (e *TimeoutExceededError) Error() string {
	return fmt.Sprintf("timeout exceeded: %v elapsed, %v allowed", e.Elapsed, e.Timeout)
}

// ConfigurationError представляет ошибку конфигурации
type ConfigurationError struct {
	Field   string
	Value   interface{}
	Message string
}

// Error реализует интерфейс error
func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// NewConfigurationError создаёт новую ошибку конфигурации
func NewConfigurationError(field string, value interface{}, message string) *ConfigurationError {
	return &ConfigurationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// TimeoutError представляет детализированную ошибку тайм-аута с контекстом
type TimeoutError struct {
	// Основная информация о запросе
	Method string
	URL    string
	Host   string
	// Информация о тайм-аутах
	Timeout       time.Duration // Общий тайм-аут
	PerTryTimeout time.Duration // Тайм-аут на попытку
	Elapsed       time.Duration // Время выполнения до ошибки
	// Контекст retry
	Attempt      int  // Номер попытки на которой произошёл тайм-аут
	MaxAttempts  int  // Максимальное количество попыток
	RetryEnabled bool // Был ли включён retry
	// Дополнительный контекст
	TimeoutType string // Тип тайм-аута: "overall", "per-try", "context"
	OriginalErr error  // Оригинальная ошибка
	// Предложения по решению
	Suggestions []string
}

// Error реализует интерфейс error с детализированным сообщением
func (e *TimeoutError) Error() string {
	var suggestions string
	if len(e.Suggestions) > 0 {
		suggestions = fmt.Sprintf(" Предложения: %v", e.Suggestions)
	}

	return fmt.Sprintf(
		"timeout error: %s %s (host: %s) failed after %v on attempt %d/%d. "+
			"Timeout config: overall=%v, per-try=%v, retry=%t. Type: %s.%s",
		e.Method, e.URL, e.Host, e.Elapsed, e.Attempt, e.MaxAttempts,
		e.Timeout, e.PerTryTimeout, e.RetryEnabled, e.TimeoutType, suggestions,
	)
}

// Unwrap возвращает оригинальную ошибку для поддержки errors.Unwrap
func (e *TimeoutError) Unwrap() error {
	return e.OriginalErr
}

// NewTimeoutError создаёт детализированную ошибку тайм-аута
func NewTimeoutError(
	req *http.Request,
	config Config,
	attempt, maxAttempts int,
	elapsed time.Duration,
	timeoutType string,
	originalErr error,
) *TimeoutError {
	host := getHost(req.URL)

	// Генерируем предложения по решению проблемы
	suggestions := generateTimeoutSuggestions(config, elapsed, timeoutType, attempt, maxAttempts)

	return &TimeoutError{
		Method:        req.Method,
		URL:           req.URL.String(),
		Host:          host,
		Timeout:       config.Timeout,
		PerTryTimeout: config.PerTryTimeout,
		Elapsed:       elapsed,
		Attempt:       attempt,
		MaxAttempts:   maxAttempts,
		RetryEnabled:  config.RetryEnabled,
		TimeoutType:   timeoutType,
		OriginalErr:   originalErr,
		Suggestions:   suggestions,
	}
}

// generateTimeoutSuggestions генерирует предложения по решению проблем с тайм-аутом
func generateTimeoutSuggestions(
	config Config,
	elapsed time.Duration,
	timeoutType string,
	attempt, maxAttempts int,
) []string {
	var suggestions []string

	switch timeoutType {
	case "overall":
		if elapsed >= config.Timeout {
			suggestions = append(suggestions, fmt.Sprintf("увеличьте общий тайм-аут (текущий: %v)", config.Timeout))
		}
		if !config.RetryEnabled {
			suggestions = append(suggestions, "включите retry для устойчивости к временным сбоям")
		}

	case "per-try":
		if elapsed >= config.PerTryTimeout {
			suggestions = append(suggestions, fmt.Sprintf("увеличьте per-try тайм-аут (текущий: %v)", config.PerTryTimeout))
		}
		if attempt < maxAttempts {
			suggestions = append(suggestions, "попытки retry продолжаются")
		}

	case "context":
		suggestions = append(suggestions, "тайм-аут был задан в context.WithTimeout() или context.WithDeadline()")
		suggestions = append(suggestions, "проверьте настройки контекста вызывающего кода")
	}

	// Общие предложения
	if config.RetryEnabled && attempt >= maxAttempts {
		suggestions = append(suggestions, fmt.Sprintf("увеличьте количество попыток (текущий: %d)", maxAttempts))
	}

	if elapsed > 10*time.Second {
		suggestions = append(suggestions, "проверьте доступность и производительность удалённого сервиса")
	}

	return suggestions
}
