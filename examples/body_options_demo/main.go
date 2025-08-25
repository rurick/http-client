// Демонстрация новых опций для работы с телом запроса
package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// User структура для JSON/XML примеров
type User struct {
	XMLName xml.Name `xml:"user" json:"-"`
	ID      int      `json:"id" xml:"id"`
	Name    string   `json:"name" xml:"name"`
	Email   string   `json:"email" xml:"email"`
	Active  bool     `json:"active" xml:"active"`
}

func main() {
	// Создаем клиент
	client := httpclient.New(httpclient.Config{}, "body-options-demo")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Демонстрация опций для работы с телом запроса ===")

	// 1. JSON body с автоматической сериализацией
	fmt.Println("1. POST с JSON body (автоматическая сериализация):")
	user := User{
		ID:     123,
		Name:   "John Doe",
		Email:  "john@example.com",
		Active: true,
	}

	resp, err := client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithJSONBody(user),
		httpclient.WithHeader("X-Request-ID", "json-test-1"),
	)
	if err != nil {
		log.Printf("Ошибка JSON POST: %v", err)
	} else {
		fmt.Printf("JSON POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 2. Form body
	fmt.Println("\n2. POST с Form body:")
	formData := url.Values{
		"username": {"johndoe"},
		"password": {"secret123"},
		"remember": {"true"},
	}

	resp, err = client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithFormBody(formData),
		httpclient.WithHeader("X-Form-Type", "login"),
	)
	if err != nil {
		log.Printf("Ошибка Form POST: %v", err)
	} else {
		fmt.Printf("Form POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 3. XML body
	fmt.Println("\n3. POST с XML body:")
	resp, err = client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithXMLBody(user),
		httpclient.WithHeader("X-API-Version", "2.0"),
	)
	if err != nil {
		log.Printf("Ошибка XML POST: %v", err)
	} else {
		fmt.Printf("XML POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 4. Text body
	fmt.Println("\n4. POST с Text body:")
	textContent := "This is a plain text message.\nIt can have multiple lines."

	resp, err = client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithTextBody(textContent),
		httpclient.WithHeader("X-Message-Type", "notification"),
	)
	if err != nil {
		log.Printf("Ошибка Text POST: %v", err)
	} else {
		fmt.Printf("Text POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 5. Multipart form data
	fmt.Println("\n5. POST с Multipart form data:")
	fields := map[string]string{
		"name":        "John Doe",
		"description": "Test upload",
		"type":        "document",
	}

	resp, err = client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithMultipartFormData(fields, "boundary123"),
		httpclient.WithHeader("X-Upload-Session", "sess-456"),
	)
	if err != nil {
		log.Printf("Ошибка Multipart POST: %v", err)
	} else {
		fmt.Printf("Multipart POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 6. Комбинированный подход - обычный body параметр + опции
	fmt.Println("\n6. POST с обычным body параметром + опции:")
	jsonData := []byte(`{"custom": "data", "version": 3}`)

	resp, err = client.Post(ctx, "https://httpbin.org/post", bytes.NewReader(jsonData),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("operation-789"),
		httpclient.WithBearerToken("token-xyz"),
	)
	if err != nil {
		log.Printf("Ошибка комбинированного POST: %v", err)
	} else {
		fmt.Printf("Комбинированный POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 7. Raw body для полного контроля
	fmt.Println("\n7. PUT с Raw body:")
	customData := bytes.NewBufferString("custom-binary-data")

	resp, err = client.Put(ctx, "https://httpbin.org/put", nil,
		httpclient.WithRawBody(customData),
		httpclient.WithContentType("application/octet-stream"),
		httpclient.WithHeader("X-Data-Format", "binary"),
	)
	if err != nil {
		log.Printf("Ошибка Raw PUT: %v", err)
	} else {
		fmt.Printf("Raw PUT успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	// 8. Использование нескольких body опций (последняя побеждает)
	fmt.Println("\n8. POST с несколькими body опциями (демонстрация приоритета):")
	resp, err = client.Post(ctx, "https://httpbin.org/post", nil,
		httpclient.WithTextBody("This will be overwritten"),
		httpclient.WithJSONBody(map[string]string{"final": "content"}), // Эта опция перезапишет предыдущую
		httpclient.WithHeader("X-Test", "priority"),
	)
	if err != nil {
		log.Printf("Ошибка приоритета: %v", err)
	} else {
		fmt.Printf("Приоритет POST успешно: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println("\n=== Демонстрация завершена ===")
}
