//go:build integration

package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC021: Rate limiter with retry - each retry waits for a token.
func TestRateLimiterWithRetryIntegration(t *testing.T) {
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError) // First 2 requests fail.
			return
		}
		w.WriteHeader(http.StatusOK) // 3rd request succeeds.
	}))
	defer server.Close()

	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 2.0, // 2 requests per second.
			BurstCapacity:     2,   // 2 tokens in bucket.
		},
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}

	client := New(config, "test-client")
	defer client.Close()

	// First request - should use 3 tokens (3 attempts) and eventually succeed.
	resp, err := client.Get(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Should make 3 attempts (2 failed + 1 successful).
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount)

	// After using 2 tokens (burst capacity), third request should wait.
	// Since rate = 2.0 requests/sec, wait should be at least 500ms.
	atomic.StoreInt32(&serverCallCount, 0) // Reset counter.

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	// Next request should wait for token.
	start := time.Now()
	resp2, err2 := client.Get(context.Background(), server2.URL)
	elapsed2 := time.Since(start)

	require.NoError(t, err2)
	resp2.Body.Close()

	// Should wait at least 400ms (considering 0 tokens remaining).
	assert.GreaterOrEqual(t, elapsed2, 400*time.Millisecond,
		"Expected rate limiter to delay request, but got %v", elapsed2)
}

// TC022: Rate limiter with circuit breaker - open CB does not consume tokens.
func TestRateLimiterWithCircuitBreakerIntegration(t *testing.T) {
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Always fails.
	}))
	defer server.Close()

	cbConfig := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 2.0, // Slow rate limiter for testing.
			BurstCapacity:     2,   // Only 2 tokens.
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       NewCircuitBreakerWithConfig(cbConfig),
		RetryEnabled:         false, // Disable retry to focus on CB.
	}

	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// First 2 requests open circuit breaker and use 2 tokens.
	resp1, _ := client.Get(ctx, server.URL)
	if resp1 != nil {
		resp1.Body.Close()
	}
	resp2, _ := client.Get(ctx, server.URL)
	if resp2 != nil {
		resp2.Body.Close()
	}

	// Circuit breaker should be open.
	assert.Equal(t, CircuitBreakerOpen, config.CircuitBreaker.State())

	// Check that subsequent requests are quickly blocked by CB.
	for i := 0; i < 3; i++ {
		start := time.Now()
		resp, err := client.Get(ctx, server.URL)
		elapsed := time.Since(start)

		// Requests should quickly return with CB error.
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
		assert.Less(t, elapsed, 50*time.Millisecond)

		if resp != nil {
			resp.Body.Close()
		}
	}

	// Main test: check that after CB recovery tokens are preserved.
	// Wait for circuit breaker recovery.
	time.Sleep(150 * time.Millisecond)

	// Create successful server for CB recovery.
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer successServer.Close()

	// First request after recovery should pass.
	resp3, err3 := client.Get(ctx, successServer.URL)
	require.NoError(t, err3)
	resp3.Body.Close()
	assert.Equal(t, CircuitBreakerClosed, config.CircuitBreaker.State())

	// Next request should wait for token.
	// By this time we've spent 3 tokens (2 on CB + 1 on recovery).
	start := time.Now()
	resp4, err4 := client.Get(ctx, successServer.URL)
	elapsed4 := time.Since(start)

	require.NoError(t, err4)
	resp4.Body.Close()

	// Should wait for token (rate = 2.0/sec, need at least ~500ms).
	assert.GreaterOrEqual(t, elapsed4, 400*time.Millisecond,
		"Rate limiter should delay request after tokens are consumed")
}

// TC023: Full integration of rate limiter + retry + circuit breaker.
func TestRateLimiterWithRetryAndCircuitBreakerIntegration(t *testing.T) {
	// Create scenario: server first unavailable (opens CB),
	// then recovers, but rate limiter controls frequency.
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count <= 4 {
			w.WriteHeader(http.StatusInternalServerError) // First 4 fail.
			return
		}
		w.WriteHeader(http.StatusOK) // Then succeed.
	}))
	defer server.Close()

	cbConfig := CircuitBreakerConfig{
		FailureThreshold: 2, // Will open after 2 failures.
		SuccessThreshold: 1,
		Timeout:          200 * time.Millisecond, // Fast recovery.
	}

	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 3.0, // 3 requests per second.
			BurstCapacity:     2,   // Only 2 tokens in burst.
		},
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      2, // Only 2 attempts.
			BaseDelay:        20 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       NewCircuitBreakerWithConfig(cbConfig),
	}

	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Первый запрос: rate limiter пропустит (1-й токен), retry сделает 2 попытки,
	// что откроет circuit breaker.
	resp1, err1 := client.Get(ctx, server.URL)
	if err1 == nil {
		resp1.Body.Close()
	}

	// CB должен быть открыт после первого запроса (2 неудачные попытки).
	assert.Equal(t, CircuitBreakerOpen, config.CircuitBreaker.State())

	// Второй запрос: rate limiter пропустит (2-й токен), но CB заблокирует.
	start := time.Now()
	resp2, err2 := client.Get(ctx, server.URL)
	elapsed := time.Since(start)

	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "circuit breaker is open")
	assert.Less(t, elapsed, 50*time.Millisecond, "CB should fail fast")
	if resp2 != nil {
		resp2.Body.Close()
	}

	// Третий запрос: CB всё ещё открыт, должен быстро фейлиться.
	start3 := time.Now()
	resp3, err3 := client.Get(ctx, server.URL)
	elapsed3 := time.Since(start3)

	assert.Error(t, err3)
	// Ожидаем ошибку от CB (rate limiter может либо пропустить, либо нет).
	assert.Contains(t, err3.Error(), "circuit breaker is open")
	assert.Less(t, elapsed3, 100*time.Millisecond, "CB should fail fast")
	if resp3 != nil {
		resp3.Body.Close()
	}

	// Ждем восстановления CB.
	time.Sleep(250 * time.Millisecond)

	// Проверяем что сервер может восстановиться через все компоненты.
	atomic.StoreInt32(&serverCallCount, 10) // "Починиваем" сервер.

	// Следующий запрос должен пройти (CB перейдет в half-open, сервер вернет 200).
	resp4, err4 := client.Get(ctx, server.URL)
	require.NoError(t, err4, "Expected successful request after recovery")
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	resp4.Body.Close()

	// CB должен закрыться после успешного запроса.
	assert.Equal(t, CircuitBreakerClosed, config.CircuitBreaker.State())
}
