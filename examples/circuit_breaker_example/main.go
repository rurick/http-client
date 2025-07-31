package main

import (
	"fmt"
	"log"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.uber.org/zap"
)

func main() {
	basicCircuitBreakerExample()
	circuitBreakerWithConfigExample()
	circuitBreakerStateMonitoringExample()
	circuitBreakerRecoveryExample()
}

// basicCircuitBreakerExample демонстрирует базовое использование автоматического выключателя
func basicCircuitBreakerExample() {
	fmt.Println("=== Basic Circuit Breaker Example ===")

	// Создаем простой автоматический выключатель
	circuitBreaker := httpclient.NewSimpleCircuitBreaker()

	client, err := httpclient.NewClient(
		httpclient.WithCircuitBreaker(circuitBreaker),
		httpclient.WithTimeout(2*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Выполняем запросы к эндпоинту, который будет возвращать ошибки
	for i := 0; i < 10; i++ {
		resp, err := client.Get("https://httpbin.org/status/500")

		fmt.Printf("Request %d: ", i+1)
		if err != nil {
			fmt.Printf("Failed - %v (Circuit: %s)\n", err, circuitBreaker.State())
		} else {
			defer resp.Body.Close()
			fmt.Printf("Success - Status: %s (Circuit: %s)\n", resp.Status, circuitBreaker.State())
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// circuitBreakerWithConfigExample демонстрирует пользовательскую настройку автоматического выключателя
func circuitBreakerWithConfigExample() {
	fmt.Println("\n=== Circuit Breaker with Custom Config ===")

	// Создаем автоматический выключатель с пользовательской конфигурацией
	config := httpclient.CircuitBreakerConfig{
		FailureThreshold: 3,               // Открыть после 3 сбоев
		SuccessThreshold: 2,               // Закрыть после 2 успехов в полуоткрытом состоянии
		Timeout:          3 * time.Second, // Попробовать полуоткрытое состояние через 3 секунды
		OnStateChange: func(from, to httpclient.CircuitBreakerState) {
			fmt.Printf("Circuit breaker state changed: %s -> %s\n", from, to)
		},
	}

	circuitBreaker := httpclient.NewCircuitBreakerWithConfig(config)

	client, err := httpclient.NewClient(
		httpclient.WithCircuitBreaker(circuitBreaker),
		httpclient.WithTimeout(2*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Имитируем сбои для активации автоматического выключателя
	fmt.Println("Simulating failures to trigger circuit breaker...")
	for i := 0; i < 5; i++ {
		resp, err := client.Get("https://httpbin.org/delay/5") // This will timeout

		fmt.Printf("Failure simulation %d: ", i+1)
		if err != nil {
			fmt.Printf("Failed - %v\n", err)
		} else {
			defer resp.Body.Close()
			fmt.Printf("Unexpected success - Status: %s\n", resp.Status)
		}
	}

	fmt.Println("Waiting for circuit breaker to enter half-open state...")
	time.Sleep(4 * time.Second)

	// Try successful requests
	fmt.Println("Making successful requests...")
	for i := 0; i < 3; i++ {
		resp, err := client.Get("https://httpbin.org/get")

		fmt.Printf("Recovery attempt %d: ", i+1)
		if err != nil {
			fmt.Printf("Failed - %v\n", err)
		} else {
			defer resp.Body.Close()
			fmt.Printf("Success - Status: %s\n", resp.Status)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// circuitBreakerStateMonitoringExample demonstrates monitoring circuit breaker state
func circuitBreakerStateMonitoringExample() {
	fmt.Println("\n=== Circuit Breaker State Monitoring ===")

	// Create logger for state changes
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create circuit breaker with state change monitoring
	config := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          2 * time.Second,
		OnStateChange: func(from, to httpclient.CircuitBreakerState) {
			logger.Info("Circuit breaker state change",
				zap.String("from", from.String()),
				zap.String("to", to.String()),
				zap.Time("timestamp", time.Now()),
			)
		},
	}

	circuitBreaker := httpclient.NewCircuitBreakerWithConfig(config)

	client, err := httpclient.NewClient(
		httpclient.WithCircuitBreaker(circuitBreaker),
		httpclient.WithTimeout(1*time.Second),
		httpclient.WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Monitor state with a goroutine
	stopMonitoring := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				state := circuitBreaker.State()
				fmt.Printf("Current circuit breaker state: %s\n", state)
			case <-stopMonitoring:
				return
			}
		}
	}()

	// Make some failing requests
	fmt.Println("Making failing requests...")
	for i := 0; i < 4; i++ {
		_, err := client.Get("https://httpbin.org/delay/2") // Will timeout
		if err != nil {
			fmt.Printf("Request %d failed as expected\n", i+1)
		}
		time.Sleep(200 * time.Millisecond)
	}

	time.Sleep(3 * time.Second) // Wait for half-open transition

	// Make a successful request
	fmt.Println("Making successful request...")
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Printf("Recovery request failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Recovery successful: %s\n", resp.Status)
	}

	time.Sleep(1 * time.Second)
	stopMonitoring <- true
}

// circuitBreakerRecoveryExample demonstrates circuit breaker recovery scenarios
func circuitBreakerRecoveryExample() {
	fmt.Println("\n=== Circuit Breaker Recovery Example ===")

	circuitBreaker := httpclient.NewSimpleCircuitBreaker()

	client, err := httpclient.NewClient(
		httpclient.WithCircuitBreaker(circuitBreaker),
		httpclient.WithTimeout(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Helper function to show current state and metrics
	showStatus := func(label string) {
		metrics := client.GetMetrics()
		fmt.Printf("%s - Circuit: %s, Failed: %d, Successful: %d\n",
			label, circuitBreaker.State(), metrics.FailedRequests, metrics.SuccessfulReqs)
	}

	showStatus("Initial state")

	// Force failures to open circuit
	fmt.Println("Forcing failures to open circuit...")
	for i := 0; i < 6; i++ {
		client.Get("https://httpbin.org/status/500")
	}
	showStatus("After failures")

	// Try request while circuit is open (should fail immediately)
	fmt.Println("Trying request while circuit is open...")
	start := time.Now()
	_, err = client.Get("https://httpbin.org/get")
	elapsed := time.Since(start)
	fmt.Printf("Request failed in %v (should be immediate): %v\n", elapsed, err)

	// Manual reset
	fmt.Println("Manually resetting circuit breaker...")
	circuitBreaker.Reset()
	showStatus("After manual reset")

	// Successful request after reset
	fmt.Println("Making successful request after reset...")
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Request successful: %s\n", resp.Status)
	}
	showStatus("After successful request")

	// Final metrics
	finalMetrics := client.GetMetrics()
	fmt.Printf("\n=== Final Metrics ===\n")
	fmt.Printf("Total Requests: %d\n", finalMetrics.TotalRequests)
	fmt.Printf("Successful: %d\n", finalMetrics.SuccessfulReqs)
	fmt.Printf("Failed: %d\n", finalMetrics.FailedRequests)
	fmt.Printf("Circuit Breaker Trips: %d\n", finalMetrics.CircuitBreakerTrips)
	fmt.Printf("Current State: %s\n", finalMetrics.CircuitBreakerState)
}
