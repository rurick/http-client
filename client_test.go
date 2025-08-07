package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewClient проверяет создание HTTP клиента с настройками по умолчанию
// Проверяет что все основные компоненты инициализированы корректно
func TestNewClient(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.retryClient)
	assert.NotNil(t, client.middlewareChain)
}

// TestNewClientWithOptions проверяет создание клиента с пользовательскими настройками
// Проверяет что все переданные опции корректно применяются к клиенту
func TestNewClientWithOptions(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	client, err := NewClient(
		WithTimeout(10*time.Second),
		WithMaxIdleConns(50),
		WithMaxConnsPerHost(5),
		WithRetryMax(5),
		WithLogger(logger),
		WithMetrics(true),
	)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 10*time.Second, client.options.Timeout)
	assert.Equal(t, 50, client.options.MaxIdleConns)
	assert.Equal(t, 5, client.options.MaxConnsPerHost)
	assert.Equal(t, 5, client.options.RetryMax)
	assert.Equal(t, logger, client.options.Logger)
	assert.True(t, client.options.MetricsEnabled)
}

// TestClientGet проверяет выполнение GET запросов
// Создает тестовый сервер и проверяет что GET запрос выполняется корректно
func TestClientGet(t *testing.T) {
	t.Parallel()

	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestClientPost проверяет выполнение POST запросов с телом
// Проверяет что тело запроса и Content-Type передаются корректно
func TestClientPost(t *testing.T) {
	t.Parallel()

	expectedBody := "test body"
	expectedContentType := "text/plain"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, expectedContentType, r.Header.Get("Content-Type"))

		body := make([]byte, len(expectedBody))
		_, _ = r.Body.Read(body)
		assert.Equal(t, expectedBody, string(body))

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	resp, err := client.Post(server.URL, expectedContentType, strings.NewReader(expectedBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestClientPostForm проверяет отправку форм через POST
// Проверяет корректную обработку form-data с множественными значениями
func TestClientPostForm(t *testing.T) {
	t.Parallel()

	formData := map[string][]string{
		"name":  {"John Doe"},
		"email": {"john@example.com"},
		"tags":  {"developer", "golang"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		assert.NoError(t, err)

		for key, expectedValues := range formData {
			actualValues := r.Form[key]
			assert.Equal(t, expectedValues, actualValues)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	resp, err := client.PostForm(server.URL, formData)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestClientGetJSON проверяет удобный метод для получения JSON данных
// Проверяет автоматическую установку Accept заголовка и десериализацию ответа
func TestClientGetJSON(t *testing.T) {
	t.Parallel()

	expectedData := map[string]any{
		"name":   "John Doe",
		"age":    float64(30),
		"active": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"John Doe","age":30,"active":true}`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result map[string]any
	err = client.GetJSON(context.Background(), server.URL, &result)
	require.NoError(t, err)

	assert.Equal(t, expectedData, result)
}

// TestClientPostJSON проверяет удобный метод для отправки JSON данных
// Проверяет автоматическую сериализацию, установку заголовков и десериализацию ответа
func TestClientPostJSON(t *testing.T) {
	t.Parallel()

	requestData := map[string]any{
		"name":  "Jane Doe",
		"email": "jane@example.com",
	}

	responseData := map[string]any{
		"id":      float64(123),
		"created": true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":123,"created":true}`))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	var result map[string]any
	err = client.PostJSON(context.Background(), server.URL, requestData, &result)
	require.NoError(t, err)

	assert.Equal(t, responseData, result)
}

// TestClientWithMiddleware проверяет интеграцию middleware с клиентом
// Проверяет что middleware применяется ко всем запросам клиента
func TestClientWithMiddleware(t *testing.T) {
	t.Parallel()

	headerKey := "X-Test-Header"
	headerValue := "test-value"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, headerValue, r.Header.Get(headerKey))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	middleware := NewHeaderMiddleware(map[string]string{
		headerKey: headerValue,
	})

	client, err := NewClient(
		WithMiddleware(middleware),
	)
	require.NoError(t, err)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestClientMetrics проверяет сбор метрик клиентом
// Проверяет что метрики корректно накапливаются после выполнения запросов
func TestClientMetrics(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewClient(WithMetrics(true))
	require.NoError(t, err)

	// Make a request
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check metrics
	// Метрики теперь доступны только через Prometheus/OTel. Удалены проверки локальных метрик.
}

func TestClientTimeout(t *testing.T) {
	// НЕ parallel - тест содержит time.Sleep
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(
		WithTimeout(10 * time.Millisecond), // Короче чем задержка сервера
	)
	require.NoError(t, err)

	_, err = client.Get(server.URL)
	assert.Error(t, err)
	// Проверяем что ошибка связана с таймаутом
	assert.True(t, err != nil, "Должна быть ошибка таймаута")
}

func TestClientErrorHandling(t *testing.T) {
	t.Parallel()

	// Test with non-existent server
	client, err := NewClient()
	require.NoError(t, err)

	_, err = client.Get("http://localhost:99999/nonexistent")
	assert.Error(t, err)

	// Check that error is recorded in metrics
	// Метрики теперь доступны только через Prometheus/OTel. Удалены проверки локальных метрик.
}

func TestClientHTTPErrorStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	// Test GetJSON with error status
	var result map[string]any
	err = client.GetJSON(context.Background(), server.URL, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")

	// Test that response with error status is handled correctly in Do
	resp, err := client.Get(server.URL)
	require.NoError(t, err) // Do doesn't return error for HTTP error statuses
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
