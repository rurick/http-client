package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.uber.org/zap"
)

func main() {
	loggingMiddlewareExample()
	authenticationExample()
	customMiddlewareExample()
	middlewareChainExample()
}

// loggingMiddlewareExample демонстрирует middleware логирования
func loggingMiddlewareExample() {
	fmt.Println("=== Logging Middleware Example ===")

	// Создаем логгер
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Создаем клиент с middleware логирования
	client, err := httpclient.NewClient(
		httpclient.WithLogger(logger),
		httpclient.WithMiddleware(
			httpclient.NewLoggingMiddleware(logger),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Выполняем запросы - они будут залогированы
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Request completed with status: %s\n", resp.Status)
}

// authenticationExample демонстрирует middleware аутентификации
func authenticationExample() {
	fmt.Println("\n=== Authentication Example ===")

	// Аутентификация с Bearer токеном
	bearerClient, err := httpclient.NewClient(
		httpclient.WithMiddleware(
			httpclient.NewBearerAuthMiddleware("your-api-token-here"),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := bearerClient.Get("https://httpbin.org/bearer")
	if err != nil {
		log.Printf("Bearer auth request failed: %v", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Bearer auth status: %s\n", resp.Status)
	}

	// Базовая аутентификация
	basicClient, err := httpclient.NewClient(
		httpclient.WithMiddleware(
			httpclient.NewBasicAuthMiddleware("testuser", "testpass"),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err = basicClient.Get("https://httpbin.org/basic-auth/testuser/testpass")
	if err != nil {
		log.Printf("Basic auth request failed: %v", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Basic auth status: %s\n", resp.Status)
	}
}

// customMiddlewareExample demonstrates creating custom middleware
func customMiddlewareExample() {
	fmt.Println("\n=== Custom Middleware Example ===")

	// Create custom rate limiting middleware
	rateLimiter := &RateLimitMiddleware{
		requestsPerSecond: 2,
		lastRequest:       time.Now().Add(-1 * time.Second),
	}

	// Create custom request ID middleware
	requestIDMiddleware := &RequestIDMiddleware{}

	client, err := httpclient.NewClient(
		httpclient.WithMiddleware(rateLimiter),
		httpclient.WithMiddleware(requestIDMiddleware),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Make multiple requests quickly - rate limiter should delay them
	fmt.Println("Making rapid requests (should be rate limited)...")
	for i := 0; i < 3; i++ {
		start := time.Now()
		resp, err := client.Get("https://httpbin.org/get")
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}
		defer resp.Body.Close()

		fmt.Printf("Request %d completed in %v, status: %s\n", i+1, elapsed, resp.Status)
	}
}

// middlewareChainExample demonstrates complex middleware chains
func middlewareChainExample() {
	fmt.Println("\n=== Middleware Chain Example ===")

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	client, err := httpclient.NewClient(
		// Middleware will be executed in the order they are added
		httpclient.WithMiddleware(
			httpclient.NewLoggingMiddleware(logger),
		),
		httpclient.WithMiddleware(
			httpclient.NewTimeoutMiddleware(5*time.Second),
		),
		httpclient.WithMiddleware(
			httpclient.NewUserAgentMiddleware("MiddlewareChain/1.0"),
		),
		httpclient.WithMiddleware(
			httpclient.NewHeaderMiddleware(map[string]string{
				"X-Request-Source": "middleware-example",
				"X-API-Version":    "v1",
			}),
		),
		httpclient.WithMiddleware(
			&RequestTimingMiddleware{},
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Get("https://httpbin.org/headers")
	if err != nil {
		log.Printf("Middleware chain request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Middleware chain request completed with status: %s\n", resp.Status)
}

// RateLimitMiddleware implements a simple rate limiting middleware
type RateLimitMiddleware struct {
	requestsPerSecond int
	lastRequest       time.Time
}

func (rlm *RateLimitMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	now := time.Now()
	timeSinceLastRequest := now.Sub(rlm.lastRequest)
	minInterval := time.Second / time.Duration(rlm.requestsPerSecond)

	if timeSinceLastRequest < minInterval {
		sleepTime := minInterval - timeSinceLastRequest
		fmt.Printf("Rate limiting: sleeping for %v\n", sleepTime)
		time.Sleep(sleepTime)
	}

	rlm.lastRequest = time.Now()
	return next(req)
}

// RequestIDMiddleware adds a unique request ID to each request
type RequestIDMiddleware struct {
	counter int
}

func (rim *RequestIDMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	rim.counter++
	requestID := fmt.Sprintf("req-%d-%d", time.Now().Unix(), rim.counter)
	req.Header.Set("X-Request-ID", requestID)

	fmt.Printf("Processing request with ID: %s\n", requestID)
	return next(req)
}

// RequestTimingMiddleware measures and logs request timing
type RequestTimingMiddleware struct{}

func (rtm *RequestTimingMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	start := time.Now()

	resp, err := next(req)

	duration := time.Since(start)
	fmt.Printf("Request to %s took %v\n", req.URL.String(), duration)

	return resp, err
}
