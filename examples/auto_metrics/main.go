package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "github.com/rurick/http-client"
)

func main() {
	// Создаём HTTP клиент с автоматическими метриками
	// Метрики регистрируются автоматически при первом создании клиента
	client := httpclient.New(httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 2,
		},
		// MetricsEnabled: По умолчанию true, можно не указывать
	}, "example-service")

	defer client.Close()

	// Запускаем HTTP сервер для метрик в отдельной горутине
	go func() {
		http.Handle("/metrics", promhttp.Handler()) // Используем стандартный registry!
		log.Printf("Метрики доступны на http://localhost:8080/metrics")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Printf("Ошибка запуска сервера метрик: %v", err)
		}
	}()

	// Создаём второй клиент - метрики уже зарегистрированы, повторной регистрации не будет
	client2 := httpclient.New(httpclient.Config{}, "another-service")
	defer client2.Close()

	fmt.Println("🚀 HTTP клиенты созданы с автоматическими метриками!")
	fmt.Println("📊 Метрики автоматически зарегистрированы в prometheus.DefaultRegistry")
	fmt.Println("🔗 Откройте http://localhost:8080/metrics для просмотра метрик")
	fmt.Println()

	// Делаем несколько запросов для генерации метрик
	ctx := context.Background()

	fmt.Println("Выполняем тестовые запросы...")

	// Успешный запрос
	resp, err := client.Get(ctx, "https://httpbin.org/get")
	if err != nil {
		fmt.Printf("❌ Ошибка запроса: %v\n", err)
	} else {
		fmt.Printf("✅ GET запрос успешен: %s\n", resp.Status)
		resp.Body.Close()
	}

	// Запрос с retry (404)
	resp, err = client.Get(ctx, "https://httpbin.org/status/404")
	if err != nil {
		fmt.Printf("❌ Ошибка запроса: %v\n", err)
	} else {
		fmt.Printf("⚠️  GET запрос завершён: %s\n", resp.Status)
		resp.Body.Close()
	}

	// Запрос от второго клиента
	resp, err = client2.Get(ctx, "https://httpbin.org/json")
	if err != nil {
		fmt.Printf("❌ Ошибка запроса от client2: %v\n", err)
	} else {
		fmt.Printf("✅ GET запрос от client2 успешен: %s\n", resp.Status)
		resp.Body.Close()
	}

	fmt.Println()
	fmt.Println("🎯 Все запросы выполнены! Проверьте метрики:")
	fmt.Println("   curl http://localhost:8080/metrics | grep http_client")
	fmt.Println()
	fmt.Println("📈 Вы должны увидеть метрики с лейблами:")
	fmt.Println("   - client_name=\"example-service\"")
	fmt.Println("   - client_name=\"another-service\"")
	fmt.Println()

	// Ждём чтобы дать время просмотреть метрики
	fmt.Println("⏳ Сервер метрик будет работать 30 секунд...")
	time.Sleep(30 * time.Second)

	fmt.Println("✨ Пример завершён!")
}
