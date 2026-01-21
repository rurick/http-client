// Demonstration of new API with functional options and WithHeaders methods
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	httpclient "github.com/rurick/http-client"
)

func main() {
	// Create client
	client := httpclient.New(httpclient.Config{}, "demo-service")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== New API Demonstration ===")

	// 1. GET with functional options
	fmt.Println("1. GET request with functional options:")
	resp, err := client.Get(ctx, "https://httpbin.org/headers",
		httpclient.WithHeader("Accept", "application/json"),
		httpclient.WithUserAgent("MyApp/1.0"),
		httpclient.WithHeader("X-Custom-Header", "custom-value"),
	)
	if err != nil {
		log.Printf("GET error: %v", err)
	} else {
		fmt.Printf("GET successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 2. POST with functional options
	fmt.Println("\n2. POST request with functional options:")
	jsonData := []byte(`{"name": "example", "value": 123}`)
	resp, err = client.Post(ctx, "https://httpbin.org/post", bytes.NewReader(jsonData),
		httpclient.WithContentType("application/json"),
		httpclient.WithBearerToken("my-token-123"),
		httpclient.WithIdempotencyKey("post-operation-456"),
		httpclient.WithHeader("X-Request-ID", "req-789"),
	)
	if err != nil {
		log.Printf("POST error: %v", err)
	} else {
		fmt.Printf("POST successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 3. GET with WithHeaders method
	fmt.Println("\n3. GET request via GetWithHeaders:")
	headers := map[string]string{
		"Accept":      "application/json",
		"User-Agent":  "MyApp/2.0",
		"X-API-Key":   "api-key-123",
		"X-Client-ID": "client-456",
	}
	resp, err = client.GetWithHeaders(ctx, "https://httpbin.org/headers", headers)
	if err != nil {
		log.Printf("GetWithHeaders error: %v", err)
	} else {
		fmt.Printf("GetWithHeaders successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 4. POST with WithHeaders method
	fmt.Println("\n4. POST request via PostWithHeaders:")
	postHeaders := map[string]string{
		"Content-Type":    "application/json",
		"Authorization":   "Bearer another-token",
		"Idempotency-Key": "payment-789",
		"X-Trace-ID":      "trace-123",
	}
	resp, err = client.PostWithHeaders(ctx, "https://httpbin.org/post", bytes.NewReader(jsonData), postHeaders)
	if err != nil {
		log.Printf("PostWithHeaders error: %v", err)
	} else {
		fmt.Printf("PostWithHeaders successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 5. PATCH - new method
	fmt.Println("\n5. PATCH request (new method):")
	patchData := []byte(`{"status": "updated"}`)
	resp, err = client.Patch(ctx, "https://httpbin.org/patch", bytes.NewReader(patchData),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("patch-operation-123"),
	)
	if err != nil {
		log.Printf("PATCH error: %v", err)
	} else {
		fmt.Printf("PATCH successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 6. Combination of options
	fmt.Println("\n6. PUT with combination of options:")
	// First create map with main headers
	baseHeaders := map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    "api-key-999",
	}
	// Then add additional ones via options
	resp, err = client.Put(ctx, "https://httpbin.org/put", bytes.NewReader(jsonData),
		httpclient.WithHeaders(baseHeaders),         // header map
		httpclient.WithBearerToken("special-token"), // additional header
		httpclient.WithHeader("X-Priority", "high"), // another header
	)
	if err != nil {
		log.Printf("PUT error: %v", err)
	} else {
		fmt.Printf("PUT successful: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println("\n=== Demonstration completed ===")
}
