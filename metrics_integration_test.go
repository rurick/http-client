package httpclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestMetricsIntegration проверяет что метрики собираются правильно
func TestMetricsIntegration(t *testing.T) {
	// Создаём in-memory metric reader для тестов
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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
		RetryConfig: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
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
		resp.Body.Close()

		// Проверяем метрики
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		if err != nil {
			t.Fatalf("failed to collect metrics: %v", err)
		}

		// Проверяем что request counter увеличился
		assertMetricExists(t, rm, "http_client_requests_total")

		// Проверяем что duration записан
		assertMetricExists(t, rm, "http_client_request_duration_seconds")
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
		resp.Body.Close()

		// Ожидаем что сделано минимум 2 запроса (первый неудачный + retry)
		if server.GetRequestCount() < 2 {
			t.Errorf("expected at least 2 requests, got %d", server.GetRequestCount())
		}

		// Проверяем метрики retry
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		if err != nil {
			t.Fatalf("failed to collect metrics: %v", err)
		}

		assertMetricExists(t, rm, "http_client_retries_total")
	})

	t.Run("error_metrics", func(t *testing.T) {
		server.Reset()
		// Добавляем только ошибочные ответы
		for i := 0; i < 5; i++ {
			server.AddResponse(TestResponse{StatusCode: 500, Body: `{"error": "persistent error"}`})
		}

		// Выполняем запрос который завершится ошибкой
		resp, err := client.Get(ctx, server.URL)
		if err == nil {
			t.Fatal("expected error but got success")
			resp.Body.Close()
		}

		// Проверяем что метрики error записаны правильно
		rm := &metricdata.ResourceMetrics{}
		err = reader.Collect(ctx, rm)
		if err != nil {
			t.Fatalf("failed to collect metrics: %v", err)
		}

		// Должны быть метрики requests с error=true
		assertMetricExists(t, rm, "http_client_requests_total")
		assertMetricExists(t, rm, "http_client_retries_total")
	})
}

// TestMetricsWithIdempotency проверяет метрики для идемпотентных запросов
func TestMetricsWithIdempotency(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

	server := NewTestServer(
		TestResponse{StatusCode: 503, Body: `{"error": "try again"}`},
		TestResponse{StatusCode: 201, Body: `{"created": true}`},
	)
	defer server.Close()

	config := Config{
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
	resp.Body.Close()

	// Проверяем что сделано 2 запроса (503 + 201)
	if server.GetRequestCount() != 2 {
		t.Errorf("expected 2 requests, got %d", server.GetRequestCount())
	}

	// Проверяем метрики
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	assertMetricExists(t, rm, "http_client_requests_total")
	assertMetricExists(t, rm, "http_client_retries_total")
}

// TestInflightMetrics проверяет метрики активных запросов
func TestInflightMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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
		resp.Body.Close()
	}()

	// Даём время запросу начаться
	time.Sleep(10 * time.Millisecond)

	// Проверяем inflight метрики
	rm := &metricdata.ResourceMetrics{}
	err := reader.Collect(ctx, rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	assertMetricExists(t, rm, "http_client_inflight_requests")

	// Ждём завершения запроса
	<-done
}

// TestRequestSizeMetrics проверяет метрики размера запросов
func TestRequestSizeMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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
	resp.Body.Close()

	// Проверяем метрики размера
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	assertMetricExists(t, rm, "http_client_request_size_bytes")
	assertMetricExists(t, rm, "http_client_response_size_bytes")
}

// assertMetricExists проверяет что метрика существует в собранных данных
func assertMetricExists(t *testing.T, rm *metricdata.ResourceMetrics, metricName string) {
	t.Helper()

	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == metricName {
				return // метрика найдена
			}
		}
	}

	t.Errorf("metric %s not found in collected metrics", metricName)
}

// TestMetricsLabels проверяет что метрики содержат правильные лейблы
func TestMetricsLabels(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)

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
	resp.Body.Close()

	// Собираем метрики
	rm := &metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	// Проверяем лейблы в метриках
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == "http_client_requests_total" {
				// Проверяем что есть правильные атрибуты
				switch data := metric.Data.(type) {
				case metricdata.Sum[int64]:
					if len(data.DataPoints) > 0 {
						attrs := data.DataPoints[0].Attributes
						hasMethod := false
						hasHost := false
						hasStatus := false

						for _, kv := range attrs.ToSlice() {
							switch kv.Key {
							case "method":
								hasMethod = true
								if kv.Value.AsString() != "GET" {
									t.Errorf("expected method=GET, got %s", kv.Value.AsString())
								}
							case "host":
								hasHost = true
							case "status":
								hasStatus = true
								if kv.Value.AsString() != "404" {
									t.Errorf("expected status=404, got %s", kv.Value.AsString())
								}
							}
						}

						if !hasMethod {
							t.Error("missing method attribute in metrics")
						}
						if !hasHost {
							t.Error("missing host attribute in metrics")
						}
						if !hasStatus {
							t.Error("missing status attribute in metrics")
						}
					}
				}
			}
		}
	}
}
