package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"crypto/tls"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleCircuitBreakerDefaultConfig tests creation of circuit breaker with default settings
// Verifies that initial state is "closed" (allows requests)
func TestSimpleCircuitBreakerDefaultConfig(t *testing.T) {
	t.Parallel()

	cb := NewSimpleCircuitBreaker()

	assert.Equal(t, CircuitBreakerClosed, cb.State())
}

// TestSimpleCircuitBreakerWithConfig tests creation of circuit breaker with custom configuration
// Verifies that callback function for tracking state changes works correctly
func TestSimpleCircuitBreakerWithConfig(t *testing.T) {
	t.Parallel()

	var stateChanges []string

	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
		OnStateChange: func(from, to CircuitBreakerState) {
			stateChanges = append(stateChanges, from.String()+"->"+to.String())
		},
	}

	cb := NewCircuitBreakerWithConfig(config)

	assert.Equal(t, CircuitBreakerClosed, cb.State())
	assert.Empty(t, stateChanges)
}

// TestCircuitBreakerStateTransitions tests state transitions of circuit breaker
// Tests full cycle: Closed -> Open -> HalfOpen -> Closed
// Verifies that callback functions are called on each state transition
func TestCircuitBreakerStateTransitions(t *testing.T) {
	// NOT parallel - test with time.Sleep and state changes
	var stateChanges []string

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
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
	time.Sleep(15 * time.Millisecond)

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

// TestCircuitBreakerFailureRecovery tests recovery after failures
// Tests that breaker requires multiple successful requests to transition to closed state
// when success threshold is greater than 1
func TestCircuitBreakerFailureRecovery(t *testing.T) {
	// НЕ parallel - тест с time.Sleep
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Инициируем сбой для открытия выключателя
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Ждем истечения таймаута
	time.Sleep(110 * time.Millisecond)

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

// TestCircuitBreakerHalfOpenFailure tests behavior on failure in half-open state
// Verifies that failure in half-open state returns breaker to open state
func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	// НЕ parallel - тест с time.Sleep
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreakerWithConfig(config)

	// Открываем выключатель
	cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Ждем истечения таймаута
	time.Sleep(110 * time.Millisecond)

	// Сбой в полуоткрытом состоянии - должен вернуться в открытое
	_, err := cb.Execute(func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, CircuitBreakerOpen, cb.State())
}

// TestCircuitBreakerReset tests forced reset of circuit breaker
// Verifies that Reset() method returns breaker to closed state
func TestCircuitBreakerReset(t *testing.T) {
	t.Parallel()

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

// TestCircuitBreakerIsSuccess tests determination of response success
// Verifies that 2xx codes are considered successful, while 5xx and 4xx are not
func TestCircuitBreakerIsSuccess(t *testing.T) {
	t.Parallel()

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
	// НЕ parallel - тест concurrency
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          10 * time.Millisecond,
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
	t.Parallel()

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
	t.Parallel()

	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
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
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCircuitBreakerStateString(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	// НЕ parallel - тест с time.Sleep
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
	time.Sleep(timeout + 1*time.Millisecond)

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
	t.Parallel()

	// Test with edge case configurations
	t.Run("zero thresholds", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 0,
			SuccessThreshold: 0,
			Timeout:          10 * time.Millisecond,
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

// TestSimpleCircuitBreaker_ReturnsOriginalResponseWhileOpenAndRecovers starts HTTP server,
// simulates sequence: successes -> long 429 -> successes, and verifies that:
//   - while breaker is in Open state, Execute returns a clone of original unsuccessful response (429)
//     and ErrCircuitBreakerOpen error, while actual request to server is not executed;
//   - after error ends and timeout expires, breaker transitions to Half-Open/Closed and responses are successful again.
func TestSimpleCircuitBreaker_ReturnsOriginalResponseWhileOpenAndRecovers(t *testing.T) {
	// NOT parallel - uses time.Sleep and shared test server
	var (
		mode atomic.Value // string: "ok" | "fail"
		hits int64
	)
	mode.Store("ok")

	const (
		failBody = "too many requests"
		okBody   = "ok"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		if mode.Load().(string) == "fail" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(failBody))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okBody))
	}))
	defer server.Close()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          20 * time.Millisecond,
	})

	doRequest := func() (*http.Response, error) {
		return http.Get(server.URL)
	}

	// Пара успешных запросов
	for i := 0; i < 2; i++ {
		resp, err := cb.Execute(doRequest)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Переводим сервер в режим ошибок (429)
	mode.Store("fail")

	// Первый неуспешный запрос выполняется реально и открывает выключатель
	respFail, err := cb.Execute(doRequest)
	require.NoError(t, err)
	require.NotNil(t, respFail)
	assert.Equal(t, http.StatusTooManyRequests, respFail.StatusCode)
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Пока выключатель открыт, следующий вызов должен вернуть клон последнего неуспешного ответа
	prevHits := atomic.LoadInt64(&hits)
	respWhileOpen, err := cb.Execute(doRequest)
	assert.Error(t, err)
	assert.Equal(t, ErrCircuitBreakerOpen, err)
	require.NotNil(t, respWhileOpen)
	assert.Equal(t, http.StatusTooManyRequests, respWhileOpen.StatusCode)
	body, readErr := io.ReadAll(respWhileOpen.Body)
	require.NoError(t, readErr)
	assert.Equal(t, failBody, string(body))
	// Убедимся, что реальный запрос не ушёл на сервер
	assert.Equal(t, prevHits, atomic.LoadInt64(&hits))

	// Ошибка пропадает: возвращаем сервер в OK и ждём таймаут для перехода в Half-Open
	mode.Store("ok")
	time.Sleep(25 * time.Millisecond)

	// Попытка в Half-Open должна быть успешной и немедленно закрыть выключатель (SuccessThreshold=1)
	respRecover, err := cb.Execute(doRequest)
	require.NoError(t, err)
	require.NotNil(t, respRecover)
	assert.Equal(t, http.StatusOK, respRecover.StatusCode)
	assert.Equal(t, CircuitBreakerClosed, cb.State())

	// Дополнительная проверка: последующие ответы также успешные
	respOk, err := cb.Execute(doRequest)
	require.NoError(t, err)
	require.NotNil(t, respOk)
	assert.Equal(t, http.StatusOK, respOk.StatusCode)
}

func TestCloneHttpResponse_EmptyResponse(t *testing.T) {
	t.Parallel()

	cb := NewSimpleCircuitBreaker()

	// Create an empty http.Response
	emptyResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Trailer:    make(http.Header),
		Body:       io.NopCloser(bytes.NewBuffer(nil)),
	}

	// Clone the empty response
	clonedResp := cb.cloneHTTPResponse(emptyResp)

	// Verify that the cloned response is not nil
	require.NotNil(t, clonedResp)

	// Verify that the cloned response has the same status code
	assert.Equal(t, emptyResp.StatusCode, clonedResp.StatusCode)

	// Verify that the cloned response has no headers
	assert.Empty(t, clonedResp.Header)

	// Verify that the cloned response has no trailers
	assert.Empty(t, clonedResp.Trailer)

	// Verify that the cloned response body is empty
	clonedBody, err := io.ReadAll(clonedResp.Body)
	require.NoError(t, err)
	assert.Empty(t, clonedBody)
}

func TestCloneHttpResponseWithNilBody(t *testing.T) {
	t.Parallel()

	cb := NewSimpleCircuitBreaker()

	// Create a response with a nil body
	originalResp := &http.Response{
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       nil, // nil body
	}

	// Clone the response
	clonedResp := cb.cloneHTTPResponse(originalResp)

	// Verify that the cloned response has the same status code and headers
	assert.Equal(t, originalResp.StatusCode, clonedResp.StatusCode)
	assert.Equal(t, originalResp.Proto, clonedResp.Proto)
	assert.Equal(t, originalResp.ProtoMajor, clonedResp.ProtoMajor)
	assert.Equal(t, originalResp.ProtoMinor, clonedResp.ProtoMinor)
	assert.Equal(t, originalResp.Header, clonedResp.Header)

	// Verify that the body is also nil in the cloned response
	assert.Nil(t, clonedResp.Body)
}

func TestCloneHttpResponseWithMultipleHeaders(t *testing.T) {
	t.Parallel()

	// Create a response with multiple headers having the same key
	originalResp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Set-Cookie": {"cookie1=value1", "cookie2=value2"},
			"X-Custom":   {"value1", "value2"},
		},
		Body: io.NopCloser(bytes.NewBufferString("response body")),
	}

	cb := NewSimpleCircuitBreaker()

	// Clone the response
	clonedResp := cb.cloneHTTPResponse(originalResp)

	// Verify that the headers are correctly cloned
	assert.Equal(t, originalResp.Header, clonedResp.Header, "Headers should be equal")
	assert.Equal(t, originalResp.StatusCode, clonedResp.StatusCode, "Status codes should be equal")

	// Verify that the body is correctly cloned
	clonedBody, err := io.ReadAll(clonedResp.Body)
	require.NoError(t, err)
	assert.Equal(t, "response body", string(clonedBody), "Body content should be equal")
}

func TestCloneHttpResponseWithMultipleTrailers(t *testing.T) {
	t.Parallel()

	// Create a response with multiple trailers having the same key
	originalResp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("response body")),
		Trailer: http.Header{
			"X-Custom-Trailer": []string{"value1", "value2"},
		},
	}

	cb := NewSimpleCircuitBreaker()
	clonedResp := cb.cloneHTTPResponse(originalResp)

	// Check that the trailers are cloned correctly
	assert.Equal(t, originalResp.Trailer, clonedResp.Trailer, "Trailers should be cloned correctly")
	assert.Equal(t, []string{"value1", "value2"}, clonedResp.Trailer["X-Custom-Trailer"], "Trailer values should match")
}

func TestCloneHttpResponseWithClosedBody(t *testing.T) {
	t.Parallel()

	// Use a real net/http response to get proper read-after-close semantics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("original body"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Close the original body to simulate "closed body" state
	require.NoError(t, resp.Body.Close())

	cb := NewSimpleCircuitBreaker()
	clonedResponse := cb.cloneHTTPResponse(resp)

	require.NotNil(t, clonedResponse)
	assert.Equal(t, http.StatusOK, clonedResponse.StatusCode)
	assert.Equal(t, "1", clonedResponse.Header.Get("X-Test"))

	// Since source body is already closed, cloneHTTPResponse cannot read it
	// and should create an empty body
	assert.NotNil(t, clonedResponse.Body)
	// The body should be empty since the original was closed
	body, err := io.ReadAll(clonedResponse.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestCloneHttpResponseWithTLS(t *testing.T) {
	t.Parallel()

	// Create a response with a non-nil TLS field
	originalResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("response body")),
		TLS:        &tls.ConnectionState{},
	}

	cb := NewSimpleCircuitBreaker()
	clonedResp := cb.cloneHTTPResponse(originalResp)

	// Verify that the cloned response has the same TLS field
	assert.NotNil(t, clonedResp.TLS, "Expected TLS field to be non-nil")
	assert.Equal(t, originalResp.TLS, clonedResp.TLS, "Expected TLS fields to be equal")
}

func TestCloneHttpResponseWithBodyReadError(t *testing.T) {
	t.Parallel()

	// Create a response with a body that will cause an error on read
	body := io.NopCloser(&errorReader{})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	cb := NewSimpleCircuitBreaker()

	// Clone the response
	clonedResp := cb.cloneHTTPResponse(resp)

	// Verify that the clone is not nil and has the same status code
	assert.NotNil(t, clonedResp, "Cloned response should not be nil")
	assert.Equal(t, resp.StatusCode, clonedResp.StatusCode, "Status code should match")

	// Verify that headers are copied correctly
	assert.Equal(t, resp.Header, clonedResp.Header, "Headers should match")

	// Verify that the body is empty due to read error
	assert.NotNil(t, clonedResp.Body, "Cloned response body should not be nil")
	// The body should be empty since reading failed
	clonedBody, err := io.ReadAll(clonedResp.Body)
	require.NoError(t, err)
	assert.Empty(t, clonedBody, "Cloned response body should be empty due to read error")
}

func TestCloneHttpResponseWithNonNilRequest(t *testing.T) {
	t.Parallel()

	originalRequest, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	originalResponse := &http.Response{
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"key": "value"}`)),
		Request:    originalRequest,
	}

	cb := NewSimpleCircuitBreaker()
	clonedResponse := cb.cloneHTTPResponse(originalResponse)

	assert.Equal(t, originalResponse.StatusCode, clonedResponse.StatusCode)
	assert.Equal(t, originalResponse.Proto, clonedResponse.Proto)
	assert.Equal(t, originalResponse.ProtoMajor, clonedResponse.ProtoMajor)
	assert.Equal(t, originalResponse.ProtoMinor, clonedResponse.ProtoMinor)
	assert.Equal(t, originalResponse.Header, clonedResponse.Header)

	clonedBody, err := io.ReadAll(clonedResponse.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"key": "value"}`, string(clonedBody))

	assert.Equal(t, originalResponse.Request, clonedResponse.Request)
}

func TestCloneHttpResponseWithContentLength(t *testing.T) {
	t.Parallel()

	originalBody := "This is a test response body"
	originalResp := &http.Response{
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"application/json"}},
		ContentLength: int64(len(originalBody)),
		Body:          io.NopCloser(strings.NewReader(originalBody)),
	}

	cb := NewSimpleCircuitBreaker()
	clonedResp := cb.cloneHTTPResponse(originalResp)

	assert.Equal(t, originalResp.StatusCode, clonedResp.StatusCode)
	assert.Equal(t, originalResp.Proto, clonedResp.Proto)
	assert.Equal(t, originalResp.ProtoMajor, clonedResp.ProtoMajor)
	assert.Equal(t, originalResp.ProtoMinor, clonedResp.ProtoMinor)
	assert.Equal(t, originalResp.ContentLength, clonedResp.ContentLength)
	assert.Equal(t, originalResp.Header, clonedResp.Header)

	clonedBody, err := io.ReadAll(clonedResp.Body)
	require.NoError(t, err)
	assert.Equal(t, originalBody, string(clonedBody))
}

func TestCloneHttpResponseWithChunkedTransferEncoding(t *testing.T) {
	t.Parallel()

	// Create a response with TransferEncoding set to chunked
	originalResp := &http.Response{
		StatusCode:       http.StatusOK,
		Proto:            "HTTP/1.1",
		ProtoMajor:       1,
		ProtoMinor:       1,
		Header:           http.Header{"Content-Type": {"application/json"}},
		ContentLength:    -1, // Indicates chunked transfer
		TransferEncoding: []string{"chunked"},
		Body:             io.NopCloser(bytes.NewBufferString(`{"key": "value"}`)),
	}

	cb := NewSimpleCircuitBreaker()

	// Clone the response
	clonedResp := cb.cloneHTTPResponse(originalResp)

	// Verify that the cloned response has the same TransferEncoding
	assert.Equal(t, originalResp.TransferEncoding, clonedResp.TransferEncoding, "TransferEncoding should be 'chunked'")

	// Verify that the body content is the same
	originalBody, _ := io.ReadAll(originalResp.Body)
	clonedBody, _ := io.ReadAll(clonedResp.Body)
	assert.Equal(t, originalBody, clonedBody, "Body content should be the same")

	// Verify that other fields are cloned correctly
	assert.Equal(t, originalResp.StatusCode, clonedResp.StatusCode)
	assert.Equal(t, originalResp.Proto, clonedResp.Proto)
	assert.Equal(t, originalResp.ProtoMajor, clonedResp.ProtoMajor)
	assert.Equal(t, originalResp.ProtoMinor, clonedResp.ProtoMinor)
	assert.Equal(t, originalResp.Header, clonedResp.Header)
}

// errorReader is a helper type that implements io.Reader and always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
