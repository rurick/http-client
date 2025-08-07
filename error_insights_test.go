package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestErrorInsightsAnalyzer_NetworkErrors тестирует анализ сетевых ошибок
func TestErrorInsightsAnalyzer_NetworkErrors(t *testing.T) {
	analyzer := NewErrorInsightsAnalyzer()

	tests := []struct {
		name             string
		errorString      string
		expectedCategory ErrorCategory
		expectedTitle    string
	}{
		{
			name:             "connection_refused",
			errorString:      "connection refused",
			expectedCategory: ErrorCategoryNetwork,
			expectedTitle:    "Соединение отклонено",
		},
		{
			name:             "timeout_error",
			errorString:      "timeout exceeded",
			expectedCategory: ErrorCategoryTimeout,
			expectedTitle:    "Таймаут запроса",
		},
		{
			name:             "no_such_host",
			errorString:      "no such host",
			expectedCategory: ErrorCategoryNetwork,
			expectedTitle:    "Хост не найден",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://example.com/test", nil)
			err := fmt.Errorf(tt.errorString)

			insight := analyzer.AnalyzeError(context.Background(), req, nil, err)

			if insight.Category != tt.expectedCategory {
				t.Errorf("Expected category %s, got %s", tt.expectedCategory, insight.Category)
			}

			if insight.Title != tt.expectedTitle {
				t.Errorf("Expected title %s, got %s", tt.expectedTitle, insight.Title)
			}

			if len(insight.Suggestions) == 0 {
				t.Error("Expected suggestions, got none")
			}

			if insight.RetryAdvice == "" {
				t.Error("Expected retry advice, got empty string")
			}
		})
	}
}

// TestErrorInsightsAnalyzer_HTTPStatusCodes тестирует анализ HTTP кодов состояния
func TestErrorInsightsAnalyzer_HTTPStatusCodes(t *testing.T) {
	analyzer := NewErrorInsightsAnalyzer()

	tests := []struct {
		statusCode       int
		expectedCategory ErrorCategory
		expectedTitle    string
		expectedSeverity string
	}{
		{
			statusCode:       401,
			expectedCategory: ErrorCategoryAuthentication,
			expectedTitle:    "Ошибка аутентификации",
			expectedSeverity: "high",
		},
		{
			statusCode:       403,
			expectedCategory: ErrorCategoryAuthentication,
			expectedTitle:    "Доступ запрещен",
			expectedSeverity: "high",
		},
		{
			statusCode:       429,
			expectedCategory: ErrorCategoryRateLimit,
			expectedTitle:    "Превышен лимит запросов",
			expectedSeverity: "medium",
		},
		{
			statusCode:       500,
			expectedCategory: ErrorCategoryServerError,
			expectedTitle:    "Ошибка сервера",
			expectedSeverity: "medium",
		},
		{
			statusCode:       404,
			expectedCategory: ErrorCategoryClientError,
			expectedTitle:    "Ошибка клиента",
			expectedSeverity: "high",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     fmt.Sprintf("%d", tt.statusCode),
				Header:     make(http.Header),
			}

			insight := analyzer.AnalyzeError(context.Background(), req, resp, nil)

			if insight.Category != tt.expectedCategory {
				t.Errorf("Expected category %s, got %s", tt.expectedCategory, insight.Category)
			}

			if insight.Title != tt.expectedTitle {
				t.Errorf("Expected title %s, got %s", tt.expectedTitle, insight.Title)
			}

			if insight.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, insight.Severity)
			}

			if len(insight.Suggestions) == 0 {
				t.Error("Expected suggestions, got none")
			}
		})
	}
}

// TestErrorInsightsAnalyzer_CustomRules тестирует пользовательские правила анализа
func TestErrorInsightsAnalyzer_CustomRules(t *testing.T) {
	analyzer := NewErrorInsightsAnalyzer()

	// Добавляем пользовательское правило
	customRule := ErrorAnalysisRule{
		Name: "custom_api_error",
		Condition: func(req *http.Request, resp *http.Response, err error) bool {
			return strings.Contains(req.URL.String(), "/api/") && resp != nil && resp.StatusCode == 422
		},
		Insight: func(req *http.Request, resp *http.Response, err error) *ErrorInsight {
			return &ErrorInsight{
				Category:    ErrorCategoryClientError,
				Title:       "Пользовательская API ошибка",
				Description: "Неверные данные в API запросе",
				Severity:    "high",
				Suggestions: []string{"Проверьте формат данных", "Убедитесь в корректности JSON"},
				RetryAdvice: "Не повторяйте до исправления данных",
				Timestamp:   time.Now(),
				TechnicalDetails: map[string]interface{}{
					"custom_rule": "custom_api_error",
				},
			}
		},
	}

	analyzer.AddCustomRule(customRule)

	req, _ := http.NewRequest("POST", "https://example.com/api/users", nil)
	resp := &http.Response{
		StatusCode: 422,
		Status:     "422 Unprocessable Entity",
		Header:     make(http.Header),
	}

	insight := analyzer.AnalyzeError(context.Background(), req, resp, nil)

	if insight.Title != "Пользовательская API ошибка" {
		t.Errorf("Expected custom rule title, got %s", insight.Title)
	}

	if insight.TechnicalDetails["custom_rule"] != "custom_api_error" {
		t.Error("Expected custom rule to be applied")
	}
}

// TestClient_ErrorInsightsIntegration тестирует интеграцию Error Insights с клиентом
func TestClient_ErrorInsightsIntegration(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
		case "/rate-limit":
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, err := NewClient(
		WithTimeout(5*time.Second),
		WithMetrics(true),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Тест 1: Ошибка аутентификации
	resp, err := client.Get(server.URL + "/unauthorized")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer resp.Body.Close()

	insight := client.AnalyzeLastError(context.Background())
	if insight == nil {
		t.Fatal("Expected error insight, got nil")
	}

	if insight.Category != ErrorCategoryAuthentication {
		t.Errorf("Expected authentication error, got %s", insight.Category)
	}

	// Тест 2: Rate limit ошибка - создаем прямой анализ
	// Создаем тестовый запрос и ответ для анализа
	req2, _ := http.NewRequest("GET", server.URL+"/rate-limit", nil)
	resp2 := &http.Response{
		StatusCode: 429,
		Status:     "429 Too Many Requests",
		Header:     make(http.Header),
	}
	resp2.Header.Set("Retry-After", "60")

	analyzer := client.GetErrorInsightsAnalyzer()
	insight2 := analyzer.AnalyzeError(context.Background(), req2, resp2, nil)

	if insight2 == nil {
		t.Fatal("Expected rate limit insight, got nil")
	}

	if insight2.Category != ErrorCategoryRateLimit {
		t.Errorf("Expected rate limit error, got %s", insight2.Category)
	}

	if insight2.TechnicalDetails["retry_after"] != "60" {
		t.Error("Expected Retry-After header to be captured")
	}

	// Тест 3: Получение анализатора
	analyzer = client.GetErrorInsightsAnalyzer()
	if analyzer == nil {
		t.Fatal("Expected error analyzer, got nil")
	}

	// Тест 4: Настройка анализатора
	analyzer.DisableCategory(ErrorCategoryServerError)
	if analyzer.IsCategoryEnabled(ErrorCategoryServerError) {
		t.Error("Expected server error category to be disabled")
	}
}

// TestErrorInsightsAnalyzer_CategoryManagement тестирует управление категориями
func TestErrorInsightsAnalyzer_CategoryManagement(t *testing.T) {
	analyzer := NewErrorInsightsAnalyzer()

	// Проверяем что все категории включены по умолчанию
	supportedCategories := analyzer.GetSupportedCategories()
	for _, category := range supportedCategories {
		if !analyzer.IsCategoryEnabled(category) {
			t.Errorf("Expected category %s to be enabled by default", category)
		}
	}

	// Отключаем категорию
	analyzer.DisableCategory(ErrorCategoryNetwork)
	if analyzer.IsCategoryEnabled(ErrorCategoryNetwork) {
		t.Error("Expected network category to be disabled")
	}

	// Включаем категорию обратно
	analyzer.EnableCategory(ErrorCategoryNetwork)
	if !analyzer.IsCategoryEnabled(ErrorCategoryNetwork) {
		t.Error("Expected network category to be enabled")
	}
}
