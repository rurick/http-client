package main

import (
	"fmt"
	"log"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	basicMetricsExample()
	realTimeMonitoringExample()
	decisionMakingExample()
	performanceTestingExample()
}

// basicMetricsExample демонстрирует основное использование встроенных метрик
func basicMetricsExample() {
	fmt.Println("=== Пример использования встроенных метрик ===")

	// Создаем клиент с включенными метриками
	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
		httpclient.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Выполняем различные типы запросов
	fmt.Println("Выполняем тестовые запросы...")

	// Успешные запросы
	client.Get("https://httpbin.org/get")
	client.Get("https://httpbin.org/json")

	// Запросы с ошибками
	client.Get("https://httpbin.org/status/404")
	client.Get("https://httpbin.org/status/500")

	// Медленный запрос
	client.Get("https://httpbin.org/delay/2")

	// Получаем и выводим метрики
	metrics := client.GetMetrics()

	fmt.Printf("\n--- Статистика запросов ---\n")
	fmt.Printf("Всего запросов: %d\n", metrics.TotalRequests)
	fmt.Printf("Успешные: %d\n", metrics.SuccessfulReqs)
	fmt.Printf("Неудачные: %d\n", metrics.FailedRequests)
	fmt.Printf("Процент успеха: %.1f%%\n",
		float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)

	fmt.Printf("\n--- Производительность ---\n")
	fmt.Printf("Средняя задержка: %v\n", metrics.AverageLatency)
	fmt.Printf("Детальные метрики (задержки, статус коды, размеры) доступны в OpenTelemetry/Prometheus\n")
}

// realTimeMonitoringExample показывает мониторинг в реальном времени
func realTimeMonitoringExample() {
	fmt.Println("\n=== Мониторинг в реальном времени ===")

	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Запускаем мониторинг в отдельной горутине
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 3; i++ {
			<-ticker.C
			metrics := client.GetMetrics()

			if metrics.TotalRequests > 0 {
				successRate := float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests) * 100
				fmt.Printf("[МОНИТОРИНГ] Запросов: %d, Успешность: %.1f%%, Средняя задержка: %v\n",
					metrics.TotalRequests, successRate, metrics.AverageLatency)

				// Предупреждения
				if successRate < 80 {
					fmt.Printf("[ПРЕДУПРЕЖДЕНИЕ] Низкая успешность запросов: %.1f%%\n", successRate)
				}

				if metrics.AverageLatency > 2*time.Second {
					fmt.Printf("[ПРЕДУПРЕЖДЕНИЕ] Высокая задержка: %v\n", metrics.AverageLatency)
				}
			}
		}
	}()

	// Имитируем нагрузку
	fmt.Println("Имитируем нагрузку...")
	for i := 0; i < 10; i++ {
		if i%3 == 0 {
			// Иногда медленные запросы
			client.Get("https://httpbin.org/delay/1")
		} else if i%4 == 0 {
			// Иногда ошибки
			client.Get("https://httpbin.org/status/500")
		} else {
			// Обычные запросы
			client.Get("https://httpbin.org/get")
		}
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(4 * time.Second) // Ждем последнего отчета мониторинга
}

// decisionMakingExample показывает принятие решений на основе метрик
func decisionMakingExample() {
	fmt.Println("\n=== Принятие решений на основе метрик ===")

	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Функция для проверки здоровья сервиса
	checkServiceHealth := func(serviceName string) bool {
		metrics := client.GetMetrics()

		if metrics.TotalRequests == 0 {
			return true // Нет данных для анализа
		}

		errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
		avgLatency := metrics.AverageLatency

		fmt.Printf("Анализ сервиса %s:\n", serviceName)
		fmt.Printf("  - Частота ошибок: %.1f%%\n", errorRate*100)
		fmt.Printf("  - Средняя задержка: %v\n", avgLatency)

		// Критерии здоровья
		if errorRate > 0.3 { // Более 30% ошибок
			fmt.Printf("  - ❌ Сервис нездоров: высокая частота ошибок\n")
			return false
		}

		if avgLatency > 3*time.Second { // Задержка более 3 секунд
			fmt.Printf("  - ❌ Сервис нездоров: высокая задержка\n")
			return false
		}

		fmt.Printf("  - ✅ Сервис здоров\n")
		return true
	}

	// Тестируем "плохой" сервис
	fmt.Println("Тестируем проблемный сервис...")
	for i := 0; i < 5; i++ {
		client.Get("https://httpbin.org/status/500") // Всегда ошибка
	}

	if !checkServiceHealth("ProblematicAPI") {
		fmt.Println("➡️  РЕШЕНИЕ: Переключаемся на резервный сервис")
	}

	// Создаем новый клиент для "хорошего" сервиса
	goodClient, _ := httpclient.NewClient(
		httpclient.WithMetrics(true),
	)

	fmt.Println("Тестируем стабильный сервис...")
	for i := 0; i < 5; i++ {
		goodClient.Get("https://httpbin.org/get") // Успешные запросы
	}

	if checkServiceHealth("StableAPI") {
		fmt.Println("➡️  РЕШЕНИЕ: Продолжаем использовать основной сервис")
	}
}

// performanceTestingExample показывает использование метрик в тестах
func performanceTestingExample() {
	fmt.Println("=== Тестирование производительности ===")

	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
		httpclient.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Имитируем performance тест
	fmt.Println("Выполняем нагрузочный тест...")

	startTime := time.Now()

	// Выполняем 10 параллельных запросов
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			client.Get("https://httpbin.org/get")
			done <- true
		}(i)
	}

	// Ждем завершения всех запросов
	for i := 0; i < 10; i++ {
		<-done
	}

	testDuration := time.Since(startTime)
	metrics := client.GetMetrics()

	fmt.Printf("\n--- Результаты нагрузочного теста ---\n")
	fmt.Printf("Время выполнения теста: %v\n", testDuration)
	fmt.Printf("Запросов выполнено: %d\n", metrics.TotalRequests)
	fmt.Printf("Запросов в секунду: %.1f\n",
		float64(metrics.TotalRequests)/testDuration.Seconds())

	// Проверка критериев производительности
	fmt.Printf("\n--- Проверка критериев ---\n")

	if metrics.AverageLatency < 1*time.Second {
		fmt.Printf("✅ Средняя задержка приемлемая: %v\n", metrics.AverageLatency)
	} else {
		fmt.Printf("❌ Средняя задержка слишком высокая: %v\n", metrics.AverageLatency)
	}

	successRate := float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests) * 100
	if successRate >= 95.0 {
		fmt.Printf("✅ Надежность приемлемая: %.1f%%\n", successRate)
	} else {
		fmt.Printf("❌ Надежность низкая: %.1f%%\n", successRate)
	}

	if testDuration < 5*time.Second {
		fmt.Printf("✅ Общее время выполнения приемлемое: %v\n", testDuration)
	} else {
		fmt.Printf("❌ Общее время выполнения слишком долгое: %v\n", testDuration)
	}

	fmt.Println("\n--- Заключение ---")
	if metrics.AverageLatency < 1*time.Second && successRate >= 95.0 && testDuration < 5*time.Second {
		fmt.Println("🎉 Все тесты производительности ПРОЙДЕНЫ!")
	} else {
		fmt.Println("⚠️  Некоторые критерии производительности НЕ выполнены")
	}
}
