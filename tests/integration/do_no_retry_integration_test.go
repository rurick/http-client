//go:build integration

// Package integration contains high-level integration tests for the client.
// This test verifies that the (*Client).Do method does not perform retry attempts
// when retries are disabled in the configuration.
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
	httpclient "github.com/rurick/http-client"
)

// TestClientDo_NoRetryOn5xx verifies that with retries disabled, the client
// returns the first server response (including 5xx) without additional attempts.
func TestClientDo_NoRetryOn5xx(t *testing.T) {
	// Start a test server that returns 503 on the first request, then 200.
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 on first attempt
			_, _ = w.Write([]byte("fail-1"))             // expected error body
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Configuration without retries
	cfg := httpclient.Config{
		RetryEnabled:  false,
		Timeout:       2 * time.Second,
		PerTryTimeout: 2 * time.Second,
	}
	client := httpclient.New(cfg, "test-no-retry")
	defer client.Close()

	// Create a request and call Do directly
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)

	// Expect no transport-level error and 503 status from the first attempt
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "should return first status without retries")
		// Check response body
		b, readErr := io.ReadAll(resp.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, "fail-1", string(b), "response body should match expected")
	}

	// Verify that the server was called exactly once (no retry attempts)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "request should be executed exactly once")
}
