package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Конфигурация с включёнными retry
	config := httpclient.Config{
		Timeout:       10 * time.Second,
		PerTryTimeout: 2 * time.Second,
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:       3,                      // Максимум 3 попытки
			BaseDelay:         100 * time.Millisecond, // Базовая задержка 100ms
			MaxDelay:          2 * time.Second,        // Максимальная задержка 2s
			Jitter:            0.2,                    // 20% jitter
			RetryMethods:      []string{"GET", "HEAD", "OPTIONS", "PUT", "DELETE"},
			RetryStatusCodes:  []int{429, 500, 502, 503, 504},
			RespectRetryAfter: true, // Учитывать заголовок Retry-After
		},
		TracingEnabled: true, // Включаем трассировку
	}

	client := httpclient.New(config, "httpclient")
	defer client.Close()

	ctx := context.Background()

	// Демонстрация различных retry сценариев
	fmt.Println("=== Advanced Retry Examples ===")

	// 1. Retry на 500 ошибку
	fmt.Println("1. Testing retry on 5xx errors...")
	if err := testRetryOn5xx(ctx, client); err != nil {
		log.Printf("5xx retry test failed: %v", err)
	}

	// 2. Respect Retry-After заголовка
	fmt.Println("\n2. Testing Retry-After header respect...")
	if err := testRetryAfter(ctx, client); err != nil {
		log.Printf("Retry-After test failed: %v", err)
	}

	// 3. Максимум попыток
	fmt.Println("\n3. Testing max attempts limit...")
	if err := testMaxAttempts(ctx, client); err != nil {
		log.Printf("Max attempts test failed: %v", err)
	}

	// 4. Метод не подлежит retry (POST без Idempotency-Key)
	fmt.Println("\n4. Testing non-retryable method (POST)...")
	if err := testNonRetryableMethod(ctx, client); err != nil {
		log.Printf("Non-retryable method test failed: %v", err)
	}

	// Запуск метрик сервера
	startMetricsServer(client)
}

// Метрики теперь создаются автоматически в клиенте

// testRetryOn5xx тестирует retry на 5xx ошибки
func testRetryOn5xx(ctx context.Context, client *httpclient.Client) error {
	// httpbin.org/status/500 всегда возвращает 500
	fmt.Println("Making request to endpoint that returns 500...")

	start := time.Now()
	resp, err := client.Get(ctx, "https://httpbin.org/status/500")
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Total time with retries: %v\n", elapsed)

	// С 3 попытками и exponential backoff время должно быть больше base delay
	if elapsed < 100*time.Millisecond {
		fmt.Printf("Warning: Expected longer duration due to retries, got %v\n", elapsed)
	}

	return nil
}

// testRetryAfter тестирует уважение Retry-After заголовка
func testRetryAfter(ctx context.Context, client *httpclient.Client) error {
	// httpbin.org не возвращает Retry-After, поэтому симулируем с помощью кастомного сервера
	fmt.Println("This would test Retry-After header in a real scenario...")
	fmt.Println("(httpbin.org doesn't return Retry-After, but our client supports it)")

	// Попробуем запрос к rate-limited endpoint
	start := time.Now()
	resp, err := client.Get(ctx, "https://httpbin.org/status/429")
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Time with retries: %v\n", elapsed)

	return nil
}

// testMaxAttempts тестирует ограничение максимального количества попыток
func testMaxAttempts(ctx context.Context, client *httpclient.Client) error {
	fmt.Println("Making request that will exhaust max attempts...")

	start := time.Now()
	resp, err := client.Get(ctx, "https://httpbin.org/status/503")
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Final response status: %s\n", resp.Status)
	fmt.Printf("Total time for all attempts: %v\n", elapsed)

	// С 3 попытками общее время должно включать все задержки
	expectedMinTime := 100*time.Millisecond + 200*time.Millisecond // base + first retry
	if elapsed < expectedMinTime {
		fmt.Printf("Warning: Expected at least %v due to retries, got %v\n", expectedMinTime, elapsed)
	}

	return nil
}

// testNonRetryableMethod тестирует метод, который не подлежит retry
func testNonRetryableMethod(ctx context.Context, client *httpclient.Client) error {
	fmt.Println("Making POST request (non-retryable method)...")

	start := time.Now()
	resp, err := client.Post(ctx, "https://httpbin.org/status/500", strings.NewReader(`{"test": "data"}`), httpclient.WithContentType("application/json"))
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Time (should be quick, no retries): %v\n", elapsed)

	// POST без Idempotency-Key не должен retry, поэтому время должно быть коротким
	if elapsed > 1*time.Second {
		fmt.Printf("Warning: POST took too long (%v), might have retried incorrectly\n", elapsed)
	}

	return nil
}

// Prometheus/client_golang использует свои стандартные buckets,
// конфигурируемые при создании метрик

func startMetricsServer(client *httpclient.Client) {
	fmt.Println("\n=== Metrics Server ===")
	fmt.Println("Starting metrics server on :2112/metrics")

	registry := client.GetMetricsRegistry()
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    ":2112",
		Handler: nil,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)
	fmt.Println("Metrics available at http://localhost:2112/metrics")

	// Показываем примеры PromQL запросов
	fmt.Println("\n=== Example PromQL Queries ===")
	fmt.Println("# Error rate:")
	fmt.Println(`sum by (status) (rate(http_client_requests_total{error="true"}[5m])) / sum(rate(http_client_requests_total[5m]))`)

	fmt.Println("\n# p95 latency:")
	fmt.Println(`histogram_quantile(0.95, sum by (le) (rate(http_client_request_duration_seconds_bucket[5m])))`)

	fmt.Println("\n# Retry rate:")
	fmt.Println(`sum(rate(http_client_retries_total[5m])) / sum(rate(http_client_requests_total[5m]))`)

	time.Sleep(5 * time.Second)
	fmt.Println("\nAdvanced retry example completed!")
}
