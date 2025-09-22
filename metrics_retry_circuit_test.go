package httpclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsWithRetryPolicy проверяет что при retry политике все метрики записываются корректно
func TestMetricsWithRetryPolicy(t *testing.T) {

	// Создаём тестовый сервер который сначала возвращает ошибки, потом успех
	server := NewTestServer(
		TestResponse{StatusCode: 500, Body: `{"error": "server error 1"}`},
		TestResponse{StatusCode: 503, Body: `{"error": "server error 2"}`},
		TestResponse{StatusCode: 200, Body: `{"success": true}`},
	)
	defer server.Close()

	// Конфигурируем клиент с retry
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			MaxDelay:         10 * time.Millisecond,
			RetryMethods:     []string{"GET"},
			RetryStatusCodes: []int{500, 503},
		},
	}
	client := New(config, "test-metrics-retry")
	defer client.Close()

	ctx := context.Background()

	// Выполняем запрос который потребует 2 retry (3 попытки всего).
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	// Проверяем что было сделано 3 запроса
	assert.Equal(t, 3, server.GetRequestCount(), "Должно быть 3 попытки")

	// Проверяем что метрики собраны
	registry := client.GetMetricsRegistry()
	require.NotNil(t, registry, "Registry должна быть доступна")

	// Проверяем наличие метрик
	assertPrometheusMetricExists(t, registry, "http_client_requests_total")
	assertPrometheusMetricExists(t, registry, "http_client_request_duration_seconds")
	assertPrometheusMetricExists(t, registry, "http_client_retries_total")
}

// TestMetricsWithCircuitBreaker проверяет что метрики записываются корректно при работе circuit breaker
func TestMetricsWithCircuitBreaker(t *testing.T) {

	// Создаём сервер который возвращает только ошибки
	server := NewTestServer()
	for i := 0; i < 10; i++ {
		server.AddResponse(TestResponse{StatusCode: 500, Body: `{"error": "server error"}`})
	}
	defer server.Close()

	// Конфигурируем клиент с circuit breaker (низкий порог для быстрого открытия)
	config := Config{
		CircuitBreakerEnable: true,
		CircuitBreaker: NewCircuitBreakerWithConfig(CircuitBreakerConfig{
			FailureThreshold: 2, // Откроется после 2 неудач
			SuccessThreshold: 1,
			Timeout:          50 * time.Millisecond,
		}),
	}
	client := New(config, "test-metrics-cb")
	defer client.Close()

	ctx := context.Background()

	// Делаем 2 запроса чтобы открыть circuit breaker
	for i := 0; i < 2; i++ {
		resp, err := client.Get(ctx, server.URL)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
	}

	// Circuit breaker должен быть открыт
	cbState := config.CircuitBreaker.State()
	assert.Equal(t, CircuitBreakerOpen, cbState, "Circuit breaker должен быть открыт")

	// Сохраняем количество запросов к серверу до открытого CB
	requestsBefore := server.GetRequestCount()

	// Делаем запрос через открытый circuit breaker - должен вернуть cached ответ
	respCached, err := client.Get(ctx, server.URL)
	assert.Error(t, err, "Должна быть ошибка ErrCircuitBreakerOpen")
	if respCached != nil {
		assert.Equal(t, 500, respCached.StatusCode, "Должен вернуть cached статус 500")
		respCached.Body.Close()
	}

	// Проверяем что дополнительный запрос к серверу НЕ был сделан
	requestsAfter := server.GetRequestCount()
	assert.Equal(t, requestsBefore, requestsAfter, "Запрос к серверу не должен был быть сделан")

	// Проверяем что метрики собраны
	registry := client.GetMetricsRegistry()
	require.NotNil(t, registry, "Registry должна быть доступна")

	// Проверяем наличие метрик
	assertPrometheusMetricExists(t, registry, "http_client_requests_total")
	assertPrometheusMetricExists(t, registry, "http_client_request_duration_seconds")
}

// TestMetricsWithRetryAndCircuitBreaker проверяет комбинированный сценарий retry + circuit breaker
func TestMetricsWithRetryAndCircuitBreaker(t *testing.T) {

	// Сервер возвращает много ошибок
	server := NewTestServer()
	for i := 0; i < 20; i++ {
		server.AddResponse(TestResponse{StatusCode: 503, Body: `{"error": "service unavailable"}`})
	}
	defer server.Close()

	// Конфигурируем клиент с retry И circuit breaker
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			RetryMethods:     []string{"GET"},
			RetryStatusCodes: []int{503},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker: NewCircuitBreakerWithConfig(CircuitBreakerConfig{
			FailureThreshold: 5, // Откроется после 5 неудач
			SuccessThreshold: 1,
			Timeout:          100 * time.Millisecond,
		}),
	}
	client := New(config, "test-metrics-retry-cb")
	defer client.Close()

	ctx := context.Background()

	// Делаем несколько запросов с retry до открытия circuit breaker
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, server.URL)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}

	// Проверяем что метрики собраны
	registry := client.GetMetricsRegistry()
	require.NotNil(t, registry, "Registry должна быть доступна")

	// Проверяем наличие метрик
	assertPrometheusMetricExists(t, registry, "http_client_requests_total")
	assertPrometheusMetricExists(t, registry, "http_client_request_duration_seconds")
	assertPrometheusMetricExists(t, registry, "http_client_retries_total")
}

// TestMetricsWithIdempotentRetry проверяет метрики для идемпотентных POST запросов
func TestMetricsWithIdempotentRetry(t *testing.T) {

	// Сервер сначала возвращает ошибку, потом успех
	server := NewTestServer(
		TestResponse{StatusCode: 503, Body: `{"error": "try again"}`},
		TestResponse{StatusCode: 201, Body: `{"created": true}`},
	)
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   1 * time.Millisecond,
		},
	}
	client := New(config, "test-metrics-idempotent")
	defer client.Close()

	ctx := context.Background()

	// POST запрос с Idempotency-Key должен повторяться
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(`{"data": "test"}`))
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "test-key-123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()

	// Должно быть 2 запроса (503 + 201)
	assert.Equal(t, 2, server.GetRequestCount(), "Должно быть 2 запроса")

	// Проверяем что метрики собраны
	registry := client.GetMetricsRegistry()
	require.NotNil(t, registry, "Registry должна быть доступна")

	// Проверяем наличие метрик
	assertPrometheusMetricExists(t, registry, "http_client_requests_total")
	assertPrometheusMetricExists(t, registry, "http_client_request_duration_seconds")
	assertPrometheusMetricExists(t, registry, "http_client_retries_total")
}
