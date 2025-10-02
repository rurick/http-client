package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "github.com/rurick/http-client"
)

func main() {
	// Создание HTTP клиента с базовой конфигурацией (retry отключён)
	config := httpclient.Config{
		Timeout:       5 * time.Second,
		PerTryTimeout: 2 * time.Second,
		RetryEnabled:  false, // Retry по умолчанию отключён
	}

	client := httpclient.New(config, "httpclient")
	defer client.Close()

	// Выполнение GET запроса
	ctx := context.Background()
	if err := performGetRequest(ctx, client); err != nil {
		log.Printf("GET request failed: %v", err)
	}

	// Выполнение POST запроса
	if err := performPostRequest(ctx, client); err != nil {
		log.Printf("POST request failed: %v", err)
	}

	// Запуск HTTP сервера для /metrics endpoint
	startMetricsServerBasic(client)
}

// Метрики теперь создаются автоматически в клиенте через prometheus/client_golang

// performGetRequest выполняет простой GET запрос
func performGetRequest(ctx context.Context, client *httpclient.Client) error {
	fmt.Println("Performing GET request...")

	resp, err := client.Get(ctx, "https://httpbin.org/get")
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("GET Response Status: %s\n", resp.Status)

	// Читаем первые 200 символов ответа
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("GET Response Body (first 200 chars): %s...\n", string(body))
	return nil
}

// performPostRequest выполняет простой POST запрос
func performPostRequest(ctx context.Context, client *httpclient.Client) error {
	fmt.Println("Performing POST request...")

	jsonData := `{"key": "value", "message": "test from http-client"}`
	resp, err := client.Post(ctx, "https://httpbin.org/post", strings.NewReader(jsonData), httpclient.WithContentType("application/json"))
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("POST Response Status: %s\n", resp.Status)

	// Читаем первые 200 символов ответа
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("POST Response Body (first 200 chars): %s...\n", string(body))
	return nil
}

// startMetricsServerBasic запускает HTTP сервер для метрик на порту 2112
func startMetricsServerBasic(client *httpclient.Client) {
	fmt.Println("Starting metrics server on :2112/metrics")

	// Метрики автоматически регистрируются в default registry
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":2112",
		Handler: nil,
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Даём серверу время на запуск
	time.Sleep(1 * time.Second)
	fmt.Println("Metrics available at http://localhost:2112/metrics")

	// Ждём некоторое время для сбора метрик
	time.Sleep(5 * time.Second)
	fmt.Println("Basic usage example completed. Check metrics at http://localhost:2112/metrics")
}
