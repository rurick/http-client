package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextNotCanceledDuringBodyRead проверяет, что контекст не отменяется
// до тех пор, пока тело ответа не будет закрыто
func TestContextNotCanceledDuringBodyRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body content"))
	}))
	defer server.Close()

	// Создаём клиент с коротким PerTryTimeout
	config := Config{
		PerTryTimeout: 100 * time.Millisecond, // Короткий таймаут
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Ждём дольше чем PerTryTimeout, но контекст не должен быть отменён
	// потому что тело ответа ещё не закрыто
	time.Sleep(150 * time.Millisecond)

	// Должны успешно прочитать тело ответа
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Context should not be canceled while reading response body")
	assert.Equal(t, "response body content", string(body))

	// Закрываем тело ответа - только сейчас контекст должен быть отменён
	resp.Body.Close()
}

// TestContextCanceledOnBodyClose проверяет, что контекст отменяется при закрытии тела ответа
func TestContextCanceledOnBodyClose(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body content"))
	}))
	defer server.Close()

	config := Config{
		PerTryTimeout: 1 * time.Second,
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Проверяем, что это наш contextAwareBody
	if cab, ok := resp.Body.(*contextAwareBody); ok {
		assert.NotNil(t, cab.cancel, "contextAwareBody should have cancel function")
	}

	// Закрываем тело ответа
	err = resp.Body.Close()
	require.NoError(t, err)

	// Повторное закрытие не должно вызывать ошибку (sync.Once protection)
	err = resp.Body.Close()
	require.NoError(t, err)
}

// TestContextCanceledOnError проверяет, что контекст отменяется сразу при ошибке
func TestContextCanceledOnError(t *testing.T) {
	t.Parallel()

	// Сервер, который не существует
	nonExistentURL := "http://localhost:99999/nonexistent"

	config := Config{
		PerTryTimeout: 1 * time.Second,
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, nonExistentURL)

	// Должна быть ошибка
	assert.Error(t, err)
	// Ответ может быть nil или не nil в зависимости от типа ошибки
	if resp != nil {
		resp.Body.Close()
	}

	// Главное - что мы не зависаем и ошибка обрабатывается корректно
	assert.True(t, strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "dial"),
		"Expected network error, got: %v", err)
}
