// Package main демонстрирует использование HTTP клиента
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Создание клиента с кастомной конфигурацией
	config := httpclient.Config{
		Timeout: 10 * time.Second,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    5 * time.Second,
		},
		TracingEnabled: false, // отключим для простоты примера
		Transport:      http.DefaultTransport,
	}

	client := httpclient.New(config, "httpclient")
	defer client.Close()

	ctx := context.Background()

	// Пример GET запроса
	fmt.Println("Выполняем GET запрос...")
	resp, err := client.Get(ctx, "https://httpbin.org/get")
	if err != nil {
		log.Printf("Ошибка GET запроса: %v", err)
	} else {
		fmt.Printf("GET ответ: %s\n", resp.Status)
		resp.Body.Close()
	}

	// Пример POST запроса с повтором в случае ошибки
	fmt.Println("Выполняем POST запрос с Idempotency-Key...")
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/503", nil)
	req.Header.Set("Idempotency-Key", "test-key-123")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("Ошибка POST запроса (ожидается из-за 503): %v", err)
	} else {
		fmt.Printf("POST ответ: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println("Пример завершён!")
}
