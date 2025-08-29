// Пример демонстрации детализированных ошибок тайм-аута для API ФНС
// Решает исходную проблему "context deadline exceeded" с минимальной информацией
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// NalogRuAuthRequest структура для запроса авторизации в API ФНС
type NalogRuAuthRequest struct {
	Inn      string `json:"inn"`
	Password string `json:"password"`
	DeviceOS string `json:"deviceOS"`
	DeviceId string `json:"deviceId"`
}

func main() {
	fmt.Println("=== Демонстрация детализированных ошибок тайм-аута ===")

	// 1. Демонстрируем проблемную конфигурацию (как было)
	fmt.Println("1. Проблемная конфигурация (как было):")
	demonstrateProblematicConfig()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")

	// 2. Демонстрируем улучшенную конфигурацию (как стало)
	fmt.Println("2. Улучшенная конфигурация (как стало):")
	demonstrateImprovedConfig()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")

	// 3. Демонстрируем обработку не-тайм-аут ошибок
	fmt.Println("3. Обработка других типов ошибок:")
	demonstrateNonTimeoutErrors()
}

// demonstrateProblematicConfig показывает поведение с проблемной конфигурацией
func demonstrateProblematicConfig() {
	// Конфигурация с короткими тайм-аутами (как было раньше)
	config := httpclient.Config{
		Timeout:       5 * time.Second, // Слишком короткий для API ФНС
		PerTryTimeout: 2 * time.Second, // Слишком короткий
		RetryEnabled:  false,           // Retry отключён
	}

	client := httpclient.New(config, "refuel-receipts-old")
	defer client.Close()

	// Создаём контекст с коротким тайм-аутом для симуляции проблемы
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Подготавливаем запрос
	authRequest := NalogRuAuthRequest{
		Inn:      "1234567890",
		Password: "password",
		DeviceOS: "iOS",
		DeviceId: "device123",
	}

	jsonBody, _ := json.Marshal(authRequest)

	// Делаем запрос к "медленному" эндпоинту (симуляция API ФНС)
	resp, err := client.Post(ctx, "https://httpbin.org/delay/10", bytes.NewReader(jsonBody),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("auth-request-12345"),
	)

	if err != nil {
		// Демонстрируем детализированную ошибку
		fmt.Printf("❌ Детализированная ошибка:\n%s\n", err.Error())

		// Проверяем, является ли это детализированной ошибкой тайм-аута
		var timeoutErr *httpclient.TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Printf("\n🔍 Анализ ошибки:\n")
			fmt.Printf("  • Метод: %s\n", timeoutErr.Method)
			fmt.Printf("  • URL: %s\n", timeoutErr.URL)
			fmt.Printf("  • Хост: %s\n", timeoutErr.Host)
			fmt.Printf("  • Попытка: %d из %d\n", timeoutErr.Attempt, timeoutErr.MaxAttempts)
			fmt.Printf("  • Общий тайм-аут: %v\n", timeoutErr.Timeout)
			fmt.Printf("  • Per-try тайм-аут: %v\n", timeoutErr.PerTryTimeout)
			fmt.Printf("  • Время выполнения: %v\n", timeoutErr.Elapsed)
			fmt.Printf("  • Тип тайм-аута: %s\n", timeoutErr.TimeoutType)
			fmt.Printf("  • Retry включён: %t\n", timeoutErr.RetryEnabled)

			fmt.Printf("\n💡 Предложения по исправлению:\n")
			for i, suggestion := range timeoutErr.Suggestions {
				fmt.Printf("  %d. %s\n", i+1, suggestion)
			}
		}
		return
	}

	if resp != nil {
		resp.Body.Close()
		fmt.Printf("✅ Неожиданный успех: %s\n", resp.Status)
	}
}

// demonstrateImprovedConfig показывает поведение с улучшенной конфигурацией
func demonstrateImprovedConfig() {
	// Улучшенная конфигурация для работы с API ФНС
	config := httpclient.Config{
		Timeout:       60 * time.Second, // Увеличенный общий тайм-аут
		PerTryTimeout: 20 * time.Second, // Увеличенный per-try тайм-аут
		RetryEnabled:  true,             // Включён retry
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:       4, // 4 попытки
			BaseDelay:         500 * time.Millisecond,
			MaxDelay:          15 * time.Second,
			Jitter:            0.3, // 30% jitter
			RespectRetryAfter: true,
			// Дополнительные статусы для retry
			RetryStatusCodes: []int{408, 429, 500, 502, 503, 504, 520, 521, 522, 524},
		},
		TracingEnabled: true,
	}

	client := httpclient.New(config, "refuel-receipts-improved")
	defer client.Close()

	ctx := context.Background()

	authRequest := NalogRuAuthRequest{
		Inn:      "1234567890",
		Password: "password",
		DeviceOS: "iOS",
		DeviceId: "device123",
	}

	jsonBody, _ := json.Marshal(authRequest)

	fmt.Println("Попытка запроса с улучшенной конфигурацией...")

	// Делаем запрос к быстрому эндпоинту для демонстрации успеха
	resp, err := client.Post(ctx, "https://httpbin.org/delay/1", bytes.NewReader(jsonBody),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("auth-request-improved-12345"),
	)

	if err != nil {
		fmt.Printf("❌ Ошибка (даже с улучшенной конфигурацией):\n%s\n", err.Error())

		var timeoutErr *httpclient.TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Printf("\n💡 Предложения:\n")
			for i, suggestion := range timeoutErr.Suggestions {
				fmt.Printf("  %d. %s\n", i+1, suggestion)
			}
		}
		return
	}

	if resp != nil {
		defer resp.Body.Close()
		fmt.Printf("✅ Успешный запрос: %s\n", resp.Status)
		fmt.Printf("📊 Конфигурация сработала:\n")
		fmt.Printf("  • Общий тайм-аут: %v\n", config.Timeout)
		fmt.Printf("  • Per-try тайм-аут: %v\n", config.PerTryTimeout)
		fmt.Printf("  • Максимум попыток: %d\n", config.RetryConfig.MaxAttempts)
		fmt.Printf("  • Retry включён: %t\n", config.RetryEnabled)
	}
}

// demonstrateNonTimeoutErrors демонстрирует, что не-тайм-аут ошибки не изменяются
func demonstrateNonTimeoutErrors() {
	config := httpclient.Config{
		Timeout:       30 * time.Second,
		PerTryTimeout: 10 * time.Second,
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
		},
	}

	client := httpclient.New(config, "error-demo")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Демонстрация различных типов ошибок:")

	// 1. DNS ошибка
	fmt.Println("\n1. DNS ошибка:")
	_, err := client.Get(ctx, "https://nonexistent-domain-12345.com/api")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Ошибка: %s\n", err.Error())
		fmt.Printf("   Это TimeoutError? %t\n", isTimeoutErr)
	}

	// 2. Connection refused
	fmt.Println("\n2. Connection refused:")
	_, err = client.Get(ctx, "http://127.0.0.1:99999/api")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Ошибка: %s\n", err.Error())
		fmt.Printf("   Это TimeoutError? %t\n", isTimeoutErr)
	}

	// 3. HTTP статус ошибка
	fmt.Println("\n3. HTTP статус ошибка:")
	resp, err := client.Get(ctx, "https://httpbin.org/status/500")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Ошибка: %s\n", err.Error())
		fmt.Printf("   Это TimeoutError? %t\n", isTimeoutErr)
	} else if resp != nil {
		defer resp.Body.Close()
		fmt.Printf("   HTTP статус: %s (не ошибка, а ответ)\n", resp.Status)
		fmt.Printf("   Это не обрабатывается как TimeoutError\n")
	}

	fmt.Printf("\n✅ Как видите, только реальные тайм-аут ошибки улучшаются.\n")
	fmt.Printf("   Все остальные ошибки остаются неизменными.\n")
}

// Вспомогательная функция для демонстрации программной обработки ошибок
func handleError(err error, operation string) {
	if err == nil {
		return
	}

	// Проверяем, является ли это детализированной ошибкой тайм-аута
	var timeoutErr *httpclient.TimeoutError
	if errors.As(err, &timeoutErr) {
		log.Printf("❌ Тайм-аут при %s:", operation)
		log.Printf("   URL: %s", timeoutErr.URL)
		log.Printf("   Попытка: %d/%d", timeoutErr.Attempt, timeoutErr.MaxAttempts)
		log.Printf("   Время выполнения: %v", timeoutErr.Elapsed)
		log.Printf("   Тип: %s", timeoutErr.TimeoutType)

		// Программно обрабатываем разные типы тайм-аутов
		switch timeoutErr.TimeoutType {
		case "overall":
			log.Printf("   → Рекомендация: увеличьте общий тайм-аут с %v", timeoutErr.Timeout)
		case "per-try":
			log.Printf("   → Рекомендация: увеличьте per-try тайм-аут с %v", timeoutErr.PerTryTimeout)
		case "context":
			log.Printf("   → Рекомендация: проверьте настройки контекста вызывающего кода")
		}

		return
	}

	// Обрабатываем другие типы ошибок как обычно
	log.Printf("❌ Ошибка при %s: %v", operation, err)
}
