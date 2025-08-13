// Пример настройки retry механизма
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
	// Конфигурация с агрессивными повторными попытками
	config := httpclient.Config{
		Timeout: 30 * time.Second,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 5,                      // До 5 попыток
			BaseDelay:   200 * time.Millisecond, // Базовая задержка
			MaxDelay:    10 * time.Second,       // Максимальная задержка
			Jitter:      0.3,                    // 30% jitter для избежания thundering herd
		},
		TracingEnabled: true,
		RetryEnabled:   true,
		Transport:      http.DefaultTransport,
	}

	client := httpclient.New(config, "retry-example")
	defer client.Close()

	ctx := context.Background()

	// Тестируем на эндпоинте, который иногда возвращает 503
	fmt.Println("Тестируем retry механизм...")

	resp, err := client.Get(ctx, "https://httpbin.org/status/200,503,503,200")
	if err != nil {
		if maxErr, ok := err.(*httpclient.MaxAttemptsExceededError); ok {
			log.Printf("Запрос не удался после %d попыток: %v", maxErr.MaxAttempts, maxErr.LastError)
		} else {
			log.Printf("Не retriable ошибка: %v", err)
		}
		return
	}

	fmt.Printf("Успешный ответ: %s\n", resp.Status)
	resp.Body.Close()

	// Пример POST запроса с идемпотентностью
	fmt.Println("Тестируем POST с Idempotency-Key...")

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://httpbin.org/status/500,500,201", nil)
	req.Header.Set("Idempotency-Key", "operation-12345")
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		log.Printf("POST запрос не удался: %v", err)
		return
	}

	fmt.Printf("POST ответ: %s\n", resp.Status)
	resp.Body.Close()
}
