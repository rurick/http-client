// Демонстрация расширенных возможностей HTTP клиента
package main

import (
	"fmt"
	"log"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("=== Демонстрация расширенных возможностей HTTP клиента ===")

	iteratorExample()
	slicesExample()
	mapsExample()
	smartRetryExample()
}

// iteratorExample демонстрирует работу с middleware цепочкой
func iteratorExample() {
	fmt.Println("1. Работа с middleware цепочкой:")

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Создаем middleware цепочку
	chain := httpclient.NewMiddlewareChain()
	chain.AddAll(
		httpclient.NewLoggingMiddleware(logger),
		httpclient.NewAuthMiddleware("Bearer", "token123"),
		httpclient.NewTimeoutMiddleware(5*time.Second),
	)

	// Получаем список middleware
	fmt.Println("Middleware в цепочке:")
	middlewares := chain.GetMiddlewares()
	for _, middleware := range middlewares {
		fmt.Printf("  - %T\n", middleware)
	}
	fmt.Println()
}

// slicesExample демонстрирует работу с коллекциями данных
func slicesExample() {
	fmt.Println("2. Эффективная работа с коллекциями данных:")

	// Создаем адаптивную стратегию повтора с историей
	smartStrategy := httpclient.NewSmartRetryStrategy(5, time.Second, 30*time.Second)

	// Проверяем статус коды для повтора
	testCodes := []int{200, 429, 500, 502, 503, 504}

	fmt.Println("Статус коды для анализа повтора:")
	for _, code := range testCodes {
		retryable := httpclient.IsRetryableStatusCode(code)
		fmt.Printf("  - %d: %s\n", code, map[bool]string{true: "повторяем", false: "не повторяем"}[retryable])
	}

	fmt.Printf("Максимум попыток: %d\n", smartStrategy.MaxAttempts())
	fmt.Println()
}

// mapsExample демонстрирует работу с метриками клиента
func mapsExample() {
	fmt.Println("3. Эффективная работа с метриками:")

	client, err := httpclient.NewClient(
		httpclient.WithTimeout(10*time.Second),
		httpclient.WithRetryMax(3),
	)
	if err != nil {
		log.Printf("Ошибка создания клиента: %v", err)
		return
	}

	// Получаем метрики клиента напрямую
	metrics := client.GetMetrics()

	fmt.Println("Текущие метрики клиента:")
	fmt.Printf("  - Всего запросов: %d\n", metrics.TotalRequests)
	fmt.Printf("  - Успешных: %d\n", metrics.SuccessfulReqs)
	fmt.Printf("  - Неудачных: %d\n", metrics.FailedRequests)

	// Получаем статус коды
	fmt.Println("  - Статус коды:")
	statusCodes := metrics.GetStatusCodes()
	for code, count := range statusCodes {
		fmt.Printf("    %d: %d раз\n", code, count)
	}
	fmt.Println()
}

// smartRetryExample демонстрирует адаптивную стратегию повтора
func smartRetryExample() {
	fmt.Println("4. Адаптивная стратегия повтора с анализом истории:")

	smartStrategy := httpclient.NewSmartRetryStrategy(5, 500*time.Millisecond, 10*time.Second)

	fmt.Println("Симуляция адаптивных задержек:")
	for attempt := 1; attempt <= 4; attempt++ {
		delay := smartStrategy.NextDelay(attempt, fmt.Errorf("network error %d", attempt))
		fmt.Printf("  - Попытка %d: задержка %v\n", attempt, delay)
	}

	// Показываем историю задержек
	delayHistory := smartStrategy.GetDelayHistory()
	fmt.Printf("История задержек: %v\n", delayHistory)

	errorHistory := smartStrategy.GetErrorHistory()
	fmt.Printf("Количество ошибок в истории: %d\n", len(errorHistory))
	fmt.Println()
}
