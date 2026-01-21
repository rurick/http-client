// Example of using idempotency
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	httpclient "github.com/rurick/http-client"
)

func main() {
	client := httpclient.New(httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "idempotency-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Testing idempotent requests ===")

	// GET requests are always idempotent and retried
	fmt.Println("1. GET request (always idempotent):")
	resp, err := client.Get(ctx, "https://httpbin.org/status/500,200")
	if err != nil {
		log.Printf("GET error: %v", err)
	} else {
		fmt.Printf("GET success: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	// PUT requests are always idempotent
	fmt.Println("2. PUT request (always idempotent):")
	req, _ := http.NewRequestWithContext(ctx, "PUT", "https://httpbin.org/status/500,200",
		strings.NewReader(`{"data": "test"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("PUT error: %v", err)
	} else {
		fmt.Printf("PUT success: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	fmt.Println("=== Testing POST requests ===")

	// POST without Idempotency-Key is NOT retried
	fmt.Println("3. POST without Idempotency-Key (NOT retried):")
	req, _ = http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/503",
		strings.NewReader(`{"order": "12345"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST without idempotency error (expected): %v", err)
	} else {
		fmt.Printf("POST without idempotency success: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	// POST with Idempotency-Key is retried
	fmt.Println("4. POST with Idempotency-Key (retried):")
	req, _ = http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/500,500,201",
		strings.NewReader(`{"payment": "67890"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "payment-operation-67890")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST with idempotency error: %v", err)
	} else {
		fmt.Printf("POST with idempotency success: %s\n", resp.Status)
		_ = resp.Body.Close()
	}

	fmt.Println("\n=== Summary ===")
	fmt.Println("✓ GET, PUT, DELETE - always retried on errors")
	fmt.Println("✓ POST, PATCH - retried only with Idempotency-Key header")
	fmt.Println("✓ Idempotency-Key must be unique for each operation")
}
