// Демонстрация нового API с функциональными опциями и методами WithHeaders
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Создаем клиент
	client := httpclient.New(httpclient.Config{}, "demo-service")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Демонстрация нового API ===")

	// 1. GET с функциональными опциями
	fmt.Println("1. GET запрос с функциональными опциями:")
	resp, err := client.Get(ctx, "https://httpbin.org/headers",
		httpclient.WithHeader("Accept", "application/json"),
		httpclient.WithUserAgent("MyApp/1.0"),
		httpclient.WithHeader("X-Custom-Header", "custom-value"),
	)
	if err != nil {
		log.Printf("Ошибка GET: %v", err)
	} else {
		fmt.Printf("GET успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 2. POST с функциональными опциями
	fmt.Println("\n2. POST запрос с функциональными опциями:")
	jsonData := []byte(`{"name": "example", "value": 123}`)
	resp, err = client.Post(ctx, "https://httpbin.org/post", bytes.NewReader(jsonData),
		httpclient.WithContentType("application/json"),
		httpclient.WithBearerToken("my-token-123"),
		httpclient.WithIdempotencyKey("post-operation-456"),
		httpclient.WithHeader("X-Request-ID", "req-789"),
	)
	if err != nil {
		log.Printf("Ошибка POST: %v", err)
	} else {
		fmt.Printf("POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 3. GET с методом WithHeaders
	fmt.Println("\n3. GET запрос через GetWithHeaders:")
	headers := map[string]string{
		"Accept":      "application/json",
		"User-Agent":  "MyApp/2.0",
		"X-API-Key":   "api-key-123",
		"X-Client-ID": "client-456",
	}
	resp, err = client.GetWithHeaders(ctx, "https://httpbin.org/headers", headers)
	if err != nil {
		log.Printf("Ошибка GetWithHeaders: %v", err)
	} else {
		fmt.Printf("GetWithHeaders успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 4. POST с методом WithHeaders
	fmt.Println("\n4. POST запрос через PostWithHeaders:")
	postHeaders := map[string]string{
		"Content-Type":    "application/json",
		"Authorization":   "Bearer another-token",
		"Idempotency-Key": "payment-789",
		"X-Trace-ID":      "trace-123",
	}
	resp, err = client.PostWithHeaders(ctx, "https://httpbin.org/post", bytes.NewReader(jsonData), postHeaders)
	if err != nil {
		log.Printf("Ошибка PostWithHeaders: %v", err)
	} else {
		fmt.Printf("PostWithHeaders успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 5. PATCH - новый метод
	fmt.Println("\n5. PATCH запрос (новый метод):")
	patchData := []byte(`{"status": "updated"}`)
	resp, err = client.Patch(ctx, "https://httpbin.org/patch", bytes.NewReader(patchData),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("patch-operation-123"),
	)
	if err != nil {
		log.Printf("Ошибка PATCH: %v", err)
	} else {
		fmt.Printf("PATCH успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 6. Комбинация опций
	fmt.Println("\n6. PUT с комбинацией опций:")
	// Сначала создаем map с основными заголовками
	baseHeaders := map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    "api-key-999",
	}
	// Затем добавляем дополнительные через опции
	resp, err = client.Put(ctx, "https://httpbin.org/put", bytes.NewReader(jsonData),
		httpclient.WithHeaders(baseHeaders),         // map заголовков
		httpclient.WithBearerToken("special-token"), // дополнительный заголовок
		httpclient.WithHeader("X-Priority", "high"), // еще один заголовок
	)
	if err != nil {
		log.Printf("Ошибка PUT: %v", err)
	} else {
		fmt.Printf("PUT успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println("\n=== Демонстрация завершена ===")
}
