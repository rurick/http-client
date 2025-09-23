package httpclient

import (
	"context"
	"sync"
	"time"
)

// RateLimiter определяет интерфейс для ограничения частоты запросов.
type RateLimiter interface {
	// Allow проверяет, можно ли выполнить запрос немедленно
	Allow() bool

	// Wait блокирует выполнение до получения разрешения на запрос
	Wait(ctx context.Context) error
}

// TokenBucketLimiter реализует алгоритм token bucket для ограничения частоты запросов.
type TokenBucketLimiter struct {
	rate     float64    // токенов в секунду
	capacity int        // максимальная емкость корзины
	tokens   float64    // текущее количество токенов
	lastTime time.Time  // время последнего обновления
	mu       sync.Mutex // защита от конкурентного доступа
}

// NewTokenBucketLimiter создает новый rate limiter с указанными параметрами.
func NewTokenBucketLimiter(rate float64, capacity int) *TokenBucketLimiter {
	if rate <= 0 {
		panic("rate must be positive")
	}
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	return &TokenBucketLimiter{
		rate:     rate,
		capacity: capacity,
		tokens:   float64(capacity), // начинаем с полной корзины
		lastTime: time.Now(),
	}
}

// Allow проверяет доступность токена без блокировки.
func (tb *TokenBucketLimiter) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// Wait ожидает доступности токена с учетом контекста.
func (tb *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		// Проверяем доступность токена
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= 1.0 {
			tb.tokens -= 1.0
			tb.mu.Unlock()
			return nil
		}

		// Вычисляем время ожидания для получения следующего токена
		deficit := 1.0 - tb.tokens
		waitTime := time.Duration(deficit/tb.rate) * time.Second
		tb.mu.Unlock()

		// Ждем либо до появления токена, либо до отмены контекста
		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
			timer.Stop()
			// Повторяем проверку (возвращаемся к началу цикла)
			continue
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

// refill пополняет корзину токенами на основе прошедшего времени.
// должен вызываться под блокировкой мьютекса.
func (tb *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	// Добавляем токены пропорционально прошедшему времени
	tokensToAdd := elapsed * tb.rate
	tb.tokens = minFloat64(tb.tokens+tokensToAdd, float64(tb.capacity))
}

// minFloat64 возвращает минимальное из двух значений float64.
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
