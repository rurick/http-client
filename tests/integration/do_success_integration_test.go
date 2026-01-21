//go:build integration

// Package integration: high-level integration tests for the HTTP client.
// This test checks a successful scenario: the server returns 200 OK,
// the (*Client).Do method returns the first response without retries and the body matches expectations.
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

func TestClientDo_SuccessNoRetry(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := httpclient.Config{
		RetryEnabled:  false,           // retries disabled
		Timeout:       2 * time.Second, // overall timeout
		PerTryTimeout: 2 * time.Second, // per-attempt timeout
	}
	client := httpclient.New(cfg, "test-success-no-retry")
	defer client.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		b, readErr := io.ReadAll(resp.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, "success", string(b))
	}

	// There should be exactly one server call
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
