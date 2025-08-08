// Пример использования идемпотентности
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	client := httpclient.New(httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "idempotency-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Тестируем идемпотентные запросы ===")

	// GET запросы всегда идемпотентны и повторяются
	fmt.Println("1. GET запрос (всегда идемпотентный):")
	resp, err := client.Get(ctx, "https://httpbin.org/status/500,200")
	if err != nil {
		log.Printf("GET ошибка: %v", err)
	} else {
		fmt.Printf("GET успех: %s\n", resp.Status)
		resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	// PUT запросы всегда идемпотентны
	fmt.Println("2. PUT запрос (всегда идемпотентный):")
	req, _ := http.NewRequestWithContext(ctx, "PUT", "https://httpbin.org/status/500,200",
		strings.NewReader(`{"data": "test"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("PUT ошибка: %v", err)
	} else {
		fmt.Printf("PUT успех: %s\n", resp.Status)
		resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	fmt.Println("=== Тестируем POST запросы ===")

	// POST без Idempotency-Key НЕ повторяется
	fmt.Println("3. POST без Idempotency-Key (НЕ повторяется):")
	req, _ = http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/503",
		strings.NewReader(`{"order": "12345"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST без idempotency ошибка (ожидается): %v", err)
	} else {
		fmt.Printf("POST без idempotency успех: %s\n", resp.Status)
		resp.Body.Close()
	}

	time.Sleep(500 * time.Millisecond)

	// POST с Idempotency-Key повторяется
	fmt.Println("4. POST с Idempotency-Key (повторяется):")
	req, _ = http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/500,500,201",
		strings.NewReader(`{"payment": "67890"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "payment-operation-67890")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST с idempotency ошибка: %v", err)
	} else {
		fmt.Printf("POST с idempotency успех: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println("\n=== Резюме ===")
	fmt.Println("✓ GET, PUT, DELETE - всегда повторяются при ошибках")
	fmt.Println("✓ POST, PATCH - повторяются только с Idempotency-Key заголовком")
	fmt.Println("✓ Idempotency-Key должен быть уникальным для каждой операции")
}
