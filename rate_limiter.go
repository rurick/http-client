package httpclient

import (
	"context"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting requests.
type RateLimiter interface {
	// Allow checks if a request can be executed immediately
	Allow() bool

	// Wait blocks execution until permission for a request is received
	Wait(ctx context.Context) error
}

// TokenBucketLimiter implements the token bucket algorithm for rate limiting requests.
type TokenBucketLimiter struct {
	rate     float64    // tokens per second
	capacity int        // maximum bucket capacity
	tokens   float64    // current number of tokens
	lastTime time.Time  // last update time
	mu       sync.Mutex // concurrent access protection
}

// NewTokenBucketLimiter creates a new rate limiter with the specified parameters.
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
		tokens:   float64(capacity), // start with full bucket
		lastTime: time.Now(),
	}
}

// Allow checks token availability without blocking.
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

// Wait waits for token availability with context consideration.
func (tb *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		// Check token availability
		tb.mu.Lock()
		tb.refill()
		if tb.tokens >= 1.0 {
			tb.tokens -= 1.0
			tb.mu.Unlock()
			return nil
		}

		// Calculate wait time to get next token
		deficit := 1.0 - tb.tokens
		waitTime := time.Duration(deficit/tb.rate) * time.Second
		tb.mu.Unlock()

		// Wait either until token appears or context is cancelled
		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
			timer.Stop()
			// Repeat check (return to start of loop)
			continue
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

// refill refills the bucket with tokens based on elapsed time.
// must be called under mutex lock.
func (tb *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	// Add tokens proportional to elapsed time
	tokensToAdd := elapsed * tb.rate
	tb.tokens = minFloat64(tb.tokens+tokensToAdd, float64(tb.capacity))
}

// minFloat64 returns the minimum of two float64 values.
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
