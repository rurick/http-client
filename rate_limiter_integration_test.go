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

// TC021: Rate limiter с retry - каждый повтор ждет токена.
func TestRateLimiterWithRetryIntegration(t *testing.T) {
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError) // Первые 2 запроса неудачны.
			return
		}
		w.WriteHeader(http.StatusOK) // 3-й запрос успешен.
	}))
	defer server.Close()

	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 2.0, // 2 запроса в секунду.
			BurstCapacity:     2,   // 2 токена в корзине.
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

	// Первый запрос - должен использовать 3 токена (3 попытки) и в итоге успешен.
	resp, err := client.Get(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Должно быть сделано 3 попытки (2 неудачных + 1 успешная).
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount)

	// После использования 2 токенов (burst capacity), третий запрос должен ждать.
	// Поскольку rate = 2.0 запроса/сек, ожидание должно быть минимум 500ms.
	atomic.StoreInt32(&serverCallCount, 0) // Сброс счетчика.

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	// Следующий запрос должен ждать токена.
	start := time.Now()
	resp2, err2 := client.Get(context.Background(), server2.URL)
	elapsed2 := time.Since(start)

	require.NoError(t, err2)
	resp2.Body.Close()

	// Должен ждать минимум 400ms (учитывая что осталось 0 токенов).
	assert.GreaterOrEqual(t, elapsed2, 400*time.Millisecond,
		"Expected rate limiter to delay request, but got %v", elapsed2)
}

// TC022: Rate limiter с circuit breaker - открытый CB не тратит токены.
func TestRateLimiterWithCircuitBreakerIntegration(t *testing.T) {
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Всегда неудачен.
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
			RequestsPerSecond: 2.0, // Медленный rate limiter для проверки.
			BurstCapacity:     2,   // Только 2 токена.
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       NewCircuitBreakerWithConfig(cbConfig),
		RetryEnabled:         false, // Отключаем retry для фокуса на CB.
	}

	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Первые 2 запроса открывают circuit breaker и используют 2 токена.
	resp1, _ := client.Get(ctx, server.URL)
	if resp1 != nil {
		resp1.Body.Close()
	}
	resp2, _ := client.Get(ctx, server.URL)
	if resp2 != nil {
		resp2.Body.Close()
	}

	// Circuit breaker должен быть открыт.
	assert.Equal(t, CircuitBreakerOpen, config.CircuitBreaker.State())

	// Проверяем, что последующие запросы быстро блокируются CB.
	for i := 0; i < 3; i++ {
		start := time.Now()
		resp, err := client.Get(ctx, server.URL)
		elapsed := time.Since(start)

		// Запросы должны быстро возвращаться с ошибкой CB.
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
		assert.Less(t, elapsed, 50*time.Millisecond)

		if resp != nil {
			resp.Body.Close()
		}
	}

	// Основной тест: проверяем что после восстановления CB токены сохранены.
	// Ждем восстановления circuit breaker.
	time.Sleep(150 * time.Millisecond)

	// Создаем успешный сервер для восстановления CB.
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer successServer.Close()

	// Первый запрос после восстановления должен пройти.
	resp3, err3 := client.Get(ctx, successServer.URL)
	require.NoError(t, err3)
	resp3.Body.Close()
	assert.Equal(t, CircuitBreakerClosed, config.CircuitBreaker.State())

	// Следующий запрос должен ждать токена.
	// К этому моменту у нас потрачено 3 токена (2 на CB + 1 на восстановление).
	start := time.Now()
	resp4, err4 := client.Get(ctx, successServer.URL)
	elapsed4 := time.Since(start)

	require.NoError(t, err4)
	resp4.Body.Close()

	// Должен ждать токена (rate = 2.0/sec, нужно минимум ~500ms).
	assert.GreaterOrEqual(t, elapsed4, 400*time.Millisecond,
		"Rate limiter should delay request after tokens are consumed")
}

// TC023: Полная интеграция rate limiter + retry + circuit breaker.
func TestRateLimiterWithRetryAndCircuitBreakerIntegration(t *testing.T) {
	// Создаем сценарий: сервер сначала недоступен (открывает CB),
	// потом восстанавливается, но rate limiter контролирует частоту.
	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count <= 4 {
			w.WriteHeader(http.StatusInternalServerError) // Первые 4 неудачны.
			return
		}
		w.WriteHeader(http.StatusOK) // Потом успешны.
	}))
	defer server.Close()

	cbConfig := CircuitBreakerConfig{
		FailureThreshold: 2, // Откроется после 2 неудач.
		SuccessThreshold: 1,
		Timeout:          200 * time.Millisecond, // Быстрое восстановление.
	}

	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 3.0, // 3 запроса в секунду.
			BurstCapacity:     2,   // Только 2 токена в burst.
		},
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      2, // Только 2 попытки.
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
