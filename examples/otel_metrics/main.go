// Пример работы с OpenTelemetry метриками
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	httpclient "github.com/rurick/http-client"
)

func main() {
	// Создаём клиент с OpenTelemetry метриками
	client := httpclient.New(httpclient.Config{
		MetricsBackend: httpclient.MetricsBackendOpenTelemetry,
		RetryEnabled:   true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "otel-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Выполняем несколько запросов с OpenTelemetry метриками...")

	// Успешные запросы
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Ошибка запроса %d: %v", i, err)
			continue
		}
		fmt.Printf("Запрос %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(200 * time.Millisecond)
	}

	// Запросы с ошибками
	fmt.Println("Тестируем запросы с ошибками...")
	for i := 0; i < 2; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/status/503")
		if err != nil {
			log.Printf("Ошибка (ожидается): %v", err)
		} else {
			fmt.Printf("Неожиданный успех: %s\n", resp.Status)
			_ = resp.Body.Close()
		}

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Println("Метрики успешно записаны в OpenTelemetry!")
	fmt.Println("Для просмотра метрик настройте OpenTelemetry SDK с подходящим экспортером.")
	fmt.Println()
	fmt.Println("Пример настройки бакетов гистограмм через Views:")
	fmt.Println(`
	view.New(view.Instrument{
		Name: "http_client_request_duration_seconds",
		Kind: view.InstrumentKindHistogram,
	}, view.Stream{
		Aggregation: view.AggregationExplicitBucketHistogram{
			Boundaries: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
	})
	`)
}