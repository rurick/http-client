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
