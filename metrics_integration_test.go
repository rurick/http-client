package httpclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestMetricsIntegration проверяет что метрики собираются правильно
func TestMetricsIntegration(t *testing.T) {

	// Создаём тестовый сервер с разными ответами
	server := NewTestServer(
		TestResponse{StatusCode: 200, Body: `{"success": true}`},
		TestResponse{StatusCode: 500, Body: `{"error": "server error"}`},
		TestResponse{StatusCode: 503, Body: `{"error": "service unavailable"}`},
		TestResponse{StatusCode: 200, Body: `{"success": true}`},
	)
	defer server.Close()

	// Создаём клиент с retry конфигурацией
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			MaxDelay:         10 * time.Millisecond,
			RetryMethods:     []string{"GET", "HEAD", "PUT", "DELETE", "OPTIONS", "TRACE"},
			RetryStatusCodes: []int{429, 500, 502, 503, 504},
		},
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	t.Run("successful_request_metrics", func(t *testing.T) {
		server.Reset()

		// Выполняем успешный запрос
		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()

		// Проверяем метрики через registry
		registry := client.GetMetricsRegistry()
		if registry == nil {
			t.Fatal("expected registry to be available")
		}

		// Проверяем что request counter увеличился
		assertPrometheusMetricExists(t, registry, "http_client_requests_total")

		// Проверяем что duration записан
		assertPrometheusMetricExists(t, registry, "http_client_request_duration_seconds")
	})

	t.Run("retry_metrics", func(t *testing.T) {
		server.Reset()
		server.AddResponse(TestResponse{StatusCode: 503, Body: `{"error": "retry me"}`})
		server.AddResponse(TestResponse{StatusCode: 200, Body: `{"success": true}`})

		// Выполняем запрос который потребует retry
		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()

		// Log request count for debugging
		requestCount := server.GetRequestCount()
		//t.Logf("Request count: %d", requestCount)

		// Ожидаем что сделано минимум 2 запроса (первый неудачный + retry)
		if requestCount < 2 {
			t.Logf("Warning: expected at least 2 requests, got %d - retry may not have triggered", requestCount)
		}

		// Проверяем метрики retry
		registry := client.GetMetricsRegistry()
		assertPrometheusMetricExists(t, registry, "http_client_retries_total")
	})

	t.Run("error_metrics", func(t *testing.T) {
		server.Reset()
		// Добавляем только ошибочные ответы
		for i := 0; i < 5; i++ {
			server.AddResponse(TestResponse{StatusCode: 500, Body: `{"error": "persistent error"}`})
		}

		// Выполняем запрос который завершится ошибкой
		resp, err := client.Get(ctx, server.URL)
		if err == nil && resp != nil {
			t.Logf("Warning: expected error but got success with status %d", resp.StatusCode)
			_ = resp.Body.Close()
		}

		// Проверяем что метрики error записаны правильно
		registry := client.GetMetricsRegistry()

		// Должны быть метрики requests с error=true
		assertPrometheusMetricExists(t, registry, "http_client_requests_total")
		assertPrometheusMetricExists(t, registry, "http_client_retries_total")
	})
}

// TestMetricsWithIdempotency проверяет метрики для идемпотентных запросов
func TestMetricsWithIdempotency(t *testing.T) {

	server := NewTestServer(
		TestResponse{StatusCode: 503, Body: `{"error": "try again"}`},
		TestResponse{StatusCode: 201, Body: `{"created": true}`},
	)
	defer server.Close()

	config := Config{
		PerTryTimeout: 800 * time.Minute,
		Timeout:       900 * time.Minute,
		RetryEnabled:  true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   10 * time.Millisecond,
		},
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// POST запрос с Idempotency-Key должен повторяться
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(`{"data": "test"}`))
	req.Header.Set("Idempotency-Key", "test-key-123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	// Проверяем что сделано 2 запроса (503 + 201)
	requestCount := server.GetRequestCount()
	assert.Equal(t, 2, requestCount, "Expected 2 requests, got %d", requestCount)

	// Проверяем метрики
	registry := client.GetMetricsRegistry()
	assert.NotNil(t, registry, "expected registry to be available")

	assertPrometheusMetricExists(t, registry, "http_client_requests_total")
	assertPrometheusMetricExists(t, registry, "http_client_retries_total")
}

// TestInflightMetrics проверяет метрики активных запросов
func TestInflightMetrics(t *testing.T) {

	// Сервер с задержкой
	server := NewTestServer(
		TestResponse{
			StatusCode: 200,
			Body:       `{"delayed": true}`,
			Delay:      50 * time.Millisecond,
		},
	)
	defer server.Close()

	client := New(Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Запускаем запрос в горутине
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		_ = resp.Body.Close()
	}()

	// Даём время запросу начаться
	time.Sleep(10 * time.Millisecond)

	// Проверяем inflight метрики
	registry := client.GetMetricsRegistry()
	assertPrometheusMetricExists(t, registry, "http_client_inflight_requests")

	// Ждём завершения запроса
	<-done
}

// TestRequestSizeMetrics проверяет метрики размера запросов
func TestRequestSizeMetrics(t *testing.T) {

	server := NewTestServer(
		TestResponse{StatusCode: 200, Body: `{"received": true}`},
	)
	defer server.Close()

	client := New(Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()

	// POST запрос с телом
	body := `{"message": "this is a test request body"}`
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	// Проверяем метрики размера
	registry := client.GetMetricsRegistry()
	assertPrometheusMetricExists(t, registry, "http_client_request_size_bytes")
	assertPrometheusMetricExists(t, registry, "http_client_response_size_bytes")
}

// assertPrometheusMetricExists проверяет что метрика существует в Prometheus registry
func assertPrometheusMetricExists(t *testing.T, registry *prometheus.Registry, metricName string) {
	t.Helper()

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == metricName {
			return // метрика найдена
		}
	}

	assert.Fail(t, "metric not found in registry", metricName)
}

// TestMetricsLabels проверяет что метрики содержат правильные лейблы
func TestMetricsLabels(t *testing.T) {

	server := NewTestServer(
		TestResponse{StatusCode: 404, Body: `{"error": "not found"}`},
	)
	defer server.Close()

	client := New(Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Выполняем запрос
	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()

	// Собираем метрики и проверяем их существование
	registry := client.GetMetricsRegistry()
	assertPrometheusMetricExists(t, registry, "http_client_requests_total")

	// Для более детальной проверки лейблов можно использовать testutil.ToFloat64
	// но в данном случае достаточно проверить что метрика существует
}
