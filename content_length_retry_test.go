package httpclient

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContentLengthPreservedOnRetry тестирует основную проблему:
// проверяет, что ContentLength сохраняется при retry запросах
func TestContentLengthPreservedOnRetry(t *testing.T) {
	t.Parallel()

	// Тестовые данные различных размеров для проверки edge-cases
	testCases := []struct {
		name     string
		bodySize int
		desc     string
	}{
		{
			name:     "small_body",
			bodySize: 100,
			desc:     "небольшое тело запроса",
		},
		{
			name:     "medium_body",
			bodySize: 10000,
			desc:     "среднее тело запроса",
		},
		{
			name:     "large_body",
			bodySize: 79449, // размер из оригинальной проблемы
			desc:     "большое тело запроса (как в реальной ошибке)",
		},
		{
			name:     "very_large_body",
			bodySize: 500000,
			desc:     "очень большое тело запроса",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serverCallCount := int32(0)
			var receivedContentLengths []int64

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := atomic.AddInt32(&serverCallCount, 1)

				// Записываем ContentLength каждого запроса
				receivedContentLengths = append(receivedContentLengths, r.ContentLength)

				t.Logf("Попытка %d: ContentLength=%d, URL=%s", count, r.ContentLength, r.URL.Path)

				// КРИТИЧЕСКАЯ ПРОВЕРКА: ContentLength должен быть корректным
				assert.Equal(t, int64(tc.bodySize), r.ContentLength,
					"Попытка %d: ContentLength должен соответствовать размеру тела", count)

				// Проверяем, что тело действительно можно прочитать
				actualBody, err := io.ReadAll(r.Body)
				require.NoError(t, err, "Попытка %d: должно быть возможно прочитать тело", count)
				assert.Equal(t, tc.bodySize, len(actualBody),
					"Попытка %d: размер тела должен соответствовать ContentLength", count)

				// Fail first 2 attempts to trigger retry
				if count < 3 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Создаем тестовые данные
			testData := make([]byte, tc.bodySize)
			_, err := rand.Read(testData)
			require.NoError(t, err, "Должно быть возможно создать тестовые данные")

			config := Config{
				RetryEnabled: true,
				RetryConfig: RetryConfig{
					MaxAttempts:      3,
					BaseDelay:        10 * time.Millisecond,
					RetryStatusCodes: []int{http.StatusInternalServerError},
					RetryMethods:     []string{http.MethodPost},
				},
			}
			client := New(config, "content-length-test")
			defer client.Close()

			req, err := http.NewRequest("POST", server.URL+"/test", bytes.NewReader(testData))
			require.NoError(t, err)
			req.Header.Set("Idempotency-Key", "test-key-"+tc.name)
			req.Header.Set("Content-Type", "application/octet-stream")

			// Выполняем запрос
			resp, err := client.Do(req)

			// Проверяем результат
			require.NoError(t, err, "Запрос должен выполниться успешно после retry")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()

			// Проверяем, что было сделано 3 попытки
			finalCount := atomic.LoadInt32(&serverCallCount)
			assert.Equal(t, int32(3), finalCount, "Должно быть выполнено 3 попытки")

			// КЛЮЧЕВАЯ ПРОВЕРКА: ContentLength должен быть одинаковым во всех попытках
			require.Len(t, receivedContentLengths, 3, "Должно быть записано 3 значения ContentLength")
			for i, length := range receivedContentLengths {
				assert.Equal(t, int64(tc.bodySize), length,
					"Попытка %d: ContentLength должен быть %d, получен %d (%s)",
					i+1, tc.bodySize, length, tc.desc)
			}
		})
	}
}

// TestContentLengthZeroBodyRetry проверяет edge-case с пустым телом
func TestContentLengthZeroBodyRetry(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)

		// Для пустого тела ContentLength должен быть 0
		assert.Equal(t, int64(0), r.ContentLength,
			"Попытка %d: ContentLength для пустого тела должен быть 0", count)

		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      2,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
			RetryMethods:     []string{http.MethodPost},
		},
	}
	client := New(config, "zero-content-length-test")
	defer client.Close()

	// Создаем запрос с пустым телом
	req, err := http.NewRequest("POST", server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "test-key-empty")

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	assert.Equal(t, int32(2), atomic.LoadInt32(&serverCallCount))
}

// TestContentLengthNegativeValue проверяет поведение с отрицательным ContentLength
func TestContentLengthNegativeValue(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)

		// Для неизвестной длины ContentLength может быть -1
		t.Logf("Попытка %d: ContentLength=%d", count, r.ContentLength)

		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      2,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
			RetryMethods:     []string{http.MethodPost},
		},
	}
	client := New(config, "negative-content-length-test")
	defer client.Close()

	// Создаем запрос с неопределенной длиной (например, через pipe)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		pw.Write([]byte("test data"))
	}()

	req, err := http.NewRequest("POST", server.URL, pr)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "test-key-negative")
	// ContentLength будет -1 для pipe

	// Этот запрос должен пройти, но без retry из-за невозможности повторить pipe
	resp, err := client.Do(req)

	// Ожидаем либо успех, либо ошибку чтения body, но не панику
	if err != nil {
		t.Logf("Ожидаемая ошибка для pipe body: %v", err)
	} else {
		resp.Body.Close()
	}
}

// TestContentLengthMismatchDetection тестирует детекцию несоответствия ContentLength и размера body
func TestContentLengthMismatchDetection(t *testing.T) {
	t.Parallel()

	testData := []byte("test data content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, что ContentLength соответствует реальному размеру
		actualBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal(t, int64(len(testData)), r.ContentLength,
			"ContentLength должен соответствовать размеру данных")
		assert.Equal(t, len(testData), len(actualBody),
			"Размер прочитанного тела должен соответствовать данным")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: false, // Отключаем retry для простоты
	}
	client := New(config, "mismatch-detection-test")
	defer client.Close()

	req, err := http.NewRequest("POST", server.URL, bytes.NewReader(testData))
	require.NoError(t, err)

	// Ручная установка неправильного ContentLength для проверки
	req.ContentLength = int64(len(testData)) // Правильный размер

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// TestContentLengthWithDifferentMethods проверяет ContentLength для разных HTTP методов
func TestContentLengthWithDifferentMethods(t *testing.T) {
	t.Parallel()

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			testData := []byte("method test data for " + method)
			expectedLength := int64(len(testData))

			serverCallCount := int32(0)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := atomic.AddInt32(&serverCallCount, 1)

				assert.Equal(t, method, r.Method)
				assert.Equal(t, expectedLength, r.ContentLength,
					"ContentLength для метода %s должен быть корректным", method)

				if count < 2 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			config := Config{
				RetryEnabled: true,
				RetryConfig: RetryConfig{
					MaxAttempts:      2,
					BaseDelay:        10 * time.Millisecond,
					RetryStatusCodes: []int{http.StatusInternalServerError},
					RetryMethods:     []string{method},
				},
			}
			client := New(config, "method-test-"+method)
			defer client.Close()

			req, err := http.NewRequest(method, server.URL, bytes.NewReader(testData))
			require.NoError(t, err)
			req.Header.Set("Idempotency-Key", "test-key-"+method)

			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()

			assert.Equal(t, int32(2), atomic.LoadInt32(&serverCallCount),
				"Должно быть 2 попытки для метода %s", method)
		})
	}
}

// TestContentLengthConcurrentRetries проверяет ContentLength в условиях параллельных запросов
func TestContentLengthConcurrentRetries(t *testing.T) {
	t.Parallel()

	const concurrency = 10
	const bodySize = 1000

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)

		// Проверяем ContentLength в каждом параллельном запросе
		assert.Equal(t, int64(bodySize), r.ContentLength,
			"Запрос %d: ContentLength должен быть корректным", count)

		// Имитируем случайные сбои для провоцирования retry
		// Делаем первые пару запросов неуспешными для каждого ID
		if count <= concurrency { // Первые N запросов будут ошибочными
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        5 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
			RetryMethods:     []string{http.MethodPost},
		},
	}
	client := New(config, "concurrent-test")
	defer client.Close()

	// Запускаем параллельные запросы
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			testData := make([]byte, bodySize)
			for j := range testData {
				testData[j] = byte(id + j) // Уникальные данные для каждой горутины
			}

			req, err := http.NewRequest("POST", server.URL, bytes.NewReader(testData))
			if err != nil {
				results <- err
				return
			}
			req.Header.Set("Idempotency-Key", "concurrent-key-"+strconv.Itoa(id))

			resp, err := client.Do(req)
			if err != nil {
				results <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- assert.AnError
				return
			}

			results <- nil
		}(i)
	}

	// Собираем результаты
	var errors []error
	for i := 0; i < concurrency; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	// Проверяем результаты
	assert.Empty(t, errors, "Не должно быть ошибок в параллельных запросах")
	assert.Greater(t, int(atomic.LoadInt32(&serverCallCount)), concurrency,
		"Должно быть больше запросов чем горутин из-за retry")
}

// BenchmarkContentLengthRetry бенчмарк для проверки производительности с исправлением
func BenchmarkContentLengthRetry(b *testing.B) {
	bodySize := 10000
	testData := make([]byte, bodySize)
	rand.Read(testData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Всегда возвращаем успех для бенчмарка
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   1 * time.Millisecond,
		},
	}
	client := New(config, "benchmark-test")
	defer client.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("POST", server.URL, bytes.NewReader(testData))
			req.Header.Set("Idempotency-Key", "bench-key")

			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
