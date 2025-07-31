package httpclient

import (
	"net/http"
	"slices"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

const defaultMetricMeterName = "default"

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
	Logger           *zap.Logger
	MetricsEnabled   bool
	MetricsMeterName string
	TracingEnabled   bool

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
		Timeout:          30 * time.Second,
		MaxIdleConns:     100,
		MaxConnsPerHost:  10,
		RetryMax:         0,
		RetryWaitMin:     1 * time.Second,
		RetryWaitMax:     10 * time.Second,
		RetryStrategy:    nil,
		MetricsEnabled:   true,
		MetricsMeterName: defaultMetricMeterName,
		TracingEnabled:   true,
		Logger:           zap.NewNop(),
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

// WithMultipleMiddleware adds multiple middleware to the client using Go 1.24.4 slices
func WithMultipleMiddleware(middlewares ...Middleware) ClientOption {
	return func(opts *ClientOptions) {
		opts.Middlewares = slices.Concat(opts.Middlewares, middlewares)
	}
}

// CloneMiddlewares returns a copy of middlewares using Go 1.24.4 slices
func (opts *ClientOptions) CloneMiddlewares() []Middleware {
	return slices.Clone(opts.Middlewares)
}

// HasMiddleware checks if a middleware type exists using Go 1.24.4 slices
func (opts *ClientOptions) HasMiddleware(target Middleware) bool {
	return slices.ContainsFunc(opts.Middlewares, func(m Middleware) bool {
		return m == target
	})
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

// WithMetricsName sets a custom meterName for Prometheus metrics names
func WithMetricsMeterName(meterName string) ClientOption {
	return func(opts *ClientOptions) {
		if meterName == "" {
			meterName = defaultMetricMeterName // Default name if empty
		}
		opts.MetricsMeterName = meterName
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
