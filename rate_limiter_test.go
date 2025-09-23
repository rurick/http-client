package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC001: Создание рейтлимитера с валидными параметрами
func TestNewTokenBucketLimiter_ValidParams(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 5)

	assert.NotNil(t, limiter)
	assert.Equal(t, 10.0, limiter.rate)
	assert.Equal(t, 5, limiter.capacity)
	assert.Equal(t, 5.0, limiter.tokens) // начинаем с полной корзины
}

// TC001: Создание рейтлимитера с невалидными параметрами
func TestNewTokenBucketLimiter_InvalidParams(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		capacity int
	}{
		{"zero rate", 0.0, 5},
		{"negative rate", -1.0, 5},
		{"zero capacity", 10.0, 0},
		{"negative capacity", 10.0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				NewTokenBucketLimiter(tt.rate, tt.capacity)
			})
		})
	}
}

// TC002: Токены пополняются с заданной скоростью
func TestTokenBucketLimiter_Refill(t *testing.T) {
	limiter := NewTokenBucketLimiter(2.0, 5) // 2 токена в секунду

	// Потребляем все токены
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow())
	}
	assert.False(t, limiter.Allow()) // корзина пуста

	// Ждем половину секунды - должен появиться 1 токен
	time.Sleep(500 * time.Millisecond)
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow())
}

// TC003: Allow() возвращает true при наличии токенов
func TestTokenBucketLimiter_Allow_WithTokens(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 3)

	// Должны быть доступны 3 токена
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
}

// TC004: Allow() возвращает false при отсутствии токенов
func TestTokenBucketLimiter_Allow_NoTokens(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 2)

	// Потребляем все токены
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())

	// Следующий вызов должен вернуть false
	assert.False(t, limiter.Allow())
}

// TC005: Wait() ждет появления токена
func TestTokenBucketLimiter_Wait_Success(t *testing.T) {
	limiter := NewTokenBucketLimiter(4.0, 1) // 4 токена в секунду

	// Потребляем единственный токен
	assert.True(t, limiter.Allow())

	ctx := context.Background()
	start := time.Now()

	// Wait должен подождать ~250ms до появления следующего токена
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	assert.Less(t, elapsed, 500*time.Millisecond)
}

// TC006: Wait() отменяется по context timeout
func TestTokenBucketLimiter_Wait_ContextTimeout(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 1) // 1 токен в секунду

	// Потребляем единственный токен
	assert.True(t, limiter.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.GreaterOrEqual(t, elapsed, 90*time.Millisecond)
	assert.Less(t, elapsed, 150*time.Millisecond)
}

// TC007: Rate limiter по умолчанию выключен
func TestConfig_RateLimiterDisabledByDefault(t *testing.T) {
	config := Config{}
	config = config.withDefaults()

	assert.False(t, config.RateLimiterEnabled)
}

// TC008: Включение rate limiter через конфигурацию
func TestConfig_RateLimiterEnabled(t *testing.T) {
	config := Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 5.0,
			BurstCapacity:     10,
		},
	}
	config = config.withDefaults()

	assert.True(t, config.RateLimiterEnabled)
	assert.Equal(t, 5.0, config.RateLimiterConfig.RequestsPerSecond)
	assert.Equal(t, 10, config.RateLimiterConfig.BurstCapacity)
}

// TC009: Валидация параметров конфигурации (значения по умолчанию)
func TestRateLimiterConfig_WithDefaults(t *testing.T) {
	config := RateLimiterConfig{}
	config = config.withDefaults()

	assert.Equal(t, 10.0, config.RequestsPerSecond)
	assert.Equal(t, 10, config.BurstCapacity) // равен rate
}

// TC010: Использование пользовательских значений.
func TestRateLimiterConfig_UserValues(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerSecond: 20.0,
		BurstCapacity:     5,
	}
	config = config.withDefaults()

	assert.Equal(t, 20.0, config.RequestsPerSecond)
	assert.Equal(t, 5, config.BurstCapacity)
}

// TC011: Запрос проходит при наличии токенов
func TestRateLimiterRoundTripper_RequestPasses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 10.0,
		BurstCapacity:     5,
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TC012: Запрос ждет при отсутствии токенов (wait strategy)
func TestRateLimiterRoundTripper_WaitStrategy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 2.0, // 2 запроса в секунду
		BurstCapacity:     1,   // только 1 токен в корзине
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	req1, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	req2, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	// Первый запрос должен пройти сразу
	start := time.Now()
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	resp1.Body.Close()
	elapsed1 := time.Since(start)

	// Второй запрос должен ждать
	start = time.Now()
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	resp2.Body.Close()
	elapsed2 := time.Since(start)

	assert.Less(t, elapsed1, 100*time.Millisecond)           // первый быстро
	assert.GreaterOrEqual(t, elapsed2, 400*time.Millisecond) // второй ждет
}

// TC013: Контекстная отмена во время ожидания
func TestRateLimiterRoundTripper_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 1.0, // очень медленно
		BurstCapacity:     1,
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	// Потребляем единственный токен
	req1, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	resp1.Body.Close()

	// Второй запрос с коротким таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req2, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	_, err = client.Do(req2)
	assert.Error(t, err)
	// Проверяем, что ошибка связана с контекстом
	if !assert.Contains(t, err.Error(), "context deadline exceeded") {
		// Если не содержит эту строку, проверяем другие варианты
		assert.Contains(t, err.Error(), "failed to acquire token")
	}
}

// TC014: Rate limiter не влияет на запросы когда отключен
func TestClient_RateLimiterDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RateLimiterEnabled: false, // выключен
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 0.1, // очень медленно, но не должно влиять
			BurstCapacity:     1,
		},
	}

	client := New(config, "test")
	defer client.Close()

	// Делаем много быстрых запросов
	for i := 0; i < 5; i++ {
		start := time.Now()
		resp, err := client.Get(context.Background(), server.URL)
		elapsed := time.Since(start)

		require.NoError(t, err)
		resp.Body.Close()

		// Все запросы должны быть быстрыми (rate limiter отключен)
		assert.Less(t, elapsed, 100*time.Millisecond)
	}
}

// TC015: Глобальный rate limiter ограничивает все запросы.
func TestRateLimiterRoundTripper_GlobalLimiter(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 2.0,
		BurstCapacity:     2,
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	// Используем оба токена на разных серверах.
	req1, _ := http.NewRequest("GET", server1.URL, nil)
	req2, _ := http.NewRequest("GET", server2.URL, nil)

	resp1, err1 := client.Do(req1)
	resp2, err2 := client.Do(req2)

	require.NoError(t, err1)
	require.NoError(t, err2)
	resp1.Body.Close()
	resp2.Body.Close()

	// Третий запрос должен ждать (глобальный лимит исчерпан).
	req3, _ := http.NewRequest("GET", server1.URL, nil)
	start := time.Now()
	resp3, err3 := client.Do(req3)
	elapsed := time.Since(start)

	require.NoError(t, err3)
	resp3.Body.Close()

	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond) // должен ждать
}

// TC018: Burst capacity позволяет превысить rate на короткое время
func TestTokenBucketLimiter_BurstCapacity(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 5) // 1 токен/сек, но корзина на 5

	// Должны сразу получить 5 токенов (burst)
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(), "token %d should be available", i+1)
	}

	// 6-й токен недоступен
	assert.False(t, limiter.Allow())
}

// TC019: Конкурентный доступ к рейтлимитеру (race conditions)
func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(100.0, 50)

	var wg sync.WaitGroup

	// Запускаем 100 горутин, каждая пытается получить токен
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Allow() // Просто вызываем, результат не важен
		}()
	}

	wg.Wait()
	// Главное - не должно быть race condition'ов и panic'ов
}

// TC020: Большие временные интервалы не переполняют capacity
func TestTokenBucketLimiter_CapacityLimit(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 3) // 1 токен/сек, макс 3

	// Потребляем все токены
	limiter.Allow()
	limiter.Allow()
	limiter.Allow()
	assert.False(t, limiter.Allow())

	// Ждем долго (больше чем нужно для заполнения корзины)
	time.Sleep(5 * time.Second)

	// Должно быть доступно максимум 3 токена, не больше
	count := 0
	for limiter.Allow() {
		count++
		if count > 5 { // защита от бесконечного цикла
			break
		}
	}

	assert.Equal(t, 3, count, "should have exactly 3 tokens after long wait")
}
