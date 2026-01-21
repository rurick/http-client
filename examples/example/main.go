// Package main demonstrates HTTP client usage
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
	// Create client with custom configuration
	config := httpclient.Config{
		Timeout: 10 * time.Second,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    5 * time.Second,
		},
		TracingEnabled: false, // disable for simplicity of example
		Transport:      http.DefaultTransport,
	}

	client := httpclient.New(config, "httpclient")
	defer client.Close()

	ctx := context.Background()

	// Example GET request
	fmt.Println("Executing GET request...")
	resp, err := client.Get(ctx, "https://httpbin.org/get")
	if err != nil {
		log.Printf("GET request error: %v", err)
	} else {
		fmt.Printf("GET response: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	// Example POST request with retry on error
	fmt.Println("Executing POST request with Idempotency-Key...")
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/503", nil)
	req.Header.Set("Idempotency-Key", "test-key-123")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST request error (expected due to 503): %v", err)
	} else {
		fmt.Printf("POST response: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	fmt.Println("Example completed!")
}
