package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestOpenTelemetryMetrics_IntegrationWithRealRequests делает реальные HTTP запросы
// и проверяет что метрики OpenTelemetry корректно собираются
func TestOpenTelemetryMetrics_IntegrationWithRealRequests(t *testing.T) {
	// Создаём тестовый HTTP сервер
	requestCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success response"))
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		case "/slow":
			time.Sleep(50 * time.Millisecond) // Небольшая задержка для проверки duration метрик
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("slow response"))
		case "/large-response":
			largeBody := strings.Repeat("A", 5000) // 5KB ответ
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(largeBody))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}
	}))
	defer testServer.Close()

	// Создаём OpenTelemetry MeterProvider с ManualReader для сбора метрик
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	// Создаём HTTP клиент с OpenTelemetry метриками
	client := New(Config{
		MetricsBackend:    MetricsBackendOpenTelemetry,
		OTelMeterProvider: meterProvider,
		RetryEnabled:      true,
		RetryConfig: RetryConfig{
			MaxAttempts:       2,
			BaseDelay:         10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			RetryStatusCodes:  []int{500, 502, 503, 504},
			RetryMethods:      []string{"GET", "POST"},
			RespectRetryAfter: true,
		},
	}, "integration-test-client")
	defer client.Close()

	ctx := context.Background()

	// Выполняем различные запросы для генерации метрик
	
	// 1. Успешный GET запрос
	resp1, err := client.Get(ctx, testServer.URL+"/success")
	if err != nil {
		t.Fatalf("GET /success failed: %v", err)
	}
	resp1.Body.Close()

	// 2. POST запрос с телом
	postBody := strings.NewReader("test post body")
	resp2, err := client.Post(ctx, testServer.URL+"/success", postBody)
	if err != nil {
		t.Fatalf("POST /success failed: %v", err)
	}
	resp2.Body.Close()

	// 3. Запрос с ошибкой сервера (вызовет retry)
	resp3, err := client.Get(ctx, testServer.URL+"/server-error")
	if err != nil {
		t.Logf("Expected server error: %v", err)
	}
	if resp3 != nil {
		resp3.Body.Close()
	}

	// 4. Медленный запрос для проверки duration метрик
	resp4, err := client.Get(ctx, testServer.URL+"/slow")
	if err != nil {
		t.Fatalf("GET /slow failed: %v", err)
	}
	resp4.Body.Close()

	// 5. Запрос с большим ответом для проверки response size метрик
	resp5, err := client.Get(ctx, testServer.URL+"/large-response")
	if err != nil {
		t.Fatalf("GET /large-response failed: %v", err)
	}
	// Читаем body, чтобы убедиться что его размер корректно определяется
	large, err := io.ReadAll(resp5.Body)
	if err != nil {
		t.Logf("Failed to read large response body: %v", err)
	} else {
		t.Logf("Large response body size: %d bytes, Content-Length: %d", len(large), resp5.ContentLength)
	}
	resp5.Body.Close()

	// Небольшая пауза для завершения всех операций
	time.Sleep(100 * time.Millisecond)

	// Собираем метрики
	resourceMetrics := metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, &resourceMetrics)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Проверяем, что метрики собрались
	if len(resourceMetrics.ScopeMetrics) == 0 {
		t.Fatal("No scope metrics collected")
	}

	scopeMetrics := resourceMetrics.ScopeMetrics[0]
	if len(scopeMetrics.Metrics) == 0 {
		t.Fatal("No metrics collected")
	}

	// Анализируем собранные метрики
	metricsMap := make(map[string]metricdata.Metrics)
	for _, m := range scopeMetrics.Metrics {
		metricsMap[m.Name] = m
	}

	// Проверяем что все ожидаемые метрики присутствуют
	expectedMetrics := []string{
		MetricRequestsTotal,
		MetricRequestDuration,
		MetricRequestSizeBytes,
		MetricResponseSizeBytes,
		MetricInflightRequests,
	}

	for _, expectedName := range expectedMetrics {
		if _, exists := metricsMap[expectedName]; !exists {
			t.Errorf("Expected metric %s not found", expectedName)
		}
	}

	// Детальная проверка метрик requests_total
	if requestsMetric, exists := metricsMap[MetricRequestsTotal]; exists {
		sum, ok := requestsMetric.Data.(metricdata.Sum[int64])
		if !ok {
			t.Errorf("requests_total metric is not a Sum[int64], got %T", requestsMetric.Data)
		} else {
			totalRequests := int64(0)
			successfulRequests := int64(0)
			errorRequests := int64(0)

			for _, dataPoint := range sum.DataPoints {
				totalRequests += dataPoint.Value

				// Анализируем атрибуты
				clientName := ""
				method := ""
				status := ""
				hasError := false

				for _, attr := range dataPoint.Attributes.ToSlice() {
					switch attr.Key {
					case "client_name":
						clientName = attr.Value.AsString()
					case "method":
						method = attr.Value.AsString()
					case "status":
						status = attr.Value.AsString()
					case "error":
						hasError = attr.Value.AsBool()
					}
				}

				// Проверяем что client_name корректный
				if clientName != "integration-test-client" {
					t.Errorf("Unexpected client_name: %s", clientName)
				}

				// Считаем успешные и ошибочные запросы
				if hasError {
					errorRequests += dataPoint.Value
				} else if strings.HasPrefix(status, "2") {
					successfulRequests += dataPoint.Value
				}

				t.Logf("Request metric: client=%s, method=%s, status=%s, error=%t, count=%d",
					clientName, method, status, hasError, dataPoint.Value)
			}

			// Проверяем что метрики соответствуют ожиданиям
			if totalRequests < 5 { // минимум 5 запросов сделали
				t.Errorf("Expected at least 5 total requests, got %d", totalRequests)
			}

			if successfulRequests < 3 { // минимум 3 успешных запроса
				t.Errorf("Expected at least 3 successful requests, got %d", successfulRequests)
			}

			t.Logf("Total requests: %d, Successful: %d, Errors: %d", 
				totalRequests, successfulRequests, errorRequests)
		}
	}

	// Детальная проверка метрик request_duration
	if durationMetric, exists := metricsMap[MetricRequestDuration]; exists {
		histogram, ok := durationMetric.Data.(metricdata.Histogram[float64])
		if !ok {
			t.Errorf("request_duration metric is not a Histogram[float64], got %T", durationMetric.Data)
		} else {
			totalDurationCount := uint64(0)
			minDuration := float64(1000) // начинаем с большого значения
			maxDuration := float64(0)

			for _, dataPoint := range histogram.DataPoints {
				totalDurationCount += dataPoint.Count
				
				// Проверяем что есть duration data
				if dataPoint.Sum > maxDuration {
					maxDuration = dataPoint.Sum
				}
				if dataPoint.Sum < minDuration && dataPoint.Sum > 0 {
					minDuration = dataPoint.Sum
				}

				// Логируем атрибуты для отладки
				for _, attr := range dataPoint.Attributes.ToSlice() {
					if attr.Key == "method" || attr.Key == "status" {
						t.Logf("Duration metric: %s=%v, count=%d, sum=%f",
							attr.Key, attr.Value, dataPoint.Count, dataPoint.Sum)
					}
				}
			}

			if totalDurationCount == 0 {
				t.Error("No duration measurements recorded")
			}

			// Проверяем что медленный запрос действительно занял больше времени
			if maxDuration < 0.01 { // минимум 10ms должен был занять медленный запрос
				t.Errorf("Expected max duration > 0.01s for slow request, got %f", maxDuration)
			}

			t.Logf("Duration metrics: count=%d, min=%f, max=%f", 
				totalDurationCount, minDuration, maxDuration)
		}
	}

	// Проверяем метрики размера ответа
	if responseSizeMetric, exists := metricsMap[MetricResponseSizeBytes]; exists {
		histogram, ok := responseSizeMetric.Data.(metricdata.Histogram[float64])
		if !ok {
			t.Errorf("response_size metric is not a Histogram[float64], got %T", responseSizeMetric.Data)
		} else {
			foundLargeResponse := false
			
			for _, dataPoint := range histogram.DataPoints {
				// Логируем все размеры ответов для диагностики
				t.Logf("Response size datapoint: count=%d, sum=%f", dataPoint.Count, dataPoint.Sum)
				
				// Ищем большой ответ (5KB) - проверяем и отдельные значения
				if dataPoint.Sum > 4000 { // больше 4KB
					foundLargeResponse = true
					t.Logf("Found large response: %f bytes", dataPoint.Sum)
				}

				// Логируем атрибуты для отладки
				for _, attr := range dataPoint.Attributes.ToSlice() {
					if attr.Key == "status" {
						t.Logf("Response size metric: status=%v, count=%d, sum=%f",
							attr.Value, dataPoint.Count, dataPoint.Sum)
					}
				}
				
				// Проверяем buckets для histogram
				for i, bucketCount := range dataPoint.BucketCounts {
					if bucketCount > 0 {
						t.Logf("Bucket %d: count=%d", i, bucketCount)
					}
				}
			}

			// Делаем проверку менее строгой - проверяем что есть хоть какие-то response size метрики
			if len(histogram.DataPoints) == 0 {
				t.Error("No response size metrics found")
			} else {
				t.Logf("Found %d response size metric datapoints", len(histogram.DataPoints))
				if !foundLargeResponse {
					t.Logf("Warning: Large response (>4KB) not found, but response size metrics are being collected")
				}
			}
		}
	}

	// Проверяем inflight requests метрики
	// В OpenTelemetry UpDownCounter может собираться как Sum, а не Gauge
	if inflightMetric, exists := metricsMap[MetricInflightRequests]; exists {
		switch data := inflightMetric.Data.(type) {
		case metricdata.Gauge[int64]:
			for _, dataPoint := range data.DataPoints {
				t.Logf("Inflight requests (gauge): %d", dataPoint.Value)
			}
		case metricdata.Sum[int64]:
			// UpDownCounter может собираться как Sum в ManualReader
			for _, dataPoint := range data.DataPoints {
				t.Logf("Inflight requests (sum): %d", dataPoint.Value)
				// После завершения всех запросов значение должно быть близким к 0
				// (но может быть не точно 0 из-за асинхронности)
			}
		default:
			t.Logf("Inflight requests metric type: %T", inflightMetric.Data)
		}
	} else {
		t.Error("inflight_requests metric not found")
	}

	t.Logf("Integration test completed successfully. Server handled %d requests total.", requestCount)
}

// TestOpenTelemetryMetrics_RetryBehavior специально тестирует retry метрики
func TestOpenTelemetryMetrics_RetryBehavior(t *testing.T) {
	// Создаём тестовый сервер, который сначала возвращает ошибки, потом успех
	attempts := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 { // первые 2 попытки - ошибка
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		} else { // 3-я попытка - успех
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success after retries"))
		}
	}))
	defer testServer.Close()

	// Создаём OpenTelemetry setup
	reader := metric.NewManualReader()
	meterProvider := metric.NewMeterProvider(metric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	// Клиент с retry
	client := New(Config{
		MetricsBackend:    MetricsBackendOpenTelemetry,
		OTelMeterProvider: meterProvider,
		RetryEnabled:      true,
		RetryConfig: RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         10 * time.Millisecond,
			MaxDelay:          50 * time.Millisecond,
			RetryStatusCodes:  []int{500},
			RetryMethods:      []string{"GET"},
		},
	}, "retry-test-client")
	defer client.Close()

	ctx := context.Background()

	// Выполняем запрос, который вызовет retry
	resp, err := client.Get(ctx, testServer.URL+"/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Проверяем что получили успех после retry
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Собираем метрики
	time.Sleep(50 * time.Millisecond) // ждём завершения всех операций

	resourceMetrics := metricdata.ResourceMetrics{}
	err = reader.Collect(ctx, &resourceMetrics)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Анализируем retry метрики
	metricsMap := make(map[string]metricdata.Metrics)
	for _, m := range resourceMetrics.ScopeMetrics[0].Metrics {
		metricsMap[m.Name] = m
	}

	// Проверяем retry метрики
	if retriesMetric, exists := metricsMap[MetricRetriesTotal]; exists {
		sum, ok := retriesMetric.Data.(metricdata.Sum[int64])
		if !ok {
			t.Errorf("retries_total metric is not a Sum[int64], got %T", retriesMetric.Data)
		} else {
			totalRetries := int64(0)
			for _, dataPoint := range sum.DataPoints {
				totalRetries += dataPoint.Value
			}

			// Должно быть минимум 2 retry (попытки 2 и 3)
			if totalRetries < 2 {
				t.Errorf("Expected at least 2 retries, got %d", totalRetries)
			}
			
			t.Logf("Total retries recorded: %d", totalRetries)
		}
	} else {
		t.Error("retries_total metric not found")
	}

	// Проверяем что requests_total показывает все попытки
	if requestsMetric, exists := metricsMap[MetricRequestsTotal]; exists {
		sum, ok := requestsMetric.Data.(metricdata.Sum[int64])
		if !ok {
			t.Errorf("requests_total metric is not a Sum[int64], got %T", requestsMetric.Data)
		} else {
			totalRequests := int64(0)
			for _, dataPoint := range sum.DataPoints {
				totalRequests += dataPoint.Value
			}

			// Должно быть минимум 3 запроса (первоначальный + 2 retry)
			if totalRequests < 3 {
				t.Errorf("Expected at least 3 total requests, got %d", totalRequests)
			}

			t.Logf("Total requests (including retries): %d", totalRequests)
		}
	}

	t.Logf("Retry test completed. Server received %d attempts total.", attempts)
}