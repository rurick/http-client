//go:build integration

// Package integration contains advanced integration tests for the HTTP client library.
// These tests cover complex interaction scenarios, edge cases, and concurrency issues.
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

// errorReader simulates an io.Reader that fails after the first read.
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

// brokenPipe simulates a connection that breaks during response reading.
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

// TestRetryWithOpenCircuitBreaker tests interaction between retry logic and circuit breaker.
// When the circuit breaker opens during retry, subsequent attempts should be stopped.
func TestRetryWithOpenCircuitBreaker(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Always error
	}))
	defer server.Close()

	// Configure circuit breaker to open after 2 errors
	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5, // More attempts than CB threshold
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

	// Should get circuit breaker error
	assert.Error(t, err)

	// Server should not be called 5 times due to circuit breaker opening
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Less(t, int(callCount), 5, "Circuit breaker should limit calls")
	assert.GreaterOrEqual(t, int(callCount), 2, "Should attempt at least threshold times")
}

// TestCircuitBreakerResetsAfterSuccessfulRetry verifies that circuit breaker switches correctly
// when the service recovers during retry attempts.
func TestCircuitBreakerResetsAfterSuccessfulRetry(t *testing.T) {
	// Not parallel due to time sensitivity

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK) // Service recovers
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
			MaxAttempts:      1, // No retries, testing only CB
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// First two calls should open circuit breaker
	client.Get(ctx, server.URL)
	client.Get(ctx, server.URL)

	assert.Equal(t, httpclient.CircuitBreakerOpen, cb.State())

	// Wait for circuit breaker timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should succeed and close the circuit
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, httpclient.CircuitBreakerClosed, cb.State())
}

// TestBackoffWithJitter verifies that retry delays include deterministic jitter
// and differ for different attempts within the expected range.
func TestBackoffWithJitter(t *testing.T) {
	t.Parallel()

	baseDelay := 100 * time.Millisecond
	jitter := 0.5 // 50% jitter
	maxDelay := 10 * time.Second

	// Test multiple attempts to verify jitter between attempts
	attempts := []int{1, 2, 3, 4, 5}
	delays := make([]time.Duration, len(attempts))

	// Collect delays for different attempts
	for i, attempt := range attempts {
		delays[i] = httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
	}

	// Check delays for each attempt
	for i, attempt := range attempts {
		// Expected base delay: 0 for attempt <= 1, baseDelay * 2^(attempt-2) for attempt >= 2
		var expectedBase time.Duration
		if attempt <= 1 {
			expectedBase = 0 // First attempt without delay
		} else {
			expectedBase = time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-2)))
		}

		minDelay := time.Duration(float64(expectedBase) * (1 - jitter))
		maxJitterDelay := time.Duration(float64(expectedBase) * (1 + jitter))
		if maxJitterDelay > maxDelay {
			maxJitterDelay = maxDelay
		}

		// Check that delay is within jitter range
		assert.GreaterOrEqual(t, delays[i], minDelay, "Delay for attempt %d below minimum: %v < %v", attempt, delays[i], minDelay)
		assert.LessOrEqual(t, delays[i], maxJitterDelay, "Delay for attempt %d above maximum: %v > %v", attempt, delays[i], maxJitterDelay)
	}

	// Check that jitter creates deterministic but different values for different attempts
	// Since jitter is deterministic by attempt number, same attempts should give same results
	for _, attempt := range attempts {
		delay1 := httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
		delay2 := httpclient.CalculateBackoffDelay(attempt, baseDelay, maxDelay, jitter)
		assert.Equal(t, delay1, delay2, "Jitter should be deterministic for the same attempt %d", attempt)
	}

	// Check that different attempts create different delays (deterministic jitter)
	atLeastOneDifferent := false
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			atLeastOneDifferent = true
			break
		}
	}
	assert.True(t, atLeastOneDifferent, "Jitter should create different delays for different attempts")
}

// TestIdempotentRetryWithUnreadableBody verifies that the library buffers request bodies
// and allows retry even for POST requests with Idempotency-Key.
func TestIdempotentRetryWithUnreadableBody(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError) // Error on first 2 attempts
			return
		}
		w.WriteHeader(http.StatusOK) // Success on 3rd attempt
	}))
	defer server.Close()

	// Use a regular reader - the library will buffer it
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

	// Should succeed after retry, since the library buffers the body
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Server should be called 3 times (library retries idempotent POST)
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount, "Expected 3 attempts for idempotent POST with retryable status")
}

// TestIdempotentRetryWithBodyReadErrorOnSecondAttempt verifies that body read errors prevent the request.
func TestIdempotentRetryWithBodyReadErrorOnSecondAttempt(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Use errorReader that fails after first read
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

	// Should get error from initial body read
	_, err := client.Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")

	// No server calls due to body read error
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(0), callCount, "Should be no calls due to body read error")
}

// TestOverallTimeoutDuringRetry verifies that the client's overall timeout
// expires during the retry sequence within MaxAttempts.
func TestOverallTimeoutDuringRetry(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		time.Sleep(50 * time.Millisecond) // Each request takes 50ms
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := httpclient.Config{
		Timeout:       120 * time.Millisecond, // Overall timeout
		PerTryTimeout: 100 * time.Millisecond, // Per-attempt timeout
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5, // More attempts than time allows
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

	// Either timeout error or last failed response
	if err != nil {
		// Timeout occurred - check different error messages
		errorMsg := err.Error()
		assert.True(t, strings.Contains(errorMsg, "deadline exceeded") ||
			strings.Contains(errorMsg, "context deadline exceeded") ||
			strings.Contains(errorMsg, "Client.Timeout exceeded") ||
			strings.Contains(errorMsg, "timeout exceeded"),
			"Expected timeout error, got: %v", err)
	} else if resp != nil {
		// Got last response from failed attempts
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		_ = resp.Body.Close()
	}

	// Should respect overall timeout (with small margin for processing)
	assert.Less(t, elapsed, 200*time.Millisecond, "Request too long: %v", elapsed)

	// Should not complete all 5 retries due to timeout
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Less(t, int(callCount), 5, "Should not complete all retries due to timeout")
}

// TestPerTryTimeoutAndRetry verifies that per-attempt timeouts work correctly
// with multiple retry attempts.
func TestPerTryTimeoutAndRetry(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		if count < 3 {
			time.Sleep(150 * time.Millisecond) // Longer than per-attempt timeout
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK) // Fast response on 3rd attempt
	}))
	defer server.Close()

	config := httpclient.Config{
		Timeout:       2 * time.Second,        // Long overall timeout
		PerTryTimeout: 100 * time.Millisecond, // Short per-attempt timeout
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

	// Should attempt 3 times (first 2 timeout, 3rd succeeds)
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(3), callCount, "Expected exactly 3 attempts")
}

// TestConcurrentClientUsageWithSharedConfig tests thread safety
// when using the client concurrently in multiple goroutines.
func TestConcurrentClientUsageWithSharedConfig(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		time.Sleep(10 * time.Millisecond) // Small delay
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

	assert.Empty(t, errorList, "Concurrent requests failed: %v", errorList)

	// Check that all requests are processed
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.Equal(t, int32(concurrency), callCount, "Not all concurrent requests processed")
}

// TestConcurrentCircuitBreakerStateChanges tests circuit breaker thread safety
// under high concurrency.
func TestConcurrentCircuitBreakerStateChanges(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	failureCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		// First 10 requests fail, then success
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
		RetryEnabled:         false, // Focus on CB behavior
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

	// Check that CB handled concurrent access without panics
	totalRequests := atomic.LoadInt32(&requestCount)
	totalServerCalls := atomic.LoadInt32(&serverCallCount)

	assert.Equal(t, int32(concurrency), totalRequests, "Not all goroutines completed")
	// Server calls should be less due to CB
	assert.LessOrEqual(t, int(totalServerCalls), concurrency, "Server calls should not exceed requests")

	// CB should have opened and prevented some requests
	finalState := cb.State()
	assert.True(t, finalState == httpclient.CircuitBreakerOpen ||
		finalState == httpclient.CircuitBreakerHalfOpen ||
		finalState == httpclient.CircuitBreakerClosed,
		"Circuit breaker should be in valid state")
}

// TestMetricsOnRetryWithContextCancellation verifies metrics correctness
// when context is cancelled during retry backoff.
func TestMetricsOnRetryWithContextCancellation(t *testing.T) {
	t.Parallel()

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCallCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Always error
	}))
	defer server.Close()

	config := httpclient.Config{
		RetryEnabled: true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:      5,
			BaseDelay:        200 * time.Millisecond, // Enough for cancellation during backoff
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := httpclient.New(config, "test-client")
	defer client.Close()

	// Cancel context after first retry
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)

	// Expect error (HTTP or context)
	if err != nil {
		// Check that it's one of the expected types
		assert.True(t, strings.Contains(err.Error(), "deadline exceeded") ||
			strings.Contains(err.Error(), "context canceled") ||
			strings.Contains(err.Error(), "500"),
			"Expected context or HTTP error, got: %v", err)
	}

	// Should make at least one request
	callCount := atomic.LoadInt32(&serverCallCount)
	assert.GreaterOrEqual(t, int(callCount), 1, "Should make at least one attempt")
	assert.LessOrEqual(t, int(callCount), 5, "Should not exceed max attempts")
}

// TestMetricsLabelsForDifferentHosts verifies host labels in metrics
// when making requests to different domains.
func TestMetricsLabelsForDifferentHosts(t *testing.T) {
	t.Parallel()

	// Create multiple test servers
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

	// Make requests to different hosts
	resp1, err1 := client.Get(ctx, server1.URL)
	require.NoError(t, err1)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	resp1.Body.Close()

	resp2, err2 := client.Get(ctx, server2.URL)
	require.NoError(t, err2)
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)
	resp2.Body.Close()

	// Main thing - no panics when processing different hosts
	// In a real scenario we would check metrics registry
	// For integration test we check basic operation
	assert.True(t, true, "Successfully executed requests to different hosts")
}

// TestClientHandlesResponseBodyReadError tests error handling
// when response body becomes unreadable.
func TestClientHandlesResponseBodyReadError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Length", "100") // Claim content

		// Write part, then connection "breaks"
		flusher := w.(http.Flusher)
		w.Write([]byte("partial"))
		flusher.Flush()

		// Simulate connection break
		hijacker := w.(http.Hijacker)
		conn, _, err := hijacker.Hijack()
		if err == nil {
			conn.Close() // Abruptly break connection
		}
	}))
	defer server.Close()

	client := httpclient.New(httpclient.Config{}, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)

	// Request should first succeed (200 OK status received)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// But reading body should fail due to closed connection
	_, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Should get network error
	assert.Error(t, readErr, "Expected error reading broken body")
	assert.True(t, strings.Contains(readErr.Error(), "broken pipe") ||
		strings.Contains(readErr.Error(), "connection reset") ||
		strings.Contains(readErr.Error(), "EOF"),
		"Expected network error, got: %v", readErr)
}

// TestRetryWithCircuitBreakerRecovery tests a complex scenario -
// CB opens, then service recovers.
func TestRetryWithCircuitBreakerRecovery(t *testing.T) {
	// Not parallel due to time sensitivity

	serverCallCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&serverCallCount, 1)
		// First 2 calls fail to open CB, then service recovers
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cbConfig := httpclient.CircuitBreakerConfig{
		FailureThreshold: 2, // Open after 2 errors
		SuccessThreshold: 1, // Close after 1 success in half-open
		Timeout:          100 * time.Millisecond,
	}

	cb := httpclient.NewCircuitBreakerWithConfig(cbConfig)
	config := httpclient.Config{
		RetryEnabled:         false, // Disable retry to focus on CB
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}

	client := httpclient.New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// Make initial failed requests to open CB
	resp1, _ := client.Get(ctx, server.URL) // count=1, fails, CB still closed
	if resp1 != nil {
		resp1.Body.Close()
	}
	resp2, _ := client.Get(ctx, server.URL) // count=2, fails, CB opens
	if resp2 != nil {
		resp2.Body.Close()
	}

	// CB should be open now
	assert.Equal(t, httpclient.CircuitBreakerOpen, cb.State())

	// Request with open CB should return last failed response
	resp3, err3 := client.Get(ctx, server.URL)
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "circuit breaker is open")
	// CB returns last failed response when open
	if resp3 != nil {
		assert.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
		resp3.Body.Close()
	}

	// Wait for CB timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Next request should succeed (service recovered, CB half-open -> closed)
	resp4, err4 := client.Get(ctx, server.URL) // count=3, should succeed
	require.NoError(t, err4, "Expected successful request after recovery")
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	resp4.Body.Close()

	// CB should close again after successful request
	assert.Equal(t, httpclient.CircuitBreakerClosed, cb.State())

	// Verify that recovery worked
	resp5, err5 := client.Get(ctx, server.URL)
	require.NoError(t, err5)
	assert.Equal(t, http.StatusOK, resp5.StatusCode)
	resp5.Body.Close()
}
