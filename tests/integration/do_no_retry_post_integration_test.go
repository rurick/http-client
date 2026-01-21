//go:build integration

// Package integration: high-level integration tests for the HTTP client.
// This test verifies that even for an idempotent POST (with Idempotency-Key)
// when retries are disabled, the client does not perform retry attempts and returns
// the first server response.
package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	httpclient "github.com/rurick/http-client"
)

// TestClientDo_NoRetryOnIdempotentPOST verifies absence of retries on POST with Idempotency-Key,
// when RetryEnabled=false.
func TestClientDo_NoRetryOnIdempotentPOST(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 on first attempt
			_, _ = w.Write([]byte("fail-1"))             // expected error body
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer server.Close()

	cfg := httpclient.Config{
		RetryEnabled:  false,
		Timeout:       2 * time.Second,
		PerTryTimeout: 2 * time.Second,
	}
	client := httpclient.New(cfg, "test-no-retry-post")
	defer client.Close()

	ctx := context.Background()
	body := strings.NewReader(`{"data":"x"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, body)
	assert.NoError(t, err)
	req.Header.Set("Idempotency-Key", "op-123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		defer resp.Body.Close()
		// Should return the first status from the server without retries
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		b, readErr := io.ReadAll(resp.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, "fail-1", string(b), "response body should match expected")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "there should be exactly one request attempt")
}
