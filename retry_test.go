package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestExponentialBackoffStrategy проверяет стратегию экспоненциальной задержки
// Тестирует правильное увеличение задержки: 1с -> 1с -> 2с -> 4с -> 8с...
// и ограничение максимальной задержки
func TestExponentialBackoffStrategy(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(3, 1*time.Second, 10*time.Second)

	// Проверяем максимальное количество попыток
	assert.Equal(t, 3, strategy.MaxAttempts())

	// Проверяем прогрессию задержек
	delay1 := strategy.NextDelay(0, nil)
	assert.Equal(t, 1*time.Second, delay1)

	delay2 := strategy.NextDelay(1, nil)
	assert.Equal(t, 1*time.Second, delay2)

	delay3 := strategy.NextDelay(2, nil)
	assert.Equal(t, 2*time.Second, delay3)

	delay4 := strategy.NextDelay(3, nil)
	assert.Equal(t, 4*time.Second, delay4)

	// Проверяем ограничение максимальной задержки
	delay5 := strategy.NextDelay(10, nil)
	assert.Equal(t, 10*time.Second, delay5)
}

// TestExponentialBackoffShouldRetry проверяет логику принятия решений о повторе
// Проверяет что повторы происходят только для сетевых ошибок и серверных ошибок (5xx, 429)
// Клиентские ошибки (4xx) и успешные ответы (2xx, 3xx) не должны повторяться
func TestExponentialBackoffShouldRetry(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(3, 1*time.Second, 10*time.Second)

	tests := []struct {
		name       string
		statusCode int
		err        error
		expected   bool
	}{
		{
			name:     "сетевая ошибка должна повторяться",
			err:      assert.AnError,
			expected: true,
		},
		{
			name:       "500 ошибка должна повторяться",
			statusCode: http.StatusInternalServerError,
			expected:   true,
		},
		{
			name:       "502 ошибка должна повторяться",
			statusCode: http.StatusBadGateway,
			expected:   true,
		},
		{
			name:       "503 ошибка должна повторяться",
			statusCode: http.StatusServiceUnavailable,
			expected:   true,
		},
		{
			name:       "504 ошибка должна повторяться",
			statusCode: http.StatusGatewayTimeout,
			expected:   true,
		},
		{
			name:       "429 ошибка должна повторяться",
			statusCode: http.StatusTooManyRequests,
			expected:   true,
		},
		{
			name:       "404 ошибка НЕ должна повторяться",
			statusCode: http.StatusNotFound,
			expected:   false,
		},
		{
			name:       "200 успех НЕ должен повторяться",
			statusCode: http.StatusOK,
			expected:   false,
		},
		{
			name:       "400 плохой запрос НЕ должен повторяться",
			statusCode: http.StatusBadRequest,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.statusCode > 0 {
				resp = &http.Response{StatusCode: tt.statusCode}
			}

			result := strategy.ShouldRetry(resp, tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFixedDelayStrategy проверяет стратегию фиксированной задержки
// Проверяет что задержка между повторами всегда одинаковая
func TestFixedDelayStrategy(t *testing.T) {
	maxAttempts := 5
	delay := 2 * time.Second
	strategy := NewFixedDelayStrategy(maxAttempts, delay)

	// Проверяем максимальное количество попыток
	assert.Equal(t, maxAttempts, strategy.MaxAttempts())

	// Проверяем что задержка всегда одинаковая
	for i := 0; i < 10; i++ {
		actualDelay := strategy.NextDelay(i, nil)
		assert.Equal(t, delay, actualDelay)
	}
}

// TestFixedDelayShouldRetry проверяет логику принятия решений для фиксированной задержки
// Должна работать аналогично экспоненциальной стратегии по критериям повтора
func TestFixedDelayShouldRetry(t *testing.T) {
	strategy := NewFixedDelayStrategy(3, 1*time.Second)

	// Проверяем сетевую ошибку
	assert.True(t, strategy.ShouldRetry(nil, assert.AnError))

	// Проверяем серверные ошибки
	resp500 := &http.Response{StatusCode: http.StatusInternalServerError}
	assert.True(t, strategy.ShouldRetry(resp500, nil))

	// Проверяем клиентские ошибки
	resp404 := &http.Response{StatusCode: http.StatusNotFound}
	assert.False(t, strategy.ShouldRetry(resp404, nil))

	// Проверяем успешные ответы
	resp200 := &http.Response{StatusCode: http.StatusOK}
	assert.False(t, strategy.ShouldRetry(resp200, nil))
}

// TestCustomRetryStrategy проверяет пользовательскую стратегию повтора
// Позволяет определить собственную логику расчета задержки и условий повтора
func TestCustomRetryStrategy(t *testing.T) {
	maxAttempts := 4

	// Пользовательская функция задержки, возвращающая номер попытки в секундах
	delayFunc := func(attempt int, lastErr error) time.Duration {
		return time.Duration(attempt) * time.Second
	}

	// Пользовательская функция условий повтора - только для конкретных ошибок
	shouldRetryFunc := func(resp *http.Response, err error) bool {
		if err != nil {
			return true
		}
		if resp != nil && resp.StatusCode == http.StatusServiceUnavailable {
			return true
		}
		return false
	}

	strategy := NewCustomRetryStrategy(maxAttempts, shouldRetryFunc, delayFunc)

	// Проверяем максимальное количество попыток
	assert.Equal(t, maxAttempts, strategy.MaxAttempts())

	// Проверяем пользовательскую функцию задержки
	assert.Equal(t, 0*time.Second, strategy.NextDelay(0, nil))
	assert.Equal(t, 1*time.Second, strategy.NextDelay(1, nil))
	assert.Equal(t, 2*time.Second, strategy.NextDelay(2, nil))

	// Проверяем пользовательскую функцию условий повтора
	assert.True(t, strategy.ShouldRetry(nil, assert.AnError))

	resp503 := &http.Response{StatusCode: http.StatusServiceUnavailable}
	assert.True(t, strategy.ShouldRetry(resp503, nil))

	resp500 := &http.Response{StatusCode: http.StatusInternalServerError}
	assert.False(t, strategy.ShouldRetry(resp500, nil))

	resp200 := &http.Response{StatusCode: http.StatusOK}
	assert.False(t, strategy.ShouldRetry(resp200, nil))
}

// TestCustomRetryStrategyWithError проверяет передачу ошибки в функцию задержки
// Проверяет что последняя ошибка передается для анализа при расчете задержки
func TestCustomRetryStrategyWithError(t *testing.T) {
	// Проверяем что lastErr передается в функцию задержки
	var capturedErr error
	delayFunc := func(attempt int, lastErr error) time.Duration {
		capturedErr = lastErr
		return 1 * time.Second
	}

	shouldRetryFunc := func(resp *http.Response, err error) bool {
		return err != nil
	}

	strategy := NewCustomRetryStrategy(3, shouldRetryFunc, delayFunc)

	testErr := assert.AnError
	strategy.NextDelay(1, testErr)

	assert.Equal(t, testErr, capturedErr)
}

// BenchmarkExponentialBackoffNextDelay бенчмарк производительности расчета задержки
// Измеряет скорость вычисления экспоненциальной задержки
func BenchmarkExponentialBackoffNextDelay(b *testing.B) {
	strategy := NewExponentialBackoffStrategy(10, 1*time.Second, 30*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.NextDelay(i%10, nil)
	}
}

// BenchmarkExponentialBackoffShouldRetry бенчмарк производительности решения о повторе
// Измеряет скорость принятия решения нужен ли повтор запроса
func BenchmarkExponentialBackoffShouldRetry(b *testing.B) {
	strategy := NewExponentialBackoffStrategy(10, 1*time.Second, 30*time.Second)
	resp := &http.Response{StatusCode: http.StatusInternalServerError}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.ShouldRetry(resp, nil)
	}
}

func BenchmarkFixedDelayNextDelay(b *testing.B) {
	strategy := NewFixedDelayStrategy(10, 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.NextDelay(i, nil)
	}
}

func TestRetryStrategyEdgeCases(t *testing.T) {
	t.Run("exponential backoff with zero base delay", func(t *testing.T) {
		strategy := NewExponentialBackoffStrategy(3, 0, 10*time.Second)
		delay := strategy.NextDelay(1, nil)
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("exponential backoff with max delay smaller than base", func(t *testing.T) {
		strategy := NewExponentialBackoffStrategy(3, 5*time.Second, 2*time.Second)
		delay := strategy.NextDelay(1, nil)
		assert.Equal(t, 2*time.Second, delay) // Should cap at max delay
	})

	t.Run("fixed delay with zero delay", func(t *testing.T) {
		strategy := NewFixedDelayStrategy(3, 0)
		delay := strategy.NextDelay(1, nil)
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("should retry with nil response and nil error", func(t *testing.T) {
		strategy := NewExponentialBackoffStrategy(3, 1*time.Second, 10*time.Second)
		result := strategy.ShouldRetry(nil, nil)
		assert.False(t, result)
	})
}

func TestRetryStrategyThreadSafety(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(5, 1*time.Second, 10*time.Second)

	// Run multiple goroutines concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(attempt int) {
			defer func() { done <- true }()

			// These operations should be thread-safe
			strategy.NextDelay(attempt%5, nil)
			strategy.ShouldRetry(&http.Response{StatusCode: 500}, nil)
			strategy.MaxAttempts()
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// If we reach here without data races, the test passes
	assert.True(t, true)
}
