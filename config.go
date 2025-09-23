package httpclient

import (
	"net/http"
	"slices"
	"time"
)

// Константы по умолчанию для конфигурации.
const (
	// Таймауты по умолчанию.
	defaultTimeout       = 5 * time.Second
	defaultPerTryTimeout = 2 * time.Second

	// Retry настройки по умолчанию.
	defaultMaxAttempts = 3
	defaultBaseDelay   = 100 * time.Millisecond
	defaultMaxDelay    = 2 * time.Second
	defaultJitter      = 0.2

	// CircuitBreaker настройки по умолчанию.
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 3
	defaultCircuitTimeout   = 60 * time.Second
)

// Config содержит конфигурацию HTTP клиента.
type Config struct {
	// Timeout общий таймаут для всей операции (включая ретраи)
	Timeout time.Duration

	// PerTryTimeout таймаут для каждой попытки
	PerTryTimeout time.Duration

	// Transport базовый HTTP транспорт (опционально)
	Transport http.RoundTripper

	// RetryEnabled включает/выключает retry механизм
	RetryEnabled bool

	// RetryConfig конфигурация retry механизма
	RetryConfig RetryConfig

	// TracingEnabled включает/выключает OpenTelemetry трассировку
	TracingEnabled bool

	// MaxResponseBytes ограничивает максимальный размер ответа
	MaxResponseBytes *int64

	// CircuitBreakerEnable включает/выключает использование CircuitBreaker
	CircuitBreakerEnable bool

	// CircuitBreaker настраиваемый автоматический выключатель
	CircuitBreaker CircuitBreaker

	// RateLimiterEnabled включает/выключает rate limiting
	RateLimiterEnabled bool

	// RateLimiterConfig конфигурация rate limiter
	RateLimiterConfig RateLimiterConfig

	// MetricsEnabled включает/выключает сбор Prometheus метрик
	// По умолчанию true - метрики включены
	MetricsEnabled *bool
}

// RetryConfig содержит настройки retry механизма.
type RetryConfig struct {
	// MaxAttempts максимальное количество попыток (включая первоначальную)
	MaxAttempts int

	// BaseDelay базовая задержка для exponential backoff
	BaseDelay time.Duration

	// MaxDelay максимальная задержка между попытками
	MaxDelay time.Duration

	// Jitter коэффициент джиттера (0.0 - 1.0)
	Jitter float64

	// RetryMethods список HTTP методов для retry
	RetryMethods []string

	// RetryStatusCodes список HTTP статусов для retry
	RetryStatusCodes []int

	// RespectRetryAfter учитывать заголовок Retry-After
	RespectRetryAfter bool
}

// RateLimiterConfig содержит настройки rate limiter.
// Rate limiter работает глобально для всех запросов клиента.
type RateLimiterConfig struct {
	// RequestsPerSecond максимальное количество запросов в секунду.
	RequestsPerSecond float64

	// BurstCapacity размер корзины для пиковых запросов.
	BurstCapacity int
}

// withDefaults применяет значения по умолчанию к конфигурации.
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

	// Circuit breaker по умолчанию выключен. Если включён и не задан — используем простой.
	if c.CircuitBreakerEnable && c.CircuitBreaker == nil {
		c.CircuitBreaker = NewSimpleCircuitBreaker()
	}

	// Rate limiter по умолчанию выключен
	if c.RateLimiterEnabled {
		c.RateLimiterConfig = c.RateLimiterConfig.withDefaults()
	}

	return c
}

// withDefaults применяет значения по умолчанию к конфигурации retry.
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

	// RespectRetryAfter по умолчанию true, но проверим явное присвоение false
	if !rc.RespectRetryAfter {
		rc.RespectRetryAfter = true
	}

	return rc
}

// isRequestRetryable проверяет, можно ли повторять конкретный запрос с учетом идемпотентности.
func (rc RetryConfig) isRequestRetryable(req *http.Request) bool {
	method := req.Method

	// Проверяем основные идемпотентные методы
	if slices.Contains(rc.RetryMethods, method) {
		return true
	}

	// Для POST и PATCH проверяем наличие Idempotency-Key
	if method == "POST" || method == "PATCH" {
		return req.Header.Get("Idempotency-Key") != ""
	}

	return false
}

// isStatusRetryable проверяет, можно ли повторять запрос для данного HTTP статуса.
func (rc RetryConfig) isStatusRetryable(status int) bool {
	return slices.Contains(rc.RetryStatusCodes, status)
}

// withDefaults применяет значения по умолчанию к конфигурации rate limiter.
func (rl RateLimiterConfig) withDefaults() RateLimiterConfig {
	if rl.RequestsPerSecond == 0 {
		rl.RequestsPerSecond = 10.0 // 10 запросов в секунду
	}

	if rl.BurstCapacity == 0 {
		rl.BurstCapacity = int(rl.RequestsPerSecond) // размер корзины равен rate
	}

	return rl
}
