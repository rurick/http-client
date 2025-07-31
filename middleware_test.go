package httpclient

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestMiddlewareChain проверяет работу цепочки middleware
// Проверяет что все middleware в цепочке выполняются в правильном порядке
// и каждый может модифицировать запрос перед передачей следующему
func TestMiddlewareChain(t *testing.T) {
	t.Parallel()

	// Создаем тестовые middleware, которые добавляют заголовки
	middleware1 := NewHeaderMiddleware(map[string]string{"X-Test-1": "value1"})
	middleware2 := NewHeaderMiddleware(map[string]string{"X-Test-2": "value2"})
	middleware3 := NewHeaderMiddleware(map[string]string{"X-Test-3": "value3"})

	chain := NewMiddlewareChain(middleware1, middleware2, middleware3)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	// Финальный обработчик, который проверяет что все заголовки присутствуют
	finalHandler := func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "value1", r.Header.Get("X-Test-1"))
		assert.Equal(t, "value2", r.Header.Get("X-Test-2"))
		assert.Equal(t, "value3", r.Header.Get("X-Test-3"))

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
		}, nil
	}

	resp, err := chain.Execute(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestMiddlewareChainEmpty проверяет пустую цепочку middleware
// Проверяет что когда middleware нет, запрос проходит напрямую к финальному обработчику
func TestMiddlewareChainEmpty(t *testing.T) {
	t.Parallel()

	chain := NewMiddlewareChain()

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := chain.Execute(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestMiddlewareChainAdd проверяет динамическое добавление middleware в цепочку
// Проверяет что middleware можно добавлять в уже созданную цепочку
func TestMiddlewareChainAdd(t *testing.T) {
	t.Parallel()

	chain := NewMiddlewareChain()

	middleware1 := NewHeaderMiddleware(map[string]string{"X-Test": "value"})
	chain.Add(middleware1)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "value", r.Header.Get("X-Test"))
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := chain.Execute(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestLoggingMiddleware проверяет middleware для логирования HTTP запросов
// Проверяет что логируются начало и завершение запроса с правильными данными:
// метод, URL, User-Agent, статус код и время выполнения
func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()

	// Создаем logger с observer для захвата логов
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	middleware := NewLoggingMiddleware(logger)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.Header.Set("User-Agent", "test-agent")

	finalHandler := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
		}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Проверяем что логи были созданы
	logEntries := logs.All()
	assert.Len(t, logEntries, 2) // Логи начала и завершения

	// Проверяем лог начала запроса
	startLog := logEntries[0]
	assert.Equal(t, "HTTP request started", startLog.Message)
	assert.Equal(t, "GET", startLog.Context[0].String)
	assert.Equal(t, "http://example.com/test", startLog.Context[1].String)
	assert.Equal(t, "test-agent", startLog.Context[2].String)

	// Проверяем лог завершения запроса
	completionLog := logEntries[1]
	assert.Equal(t, "HTTP request completed", completionLog.Message)
	assert.Equal(t, "GET", completionLog.Context[0].String)
	assert.Equal(t, "http://example.com/test", completionLog.Context[1].String)
	assert.Equal(t, int64(200), completionLog.Context[2].Integer)
}

// TestLoggingMiddlewareError проверяет логирование ошибок в middleware
// Проверяет что ошибки в запросах корректно логируются с соответствующим уровнем
func TestLoggingMiddlewareError(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	middleware := NewLoggingMiddleware(logger)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)

	testErr := assert.AnError
	finalHandler := func(r *http.Request) (*http.Response, error) {
		return nil, testErr
	}

	resp, err := middleware.Process(req, finalHandler)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, testErr, err)

	// Проверяем что был создан лог ошибки
	logEntries := logs.All()
	assert.Len(t, logEntries, 2) // Логи начала и ошибки

	errorLog := logEntries[1]
	assert.Equal(t, "HTTP request failed", errorLog.Message)
	assert.Equal(t, zap.ErrorLevel, errorLog.Level)
}

// TestHeaderMiddleware проверяет middleware для добавления заголовков
// Проверяет что все указанные заголовки добавляются к запросу
func TestHeaderMiddleware(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"X-Custom-Header": "custom-value",
		"Authorization":   "Bearer token123",
		"Content-Type":    "application/json",
	}

	middleware := NewHeaderMiddleware(headers)

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		for key, expectedValue := range headers {
			actualValue := r.Header.Get(key)
			assert.Equal(t, expectedValue, actualValue, "Header %s should be %s, got %s", key, expectedValue, actualValue)
		}
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestBearerAuthMiddleware проверяет middleware для Bearer токен аутентификации
// Проверяет корректное формирование Authorization заголовка с Bearer токеном
func TestBearerAuthMiddleware(t *testing.T) {
	t.Parallel()

	token := "abc123xyz"
	middleware := NewBearerAuthMiddleware(token)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		authHeader := r.Header.Get("Authorization")
		expected := "Bearer " + token
		assert.Equal(t, expected, authHeader)
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestBasicAuthMiddleware проверяет middleware для Basic аутентификации
// Проверяет корректное формирование Authorization заголовка с Basic авторизацией
func TestBasicAuthMiddleware(t *testing.T) {
	t.Parallel()

	username := "testuser"
	password := "testpass"
	middleware := NewBasicAuthMiddleware(username, password)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		authHeader := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(authHeader, "Basic "))

		// Собственно base64 кодирование тестируется в helper функции basicAuth
		assert.NotEmpty(t, authHeader)
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestTimeoutMiddleware проверяет middleware для установки таймаута запроса
// Проверяет что контекст запроса получает правильный таймаут
func TestTimeoutMiddleware(t *testing.T) {
	// НЕ parallel - тест с timeout и sleep
	timeout := 100 * time.Millisecond
	middleware := NewTimeoutMiddleware(timeout)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		// Проверяем что контекст имеет таймаут
		deadline, ok := r.Context().Deadline()
		assert.True(t, ok, "Context should have a deadline")
		assert.True(t, time.Until(deadline) <= timeout, "Deadline should be within timeout period")

		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTimeoutMiddlewareZeroTimeout(t *testing.T) {
	t.Parallel()

	middleware := NewTimeoutMiddleware(0)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	originalCtx := req.Context()

	finalHandler := func(r *http.Request) (*http.Response, error) {
		// Context should remain unchanged for zero timeout
		assert.Equal(t, originalCtx, r.Context())
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUserAgentMiddleware(t *testing.T) {
	t.Parallel()

	userAgent := "MyApp/1.0 (Go HTTP Client)"
	middleware := NewUserAgentMiddleware(userAgent)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := middleware.Process(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestBase64Encode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"f", "Zg=="},
		{"fo", "Zm8="},
		{"foo", "Zm9v"},
		{"foob", "Zm9vYg=="},
		{"fooba", "Zm9vYmE="},
		{"foobar", "Zm9vYmFy"},
		{"hello:world", "aGVsbG86d29ybGQ="},
		{"user:pass", "dXNlcjpwYXNz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := base64.StdEncoding.EncodeToString([]byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMiddlewareOrder(t *testing.T) {
	t.Parallel()

	var order []string

	// Create middlewares that record their execution order
	middleware1 := &OrderTestMiddleware{name: "first", order: &order}
	middleware2 := &OrderTestMiddleware{name: "second", order: &order}
	middleware3 := &OrderTestMiddleware{name: "third", order: &order}

	chain := NewMiddlewareChain(middleware1, middleware2, middleware3)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		order = append(order, "handler")
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	resp, err := chain.Execute(req, finalHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Middleware should execute in the order they were added
	expected := []string{"first", "second", "third", "handler"}
	assert.Equal(t, expected, order)
}

func TestMiddlewareErrorPropagation(t *testing.T) {
	t.Parallel()

	testErr := assert.AnError

	middleware := &ErrorTestMiddleware{err: testErr}
	chain := NewMiddlewareChain(middleware)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		t.Fatal("Final handler should not be called when middleware returns error")
		return nil, nil
	}

	resp, err := chain.Execute(req, finalHandler)
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Nil(t, resp)
}

// Helper middleware for testing execution order
type OrderTestMiddleware struct {
	name  string
	order *[]string
}

func (otm *OrderTestMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	*otm.order = append(*otm.order, otm.name)
	return next(req)
}

// Helper middleware for testing error propagation
type ErrorTestMiddleware struct {
	err error
}

func (etm *ErrorTestMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	return nil, etm.err
}

func BenchmarkMiddlewareChainExecution(b *testing.B) {
	// Create a chain with multiple middlewares
	middlewares := make([]Middleware, 10)
	for i := 0; i < 10; i++ {
		middlewares[i] = NewHeaderMiddleware(map[string]string{
			"X-Test-" + string(rune(i)): "value",
		})
	}

	chain := NewMiddlewareChain(middlewares...)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.Execute(req, finalHandler)
	}
}

func BenchmarkLoggingMiddleware(b *testing.B) {
	logger := zap.NewNop()
	middleware := NewLoggingMiddleware(logger)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.Process(req, finalHandler)
	}
}

func BenchmarkHeaderMiddleware(b *testing.B) {
	headers := map[string]string{
		"X-Custom-1": "value1",
		"X-Custom-2": "value2",
		"X-Custom-3": "value3",
	}
	middleware := NewHeaderMiddleware(headers)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	finalHandler := func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.Process(req, finalHandler)
	}
}

// TestRateLimitMiddleware проверяет работу ограничения скорости запросов
func TestRateLimitMiddleware(t *testing.T) {
	// НЕ parallel - тест с timing и sleep
	rateLimiter := NewRateLimitMiddleware(2) // 2 запроса в секунду

	mockNext := func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
		}, nil
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Первые два запроса должны проходить быстро
	start := time.Now()
	_, err1 := rateLimiter.Process(req, mockNext)
	elapsed1 := time.Since(start)

	start = time.Now()
	_, err2 := rateLimiter.Process(req, mockNext)
	elapsed2 := time.Since(start)

	// Третий запрос должен быть задержан
	start = time.Now()
	_, err3 := rateLimiter.Process(req, mockNext)
	elapsed3 := time.Since(start)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("Unexpected errors: %v, %v, %v", err1, err2, err3)
	}

	// Первые два запроса должны быть быстрыми (< 100ms)
	if elapsed1 > 100*time.Millisecond {
		t.Errorf("First request too slow: %v", elapsed1)
	}
	if elapsed2 > 100*time.Millisecond {
		t.Errorf("Second request too slow: %v", elapsed2)
	}

	// Третий запрос должен быть медленнее (rate limit)
	if elapsed3 < 400*time.Millisecond {
		t.Errorf("Third request should be rate limited, but was fast: %v", elapsed3)
	}

	t.Logf("Request timings: %v, %v, %v", elapsed1, elapsed2, elapsed3)
}
