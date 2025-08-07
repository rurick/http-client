package main

import (
	"context"
	"fmt"
	"log"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.uber.org/zap"
)

func main() {
	basicUsageExample()
	retryExample()
	jsonExample()
	customOptionsExample()
}

// basicUsageExample демонстрирует базовое использование HTTP клиента
func basicUsageExample() {
	fmt.Println("=== Basic Usage Example ===")

	// Создаем клиент с параметрами по умолчанию
	client, err := httpclient.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Выполняем простой GET запрос
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		log.Printf("GET request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))

	// Получаем метрики
	metrics := client.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful requests: %d\n", metrics.SuccessfulReqs)
	fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
}

// retryExample демонстрирует функциональность повторов
func retryExample() {
	fmt.Println("\n=== Retry Example ===")

	// Создаем клиент с пользовательской стратегией повтора
	retryStrategy := httpclient.NewExponentialBackoffStrategy(5, 1*time.Second, 30*time.Second)

	client, err := httpclient.NewClient(
		httpclient.WithRetryStrategy(retryStrategy),
		httpclient.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Этот эндпоинт случайно возвращает ошибки 500
	resp, err := client.Get("https://httpbin.org/status/500")
	if err != nil {
		log.Printf("Request failed after retries: %v", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Status: %s\n", resp.Status)
	}

	// Проверяем метрики повторов
	metrics := client.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Failed requests: %d\n", metrics.FailedRequests)
}

// jsonExample demonstrates JSON request/response handling
func jsonExample() {
	fmt.Println("\n=== JSON Example ===")

	client, err := httpclient.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// GET JSON
	var getResult map[string]any
	err = client.GetJSON(ctx, "https://httpbin.org/json", &getResult)
	if err != nil {
		log.Printf("GET JSON failed: %v", err)
		return
	}

	fmt.Printf("GET JSON result: %+v\n", getResult)

	// POST JSON
	postData := map[string]any{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	var postResult map[string]any
	err = client.PostJSON(ctx, "https://httpbin.org/post", postData, &postResult)
	if err != nil {
		log.Printf("POST JSON failed: %v", err)
		return
	}

	fmt.Printf("POST JSON successful\n")
}

// customOptionsExample demonstrates advanced client configuration
func customOptionsExample() {
	fmt.Println("\n=== Custom Options Example ===")

	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Printf("Failed to create logger: %v", err)
		return
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	// Create client with custom options
	client, err := httpclient.NewClient(
		httpclient.WithTimeout(30*time.Second),
		httpclient.WithMaxIdleConns(50),
		httpclient.WithMaxConnsPerHost(5),
		httpclient.WithRetryMax(3),
		httpclient.WithRetryWait(500*time.Millisecond, 5*time.Second),
		httpclient.WithLogger(logger),
		httpclient.WithMetrics(true),
		httpclient.WithTracing(true),
		// Add middleware
		httpclient.WithMiddleware(
			httpclient.NewUserAgentMiddleware("MyApp/1.0"),
		),
		httpclient.WithMiddleware(
			httpclient.NewHeaderMiddleware(map[string]string{
				"X-API-Version": "v1",
				"X-Client-ID":   "my-client",
			}),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Make request
	resp, err := client.Get("https://httpbin.org/headers")
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)

	// Display comprehensive metrics
	metrics := client.GetMetrics()
	fmt.Printf("\n=== Metrics ===\n")
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulReqs)
	fmt.Printf("Failed Requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
	fmt.Printf("Note: Detailed metrics (retries, sizes, status codes) available via OpenTelemetry/Prometheus\n")
}
