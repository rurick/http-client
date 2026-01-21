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
	assert.Contains(t, errorMsg, "increase number of attempts") // Suggestions

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
				"increase overall timeout (current: 5s)",
				"enable retry for resilience to temporary failures",
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
				"increase per-try timeout (current: 5s)",
				"increase number of attempts (current: 3)",
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
				"timeout was set in context.WithTimeout() or context.WithDeadline()",
				"check context settings in calling code",
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
				"check availability and performance of remote service",
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
				assert.True(t, found, "Expected suggestion: '%s', but got: %v", expected, suggestions)
			}

			// Create error and check that suggestions are included in message
			timeoutErr := NewTimeoutError(req, tt.config, tt.attempt, tt.maxAttempts, tt.elapsed, tt.timeoutType, errors.New("deadline exceeded"))
			errorMsg := timeoutErr.Error()

			t.Logf("Scenario '%s':\n%s\n", tt.name, errorMsg)
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
	// Integration test: verify that RoundTripper correctly enhances timeout errors

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

	// Simulate timeout error
	originalErr := &url.Error{
		Op:  "Get",
		URL: "https://example.com/api",
		Err: errors.New("context deadline exceeded"),
	}

	enhanced := rt.enhanceTimeoutError(originalErr, req, config, 1, 2, 2*time.Second)

	// Check that error was enhanced
	var timeoutErr *TimeoutError
	assert.True(t, errors.As(enhanced, &timeoutErr))
	assert.Equal(t, "GET", timeoutErr.Method)
	assert.Equal(t, "https://example.com/api", timeoutErr.URL)
	assert.Equal(t, "example.com", timeoutErr.Host)
	assert.Equal(t, 1, timeoutErr.Attempt)
	assert.Equal(t, 2, timeoutErr.MaxAttempts)
	assert.Equal(t, "per-try", timeoutErr.TimeoutType)
	assert.True(t, len(timeoutErr.Suggestions) > 0)

	// Check that original error is preserved
	assert.Equal(t, originalErr, errors.Unwrap(enhanced))
}

func TestEnhanceTimeoutError_NonTimeoutErrors(t *testing.T) {
	// Test to verify that non-timeout errors are NOT changed by enhanceTimeoutError function

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
			description: "connection error - server unavailable",
		},
		{
			name:        "dns resolution error",
			err:         errors.New("dial tcp: lookup nonexistent.domain: no such host"),
			description: "DNS error - domain does not exist",
		},
		{
			name:        "network unreachable",
			err:         &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("network is unreachable")},
			description: "network error - network unavailable",
		},
		{
			name:        "http status error",
			err:         errors.New("server returned HTTP 500"),
			description: "HTTP status error - not timeout",
		},
		{
			name:        "json parsing error",
			err:         errors.New("invalid character '}' looking for beginning of object key string"),
			description: "JSON parsing error - not network related",
		},
		{
			name:        "circuit breaker open",
			err:         ErrCircuitBreakerOpen,
			description: "circuit breaker open - not timeout",
		},
		{
			name:        "custom application error",
			err:         errors.New("business logic validation failed"),
			description: "custom application logic error",
		},
		{
			name:        "nil error",
			err:         nil,
			description: "no error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call enhanceTimeoutError with non-timeout error
			result := rt.enhanceTimeoutError(tc.err, req, config, 1, 3, 500*time.Millisecond)

			// Check that error was NOT changed
			assert.Equal(t, tc.err, result,
				"Error '%s' should not be changed by enhanceTimeoutError. Description: %s",
				tc.name, tc.description)

			// Additionally check that result is NOT TimeoutError
			var timeoutErr *TimeoutError
			assert.False(t, errors.As(result, &timeoutErr),
				"Non-timeout error '%s' should not be converted to TimeoutError", tc.name)

			t.Logf("✓ Error '%s' correctly not changed: %v", tc.name, result)
		})
	}
}

func TestTimeoutError_IsTimeoutError(t *testing.T) {
	// Check that detailed error is correctly identified as timeout

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	config := Config{Timeout: 5 * time.Second}
	originalErr := errors.New("context deadline exceeded")

	timeoutErr := NewTimeoutError(req, config, 1, 1, 5*time.Second, "overall", originalErr)

	// Check that isTimeoutError correctly identifies our error as timeout
	assert.True(t, isTimeoutError(timeoutErr))
	assert.True(t, isTimeoutError(originalErr))

	// Check various types of timeout errors
	timeoutErrors := []error{
		errors.New("context deadline exceeded"),
		errors.New("i/o timeout"),
		errors.New("net/http: request canceled while waiting for connection (Client.Timeout exceeded)"),
		&url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("context deadline exceeded")},
		timeoutErr, // our detailed error
	}

	for i, err := range timeoutErrors {
		assert.True(t, isTimeoutError(err), "Error #%d should be identified as timeout: %v", i, err)
	}

	// Check that regular errors are NOT identified as timeout
	nonTimeoutErrors := []error{
		errors.New("connection refused"),
		errors.New("no such host"),
		errors.New("network is unreachable"),
		errors.New("invalid JSON"),
		ErrCircuitBreakerOpen,
		nil,
	}

	for i, err := range nonTimeoutErrors {
		assert.False(t, isTimeoutError(err), "Error #%d should NOT be identified as timeout: %v", i, err)
	}
}

func TestTimeoutError_RealWorldScenarios(t *testing.T) {
	// Test real-world scenarios similar to tax API problem

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
			name: "Tax API - slow response",
			url:  "https://openapi.nalog.ru:8090/open-api/AuthService/0.1",
			config: Config{
				Timeout:       5 * time.Second, // Current setting - too low
				PerTryTimeout: 2 * time.Second, // Too low for tax API
				RetryEnabled:  false,           // Retry disabled
			},
			attempt:            1,
			maxAttempts:        1,
			elapsed:            5 * time.Second,
			timeoutType:        "overall",
			expectedSuggestion: "increase overall timeout",
		},
		{
			name: "Improved configuration for tax API",
			url:  "https://openapi.nalog.ru:8090/open-api/AuthService/0.1",
			config: Config{
				Timeout:       60 * time.Second, // Increased timeout
				PerTryTimeout: 20 * time.Second, // Increased per-try timeout
				RetryEnabled:  true,             // Retry enabled
				RetryConfig: RetryConfig{
					MaxAttempts: 4,
				},
			},
			attempt:            2,
			maxAttempts:        4,
			elapsed:            18 * time.Second, // Long response, but within normal range
			timeoutType:        "per-try",
			expectedSuggestion: "check availability and performance of remote service",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", tc.url, nil)
			require.NoError(t, err)

			originalErr := errors.New("context deadline exceeded")
			timeoutErr := NewTimeoutError(req, tc.config, tc.attempt, tc.maxAttempts, tc.elapsed, tc.timeoutType, originalErr)

			errorMsg := timeoutErr.Error()

			// Check presence of expected suggestion
			assert.Contains(t, errorMsg, tc.expectedSuggestion)

			// Check that error contains tax API information
			assert.Contains(t, errorMsg, "nalog.ru")

			t.Logf("Scenario '%s':\n%s\n", tc.name, errorMsg)

			// Demonstrate that error type can be analyzed programmatically
			suggestions := timeoutErr.Suggestions
			assert.True(t, len(suggestions) > 0, "Should have suggestions for fixing")

			t.Logf("Suggestions for fixing '%s':", tc.name)
			for i, suggestion := range suggestions {
				t.Logf("  %d. %s", i+1, suggestion)
			}
		})
	}
}
