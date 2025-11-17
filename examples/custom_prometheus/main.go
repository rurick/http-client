// Пример работы с кастомным Prometheus регистратором
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "github.com/rurick/http-client"
)

func main() {
	// Создаём кастомный Prometheus registry
	customRegistry := prometheus.NewRegistry()

	// Создаём клиент с кастомным регистратором
	client := httpclient.New(httpclient.Config{
		MetricsBackend:       httpclient.MetricsBackendPrometheus, // явно указываем prometheus
		PrometheusRegisterer: customRegistry,
		RetryEnabled:         true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
	}, "custom-prometheus-example")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Выполняем запросы с кастомным Prometheus регистратором...")

	// Успешные запросы
	for i := 0; i < 3; i++ {
		resp, err := client.Get(ctx, "https://httpbin.org/get")
		if err != nil {
			log.Printf("Ошибка запроса %d: %v", i, err)
			continue
		}
		fmt.Printf("Запрос %d: %s\n", i+1, resp.Status)
		_ = resp.Body.Close()

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Метрики сохранены в кастомном регистраторе!")
	fmt.Println("Доступ к метрикам через кастомный handler на http://localhost:8081/custom-metrics")

	// Создаём HTTP сервер с кастомным handler
	http.Handle("/custom-metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))
	fmt.Println("Сервер запущен на :8081")
	fmt.Println("Откройте http://localhost:8081/custom-metrics для просмотра метрик")
	
	// В этом примере метрики будут доступны только через кастомный registry,
	// а не через стандартный DefaultRegistry
	
	// Запускаем сервер (закомментировано, чтобы не блокировать выполнение)
	// log.Fatal(http.ListenAndServe(":8081", nil))
	
	fmt.Println("Пример завершен. В production используйте кастомный registry для изоляции метрик.")
}