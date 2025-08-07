package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	fmt.Println("🤖 Демонстрация AI-Powered Error Insights")
	fmt.Println("=========================================")

	// Создание клиента с включенными метриками и error insights
	client, err := httpclient.NewClient(
		httpclient.WithTimeout(2*time.Second),
		httpclient.WithMetrics(true),
		httpclient.WithMetricsMeterName("error-insights-demo"),
	)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}

	fmt.Println("✅ HTTP клиент создан с AI-powered error insights")

	// Демонстрация различных типов ошибок

	// 1. Сетевая ошибка - несуществующий хост
	fmt.Println("\n🧪 Тест 1: Сетевая ошибка (несуществующий хост)")
	_, err1 := client.Get("http://non-existent-host-12345.com")
	if err1 != nil {
		insight := client.AnalyzeLastError(context.Background())
		if insight != nil {
			printInsight("Сетевая ошибка", insight)
		}
	}

	// 2. Таймаут ошибка
	fmt.Println("\n🧪 Тест 2: Таймаут (медленный сервер)")
	_, err2 := client.Get("http://httpbin.org/delay/5") // 5 секунд, но таймаут 2 секунды
	if err2 != nil {
		insight := client.AnalyzeLastError(context.Background())
		if insight != nil {
			printInsight("Таймаут", insight)
		}
	}

	// 3. HTTP ошибки
	fmt.Println("\n🧪 Тест 3: Ошибка аутентификации (401)")
	resp3, _ := client.Get("http://httpbin.org/status/401")
	if resp3 != nil {
		defer resp3.Body.Close()
		insight := client.AnalyzeLastError(context.Background())
		if insight != nil {
			printInsight("401 Unauthorized", insight)
		}
	}

	fmt.Println("\n🧪 Тест 4: Rate Limit (429)")
	resp4, _ := client.Get("http://httpbin.org/status/429")
	if resp4 != nil {
		defer resp4.Body.Close()
		insight := client.AnalyzeLastError(context.Background())
		if insight != nil {
			printInsight("429 Rate Limit", insight)
		}
	}

	fmt.Println("\n🧪 Тест 5: Серверная ошибка (500)")
	resp5, _ := client.Get("http://httpbin.org/status/500")
	if resp5 != nil {
		defer resp5.Body.Close()
		insight := client.AnalyzeLastError(context.Background())
		if insight != nil {
			printInsight("500 Server Error", insight)
		}
	}

	// Демонстрация пользовательских правил
	fmt.Println("\n🧪 Тест 6: Пользовательское правило анализа")
	analyzer := client.GetErrorInsightsAnalyzer()

	// Добавляем пользовательское правило для API endpoints
	customRule := httpclient.ErrorAnalysisRule{
		Name: "custom_json_api_error",
		Condition: func(req *http.Request, resp *http.Response, err error) bool {
			return resp != nil && resp.StatusCode == 422 &&
				req.Header.Get("Content-Type") == "application/json"
		},
		Insight: func(req *http.Request, resp *http.Response, err error) *httpclient.ErrorInsight {
			return &httpclient.ErrorInsight{
				Category:    httpclient.ErrorCategoryClientError,
				Title:       "JSON API Валидация",
				Description: "Ошибка валидации JSON данных в API запросе",
				Severity:    "high",
				Suggestions: []string{
					"Проверьте схему JSON данных",
					"Убедитесь что все обязательные поля присутствуют",
					"Проверьте типы данных в полях",
				},
				RetryAdvice: "Исправьте JSON данные перед повтором",
				TechnicalDetails: map[string]interface{}{
					"rule_type": "custom_json_validation",
					"endpoint":  req.URL.String(),
				},
				Timestamp: time.Now(),
			}
		},
	}

	analyzer.AddCustomRule(customRule)
	fmt.Println("✅ Добавлено пользовательское правило для JSON API валидации")

	// Демонстрация управления категориями
	fmt.Println("\n📊 Управление категориями анализа:")
	categories := analyzer.GetSupportedCategories()
	for _, category := range categories {
		enabled := analyzer.IsCategoryEnabled(category)
		status := "включена"
		if !enabled {
			status = "отключена"
		}
		fmt.Printf("  📁 %s: %s\n", category, status)
	}

	// Отключаем категорию timeout
	analyzer.DisableCategory(httpclient.ErrorCategoryTimeout)
	fmt.Println("\n⚠️  Отключена категория 'timeout'")

	fmt.Println("\n✨ Демонстрация AI-Powered Error Insights завершена!")
	fmt.Println("🎯 Возможности:")
	fmt.Println("  ✓ Автоматический анализ всех типов ошибок")
	fmt.Println("  ✓ Контекстные рекомендации и советы")
	fmt.Println("  ✓ Пользовательские правила анализа")
	fmt.Println("  ✓ Гибкое управление категориями")
}

func printInsight(testName string, insight *httpclient.ErrorInsight) {
	fmt.Printf("📋 Анализ ошибки: %s\n", testName)
	fmt.Printf("   🏷️  Категория: %s\n", insight.Category)
	fmt.Printf("   📝 Заголовок: %s\n", insight.Title)
	fmt.Printf("   📖 Описание: %s\n", insight.Description)
	fmt.Printf("   ⚠️  Серьезность: %s\n", insight.Severity)
	fmt.Printf("   🔄 Совет по повторам: %s\n", insight.RetryAdvice)

	if len(insight.Suggestions) > 0 {
		fmt.Printf("   💡 Рекомендации:\n")
		for i, suggestion := range insight.Suggestions {
			fmt.Printf("      %d. %s\n", i+1, suggestion)
		}
	}

	if len(insight.TechnicalDetails) > 0 {
		fmt.Printf("   🔧 Технические детали:\n")
		for key, value := range insight.TechnicalDetails {
			fmt.Printf("      %s: %v\n", key, value)
		}
	}
	fmt.Println()
}
