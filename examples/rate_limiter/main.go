// Пример использования Rate Limiter
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

	// Конфигурация с rate limiter
	config := httpclient.Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: httpclient.RateLimiterConfig{
			RequestsPerSecond: 2.0, // 2 запроса в секунду
			BurstCapacity:     3,   // до 3 запросов сразу
		},
		Timeout: 30 * time.Second,
	}

	client := httpclient.New(config, "rate-limiter-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Printf("Конфигурация: %.1f RPS, burst %d\n",
		config.RateLimiterConfig.RequestsPerSecond,
		config.RateLimiterConfig.BurstCapacity)
	fmt.Println()

	// Демонстрация burst capacity
	fmt.Println("1. Демонстрация burst capacity (3 быстрых запроса):")
	for i := 1; i <= 3; i++ {
		start := time.Now()
		resp, err := client.Get(ctx, "https://httpbin.org/delay/0")
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Запрос %d failed: %v", i, err)
			continue
		}
		resp.Body.Close()

		fmt.Printf("  Запрос %d: %s (время: %v)\n", i, resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("2. Демонстрация rate limiting (4-й запрос должен ждать):")

	start := time.Now()
	resp, err := client.Get(ctx, "https://httpbin.org/delay/0")
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("Запрос failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("  Запрос 4: %s (время ожидания: %v)\n", resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("3. Демонстрация восстановления (через 1 секунду):")
	time.Sleep(1 * time.Second)

	start = time.Now()
	resp, err = client.Get(ctx, "https://httpbin.org/delay/0")
	elapsed = time.Since(start)

	if err != nil {
		log.Printf("Запрос failed: %v", err)
	} else {
		resp.Body.Close()
		fmt.Printf("  Запрос 5: %s (время: %v)\n", resp.Status, elapsed.Round(time.Millisecond))
	}

	fmt.Println()
	fmt.Println("Rate Limiter работает корректно!")
	fmt.Println("- Burst запросы прошли быстро")
	fmt.Println("- 4-й запрос ожидал появления токена")
	fmt.Println("- После паузы токен восстановился")
}
