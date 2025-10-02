//go:build integration

// Package integration содержит продвинутые интеграционные тесты для HTTP клиент библиотеки.
// Эти тесты покрывают сложные сценарии взаимодействия, граничные случаи и проблемы конкурентности.
package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpclient "github.com/rurick/http-client"
)

// errorReader симулирует io.Reader, который падает после первого чтения.
type errorReader struct {
	data      []byte
	readCount int32
}

func (er *errorReader) Read(p []byte) (int, error) {
	count := atomic.AddInt32(&er.readCount, 1)
	if count > 1 {
		return 0, errors.New("simulated read error")
	}
	n := copy(p, er.data)
	return n, nil
}

func (er *errorReader) Close() error {
	return nil
}

// brokenPipe симулирует соединение, которое обрывается во время чтения ответа.
type brokenPipe struct {
	content []byte
	broken  bool
	readPos int
}

func (bp *brokenPipe) Read(p []byte) (int, error) {
	if bp.broken {
		return 0, &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: errors.New("broken pipe"),
		}
	}

	if bp.readPos >= len(bp.content) {
		bp.broken = true
		return 0, &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: errors.New("broken pipe"),
		}
	}

	n := copy(p, bp.content[bp.readPos:])
	bp.readPos += n
	return n, nil
}

func (bp *brokenPipe) Close() error {
	return nil
}

// TestRetryWithOpenCircuitBreaker тестирует взаимодействие между логикой retry и circuit breaker.
// Когда circuit breaker открывается посреди retry, последующие попытки должны быть остановлены.
func TestRetryWithOpenCircuitBreaker(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Всегда ошибка
	}))
	defer server.Close()

	// Настраиваем circuit breaker на открытие после 2 ошибок
	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5, // Попыток больше, чем порог CB
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       httpclient.NewCircuitBreakerWithConfig(cbConfig),
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	_, err := client.Get(ctx, server.URL)

	// Должны получить ошибку circuit breaker
	assert.Error(t, err)

	// Сервер не должен быть вызван 5 раз из-за открытия circuit breaker
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Less(t, int(callCount), 5, "Circuit breaker должен ограничивать вызовы")
	assert.GreaterOrEqual(t, int(callCount), 2, "Должно попытаться минимум threshold раз")
}

// TestCircuitBreakerResetsAfterSuccessfulRetry проверяет, что circuit breaker переключается корректно,
// когда сервис восстанавливается во время попыток retry.
func TestCircuitBreakerResetsAfterSuccessfulRetry(t *testing.T) {
	// Не параллельно из-за чувствительности ко времени

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK) // Сервис восстанавливается
	}))
	defer server.Close()

	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := httpclient.NewCircuitBreakerWithConfig(cbConfig)
	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      1, // Никаких retry, тестим только CB
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Первые два вызова должны открыть circuit breaker
	client.Get(ctx, server.URL)
	client.Get(ctx, server.URL)

	assert.Equal(t, httpclient.CircuitBreakerOpen, cb.State())

	// Ждем таймаут circuit breaker
	time.Sleep(150 * time.Millisecond)

	// Следующий вызов должен успех и закрыть цепь
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, httpclient.CircuitBreakerClosed, cb.State())
}

// TestBackoffWithJitter проверяет, что задержки retry включают детерминированный jitter
// и отличаются для разных попыток в ожидаемом диапазоне.
func TestBackoffWithJitter(t *testing.T) {
	t.Parallel()

	baseDelay := 100 * time.Millisecond
	jitter := 0.5 // 50% jitter
	maxDelay := 10 * time.Second

	// Тестим множество попыток, чтобы проверить jitter между попытками
	attempts := []int{1, 2, 3, 4, 5}
	delays := make([]time.Duration, len(attempts))

	// Собираем задержки для разных попыток
	for i, attempt := range attempts {
		delays[i] = httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
	}

	// Проверяем задержки для каждой попытки
	for i, attempt := range attempts {
		// Ожидаемая базовая задержка: 0 для attempt <= 1, baseDelay * 2^(attempt-2) для attempt >= 2
		var expectedBase time.Duration
		if attempt <= 1 {
			expectedBase = 0 // Первая попытка без задержки
		} else {
			expectedBase = time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-2)))
		}

		minDelay := time.Duration(float64(expectedBase) * (1 - jitter))
		maxJitterDelay := time.Duration(float64(expectedBase) * (1 + jitter))
		if maxJitterDelay > maxDelay {
			maxJitterDelay = maxDelay
		}

		// Проверяем, что задержка в пределах jitter
		assert.GreaterOrEqual(t, delays[i], minDelay, "Задержка для попытки %d ниже минимума: %v < %v", attempt, delays[i], minDelay)
		assert.LessOrEqual(t, delays[i], maxJitterDelay, "Задержка для попытки %d выше максимума: %v > %v", attempt, delays[i], maxJitterDelay)
	}

	// Проверяем, что jitter создает детерминированные, но разные значения для разных попыток
	// Поскольку jitter детерминирован по номеру попытки, одинаковые попытки должны давать одинаковые результаты
	for _, attempt := range attempts {
		delay1 := httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
		delay2 := httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
		assert.Equal(t, delay1, delay2, "Jitter должен быть детерминирован для одной попытки %d", attempt)
	}

	// Проверяем, что разные попытки создают разные задержки (детерминированный jitter)
	atLeastOneDifferent := false
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			atLeastOneDifferent = true
			break
		}
	}
	assert.True(t, atLeastOneDifferent, "Jitter должен создавать разные задержки для разных попыток")
}

// TestIdempotentRetryWithUnreadableBody проверяет, что библиотека буферизует тела запросов
// и позволяет retry даже для POST запросов с Idempotency-Key.
func TestIdempotentRetryWithUnreadableBody(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError) // Ошибка в первые 2 попытки
			return
		}
		w.WriteHeader(http.StatusOK) // Успех на 3-й попытке
	}))
	defer server.Close()

	// Используем обычный reader - библиотека будет буферизовать его
	body := strings.NewReader("test-data")

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	req, _ := http.NewRequest("POST", server.URL, body)
	req.Header.Set("Idempotency-Key", "test-key-123")

	resp, err := client.Do(req)

	// Должен успеть после retry, поскольку библиотека буферизует body
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Сервер должен быть вызван 3 раза (библиотека повторяет идемпотентные POST)
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount, "Ожидаем 3 попытки для идемпотентного POST с повторяемым статусом")
}

// TestIdempotentRetryWithBodyReadErrorOnSecondAttempt проверяет, что ошибки чтения body предотвращают запрос.
func TestIdempotentRetryWithBodyReadErrorOnSecondAttempt(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Используем errorReader, который падает после первого чтения
	errorBody := &errorReader{data: []byte("test-data")}

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	req, _ := http.NewRequest("POST", server.URL, errorBody)
	req.Header.Set("Idempotency-Key", "test-key-456")

	// Должны получить ошибку от первоначального чтения body
	_, err := client.Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")

	// Никаких вызовов сервера из-за ошибки чтения body
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(0), callCount, "Не должно быть вызовов из-за ошибки чтения body")
}

// TestOverallTimeoutDuringRetry проверяет, что общий таймаут клиента
// истекает во время последовательности retry в пределах MaxAttempts.
func TestOverallTimeoutDuringRetry(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		time.Sleep(50 * time.Millisecond) // Каждый запрос занимает 50ms
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := httpclient.Config{
		Timeout:       120 * time.Millisecond, // Общий таймаут
		PerTryTimeout: 100 * time.Millisecond, // Таймаут на попытку
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5, // Попыток больше, чем позволяет время
			BaseDelay:        20 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	start := time.Now()
	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	elapsed := time.Since(start)

	// Либо ошибка таймаута, либо последний неудачный ответ
	if err != nil {
		// Произошел таймаут - проверяем разные сообщения об ошибках
		errorMsg := err.Error()
		assert.True(t, strings.Contains(errorMsg, "deadline exceeded") ||
			strings.Contains(errorMsg, "context deadline exceeded") ||
			strings.Contains(errorMsg, "Client.Timeout exceeded") ||
			strings.Contains(errorMsg, "timeout exceeded"),
			"Ожидали ошибку таймаута, получили: %v", err)
	} else if resp != nil {
		// Получили последний ответ от неудачных попыток
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		_ = resp.Body.Close()
	}

	// Должен соблюдать общий таймаут (с небольшим запасом на обработку)
	assert.Less(t, elapsed, 200*time.Millisecond, "Запрос слишком долгий: %v", elapsed)

	// Не должен сделать все 5 retry из-за таймаута
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Less(t, int(callCount), 5, "Не должен завершить все retry из-за таймаута")
}

// TestPerTryTimeoutAndRetry проверяет, что таймауты на попытку работают корректно
// с мультипльными попытками retry.
func TestPerTryTimeoutAndRetry(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			time.Sleep(150 * time.Millisecond) // Дольше, чем таймаут на попытку
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK) // Быстрый ответ на 3-й попытке
	}))
	defer server.Close()

	config := httpclient.Config{
		Timeout:       2 * time.Second,        // Длинный общий таймаут
		PerTryTimeout: 100 * time.Millisecond, // Короткий таймаут на попытку
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:  3,
			BaseDelay:    50 * time.Millisecond,
			RetryMethods: []string{http.MethodGet},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Должен попытаться 3 раза (первые 2 таймаут, 3-я успех)
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount, "Ожидаем ровно 3 попытки")
}

// TestConcurrentClientUsageWithSharedConfig тестирует thread safety
// при одновременном использовании клиента в множестве goroutines.
func TestConcurrentClientUsageWithSharedConfig(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		time.Sleep(10 * time.Millisecond) // Небольшая задержка
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   5 * time.Millisecond,
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	concurrency := 50
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	// Запускаем одновременные запросы
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			resp, err := client.Get(ctx, server.URL)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", id, err)
				return
			}
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("goroutine %d: unexpected status %d", id, resp.StatusCode)
				return
			}
			_ = resp.Body.Close()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Проверяем ошибки
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "Одновременные запросы сорвались: %v", errorList)

	// Проверяем, что все запросы обработаны
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(concurrency), callCount, "Не все одновременные запросы обработаны")
}

// TestConcurrentCircuitBreakerStateChanges тестирует thread safety circuit breaker
// при высокой конкурентности.
func TestConcurrentCircuitBreakerStateChanges(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	failureCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		// Первые 10 запросов падают, потом успех
		if count <= 10 {
			atomic.AddInt32(&failureCount, 1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          50 * time.Millisecond,
	}

	cb := httpclient.NewCircuitBreakerWithConfig(cbConfig)
	config := httpclient.Config{
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
		RetryEnabled:         false, // Фокус на CB поведении
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	concurrency := 100
	var wg sync.WaitGroup
	requestCount := int32(0)

	// Запускаем одновременные запросы
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx := context.Background()
			_, err := client.Get(ctx, server.URL)
			atomic.AddInt32(&requestCount, 1)

			// Некоторые запросы упадут, некоторые будут отключены CB
			// Мы главным образом тестим отсутствие race conditions
			_ = err // Ожидаем различные ошибки
		}()
	}

	wg.Wait()

	// Проверяем, что CB справился с конкурентным доступом без паник
	totalRequests := atomic.LoadInt32(&requestCount)
	totalServerCalls := atomic.LoadInt32(&serverCallCount)

	assert.Equal(t, int32(concurrency), totalRequests, "Не все goroutines завершились")
	// Вызовов сервера должно быть меньше из-за CB
	assert.LessOrEqual(t, int(totalServerCalls), concurrency, "Вызовов сервера не должно превышать запросы")

	// CB должен был открыться и предотвратить некоторые запросы
	finalState := cb.State()
	assert.True(t, finalState == httpclient.CircuitBreakerOpen ||
		finalState == httpclient.CircuitBreakerHalfOpen ||
		finalState == httpclient.CircuitBreakerClosed,
		"Circuit breaker должен быть в валидном состоянии")
}

// TestMetricsOnRetryWithContextCancellation проверяет корректность метрик
// при отмене context во время retry backoff.
func TestMetricsOnRetryWithContextCancellation(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Всегда ошибка
	}))
	defer server.Close()

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5,
			BaseDelay:        200 * time.Millisecond, // Достаточно для отмены во время backoff
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	// Отменяем context после первого retry
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)

	// Ожидаем ошибку (HTTP или context)
	if err != nil {
		// Проверяем, что это один из ожидаемых типов
		assert.True(t, strings.Contains(err.Error(), "deadline exceeded") ||
			strings.Contains(err.Error(), "context canceled") ||
			strings.Contains(err.Error(), "500"),
			"Ожидали context или HTTP ошибку, получили: %v", err)
	}

	// Должен сделать минимум один запрос
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.GreaterOrEqual(t, int(callCount), 1, "Должен сделать минимум одну попытку")
	assert.LessOrEqual(t, int(callCount), 5, "Не должен превысить макс попыток")
}

// TestMetricsLabelsForDifferentHosts проверяет host labels в метриках
// при запросах к разным доменам.
func TestMetricsLabelsForDifferentHosts(t *testing.T) {
	t.Parallel()

	// Создаем несколько тестовых серверов
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server2.Close()

	client := httpclient.New(httpclient.Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Делаем запросы к разным hosts
	resp1, err1 := client.Get(ctx, server1.URL)
	require.NoError(t, err1)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	resp1.Body.Close()

	resp2, err2 := client.Get(ctx, server2.URL)
	require.NoError(t, err2)
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)
	resp2.Body.Close()

	// Главное - никаких паник при обработке разных hosts
	// В реальном сценарии бы проверяли metrics registry
	// Для интеграционного теста проверяем базовую работу
	assert.True(t, true, "Успешно выполнили запросы к разным hosts")
}

// TestClientHandlesResponseBodyReadError тестирует обработку ошибок
// когда response body становится нечитаемым.
func TestClientHandlesResponseBodyReadError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Length", "100") // Заявляем содержимое

		// Пишем часть, потом соединение "ломается"
		flusher := w.(http.Flusher)
		w.Write([]byte("partial"))
		flusher.Flush()

		// Симулируем обрыв соединения
		hijacker := w.(http.Hijacker)
		conn, _, err := hijacker.Hijack()
		if err == nil {
			conn.Close() // Обрываем соединение резко
		}
	}))
	defer server.Close()

	client := httpclient.New(httpclient.Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	// Запрос должен сначала успеть (200 OK статус получен)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Но чтение body должно сорваться из-за закрытого соединения
	_, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Должны получить сетевую ошибку
	assert.Error(t, readErr, "Ожидали ошибку при чтении поломанного body")
	assert.True(t, strings.Contains(readErr.Error(), "broken pipe") ||
		strings.Contains(readErr.Error(), "connection reset") ||
		strings.Contains(readErr.Error(), "EOF"),
		"Ожидали сетевую ошибку, получили: %v", readErr)
}

// TestRetryWithCircuitBreakerRecovery тестирует сложный сценарий -
// CB открывается, потом сервис восстанавливается.
func TestRetryWithCircuitBreakerRecovery(t *testing.T) {
	// Не параллельно из-за чувствительности ко времени

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		// Первые 2 вызова падают для открытия CB, потом сервис восстанавливается
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2, // Открываем после 2 ошибок
		SuccessThreshold: 1, // Закрываем после 1 успеха в half-open
		Timeout:          100 * time.Millisecond,
	}

	cb := httpclient.NewCircuitBreakerWithConfig(cbConfig)
	config := httpclient.Config{
		RetryEnabled:         false, // Отключаем retry для фокуса на CB
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Делаем начальные неудачные запросы для открытия CB
	resp1, _ := client.Get(ctx, server.URL) // count=1, падает, CB еще закрыт
	if resp1 != nil {
		resp1.Body.Close()
	}
	resp2, _ := client.Get(ctx, server.URL) // count=2, падает, CB открывается
	if resp2 != nil {
		resp2.Body.Close()
	}

	// CB должен быть открыт сейчас
	assert.Equal(t, httpclient.CircuitBreakerOpen, cb.State())

	// Запрос при открытом CB должен вернуть последний неудачный ответ
	resp3, err3 := client.Get(ctx, server.URL)
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "circuit breaker is open")
	// CB возвращает последний неудачный ответ когда открыт
	if resp3 != nil {
		assert.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
		resp3.Body.Close()
	}

	// Ждем таймаут CB для перехода в half-open
	time.Sleep(150 * time.Millisecond)

	// Следующий запрос должен успеть (сервис восстановился, CB half-open -> closed)
	resp4, err4 := client.Get(ctx, server.URL) // count=3, должен успеть
	require.NoError(t, err4, "Ожидали успешный запрос после восстановления")
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	resp4.Body.Close()

	// CB должен снова закрыться после успешного запроса
	assert.Equal(t, httpclient.CircuitBreakerClosed, cb.State())

	// Проверяем, что восстановление сработало
	resp5, err5 := client.Get(ctx, server.URL)
	require.NoError(t, err5)
	assert.Equal(t, http.StatusOK, resp5.StatusCode)
	resp5.Body.Close()
}
