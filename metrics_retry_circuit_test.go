package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestMetricsWithRetryPolicy проверяет что при retry политике все метрики записываются корректно
func TestMetricsWithRetryPolicy(t *testing.T) {
	// Создаём in-memory metric reader для проверки метрик
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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

	// Выполняем запрос который потребует 2 retry (3 попытки всего)
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	// Проверяем что было сделано 3 запроса
	assert.Equal(t, 3, server.GetRequestCount(), "Должно быть 3 попытки")

	// Собираем метрики
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	require.NoError(t, err)

	// Проверяем что записаны правильные метрики
	metricsMap := extractMetricsMap(rm)

	// 1. Должно быть 3 записи в http_client_requests_total (по одной на каждую попытку)
	requestsTotal, ok := metricsMap["http_client_requests_total"]
	assert.True(t, ok, "Метрика http_client_requests_total должна существовать")

	// Проверяем что есть записи для разных статусов
	requestCounts := getCounterSumByAttribute(requestsTotal, "status")
	assert.Contains(t, requestCounts, "500", "Должна быть запись для статуса 500")
	assert.Contains(t, requestCounts, "503", "Должна быть запись для статуса 503")
	assert.Contains(t, requestCounts, "200", "Должна быть запись для статуса 200")

	// 2. Должно быть 3 записи в http_client_request_duration_seconds (по одной на каждую попытку)
	requestDuration, ok := metricsMap["http_client_request_duration_seconds"]
	assert.True(t, ok, "Метрика http_client_request_duration_seconds должна существовать")

	// Проверяем что есть записи с разными attempt номерами
	durationCounts := getHistogramCountByAttribute(requestDuration, "attempt")
	assert.Contains(t, durationCounts, "1", "Должна быть запись для attempt=1")
	assert.Contains(t, durationCounts, "2", "Должна быть запись для attempt=2")
	assert.Contains(t, durationCounts, "3", "Должна быть запись для attempt=3")

	// 3. Должно быть 2 записи в http_client_retries_total (2 retry попытки)
	retriesTotal, ok := metricsMap["http_client_retries_total"]
	assert.True(t, ok, "Метрика http_client_retries_total должна существовать")

	// Должно быть 2 retry (первый на 500, второй на 503)
	retrySum := getCounterSum(retriesTotal)
	assert.Equal(t, int64(2), retrySum, "Должно быть 2 retry")
}

// TestMetricsWithCircuitBreaker проверяет что метрики записываются корректно при работе circuit breaker
func TestMetricsWithCircuitBreaker(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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
			resp.Body.Close()
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

	// Собираем метрики
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	require.NoError(t, err)

	metricsMap := extractMetricsMap(rm)

	// 1. Должны быть записаны метрики для всех запросов (включая cached)
	requestsTotal, ok := metricsMap["http_client_requests_total"]
	assert.True(t, ok, "Метрика http_client_requests_total должна существовать")

	// Должны быть записи и для реальных запросов, и для cached
	requestCounts := getCounterSumByAttribute(requestsTotal, "status")
	assert.Contains(t, requestCounts, "500", "Должны быть записи для статуса 500")

	// 2. Duration метрики должны записываться даже для cached ответов
	requestDuration, ok := metricsMap["http_client_request_duration_seconds"]
	assert.True(t, ok, "Метрика http_client_request_duration_seconds должна существовать")

	// Должно быть минимум 3 записи (2 реальных + 1 cached)
	durationCount := getHistogramTotalCount(requestDuration)
	assert.GreaterOrEqual(t, durationCount, uint64(3), "Должно быть минимум 3 записи duration")
}

// TestMetricsWithRetryAndCircuitBreaker проверяет комбинированный сценарий retry + circuit breaker
func TestMetricsWithRetryAndCircuitBreaker(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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

	// Собираем метрики
	rm := &metricdata.ResourceMetrics{}
	err := reader.Collect(ctx, rm)
	require.NoError(t, err)

	metricsMap := extractMetricsMap(rm)

	// 1. Должны быть записаны все attempts (включая retry)
	requestsTotal, ok := metricsMap["http_client_requests_total"]
	assert.True(t, ok, "Метрика http_client_requests_total должна существовать")

	totalRequests := getCounterSum(requestsTotal)
	// Каждый из 3 запросов может делать до 3 попыток = до 9 total requests
	assert.Greater(t, totalRequests, int64(6), "Должно быть больше 6 запросов с учётом retry")

	// 2. Duration записывается для каждой попытки
	requestDuration, ok := metricsMap["http_client_request_duration_seconds"]
	assert.True(t, ok, "Метрика http_client_request_duration_seconds должна существовать")

	durationCount := getHistogramTotalCount(requestDuration)
	assert.Greater(t, durationCount, uint64(6), "Должно быть больше 6 duration записей")

	// 3. Retry метрики должны быть записаны
	retriesTotal, ok := metricsMap["http_client_retries_total"]
	assert.True(t, ok, "Метрика http_client_retries_total должна существовать")

	retryCount := getCounterSum(retriesTotal)
	assert.Greater(t, retryCount, int64(0), "Должны быть retry попытки")
}

// TestMetricsWithIdempotentRetry проверяет метрики для идемпотентных POST запросов
func TestMetricsWithIdempotentRetry(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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

	// Собираем метрики
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	require.NoError(t, err)

	metricsMap := extractMetricsMap(rm)

	// Проверяем что метрики записаны для POST запроса с retry
	requestsTotal, ok := metricsMap["http_client_requests_total"]
	assert.True(t, ok, "Метрика requests_total должна существовать")

	// Должны быть записи для POST метода
	methodCounts := getCounterSumByAttribute(requestsTotal, "method")
	assert.Contains(t, methodCounts, "POST", "Должна быть запись для метода POST")

	// Должно быть 2 запроса (первый неудачный + retry успешный)
	postCount := methodCounts["POST"]
	assert.Equal(t, int64(2), postCount, "Должно быть 2 POST запроса")

	// Duration должен записываться для каждой попытки
	requestDuration, ok := metricsMap["http_client_request_duration_seconds"]
	assert.True(t, ok, "Метрика request_duration должна существовать")

	durationCounts := getHistogramCountByAttribute(requestDuration, "method")
	assert.Contains(t, durationCounts, "POST", "Duration должен записываться для POST")

	postDurationCount := durationCounts["POST"]
	assert.Equal(t, uint64(2), postDurationCount, "Должно быть 2 duration записи для POST")
}

// Вспомогательные функции для извлечения данных из метрик

func extractMetricsMap(rm *metricdata.ResourceMetrics) map[string]metricdata.Metrics {
	metricsMap := make(map[string]metricdata.Metrics)
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			metricsMap[metric.Name] = metric
		}
	}
	return metricsMap
}

func getCounterSum(metric metricdata.Metrics) int64 {
	if data, ok := metric.Data.(metricdata.Sum[int64]); ok {
		var total int64
		for _, dp := range data.DataPoints {
			total += dp.Value
		}
		return total
	}
	return 0
}

func getCounterSumByAttribute(metric metricdata.Metrics, attrKey string) map[string]int64 {
	result := make(map[string]int64)
	if data, ok := metric.Data.(metricdata.Sum[int64]); ok {
		for _, dp := range data.DataPoints {
			for _, kv := range dp.Attributes.ToSlice() {
				if string(kv.Key) == attrKey {
					attrValue := kv.Value.AsString()
					result[attrValue] += dp.Value
				}
			}
		}
	}
	return result
}

func getHistogramTotalCount(metric metricdata.Metrics) uint64 {
	if data, ok := metric.Data.(metricdata.Histogram[float64]); ok {
		var total uint64
		for _, dp := range data.DataPoints {
			total += dp.Count
		}
		return total
	}
	return 0
}

func getHistogramCountByAttribute(metric metricdata.Metrics, attrKey string) map[string]uint64 {
	result := make(map[string]uint64)
	if data, ok := metric.Data.(metricdata.Histogram[float64]); ok {
		for _, dp := range data.DataPoints {
			for _, kv := range dp.Attributes.ToSlice() {
				if string(kv.Key) == attrKey {
					// Получаем значение атрибута (может быть строкой или числом)
					var attrValue string
					switch kv.Value.Type() {
					case attribute.STRING:
						attrValue = kv.Value.AsString()
					case attribute.INT64:
						attrValue = fmt.Sprintf("%d", kv.Value.AsInt64())
					default:
						attrValue = kv.Value.AsString()
					}
					result[attrValue] += dp.Count
				}
			}
		}
	}
	return result
}
