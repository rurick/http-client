// Example of working with metrics
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "github.com/rurick/http-client"
)

func main() {
	// Create client with standard configuration
	client := httpclient.New(httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "metrics-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Executing several requests to generate metrics...")

	// Successful requests
	for i := 0; i < 5; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Request %d error: %v", i, err)
			continue
		}
		fmt.Printf("Request %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(100 * time.Millisecond)
	}

	// Requests with errors to demonstrate retry metrics
	fmt.Println("Testing requests with errors...")
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/status/503")
		if err != nil {
			log.Printf("Error (expected): %v", err)
		} else {
			fmt.Printf("Unexpected success: %s\n", resp.Status)
			_ = resp.Body.Close()
		}

		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("Metrics collected. Check /metrics endpoint to view.")
	fmt.Println("In production environment metrics will be available through Prometheus scraper.")

	// Example of creating HTTP server with /metrics endpoint
	// Metrics are automatically available through standard Prometheus handler
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Metrics available at http://localhost:8080/metrics")
	fmt.Println("All HTTP client metrics are automatically registered in DefaultRegistry")
	// Uncomment to run:
	// log.Fatal(http.ListenAndServe(":8080", nil))
}
