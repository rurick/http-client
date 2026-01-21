// Example of using Rate Limiter
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	httpclient "github.com/rurick/http-client"
)

func main() {
	fmt.Println("Rate Limiter Example")
	fmt.Println("===================")

	// Configuration with rate limiter
	config := httpclient.Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: httpclient.RateLimiterConfig{
			RequestsPerSecond: 2.0, // 2 requests per second
			BurstCapacity:     3,   // up to 3 requests at once
		},
		Timeout: 30 * time.Second,
	}

	client := httpclient.New(config, "rate-limiter-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Printf("Configuration: %.1f RPS, burst %d\n",
		config.RateLimiterConfig.RequestsPerSecond,
		config.RateLimiterConfig.BurstCapacity)
	fmt.Println()

	// Demonstrate burst capacity
	fmt.Println("1. Demonstrating burst capacity (3 fast requests):")
	for i := 1; i <= 3; i++ {
		start := time.Now()
		resp, err := client.Get(ctx, "https://httpbin.org/delay/0")
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Request %d failed: %v", i, err)
			continue
		}
		resp.Body.Close()

		fmt.Printf("  Request %d: %s (time: %v)\n", i, resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("2. Demonstrating rate limiting (4th request should wait):")

	start := time.Now()
	resp, err := client.Get(ctx, "https://httpbin.org/delay/0")
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("Request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("  Request 4: %s (wait time: %v)\n", resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("3. Demonstrating recovery (after 1 second):")
	time.Sleep(1 * time.Second)

	start = time.Now()
	resp, err = client.Get(ctx, "https://httpbin.org/delay/0")
	elapsed = time.Since(start)

	if err != nil {
		log.Printf("Request failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("  Request 5: %s (time: %v)\n", resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("Rate Limiter works correctly!")
	fmt.Println("- Burst requests passed quickly")
	fmt.Println("- 4th request waited for token to appear")
	fmt.Println("- After pause token was restored")
}
