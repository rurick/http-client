package httpclient

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

// ClientOptions содержит параметры конфигурации для HTTP клиента
type ClientOptions struct {
	// Конфигурация HTTP клиента
	Timeout         time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int

	// Конфигурация повтора
	RetryStrategy RetryStrategy
	RetryMax      int
	RetryWaitMin  time.Duration
	RetryWaitMax  time.Duration

	// Конфигурация автоматического выключателя
	CircuitBreaker CircuitBreaker

	// Промежуточное ПО
	Middlewares []Middleware

	// Метрики и логирование
	Logger         *zap.Logger
	MetricsEnabled bool
	TracingEnabled bool

	// Пользовательский HTTP клиент
	HTTPClient *http.Client

	// Пользовательский клиент повтора
	RetryClient *retryablehttp.Client
}

// ClientOption определяет тип функции для конфигурации ClientOptions
type ClientOption func(*ClientOptions)

// DefaultOptions возвращает параметры клиента по умолчанию
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		Timeout:         30 * time.Second,
		MaxIdleConns:    100,
		MaxConnsPerHost: 10,
		RetryMax:        0,
		RetryWaitMin:    1 * time.Second,
		RetryWaitMax:    10 * time.Second,
		RetryStrategy:   nil,
		MetricsEnabled:  true,
		TracingEnabled:  true,
		Logger:          zap.NewNop(),
	}
}

// WithTimeout устанавливает таймаут клиента
func WithTimeout(timeout time.Duration) ClientOption {
	return func(opts *ClientOptions) {
		opts.Timeout = timeout
	}
}

// WithMaxIdleConns устанавливает максимальное количество неактивных соединений
func WithMaxIdleConns(maxIdle int) ClientOption {
	return func(opts *ClientOptions) {
		opts.MaxIdleConns = maxIdle
	}
}

// WithMaxConnsPerHost устанавливает максимальное количество соединений на хост
func WithMaxConnsPerHost(maxConns int) ClientOption {
	return func(opts *ClientOptions) {
		opts.MaxConnsPerHost = maxConns
	}
}

// WithRetryStrategy sets a custom retry strategy
func WithRetryStrategy(strategy RetryStrategy) ClientOption {
	return func(opts *ClientOptions) {
		opts.RetryStrategy = strategy
		opts.RetryMax = strategy.MaxAttempts()
	}
}

// WithRetryMax sets the maximum number of retries
func WithRetryMax(max int) ClientOption {
	return func(opts *ClientOptions) {
		opts.RetryMax = max
	}
}

// WithRetryWait sets the retry wait times
func WithRetryWait(min, max time.Duration) ClientOption {
	return func(opts *ClientOptions) {
		opts.RetryWaitMin = min
		opts.RetryWaitMax = max
	}
}

// WithCircuitBreaker sets a circuit breaker
func WithCircuitBreaker(cb CircuitBreaker) ClientOption {
	return func(opts *ClientOptions) {
		opts.CircuitBreaker = cb
	}
}

// WithMiddleware adds middleware to the client
func WithMiddleware(middleware Middleware) ClientOption {
	return func(opts *ClientOptions) {
		opts.Middlewares = append(opts.Middlewares, middleware)
	}
}

// WithLogger sets a custom logger
func WithLogger(logger *zap.Logger) ClientOption {
	return func(opts *ClientOptions) {
		opts.Logger = logger
	}
}

// WithMetrics enables or disables metrics collection
func WithMetrics(enabled bool) ClientOption {
	return func(opts *ClientOptions) {
		opts.MetricsEnabled = enabled
	}
}

// WithTracing enables or disables distributed tracing
func WithTracing(enabled bool) ClientOption {
	return func(opts *ClientOptions) {
		opts.TracingEnabled = enabled
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(opts *ClientOptions) {
		opts.HTTPClient = client
	}
}

// WithRetryClient sets a custom retryable HTTP client
func WithRetryClient(client *retryablehttp.Client) ClientOption {
	return func(opts *ClientOptions) {
		opts.RetryClient = client
	}
}
