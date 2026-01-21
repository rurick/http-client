// Example of configuring retry mechanism
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	httpclient "github.com/rurick/http-client"
)

func main() {
	// Configuration with aggressive retry attempts
	config := httpclient.Config{
		Timeout: 30 * time.Second,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 5,                      // Up to 5 attempts
			BaseDelay:   200 * time.Millisecond, // Base delay
			MaxDelay:    10 * time.Second,       // Maximum delay
			Jitter:      0.3,                    // 30% jitter to avoid thundering herd
		},
		TracingEnabled: true,
		RetryEnabled:   true,
		Transport:      http.DefaultTransport,
	}

	client := httpclient.New(config, "retry-example")
	defer client.Close()

	ctx := context.Background()

	// Test on endpoint that sometimes returns 503
	fmt.Println("Testing retry mechanism...")

	resp, err := client.Get(ctx, "https://httpbin.org/status/200,503,503,200")
	if err != nil {
		if maxErr, ok := err.(*httpclient.MaxAttemptsExceededError); ok {
			log.Printf("Request failed after %d attempts: %v", maxErr.MaxAttempts, maxErr.LastError)
		} else {
			log.Printf("Non-retriable error: %v", err)
		}
		return
	}

	fmt.Printf("Successful response: %s\n", resp.Status)
	_ = resp.Body.Close()

	// Example POST request with idempotency
	fmt.Println("Testing POST with Idempotency-Key...")

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/500,500,201", nil)
	req.Header.Set("Idempotency-Key", "operation-12345")
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST request failed: %v", err)
		return
	}

	fmt.Printf("POST response: %s\n", resp.Status)
	_ = resp.Body.Close()
}
