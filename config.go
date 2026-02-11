package httpclient

import (
	"net/http"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/metric"
)

// Default constants for configuration.
const (
	// Default timeouts.
	defaultTimeout       = 5 * time.Second
	defaultPerTryTimeout = 2 * time.Second

	// Default retry settings.
	defaultMaxAttempts = 3
	defaultBaseDelay   = 100 * time.Millisecond
	defaultMaxDelay    = 2 * time.Second
	defaultJitter      = 0.2

	// Default CircuitBreaker settings.
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 3
	defaultCircuitTimeout   = 60 * time.Second
)

// Config contains HTTP client configuration.
type Config struct {
	// Timeout is the overall timeout for the entire operation (including retries)
	Timeout time.Duration

	// PerTryTimeout is the timeout for each attempt
	PerTryTimeout time.Duration

	// Transport is the base HTTP transport (optional)
	Transport http.RoundTripper

	// RetryEnabled enables/disables retry mechanism
	RetryEnabled bool

	// RetryConfig is the retry mechanism configuration
	RetryConfig RetryConfig

	// TracingEnabled enables/disables OpenTelemetry tracing
	TracingEnabled bool

	// MaxResponseBytes limits the maximum response size
	MaxResponseBytes *int64

	// CircuitBreakerEnable enables/disables CircuitBreaker usage
	CircuitBreakerEnable bool

	// CircuitBreaker is a configurable automatic circuit breaker
	CircuitBreaker CircuitBreaker

	// RateLimiterEnabled enables/disables rate limiting
	RateLimiterEnabled bool

	// RateLimiterConfig is the rate limiter configuration
	RateLimiterConfig RateLimiterConfig

	// MetricsEnabled enables/disables metrics collection
	// Default is true - metrics are enabled
	MetricsEnabled *bool

	// MetricsBackend selects the metrics backend
	// Default is "otel"
	MetricsBackend MetricsBackend

	// PrometheusRegisterer is an optional Prometheus registerer
	// If nil, prometheus.DefaultRegisterer is used
	PrometheusRegisterer prometheus.Registerer

	// OTelMeterProvider is an optional OpenTelemetry metrics provider
	// If nil, otel.GetMeterProvider() is used
	OTelMeterProvider metric.MeterProvider

	// IncludePathInMetrics enables adding request path (endpoint) to metrics labels
	// Default is false to avoid high cardinality with dynamic paths containing IDs
	// When false, path label will be set to "-" in all metrics
	IncludePathInMetrics bool
}

// RetryConfig contains retry mechanism settings.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial one)
	MaxAttempts int

	// BaseDelay is the base delay for exponential backoff
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between attempts
	MaxDelay time.Duration

	// Jitter is the jitter coefficient (0.0 - 1.0)
	Jitter float64

	// RetryMethods is the list of HTTP methods for retry
	RetryMethods []string

	// RetryStatusCodes is the list of HTTP status codes for retry
	RetryStatusCodes []int

	// RespectRetryAfter respects the Retry-After header
	RespectRetryAfter bool
}

// RateLimiterConfig contains rate limiter settings.
// Rate limiter works globally for all client requests.
type RateLimiterConfig struct {
	// RequestsPerSecond is the maximum number of requests per second.
	RequestsPerSecond float64

	// BurstCapacity is the bucket size for peak requests.
	BurstCapacity int
}

// withDefaults applies default values to the configuration.
func (c Config) withDefaults() Config {
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}

	if c.PerTryTimeout == 0 {
		c.PerTryTimeout = defaultPerTryTimeout
	}

	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}

	if c.RetryEnabled {
		c.RetryConfig = c.RetryConfig.withDefaults()
	}

	// Circuit breaker is disabled by default. If enabled and not set, use a simple one.
	if c.CircuitBreakerEnable && c.CircuitBreaker == nil {
		c.CircuitBreaker = NewSimpleCircuitBreaker()
	}

	// Rate limiter is disabled by default
	if c.RateLimiterEnabled {
		c.RateLimiterConfig = c.RateLimiterConfig.withDefaults()
	}

	// Metrics are enabled by default with OpenTelemetry backend
	if c.MetricsEnabled == nil {
		enabled := true
		c.MetricsEnabled = &enabled
	}
	if c.MetricsBackend == "" {
		c.MetricsBackend = MetricsBackendOpenTelemetry
	}

	return c
}

// withDefaults applies default values to the retry configuration.
func (rc RetryConfig) withDefaults() RetryConfig {
	if rc.MaxAttempts == 0 {
		rc.MaxAttempts = defaultMaxAttempts
	}

	if rc.BaseDelay == 0 {
		rc.BaseDelay = defaultBaseDelay
	}

	if rc.MaxDelay == 0 {
		rc.MaxDelay = defaultMaxDelay
	}

	if rc.Jitter == 0 {
		rc.Jitter = defaultJitter
	}

	if len(rc.RetryMethods) == 0 {
		rc.RetryMethods = []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodOptions,
			http.MethodPut,
			http.MethodDelete,
		}
	}

	if len(rc.RetryStatusCodes) == 0 {
		rc.RetryStatusCodes = []int{429, 500, 502, 503, 504}
	}

	// RespectRetryAfter defaults to true, but check for explicit false assignment
	if !rc.RespectRetryAfter {
		rc.RespectRetryAfter = true
	}

	return rc
}

// isRequestRetryable checks if a specific request can be retried considering idempotency.
func (rc RetryConfig) isRequestRetryable(req *http.Request) bool {
	method := req.Method

	// Check main idempotent methods
	if slices.Contains(rc.RetryMethods, method) {
		return true
	}

	// For POST and PATCH, check for Idempotency-Key header
	if method == "POST" || method == "PATCH" {
		return req.Header.Get("Idempotency-Key") != ""
	}

	return false
}

// isStatusRetryable checks if a request can be retried for the given HTTP status.
func (rc RetryConfig) isStatusRetryable(status int) bool {
	return slices.Contains(rc.RetryStatusCodes, status)
}

// withDefaults applies default values to the rate limiter configuration.
func (rl RateLimiterConfig) withDefaults() RateLimiterConfig {
	if rl.RequestsPerSecond == 0 {
		rl.RequestsPerSecond = 10.0 // 10 requests per second
	}

	if rl.BurstCapacity == 0 {
		rl.BurstCapacity = int(rl.RequestsPerSecond) // bucket size equals rate
	}

	return rl
}
