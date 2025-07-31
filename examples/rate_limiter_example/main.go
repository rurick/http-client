package main

import (
	"fmt"
	"log"
	"time"

	"gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	fmt.Println("=== Rate Limiter Example ===")

	// Создаем клиент с ограничением 2 запроса в секунду
	rateLimiter := httpclient.NewRateLimitMiddleware(2)

	client, err := httpclient.NewClient(
		httpclient.WithMiddleware(rateLimiter),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Делаем 5 быстрых запросов (лимит: 2 запроса/сек)")

	for i := 1; i <= 5; i++ {
		start := time.Now()

		resp, err := client.Get("https://httpbin.org/delay/0")
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("Запрос %d: Ошибка - %v (время: %v)\n", i, err, elapsed)
			continue
		}
		defer resp.Body.Close()

		fmt.Printf("Запрос %d: Успех %s (время: %v)\n", i, resp.Status, elapsed)
	}

	fmt.Println("\nТест завершен. Rate Limiter должен был задерживать запросы после 2-го.")
}
