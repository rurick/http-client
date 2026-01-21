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

// TC001: Creating rate limiter with valid parameters
func TestNewTokenBucketLimiter_ValidParams(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 5)

	assert.NotNil(t, limiter)
	assert.Equal(t, 10.0, limiter.rate)
	assert.Equal(t, 5, limiter.capacity)
	assert.Equal(t, 5.0, limiter.tokens) // start with full bucket
}

// TC001: Creating rate limiter with invalid parameters
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

// TC002: Tokens are refilled at the specified rate
func TestTokenBucketLimiter_Refill(t *testing.T) {
	limiter := NewTokenBucketLimiter(2.0, 5) // 2 tokens per second

	// Consume all tokens
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow())
	}
	assert.False(t, limiter.Allow()) // bucket is empty

	// Wait half a second - 1 token should appear
	time.Sleep(500 * time.Millisecond)
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow())
}

// TC003: Allow() returns true when tokens are available
func TestTokenBucketLimiter_Allow_WithTokens(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 3)

	// Should have 3 tokens available
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
}

// TC004: Allow() returns false when no tokens are available
func TestTokenBucketLimiter_Allow_NoTokens(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 2)

	// Consume all tokens
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())

	// Next call should return false
	assert.False(t, limiter.Allow())
}

// TC005: Wait() waits for token to appear
func TestTokenBucketLimiter_Wait_Success(t *testing.T) {
	limiter := NewTokenBucketLimiter(4.0, 1) // 4 tokens per second

	// Consume the only token
	assert.True(t, limiter.Allow())

	ctx := context.Background()
	start := time.Now()

	// Wait should wait ~250ms until next token appears
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
	assert.Less(t, elapsed, 500*time.Millisecond)
}

// TC006: Wait() is canceled on context timeout
func TestTokenBucketLimiter_Wait_ContextTimeout(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 1) // 1 token per second

	// Consume the only token
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

// TC007: Rate limiter is disabled by default
func TestConfig_RateLimiterDisabledByDefault(t *testing.T) {
	config := Config{}
	config = config.withDefaults()

	assert.False(t, config.RateLimiterEnabled)
}

// TC008: Enabling rate limiter through configuration
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

// TC009: Configuration parameter validation (default values)
func TestRateLimiterConfig_WithDefaults(t *testing.T) {
	config := RateLimiterConfig{}
	config = config.withDefaults()

	assert.Equal(t, 10.0, config.RequestsPerSecond)
	assert.Equal(t, 10, config.BurstCapacity) // equals rate
}

// TC010: Using custom values.
func TestRateLimiterConfig_UserValues(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerSecond: 20.0,
		BurstCapacity:     5,
	}
	config = config.withDefaults()

	assert.Equal(t, 20.0, config.RequestsPerSecond)
	assert.Equal(t, 5, config.BurstCapacity)
}

// TC011: Request passes when tokens are available
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

// TC012: Request waits when no tokens are available (wait strategy)
func TestRateLimiterRoundTripper_WaitStrategy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 2.0, // 2 requests per second
		BurstCapacity:     1,   // only 1 token in bucket
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	req1, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	req2, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	// First request should pass immediately
	start := time.Now()
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	resp1.Body.Close()
	elapsed1 := time.Since(start)

	// Second request should wait
	start = time.Now()
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	resp2.Body.Close()
	elapsed2 := time.Since(start)

	assert.Less(t, elapsed1, 100*time.Millisecond)           // first is fast
	assert.GreaterOrEqual(t, elapsed2, 400*time.Millisecond) // second waits
}

// TC013: Context cancellation during wait
func TestRateLimiterRoundTripper_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RateLimiterConfig{
		RequestsPerSecond: 1.0, // very slow
		BurstCapacity:     1,
	}

	transport := NewRateLimiterRoundTripper(http.DefaultTransport, config)
	client := &http.Client{Transport: transport}

	// Consume the only token
	req1, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	resp1.Body.Close()

	// Second request with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req2, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	_, err = client.Do(req2)
	assert.Error(t, err)
	// Check that error is related to context
	if !assert.Contains(t, err.Error(), "context deadline exceeded") {
		// If it doesn't contain this string, check other variants
		assert.Contains(t, err.Error(), "failed to acquire token")
	}
}

// TC014: Rate limiter does not affect requests when disabled
func TestClient_RateLimiterDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RateLimiterEnabled: false, // disabled
		RateLimiterConfig: RateLimiterConfig{
			RequestsPerSecond: 0.1, // very slow, but should not affect
			BurstCapacity:     1,
		},
	}

	client := New(config, "test")
	defer client.Close()

	// Make many fast requests
	for i := 0; i < 5; i++ {
		start := time.Now()
		resp, err := client.Get(context.Background(), server.URL)
		elapsed := time.Since(start)

		require.NoError(t, err)
		resp.Body.Close()

		// All requests should be fast (rate limiter disabled)
		assert.Less(t, elapsed, 100*time.Millisecond)
	}
}

// TC015: Global rate limiter limits all requests.
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

	// Use both tokens on different servers.
	req1, _ := http.NewRequest("GET", server1.URL, nil)
	req2, _ := http.NewRequest("GET", server2.URL, nil)

	resp1, err1 := client.Do(req1)
	resp2, err2 := client.Do(req2)

	require.NoError(t, err1)
	require.NoError(t, err2)
	resp1.Body.Close()
	resp2.Body.Close()

	// Third request should wait (global limit exhausted).
	req3, _ := http.NewRequest("GET", server1.URL, nil)
	start := time.Now()
	resp3, err3 := client.Do(req3)
	elapsed := time.Since(start)

	require.NoError(t, err3)
	resp3.Body.Close()

	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond) // should wait
}

// TC018: Burst capacity allows exceeding rate for a short time
func TestTokenBucketLimiter_BurstCapacity(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 5) // 1 token/sec, but bucket of 5

	// Should immediately get 5 tokens (burst)
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(), "token %d should be available", i+1)
	}

	// 6th token unavailable
	assert.False(t, limiter.Allow())
}

// TC019: Concurrent access to rate limiter (race conditions)
func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(100.0, 50)

	var wg sync.WaitGroup

	// Start 100 goroutines, each trying to get a token
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Allow() // Just call, result doesn't matter
		}()
	}

	wg.Wait()
	// Main thing - should be no race conditions and panics
}

// TC020: Large time intervals do not overflow capacity
func TestTokenBucketLimiter_CapacityLimit(t *testing.T) {
	limiter := NewTokenBucketLimiter(1.0, 3) // 1 token/sec, max 3

	// Consume all tokens
	limiter.Allow()
	limiter.Allow()
	limiter.Allow()
	assert.False(t, limiter.Allow())

	// Wait long (longer than needed to fill bucket)
	time.Sleep(5 * time.Second)

	// Should have maximum 3 tokens available, no more
	count := 0
	for limiter.Allow() {
		count++
		if count > 5 { // protection against infinite loop
			break
		}
	}

	assert.Equal(t, 3, count, "should have exactly 3 tokens after long wait")
}
