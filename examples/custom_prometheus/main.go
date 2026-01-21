// Example of working with custom Prometheus registry
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "github.com/rurick/http-client"
)

func main() {
	// Create custom Prometheus registry
	customRegistry := prometheus.NewRegistry()

	// Create client with custom registerer
	client := httpclient.New(httpclient.Config{
		MetricsBackend:       httpclient.MetricsBackendPrometheus, // explicitly specify prometheus
		PrometheusRegisterer: customRegistry,
		RetryEnabled:         true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "custom-prometheus-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Executing requests with custom Prometheus registry...")

	// Successful requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Request error %d: %v", i, err)
			continue
		}
		fmt.Printf("Request %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Metrics saved in custom registry!")
	fmt.Println("Access metrics via custom handler at http://localhost:8081/custom-metrics")

	// Create HTTP server with custom handler
	http.Handle("/custom-metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))
	fmt.Println("Server started on :8081")
	fmt.Println("Open http://localhost:8081/custom-metrics to view metrics")
	
	// In this example, metrics will only be available through the custom registry,
	// not through the standard DefaultRegistry
	
	// Start server (commented out to avoid blocking execution)
	// log.Fatal(http.ListenAndServe(":8081", nil))
	
	fmt.Println("Example completed. In production, use a custom registry for metrics isolation.")
}