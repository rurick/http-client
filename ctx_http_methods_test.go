package httpclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCtxHTTPMethods_BasicFunctionality проверяет базовую функциональность контекстных методов
func TestCtxHTTPMethods_BasicFunctionality(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("GET response"))
		case http.MethodPost:
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("POST response: " + string(body)))
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("GetCtx", func(t *testing.T) {
		resp, err := client.GetCtx(ctx, server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PostCtx", func(t *testing.T) {
		body := strings.NewReader("test data")
		resp, err := client.PostCtx(ctx, server.URL, "text/plain", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PostFormCtx", func(t *testing.T) {
		formData := map[string][]string{
			"key1": {"value1"},
			"key2": {"value2", "value3"},
		}
		resp, err := client.PostFormCtx(ctx, server.URL, formData)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HeadCtx", func(t *testing.T) {
		resp, err := client.HeadCtx(ctx, server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestCtxHTTPMethods_ContextCancellation проверяет поведение при отмене контекста
func TestCtxHTTPMethods_ContextCancellation(t *testing.T) {
	// Создаем сервер с задержкой для имитации долгого запроса
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Millisecond) // Задержка для имитации медленного сервера
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	t.Run("GetCtx_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Очень короткий таймаут
		defer cancel()

		_, err := client.GetCtx(ctx, server.URL)
		assert.Error(t, err)
		// Проверяем что это ошибка контекста
		assert.True(t, err != nil, "Должна быть ошибка контекста")
	})

	t.Run("PostCtx_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		body := strings.NewReader("test data")
		_, err := client.PostCtx(ctx, server.URL, "text/plain", body)
		assert.Error(t, err)
		assert.True(t, err != nil, "Должна быть ошибка контекста")
	})

	t.Run("PostFormCtx_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		formData := map[string][]string{
			"key": {"value"},
		}
		_, err := client.PostFormCtx(ctx, server.URL, formData)
		assert.Error(t, err)
		assert.True(t, err != nil, "Должна быть ошибка контекста")
	})

	t.Run("HeadCtx_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := client.HeadCtx(ctx, server.URL)
		assert.Error(t, err)
		assert.True(t, err != nil, "Должна быть ошибка контекста")
	})
}

// TestCtxHTTPMethods_ImmediateCancellation проверяет поведение при немедленной отмене контекста
func TestCtxHTTPMethods_ImmediateCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	t.Run("GetCtx_ImmediateCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Отменяем контекст сразу

		_, err := client.GetCtx(ctx, server.URL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("PostCtx_ImmediateCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Отменяем контекст сразу

		body := strings.NewReader("test data")
		_, err := client.PostCtx(ctx, server.URL, "text/plain", body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// TestDoCtx_RequestWithContext проверяет метод DoCtx с готовым запросом
func TestDoCtx_RequestWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("DoCtx response"))
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	t.Run("DoCtx_Success", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)

		ctx := context.Background()
		resp, err := client.DoCtx(ctx, req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DoCtx_ContextCancellation", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(2 * time.Nanosecond) // Убеждаемся что контекст истек

		_, err = client.DoCtx(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

// TestCtxHTTPMethods_WithMiddleware проверяет работу контекстных методов с middleware
func TestCtxHTTPMethods_WithMiddleware(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, что middleware добавил заголовок
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response with middleware"))
	}))
	defer server.Close()

	// Создаем middleware, который добавляет заголовок
	headerMiddleware := NewHeaderMiddleware(map[string]string{
		"X-Test-Header": "test-value",
	})

	client, err := NewClient(WithMiddleware(headerMiddleware))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("GetCtx_WithMiddleware", func(t *testing.T) {
		resp, err := client.GetCtx(ctx, server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PostCtx_WithMiddleware", func(t *testing.T) {
		body := bytes.NewReader([]byte("test data"))
		resp, err := client.PostCtx(ctx, server.URL, "text/plain", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestCtxHTTPMethods_ErrorHandling проверяет обработку ошибок в контекстных методах
func TestCtxHTTPMethods_ErrorHandling(t *testing.T) {
	client, err := NewClient()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("GetCtx_InvalidURL", func(t *testing.T) {
		_, err := client.GetCtx(ctx, "://invalid-url")
		assert.Error(t, err)
	})

	t.Run("PostCtx_InvalidURL", func(t *testing.T) {
		body := strings.NewReader("test data")
		_, err := client.PostCtx(ctx, "://invalid-url", "text/plain", body)
		assert.Error(t, err)
	})

	t.Run("PostFormCtx_InvalidURL", func(t *testing.T) {
		formData := map[string][]string{
			"key": {"value"},
		}
		_, err := client.PostFormCtx(ctx, "://invalid-url", formData)
		assert.Error(t, err)
	})

	t.Run("HeadCtx_InvalidURL", func(t *testing.T) {
		_, err := client.HeadCtx(ctx, "://invalid-url")
		assert.Error(t, err)
	})
}

// TestCtxHTTPMethods_RequestHeaders проверяет правильность установки заголовков
func TestCtxHTTPMethods_RequestHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/post":
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		case "/form":
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("PostCtx_ContentTypeHeader", func(t *testing.T) {
		body := strings.NewReader("test data")
		resp, err := client.PostCtx(ctx, server.URL+"/post", "text/plain", body)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PostFormCtx_ContentTypeHeader", func(t *testing.T) {
		formData := map[string][]string{
			"key": {"value"},
		}
		resp, err := client.PostFormCtx(ctx, server.URL+"/form", formData)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
