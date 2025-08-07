package main

import (
	"fmt"
	"log"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	fmt.Println("🔄 Демонстрация автоматических retry метрик")
	fmt.Println("==========================================")

	// Создание клиента с retry стратегией и метриками
	retryStrategy := httpclient.NewExponentialBackoffStrategy(3, 100*time.Millisecond, 2*time.Second)

	client, err := httpclient.NewClient(
		httpclient.WithRetryStrategy(retryStrategy),
		httpclient.WithMetrics(true),
		httpclient.WithMetricsMeterName("retry-demo-client"),
	)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}

	fmt.Println("✅ HTTP клиент создан с автоматическими retry метриками")

	// Тест 1: Запрос, который вызовет retry (500 ошибка)
	fmt.Println("\n🧪 Тест 1: Запрос с 500 ошибкой (вызовет retry)")
	resp1, err1 := client.Get("http://localhost:5000/test/status/500")
	if err1 != nil {
		fmt.Printf("❌ Запрос завершился ошибкой: %v\n", err1)
	} else {
		defer resp1.Body.Close()
		fmt.Printf("✅ Финальный статус: %d\n", resp1.StatusCode)
	}

	// Тест 2: Запрос, который вызовет много retry попыток (503 ошибка)
	fmt.Println("\n🧪 Тест 2: Запрос с 503 ошибкой (вызовет много retry)")
	resp2, err2 := client.Get("http://localhost:5000/test/status/503")
	if err2 != nil {
		fmt.Printf("❌ Запрос завершился ошибкой: %v\n", err2)
	} else {
		defer resp2.Body.Close()
		fmt.Printf("✅ Финальный статус: %d\n", resp2.StatusCode)
	}

	// Тест 3: Успешный запрос (без retry)
	fmt.Println("\n🧪 Тест 3: Успешный запрос (без retry)")
	resp3, err3 := client.Get("http://localhost:5000/test/status/200")
	if err3 != nil {
		fmt.Printf("❌ Запрос завершился ошибкой: %v\n", err3)
	} else {
		defer resp3.Body.Close()
		fmt.Printf("✅ Успешный запрос, статус: %d\n", resp3.StatusCode)
	}

	// Получаем базовые метрики
	fmt.Println("\n📊 Базовые метрики:")
	metrics := client.GetMetrics()
	fmt.Printf("  Всего запросов: %d\n", metrics.TotalRequests)
	fmt.Printf("  Успешных: %d\n", metrics.SuccessfulReqs)
	fmt.Printf("  Неуспешных: %d\n", metrics.FailedRequests)
	fmt.Printf("  Средняя задержка: %v\n", metrics.AverageLatency)

	fmt.Println("\n🎯 Автоматические retry метрики:")
	fmt.Println("  Retry метрики записываются автоматически в OpenTelemetry/Prometheus")
	fmt.Println("  Доступны по адресу: http://localhost:5000/metrics")
	fmt.Println()
	fmt.Println("  Пример метрик:")
	fmt.Println("  http_retries_total{method=\"GET\",url=\"...\",attempt=\"2\",success=\"false\"}")
	fmt.Println("  http_retries_total{method=\"GET\",url=\"...\",attempt=\"3\",success=\"true\"}")

	fmt.Println("\n✨ Демонстрация завершена!")
	fmt.Println("🔗 Проверьте метрики: http://localhost:5000/metrics")
}
