package main

import (
	"context"
	"fmt"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Example 1: default SimpleCircuitBreaker
	client1 := httpclient.New(httpclient.Config{
		CircuitBreakerEnable: true,
	}, "cb-default")
	defer client1.Close()

	for i := 0; i < 5; i++ {
		resp, err := client1.Get(context.Background(), "https://httpbin.org/status/500")
		if err != nil {
			fmt.Printf("default %d) err: %v\n", i+1, err)
		} else {
			fmt.Printf("default %d) status: %d\n", i+1, resp.StatusCode)
			_ = resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Example 2: custom CB configuration
	cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          2 * time.Second,
		OnStateChange: func(from, to httpclient.CircuitBreakerState) {
			fmt.Printf("state changed: %s -> %s\n", from, to)
		},
	})

	client2 := httpclient.New(httpclient.Config{
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}, "cb-custom")
	defer client2.Close()

	for i := 0; i < 6; i++ {
		resp, err := client2.Get(context.Background(), "https://httpbin.org/status/500")
		if err != nil {
			fmt.Printf("custom %d) err: %v\n", i+1, err)
		} else {
			fmt.Printf("custom %d) status: %d\n", i+1, resp.StatusCode)
			_ = resp.Body.Close()
		}
		time.Sleep(300 * time.Millisecond)
	}
}
