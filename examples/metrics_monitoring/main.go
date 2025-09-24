package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Создаём клиент с полной конфигурацией
	config := httpclient.Config{
		Timeout:       15 * time.Second,
		PerTryTimeout: 5 * time.Second,
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         100 * time.Millisecond,
			MaxDelay:          2 * time.Second,
			Jitter:            0.2,
			RetryMethods:      []string{"GET", "HEAD", "OPTIONS", "PUT", "DELETE"},
			RetryStatusCodes:  []int{429, 500, 502, 503, 504},
			RespectRetryAfter: true,
		},
		TracingEnabled: true,
	}

	client := httpclient.New(config, "httpclient")
	defer client.Close()

	// Запускаем метрики сервер
	metricsServer := startMetricsServer(client)
	defer metricsServer.Close()

	fmt.Println("=== Metrics Monitoring Demo ===")
	fmt.Println("This example demonstrates all 6 types of metrics collected by the HTTP client:")
	fmt.Println("1. http_client_requests_total (counter)")
	fmt.Println("2. http_client_request_duration_seconds (histogram)")
	fmt.Println("3. http_client_retries_total (counter)")
	fmt.Println("4. http_client_inflight_requests (gauge)")
	fmt.Println("5. http_client_request_size_bytes (histogram)")
	fmt.Println("6. http_client_response_size_bytes (histogram)")

	fmt.Println("Metrics endpoint: http://localhost:2112/metrics")
	fmt.Println("Press Ctrl+C to stop")

	// Запускаём генерацию трафика в фоне
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go generateTraffic(ctx, client)

	// Показываем live метрики каждые 10 секунд
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Обрабатываем сигналы завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Starting traffic generation...")

	for {
		select {
		case <-ticker.C:
			printMetricsInfo()
		case <-sigChan:
			fmt.Println("\nShutting down...")
			return
		}
	}
}

// Метрики создаются автоматически с стандартными buckets

// Prometheus/client_golang создаёт метрики со стандартными buckets

func startMetricsServer(client *httpclient.Client) *http.Server {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint - метрики автоматически регистрируются
	mux.Handle("/metrics", promhttp.Handler())

	// Человеко-читаемая информация о метриках
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>HTTP Client Metrics</title>
    <meta http-equiv="refresh" content="5">
</head>
<body>
    <h1>HTTP Client Metrics Dashboard</h1>
    <h2>Available Endpoints:</h2>
    <ul>
        <li><a href="/metrics">/metrics</a> - Prometheus metrics</li>
        <li><a href="/promql">/promql</a> - Example PromQL queries</li>
    </ul>
    <h2>Metrics being collected:</h2>
    <ul>
        <li><b>http_client_requests_total</b> - Total requests (with labels: method, host, status, retry, error)</li>
        <li><b>http_client_request_duration_seconds</b> - Request duration histogram</li>
        <li><b>http_client_retries_total</b> - Total retries (with labels: reason, method, host)</li>
        <li><b>http_client_inflight_requests</b> - Current active requests</li>
        <li><b>http_client_request_size_bytes</b> - Request size histogram</li>
        <li><b>http_client_response_size_bytes</b> - Response size histogram</li>
    </ul>
    <p><em>Page auto-refreshes every 5 seconds</em></p>
</body>
</html>`)
	})

	// PromQL примеры
	mux.HandleFunc("/promql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, `# Example PromQL Queries for HTTP Client Metrics

# Error Rate (percentage)
sum by (status) (rate(http_client_requests_total{error="true"}[5m])) / sum(rate(http_client_requests_total[5m])) * 100

# p95 Latency
histogram_quantile(0.95, sum by (le) (rate(http_client_request_duration_seconds_bucket[5m])))

# p99 Latency  
histogram_quantile(0.99, sum by (le) (rate(http_client_request_duration_seconds_bucket[5m])))

# p99 Response Size
histogram_quantile(0.99, sum by (le) (rate(http_client_response_size_bytes_bucket[5m])))

# Retry Rate
sum(rate(http_client_retries_total[5m])) / sum(rate(http_client_requests_total[5m])) * 100

# Requests per second by host
sum by (host) (rate(http_client_requests_total[5m]))

# Current active requests
sum by (host) (http_client_inflight_requests)

# Average request size
sum(rate(http_client_request_size_bytes_sum[5m])) / sum(rate(http_client_request_size_bytes_count[5m]))

# 5xx Error rate
sum(rate(http_client_requests_total{status=~"5.."}[5m]))

# Retry reasons breakdown
sum by (reason) (rate(http_client_retries_total[5m]))

# Success rate by method
sum by (method) (rate(http_client_requests_total{error="false"}[5m])) / sum by (method) (rate(http_client_requests_total[5m])) * 100
`)
	})

	server := &http.Server{
		Addr:    ":2112",
		Handler: mux,
	}

	go func() {
		fmt.Println("Starting metrics server on http://localhost:2112")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Даём серверу время на запуск
	time.Sleep(500 * time.Millisecond)
	return server
}

// generateTraffic генерирует различные типы HTTP трафика для демонстрации метрик
func generateTraffic(ctx context.Context, client *httpclient.Client) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	scenarios := []trafficScenario{
		{name: "successful_get", url: "https://httpbin.org/get", method: "GET", expectSuccess: true},
		{name: "server_error", url: "https://httpbin.org/status/500", method: "GET", expectSuccess: false},
		{name: "rate_limited", url: "https://httpbin.org/status/429", method: "GET", expectSuccess: false},
		{name: "successful_post", url: "https://httpbin.org/post", method: "POST", expectSuccess: true, body: `{"key":"value"}`},
		{name: "large_response", url: "https://httpbin.org/bytes/10000", method: "GET", expectSuccess: true},
		{name: "slow_request", url: "https://httpbin.org/delay/1", method: "GET", expectSuccess: true},
	}

	scenarioIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scenario := scenarios[scenarioIndex%len(scenarios)]
			scenarioIndex++

			go executeScenario(ctx, client, scenario)
		}
	}
}

type trafficScenario struct {
	name          string
	url           string
	method        string
	body          string
	expectSuccess bool
}

func executeScenario(ctx context.Context, client *httpclient.Client, scenario trafficScenario) {
	var resp *http.Response
	var err error

	switch scenario.method {
	case "GET":
		resp, err = client.Get(ctx, scenario.url)
	case "POST":
		resp, err = client.Post(ctx, scenario.url, strings.NewReader(scenario.body), httpclient.WithContentType("application/json"))
	default:
		log.Printf("Unsupported method: %s", scenario.method)
		return
	}

	if err != nil {
		if scenario.expectSuccess {
			fmt.Printf("❌ %s failed: %v\n", scenario.name, err)
		} else {
			fmt.Printf("⚠️  %s failed as expected: %v\n", scenario.name, err)
		}
		return
	}

	defer resp.Body.Close()

	if scenario.expectSuccess && resp.StatusCode < 400 {
		fmt.Printf("✅ %s succeeded: %s\n", scenario.name, resp.Status)
	} else if !scenario.expectSuccess && resp.StatusCode >= 400 {
		fmt.Printf("⚠️  %s failed as expected: %s\n", scenario.name, resp.Status)
	} else {
		fmt.Printf("❓ %s unexpected result: %s\n", scenario.name, resp.Status)
	}

	// Читаем тело ответа для правильного учёта размера
	io.Copy(io.Discard, resp.Body)
}

func printMetricsInfo() {
	fmt.Println("\n=== Metrics Status ===")
	fmt.Printf("Time: %s\n", time.Now().Format("15:04:05"))
	fmt.Println("Metrics being collected automatically:")
	fmt.Println("  📊 Request counts by method, host, status")
	fmt.Println("  ⏱️  Request duration distribution")
	fmt.Println("  🔄 Retry attempts and reasons")
	fmt.Println("  📈 Active request count")
	fmt.Println("  📤 Request size distribution")
	fmt.Println("  📥 Response size distribution")
	fmt.Println("\nView live metrics at: http://localhost:2112/metrics")
	fmt.Println("View PromQL examples at: http://localhost:2112/promql")
}
