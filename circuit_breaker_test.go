package httpclient

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleCircuitBreakerDefaultConfig проверяет создание автоматического выключателя с настройками по умолчанию
// Проверяет что начальное состояние - "закрытое" (пропускает запросы)
func TestSimpleCircuitBreakerDefaultConfig(t *testing.T) {
	cb := NewSimpleCircuitBreaker()

	assert.Equal(t, CircuitBreakerClosed, cb.State())
}

// TestSimpleCircuitBreakerWithConfig проверяет создание автоматического выключателя с пользовательской конфигурацией
// Проверяет что callback функция для отслеживания изменений состояний работает корректно
func TestSimpleCircuitBreakerWithConfig(t *testing.T) {
	var stateChanges []string

	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		OnStateChange: func(from, to CircuitBreakerState) {
			stateChanges = append(stateChanges, from.String()+"->"+to.String())
		},
	}

	cb := NewCircuitBreakerWithConfig(config)

	assert.Equal(t, CircuitBreakerClosed, cb.State())
	assert.Empty(t, stateChanges)
}

// TestCircuitBreakerStateTransitions проверяет переходы между состояниями автоматического выключателя
// Тестирует полный цикл: Закрыт -> Открыт -> Полуоткрыт -> Закрыт
// Проверяет что callback функции вызываются при каждом переходе состояния
func TestCircuitBreakerStateTransitions(t *testing.T) {
	var stateChanges []string

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		OnStateChange: func(from, to CircuitBreakerState) {
			stateChanges = append(stateChanges, from.String()+"->"+to.String())
		},
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Изначально закрыт
	assert.Equal(t, CircuitBreakerClosed, cb.State())

	// Первый сбой - должен остаться закрытым
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerClosed, cb.State())

	// Второй сбой - должен открыться
	_, err = cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Должен немедленно возвращать ошибку когда открыт
	_, err = cb.Execute(func() (*http.Response, error) {
		t.Fatal("Функция не должна вызываться когда выключатель открыт")
		return nil, nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)

	// Ждем истечения таймаута
	time.Sleep(150 * time.Millisecond)

	// Должен перейти в полуоткрытое состояние при следующем вызове
	_, err = cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerClosed, cb.State()) // Должен закрыться сразу с порогом успеха = 1

	// Проверяем изменения состояний
	expected := []string{"closed->open", "open->half-open", "half-open->closed"}
	assert.Equal(t, expected, stateChanges)
}

// TestCircuitBreakerFailureRecovery проверяет восстановление после сбоев
// Тестирует что выключатель требует несколько успешных запросов для перехода в закрытое состояние
// когда порог успеха больше 1
func TestCircuitBreakerFailureRecovery(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Инициируем сбой для открытия выключателя
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Ждем истечения таймаута
	time.Sleep(60 * time.Millisecond)

	// Первый успех в полуоткрытом состоянии - должен остаться полуоткрытым
	_, err = cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerHalfOpen, cb.State())

	// Второй успех - должен закрыться
	_, err = cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerClosed, cb.State())
}

// TestCircuitBreakerHalfOpenFailure проверяет поведение при сбое в полуоткрытом состоянии
// Проверяет что сбой в полуоткрытом состоянии возвращает выключатель в открытое состояние
func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Открываем выключатель
	cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Ждем истечения таймаута
	time.Sleep(60 * time.Millisecond)

	// Сбой в полуоткрытом состоянии - должен вернуться в открытое
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerOpen, cb.State())
}

// TestCircuitBreakerReset проверяет принудительный сброс выключателя
// Проверяет что метод Reset() возвращает выключатель в закрытое состояние
func TestCircuitBreakerReset(t *testing.T) {
	cb := NewSimpleCircuitBreaker()

	// Открываем выключатель
	for i := 0; i < 6; i++ {
		cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError}, nil
		})
	}
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Reset должен закрыть выключатель
	cb.Reset()
	assert.Equal(t, CircuitBreakerClosed, cb.State())

	// Должна быть возможность снова выполнять функции
	resp, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCircuitBreakerIsSuccess проверяет определение успешности ответа
// Проверяет что 2xx и 4xx коды считаются успешными, а 5xx - нет
func TestCircuitBreakerIsSuccess(t *testing.T) {
	cb := NewSimpleCircuitBreaker()

	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{
			name:     "nil response and error",
			expected: false,
		},
		{
			name:     "error present",
			err:      assert.AnError,
			expected: false,
		},
		{
			name:     "nil response",
			resp:     nil,
			expected: false,
		},
		{
			name:     "200 OK",
			resp:     &http.Response{StatusCode: http.StatusOK},
			expected: true,
		},
		{
			name:     "404 Not Found",
			resp:     &http.Response{StatusCode: http.StatusNotFound},
			expected: true,
		},
		{
			name:     "500 Internal Server Error",
			resp:     &http.Response{StatusCode: http.StatusInternalServerError},
			expected: false,
		},
		{
			name:     "502 Bad Gateway",
			resp:     &http.Response{StatusCode: http.StatusBadGateway},
			expected: false,
		},
		{
			name:     "503 Service Unavailable",
			resp:     &http.Response{StatusCode: http.StatusServiceUnavailable},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cb.isSuccess(tt.resp, tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreakerWithConfig(config)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Run multiple goroutines concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Randomly succeed or fail
			statusCode := http.StatusOK
			if id%3 == 0 {
				statusCode = http.StatusInternalServerError
			}

			cb.Execute(func() (*http.Response, error) {
				return &http.Response{StatusCode: statusCode}, nil
			})
		}(i)
	}

	wg.Wait()

	// Circuit breaker should still be in a valid state
	state := cb.State()
	assert.True(t, state == CircuitBreakerClosed || state == CircuitBreakerOpen || state == CircuitBreakerHalfOpen)
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	cb := NewSimpleCircuitBreaker()
	middleware := NewCircuitBreakerMiddleware(cb)

	req := &http.Request{}

	// Should execute normally when closed
	called := false
	resp, err := middleware.Process(req, func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCircuitBreakerMiddlewareWithOpenCircuit(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreakerWithConfig(config)
	middleware := NewCircuitBreakerMiddleware(cb)

	req := &http.Request{}

	// Open the circuit
	cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Should return circuit breaker error
	called := false
	resp, err := middleware.Process(req, func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	assert.Error(t, err)
	assert.Equal(t, ErrCircuitBreakerOpen, err)
	assert.False(t, called)
	assert.Nil(t, resp)
}

func TestCircuitBreakerStateString(t *testing.T) {
	tests := []struct {
		state    CircuitBreakerState
		expected string
	}{
		{CircuitBreakerClosed, "closed"},
		{CircuitBreakerOpen, "open"},
		{CircuitBreakerHalfOpen, "half-open"},
		{CircuitBreakerState(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCircuitBreakerExecuteWithPanic(t *testing.T) {
	cb := NewSimpleCircuitBreaker()

	// Test that panics in executed functions don't break the circuit breaker
	assert.Panics(t, func() {
		cb.Execute(func() (*http.Response, error) {
			panic("test panic")
		})
	})

	// Circuit breaker should still work normally
	assert.Equal(t, CircuitBreakerClosed, cb.State())

	resp, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCircuitBreakerTimeoutPrecision(t *testing.T) {
	timeout := 10 * time.Millisecond
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          timeout,
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Open circuit
	cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Should still be open before timeout
	start := time.Now()
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)
	assert.True(t, time.Since(start) < timeout/2)

	// Wait for timeout
	time.Sleep(timeout + 5*time.Millisecond)

	// Should transition to half-open
	_, err = cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerClosed, cb.State())
}

func BenchmarkCircuitBreakerExecute(b *testing.B) {
	cb := NewSimpleCircuitBreaker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
	}
}

func BenchmarkCircuitBreakerExecuteWithFailures(b *testing.B) {
	cb := NewSimpleCircuitBreaker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		statusCode := http.StatusOK
		if i%10 == 0 {
			statusCode = http.StatusInternalServerError
		}

		cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: statusCode}, nil
		})
	}
}

func BenchmarkCircuitBreakerState(b *testing.B) {
	cb := NewSimpleCircuitBreaker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.State()
	}
}

func TestCircuitBreakerConfigValidation(t *testing.T) {
	// Test with edge case configurations
	t.Run("zero thresholds", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 0,
			SuccessThreshold: 0,
			Timeout:          1 * time.Second,
		}

		cb := NewCircuitBreakerWithConfig(config)
		assert.Equal(t, CircuitBreakerClosed, cb.State())

		// Should never open with 0 failure threshold
		cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError}, nil
		})
		assert.Equal(t, CircuitBreakerClosed, cb.State())
	})

	t.Run("zero timeout", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 1,
			SuccessThreshold: 1,
			Timeout:          0,
		}

		cb := NewCircuitBreakerWithConfig(config)

		// Open circuit
		cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError}, nil
		})
		assert.Equal(t, CircuitBreakerOpen, cb.State())

		// Should immediately allow execution (no timeout)
		_, err := cb.Execute(func() (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, CircuitBreakerClosed, cb.State())
	})
}
