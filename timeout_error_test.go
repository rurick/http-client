package httpclient

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutError_DetailedMessage(t *testing.T) {
	// Тест детализированных сообщений об ошибках тайм-аута

	// Подготавливаем тестовый запрос
	req, err := http.NewRequest("POST", "https://api.example.com/slow-endpoint", nil)
	require.NoError(t, err)

	config := Config{
		Timeout:       30 * time.Second,
		PerTryTimeout: 10 * time.Second,
		RetryEnabled:  true,
		RetryConfig: RetryConfig{
			MaxAttempts: 3,
		},
	}

	// Создаём детализированную ошибку тайм-аута
	originalErr := errors.New("context deadline exceeded")
	timeoutErr := NewTimeoutError(req, config, 3, 3, 9*time.Second, "per-try", originalErr)

	// Проверяем, что ошибка содержит все необходимые детали
	errorMsg := timeoutErr.Error()

	t.Logf("Детализированная ошибка тайм-аута:\n%s", errorMsg)

	// Проверяем наличие ключевой информации в сообщении об ошибке
	assert.Contains(t, errorMsg, "POST")                         // HTTP метод
	assert.Contains(t, errorMsg, "api.example.com")              // Хост
	assert.Contains(t, errorMsg, "attempt 3/3")                  // Информация о попытках
	assert.Contains(t, errorMsg, "overall=30s")                  // Общий тайм-аут
	assert.Contains(t, errorMsg, "per-try=10s")                  // Per-try тайм-аут
	assert.Contains(t, errorMsg, "retry=true")                   // Статус retry
	assert.Contains(t, errorMsg, "Type: per-try")                // Тип тайм-аута
	assert.Contains(t, errorMsg, "увеличьте количество попыток") // Предложения

	// Проверяем, что можно развернуть оригинальную ошибку
	assert.Equal(t, originalErr, errors.Unwrap(timeoutErr))
}

func TestTimeoutError_Suggestions(t *testing.T) {
	tests := []struct {
		name                string
		config              Config
		elapsed             time.Duration
		timeoutType         string
		attempt             int
		maxAttempts         int
		expectedSuggestions []string
	}{
		{
			name: "overall timeout without retry",
			config: Config{
				Timeout:      5 * time.Second,
				RetryEnabled: false,
			},
			elapsed:     5 * time.Second,
			timeoutType: "overall",
			attempt:     1,
			maxAttempts: 1,
			expectedSuggestions: []string{
				"увеличьте общий тайм-аут (текущий: 5s)",
				"включите retry для устойчивости к временным сбоям",
			},
		},
		{
			name: "per-try timeout with retry exhausted",
			config: Config{
				Timeout:       30 * time.Second,
				PerTryTimeout: 5 * time.Second,
				RetryEnabled:  true,
				RetryConfig: RetryConfig{
					MaxAttempts: 3,
				},
			},
			elapsed:     5 * time.Second,
			timeoutType: "per-try",
			attempt:     3,
			maxAttempts: 3,
			expectedSuggestions: []string{
				"увеличьте per-try тайм-аут (текущий: 5s)",
				"увеличьте количество попыток (текущий: 3)",
			},
		},
		{
			name: "context timeout",
			config: Config{
				Timeout:       30 * time.Second,
				PerTryTimeout: 10 * time.Second,
				RetryEnabled:  true,
			},
			elapsed:     8 * time.Second,
			timeoutType: "context",
			attempt:     1,
			maxAttempts: 3,
			expectedSuggestions: []string{
				"тайм-аут был задан в context.WithTimeout() или context.WithDeadline()",
				"проверьте настройки контекста вызывающего кода",
			},
		},
		{
			name: "slow service warning",
			config: Config{
				Timeout:       60 * time.Second,
				PerTryTimeout: 20 * time.Second,
				RetryEnabled:  true,
			},
			elapsed:     15 * time.Second,
			timeoutType: "per-try",
			attempt:     2,
			maxAttempts: 3,
			expectedSuggestions: []string{
				"проверьте доступность и производительность удалённого сервиса",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://slow.example.com/api", nil)

			suggestions := generateTimeoutSuggestions(tt.config, tt.elapsed, tt.timeoutType, tt.attempt, tt.maxAttempts)

			for _, expected := range tt.expectedSuggestions {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, expected) || suggestion == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Ожидалось предложение: '%s', но получены: %v", expected, suggestions)
			}

			// Создаём ошибку и проверяем, что предложения включены в сообщение
			timeoutErr := NewTimeoutError(req, tt.config, tt.attempt, tt.maxAttempts, tt.elapsed, tt.timeoutType, errors.New("deadline exceeded"))
			errorMsg := timeoutErr.Error()

			t.Logf("Сценарий '%s':\n%s\n", tt.name, errorMsg)
		})
	}
}

func TestDetermineTimeoutType(t *testing.T) {
	rt := &RoundTripper{}
	config := Config{
		Timeout:       30 * time.Second,
		PerTryTimeout: 10 * time.Second,
	}

	tests := []struct {
		name         string
		err          error
		elapsed      time.Duration
		expectedType string
	}{
		{
			name:         "per-try timeout",
			err:          errors.New("context deadline exceeded"),
			elapsed:      10 * time.Second,
			expectedType: "per-try",
		},
		{
			name:         "overall timeout",
			err:          errors.New("context deadline exceeded"),
			elapsed:      30 * time.Second,
			expectedType: "overall",
		},
		{
			name:         "external context timeout",
			err:          errors.New("context deadline exceeded"),
			elapsed:      5 * time.Second,
			expectedType: "context",
		},
		{
			name:         "network timeout",
			err:          errors.New("i/o timeout"),
			elapsed:      3 * time.Second,
			expectedType: "network",
		},
		{
			name:         "unknown timeout",
			err:          errors.New("some other error with timeout"),
			elapsed:      1 * time.Second,
			expectedType: "network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualType := rt.determineTimeoutType(tt.err, config, tt.elapsed)
			assert.Equal(t, tt.expectedType, actualType)
		})
	}
}

func TestEnhanceTimeoutError_Integration(t *testing.T) {
	// Интеграционный тест: проверяем, что RoundTripper правильно улучшает ошибки тайм-аута

	config := Config{
		Timeout:       5 * time.Second,
		PerTryTimeout: 2 * time.Second,
		RetryEnabled:  true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
		},
	}

	rt := &RoundTripper{
		config: config,
	}

	req, err := http.NewRequest("GET", "https://example.com/api", nil)
	require.NoError(t, err)

	// Симулируем ошибку тайм-аута
	originalErr := &url.Error{
		Op:  "Get",
		URL: "https://example.com/api",
		Err: errors.New("context deadline exceeded"),
	}

	enhanced := rt.enhanceTimeoutError(originalErr, req, config, 1, 2, 2*time.Second)

	// Проверяем, что ошибка была улучшена
	var timeoutErr *TimeoutError
	assert.True(t, errors.As(enhanced, &timeoutErr))
	assert.Equal(t, "GET", timeoutErr.Method)
	assert.Equal(t, "https://example.com/api", timeoutErr.URL)
	assert.Equal(t, "example.com", timeoutErr.Host)
	assert.Equal(t, 1, timeoutErr.Attempt)
	assert.Equal(t, 2, timeoutErr.MaxAttempts)
	assert.Equal(t, "per-try", timeoutErr.TimeoutType)
	assert.True(t, len(timeoutErr.Suggestions) > 0)

	// Проверяем, что оригинальная ошибка сохранена
	assert.Equal(t, originalErr, errors.Unwrap(enhanced))
}

func TestEnhanceTimeoutError_NonTimeoutErrors(t *testing.T) {
	// Тест для проверки, что не-тайм-аут ошибки НЕ изменяются функцией enhanceTimeoutError

	config := Config{
		Timeout:       5 * time.Second,
		PerTryTimeout: 2 * time.Second,
		RetryEnabled:  true,
		RetryConfig: RetryConfig{
			MaxAttempts: 3,
		},
	}

	rt := &RoundTripper{
		config: config,
	}

	req, err := http.NewRequest("POST", "https://api.nalog.ru/endpoint", nil)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		err         error
		description string
	}{
		{
			name:        "connection refused",
			err:         errors.New("dial tcp 127.0.0.1:8080: connect: connection refused"),
			description: "ошибка подключения - сервер недоступен",
		},
		{
			name:        "dns resolution error",
			err:         errors.New("dial tcp: lookup nonexistent.domain: no such host"),
			description: "ошибка DNS - домен не существует",
		},
		{
			name:        "network unreachable",
			err:         &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("network is unreachable")},
			description: "сетевая ошибка - сеть недоступна",
		},
		{
			name:        "http status error",
			err:         errors.New("server returned HTTP 500"),
			description: "ошибка HTTP статуса - не тайм-аут",
		},
		{
			name:        "json parsing error",
			err:         errors.New("invalid character '}' looking for beginning of object key string"),
			description: "ошибка парсинга JSON - не связана с сетью",
		},
		{
			name:        "circuit breaker open",
			err:         ErrCircuitBreakerOpen,
			description: "circuit breaker открыт - не тайм-аут",
		},
		{
			name:        "custom application error",
			err:         errors.New("business logic validation failed"),
			description: "пользовательская ошибка логики приложения",
		},
		{
			name:        "nil error",
			err:         nil,
			description: "нет ошибки",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Вызываем enhanceTimeoutError с не-тайм-аут ошибкой
			result := rt.enhanceTimeoutError(tc.err, req, config, 1, 3, 500*time.Millisecond)

			// Проверяем, что ошибка НЕ была изменена
			assert.Equal(t, tc.err, result,
				"Ошибка '%s' не должна быть изменена функцией enhanceTimeoutError. Описание: %s",
				tc.name, tc.description)

			// Дополнительно проверяем, что результат НЕ является TimeoutError
			var timeoutErr *TimeoutError
			assert.False(t, errors.As(result, &timeoutErr),
				"Не-тайм-аут ошибка '%s' не должна быть преобразована в TimeoutError", tc.name)

			t.Logf("✓ Ошибка '%s' корректно не изменена: %v", tc.name, result)
		})
	}
}

func TestTimeoutError_IsTimeoutError(t *testing.T) {
	// Проверяем, что детализированная ошибка правильно определяется как тайм-аут

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	config := Config{Timeout: 5 * time.Second}
	originalErr := errors.New("context deadline exceeded")

	timeoutErr := NewTimeoutError(req, config, 1, 1, 5*time.Second, "overall", originalErr)

	// Проверяем, что isTimeoutError правильно определяет нашу ошибку как тайм-аут
	assert.True(t, isTimeoutError(timeoutErr))
	assert.True(t, isTimeoutError(originalErr))

	// Проверяем различные типы тайм-аут ошибок
	timeoutErrors := []error{
		errors.New("context deadline exceeded"),
		errors.New("i/o timeout"),
		errors.New("net/http: request canceled while waiting for connection (Client.Timeout exceeded)"),
		&url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("context deadline exceeded")},
		timeoutErr, // наша детализированная ошибка
	}

	for i, err := range timeoutErrors {
		assert.True(t, isTimeoutError(err), "Ошибка #%d должна определяться как тайм-аут: %v", i, err)
	}

	// Проверяем, что обычные ошибки НЕ определяются как тайм-аут
	nonTimeoutErrors := []error{
		errors.New("connection refused"),
		errors.New("no such host"),
		errors.New("network is unreachable"),
		errors.New("invalid JSON"),
		ErrCircuitBreakerOpen,
		nil,
	}

	for i, err := range nonTimeoutErrors {
		assert.False(t, isTimeoutError(err), "Ошибка #%d НЕ должна определяться как тайм-аут: %v", i, err)
	}
}

func TestTimeoutError_RealWorldScenarios(t *testing.T) {
	// Тест реальных сценариев, подобных проблеме с API налоговой

	testCases := []struct {
		name               string
		url                string
		config             Config
		attempt            int
		maxAttempts        int
		elapsed            time.Duration
		timeoutType        string
		expectedSuggestion string
	}{
		{
			name: "API налоговой - медленный ответ",
			url:  "https://openapi.nalog.ru:8090/open-api/AuthService/0.1",
			config: Config{
				Timeout:       5 * time.Second, // Текущая настройка - слишком мало
				PerTryTimeout: 2 * time.Second, // Слишком мало для API налоговой
				RetryEnabled:  false,           // Retry отключён
			},
			attempt:            1,
			maxAttempts:        1,
			elapsed:            5 * time.Second,
			timeoutType:        "overall",
			expectedSuggestion: "увеличьте общий тайм-аут",
		},
		{
			name: "Улучшенная конфигурация для API налоговой",
			url:  "https://openapi.nalog.ru:8090/open-api/AuthService/0.1",
			config: Config{
				Timeout:       60 * time.Second, // Увеличенный тайм-аут
				PerTryTimeout: 20 * time.Second, // Увеличенный per-try тайм-аут
				RetryEnabled:  true,             // Retry включён
				RetryConfig: RetryConfig{
					MaxAttempts: 4,
				},
			},
			attempt:            2,
			maxAttempts:        4,
			elapsed:            18 * time.Second, // Долгий ответ, но в пределах нормы
			timeoutType:        "per-try",
			expectedSuggestion: "проверьте доступность и производительность удалённого сервиса",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", tc.url, nil)
			require.NoError(t, err)

			originalErr := errors.New("context deadline exceeded")
			timeoutErr := NewTimeoutError(req, tc.config, tc.attempt, tc.maxAttempts, tc.elapsed, tc.timeoutType, originalErr)

			errorMsg := timeoutErr.Error()

			// Проверяем наличие ожидаемого предложения
			assert.Contains(t, errorMsg, tc.expectedSuggestion)

			// Проверяем, что в ошибке есть информация о налоговой
			assert.Contains(t, errorMsg, "nalog.ru")

			t.Logf("Сценарий '%s':\n%s\n", tc.name, errorMsg)

			// Демонстрируем, что можно анализировать тип ошибки программно
			suggestions := timeoutErr.Suggestions
			assert.True(t, len(suggestions) > 0, "Должны быть предложения по исправлению")

			t.Logf("Предложения по исправлению для '%s':", tc.name)
			for i, suggestion := range suggestions {
				t.Logf("  %d. %s", i+1, suggestion)
			}
		})
	}
}
