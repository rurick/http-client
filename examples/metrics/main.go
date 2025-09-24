// Пример работы с метриками
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	// Создаём клиент с стандартной конфигурацией
	client := httpclient.New(httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "metrics-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Выполняем несколько запросов для генерации метрик...")

	// Успешные запросы
	for i := 0; i < 5; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Ошибка запроса %d: %v", i, err)
			continue
		}
		fmt.Printf("Запрос %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(100 * time.Millisecond)
	}

	// Запросы с ошибками для демонстрации retry метрик
	fmt.Println("Тестируем запросы с ошибками...")
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/status/503")
		if err != nil {
			log.Printf("Ошибка (ожидается): %v", err)
		} else {
			fmt.Printf("Неожиданный успех: %s\n", resp.Status)
			_ = resp.Body.Close()
		}

		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("Метрики собраны. Проверьте /metrics эндпоинт для просмотра.")
	fmt.Println("В production среде метрики будут доступны через Prometheus scraper.")

	// Пример создания HTTP сервера с /metrics endpoint
	// Метрики автоматически доступны через стандартный Prometheus handler
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Метрики доступны на http://localhost:8080/metrics")
	fmt.Println("Все метрики HTTP клиента автоматически регистрируются в DefaultRegistry")
	// Откомментируйте для запуска:
	// log.Fatal(http.ListenAndServe(":8080", nil))
}
