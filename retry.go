package httpclient

import (
	"math"
	"net/http"
	"time"
)

// ExponentialBackoffStrategy реализует стратегию повтора с экспоненциальной задержкой
type ExponentialBackoffStrategy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	multiplier  float64
}

// NewExponentialBackoffStrategy создает новую стратегию экспоненциальной задержки
func NewExponentialBackoffStrategy(maxAttempts int, baseDelay, maxDelay time.Duration) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
		multiplier:  2.0,
	}
}

// NextDelay вычисляет следующую задержку на основе экспоненциального отката
// Параметр lastErr не используется в этой стратегии
func (e *ExponentialBackoffStrategy) NextDelay(attempt int, _ error) time.Duration {
	if attempt <= 0 {
		return e.baseDelay
	}

	delay := float64(e.baseDelay) * math.Pow(e.multiplier, float64(attempt-1))

	if delay > float64(e.maxDelay) {
		return e.maxDelay
	}

	return time.Duration(delay)
}

// ShouldRetry определяет, следует ли повторить запрос (ExponentialBackoff)
func (e *ExponentialBackoffStrategy) ShouldRetry(resp *http.Response, err error) bool {
	// Повтор при сетевых ошибках
	if err != nil {
		return true
	}

	// Повтор при определенных HTTP кодах состояния
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		}
	}

	return false
}

// MaxAttempts возвращает максимальное количество попыток
func (e *ExponentialBackoffStrategy) MaxAttempts() int {
	return e.maxAttempts
}

// FixedDelayStrategy реализует стратегию повтора с фиксированной задержкой
type FixedDelayStrategy struct {
	maxAttempts int
	delay       time.Duration
}

// NewFixedDelayStrategy создает новую стратегию фиксированной задержки
func NewFixedDelayStrategy(maxAttempts int, delay time.Duration) *FixedDelayStrategy {
	return &FixedDelayStrategy{
		maxAttempts: maxAttempts,
		delay:       delay,
	}
}

// NextDelay возвращает фиксированную задержку
// Параметры attempt и lastErr не используются в этой стратегии
func (f *FixedDelayStrategy) NextDelay(_ int, _ error) time.Duration {
	return f.delay
}

// ShouldRetry определяет, следует ли повторить запрос (FixedDelay)
func (f *FixedDelayStrategy) ShouldRetry(resp *http.Response, err error) bool {
	// Повтор при сетевых ошибках
	if err != nil {
		return true
	}

	// Повтор при определенных HTTP кодах состояния
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		}
	}

	return false
}

// MaxAttempts возвращает максимальное количество попыток
func (f *FixedDelayStrategy) MaxAttempts() int {
	return f.maxAttempts
}

// CustomRetryStrategy позволяет создавать пользовательскую стратегию повтора
type CustomRetryStrategy struct {
	maxAttempts   int
	shouldRetryFn func(resp *http.Response, err error) bool
	nextDelayFn   func(attempt int, lastErr error) time.Duration
}

// NewCustomRetryStrategy создает новую пользовательскую стратегию повтора
func NewCustomRetryStrategy(maxAttempts int, shouldRetry func(resp *http.Response, err error) bool, nextDelay func(attempt int, lastErr error) time.Duration) *CustomRetryStrategy {
	return &CustomRetryStrategy{
		maxAttempts:   maxAttempts,
		shouldRetryFn: shouldRetry,
		nextDelayFn:   nextDelay,
	}
}

// NextDelay вычисляет следующую задержку с использованием пользовательской функции
func (c *CustomRetryStrategy) NextDelay(attempt int, lastErr error) time.Duration {
	if c.nextDelayFn == nil {
		return time.Second
	}
	return c.nextDelayFn(attempt, lastErr)
}

// ShouldRetry определяет, следует ли повторить запрос с использованием пользовательской функции
func (c *CustomRetryStrategy) ShouldRetry(resp *http.Response, err error) bool {
	if c.shouldRetryFn == nil {
		return false
	}
	return c.shouldRetryFn(resp, err)
}

// MaxAttempts возвращает максимальное количество попыток
func (c *CustomRetryStrategy) MaxAttempts() int {
	return c.maxAttempts
}

// RetryableHTTPCodes содержит HTTP коды состояния, для которых следует повторить запрос
var RetryableHTTPCodes = []int{
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

// IsRetryableStatusCode проверяет, является ли статус код подходящим для повтора
func IsRetryableStatusCode(statusCode int) bool {
	for _, code := range RetryableHTTPCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

// SmartRetryStrategy адаптивная стратегия повтора, которая анализирует историю ошибок
type SmartRetryStrategy struct {
	maxAttempts  int
	errorHistory []error
	delayHistory []time.Duration
	baseDelay    time.Duration
	maxDelay     time.Duration
}

// NewSmartRetryStrategy создает новую адаптивную стратегию повтора
func NewSmartRetryStrategy(maxAttempts int, baseDelay, maxDelay time.Duration) *SmartRetryStrategy {
	return &SmartRetryStrategy{
		maxAttempts:  maxAttempts,
		errorHistory: make([]error, 0, maxAttempts),
		delayHistory: make([]time.Duration, 0, maxAttempts),
		baseDelay:    baseDelay,
		maxDelay:     maxDelay,
	}
}

// NextDelay вычисляет адаптивную задержку на основе истории ошибок
func (s *SmartRetryStrategy) NextDelay(attempt int, lastErr error) time.Duration {
	if attempt <= 0 {
		return s.baseDelay
	}

	// Добавляем ошибку в историю
	s.errorHistory = append(s.errorHistory, lastErr)

	// Адаптивный расчет задержки на основе паттернов ошибок
	multiplier := 2.0
	if len(s.delayHistory) > 0 {
		// Анализируем предыдущие задержки для оптимизации
		avgDelay := s.calculateAverageDelay()
		if avgDelay > s.baseDelay {
			multiplier = 1.5 // Уменьшаем агрессивность роста
		}
	}

	delay := time.Duration(float64(s.baseDelay) * math.Pow(multiplier, float64(attempt-1)))

	// Ограничиваем задержку диапазоном
	adaptiveDelay := delay
	if adaptiveDelay > s.maxDelay {
		adaptiveDelay = s.maxDelay
	}
	if adaptiveDelay < s.baseDelay {
		adaptiveDelay = s.baseDelay
	}

	s.delayHistory = append(s.delayHistory, adaptiveDelay)

	return adaptiveDelay
}

// ShouldRetry определяет, следует ли повторить запрос на основе адаптивного анализа
func (s *SmartRetryStrategy) ShouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}

	if resp != nil {
		return IsRetryableStatusCode(resp.StatusCode)
	}

	return false
}

// MaxAttempts возвращает максимальное количество попыток
func (s *SmartRetryStrategy) MaxAttempts() int {
	return s.maxAttempts
}

// calculateAverageDelay вычисляет среднюю задержку из истории
func (s *SmartRetryStrategy) calculateAverageDelay() time.Duration {
	if len(s.delayHistory) == 0 {
		return s.baseDelay
	}

	var total time.Duration
	for _, delay := range s.delayHistory {
		total += delay
	}

	return total / time.Duration(len(s.delayHistory))
}

// GetErrorHistory возвращает историю ошибок (читаемую копию)
func (s *SmartRetryStrategy) GetErrorHistory() []error {
	result := make([]error, len(s.errorHistory))
	copy(result, s.errorHistory)
	return result
}

// GetDelayHistory возвращает историю задержек (читаемую копию)
func (s *SmartRetryStrategy) GetDelayHistory() []time.Duration {
	result := make([]time.Duration, len(s.delayHistory))
	copy(result, s.delayHistory)
	return result
}
