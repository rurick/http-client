package httpclient

import (
	"context"
	"io"
	"net/http"
	"time"
)

// HTTPClient определяет интерфейс для операций HTTP клиента
// Этот интерфейс обеспечивает совместимость со стандартным http.Client
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	PostForm(url string, data map[string][]string) (*http.Response, error)
	Head(url string) (*http.Response, error)
}

// CtxHTTPClient определяет интерфейс для HTTP операций с поддержкой контекста
// Все методы принимают context.Context для управления таймаутами и отменой запросов
type CtxHTTPClient interface {
	DoCtx(context.Context, *http.Request) (*http.Response, error)
	GetCtx(ctx context.Context, url string) (*http.Response, error)
	PostCtx(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
	PostFormCtx(ctx context.Context, url string, data map[string][]string) (*http.Response, error)
	HeadCtx(ctx context.Context, url string) (*http.Response, error)
}

// ExtendedHTTPClient предоставляет дополнительные методы помимо стандартного HTTP клиента
type ExtendedHTTPClient interface {
	HTTPClient
	CtxHTTPClient

	// JSON методы
	GetJSON(ctx context.Context, url string, result any) error
	PostJSON(ctx context.Context, url string, body any, result any) error
	PutJSON(ctx context.Context, url string, body any, result any) error
	PatchJSON(ctx context.Context, url string, body any, result any) error
	DeleteJSON(ctx context.Context, url string, result any) error

	// XML методы
	GetXML(ctx context.Context, url string, result any) error
	PostXML(ctx context.Context, url string, body any, result any) error

	// Методы с контекстом
	DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error)

	// Доступ к метрикам
}

// RetryStrategy определяет различные стратегии повтора
type RetryStrategy interface {
	NextDelay(attempt int, lastErr error) time.Duration
	ShouldRetry(resp *http.Response, err error) bool
	MaxAttempts() int
}

// Middleware определяет интерфейс промежуточного ПО для обработки запросов/ответов
type Middleware interface {
	Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
}

// CircuitBreaker определяет интерфейс автоматического выключателя
type CircuitBreaker interface {
	Execute(fn func() (*http.Response, error)) (*http.Response, error)
	State() CircuitBreakerState
	Reset()
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
