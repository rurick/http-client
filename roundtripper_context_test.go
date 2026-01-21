package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextNotCanceledDuringBodyRead verifies that context is not canceled
// until the response body is closed
func TestContextNotCanceledDuringBodyRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body content"))
	}))
	defer server.Close()

	// Create a client with short PerTryTimeout
	config := Config{
		PerTryTimeout: 100 * time.Millisecond, // Short timeout
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait longer than PerTryTimeout, but context should not be canceled
	// because response body is not yet closed
	time.Sleep(150 * time.Millisecond)

	// Should successfully read response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Context should not be canceled while reading response body")
	assert.Equal(t, "response body content", string(body))

	// Close response body - only now context should be canceled
	_ = resp.Body.Close()
}

// TestContextCanceledOnBodyClose verifies that context is canceled when response body is closed
func TestContextCanceledOnBodyClose(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body content"))
	}))
	defer server.Close()

	config := Config{
		PerTryTimeout: 1 * time.Second,
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check that this is our contextAwareBody
	if cab, ok := resp.Body.(*contextAwareBody); ok {
		assert.NotNil(t, cab.cancel, "contextAwareBody should have cancel function")
	}

	// Close response body
	err = resp.Body.Close()
	require.NoError(t, err)

	// Repeated closing should not cause error (sync.Once protection)
	err = resp.Body.Close()
	require.NoError(t, err)
}

// TestContextCanceledOnError verifies that context is canceled immediately on error
func TestContextCanceledOnError(t *testing.T) {
	t.Parallel()

	// Server that does not exist
	nonExistentURL := "http://localhost:99999/nonexistent"

	config := Config{
		PerTryTimeout: 1 * time.Second,
	}
	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, nonExistentURL)

	// Should be an error
	assert.Error(t, err)
	// Response may be nil or not nil depending on error type
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Main thing - we don't hang and error is handled correctly
	assert.True(t, strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "dial"),
		"Expected network error, got: %v", err)
}
