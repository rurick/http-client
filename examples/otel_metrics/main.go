// Example of working with OpenTelemetry metrics
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	httpclient "github.com/rurick/http-client"
)

func main() {
	// Create client with OpenTelemetry metrics
	client := httpclient.New(httpclient.Config{
		MetricsBackend: httpclient.MetricsBackendOpenTelemetry,
		RetryEnabled:   true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "otel-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Executing several requests with OpenTelemetry metrics...")

	// Successful requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Request error %d: %v", i, err)
			continue
		}
		fmt.Printf("Request %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(200 * time.Millisecond)
	}

	// Requests with errors
	fmt.Println("Testing requests with errors...")
	for i := 0; i < 2; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/status/503")
		if err != nil {
			log.Printf("Error (expected): %v", err)
		} else {
			fmt.Printf("Unexpected success: %s\n", resp.Status)
			_ = resp.Body.Close()
		}

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("Metrics successfully written to OpenTelemetry!")
	fmt.Println("To view metrics, configure OpenTelemetry SDK with a suitable exporter.")
	fmt.Println("Histogram buckets are set automatically and are the same for Prometheus and OpenTelemetry.")
}