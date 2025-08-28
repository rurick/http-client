//go:build integration

// Пакет integration содержит интеграционные тесты высокого уровня для клиента.
// Данный тест проверяет, что метод (*Client).Do не выполняет повторные попытки,
// когда ретраи отключены в конфигурации.
package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// TestClientDo_NoRetryOn5xx проверяет, что при отключенных ретраях клиент
// возвращает первый ответ сервера (в т.ч. 5xx) без дополнительных попыток.
func TestClientDo_NoRetryOn5xx(t *testing.T) {
	// Поднимаем тестовый сервер, который на первый запрос отдаёт 503, а далее 200.
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 на первую попытку
			_, _ = w.Write([]byte("fail-1"))             // ожидаемое тело ошибки
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Конфигурация без ретраев
	cfg := httpclient.Config{
		RetryEnabled:  false,
		Timeout:       2 * time.Second,
		PerTryTimeout: 2 * time.Second,
	}
	client := httpclient.New(cfg, "test-no-retry")
	defer client.Close()

	// Формируем запрос и вызываем Do напрямую
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)

	// Ожидаем отсутствие ошибки уровня транспорта и статус 503 от первой попытки
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "должны вернуть первый статус без ретраев")
		// Проверяем тело ответа
		b, readErr := io.ReadAll(resp.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, "fail-1", string(b), "тело ответа должно соответствовать ожидаемому")
	}

	// Проверяем, что сервер был вызван ровно один раз (никаких повторных попыток)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "запрос должен быть выполнен ровно один раз")
}

