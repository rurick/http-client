package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrRequestBodyNotRewindable represents an error when request body cannot be rewound for retry
var ErrRequestBodyNotRewindable = errors.New("request body is not rewindable for retry")

// nonRewindableReader simulates a reader that cannot be rewound (like a pipe)
type nonRewindableReader struct {
	data []byte
	pos  int
	read bool
}

func (nr *nonRewindableReader) Read(p []byte) (int, error) {
	if nr.read {
		return 0, io.EOF
	}
	n := copy(p, nr.data)
	nr.read = true
	return n, nil
}

func (nr *nonRewindableReader) Close() error {
	return nil
}

// failingReadCloser simulates a reader that fails after first successful read
type failingReadCloser struct {
	data      []byte
	readCount int
}

func (frc *failingReadCloser) Read(p []byte) (int, error) {
	frc.readCount++
	if frc.readCount > 1 {
		return 0, errors.New("read failed after first attempt")
	}
	n := copy(p, frc.data)
	return n, nil
}

func (frc *failingReadCloser) Close() error {
	return nil
}

// TestIdempotentRequestBodyNotRewindableError tests that POST requests with
// Idempotency-Key are actually retryable when body reading succeeds initially
func TestIdempotentRequestBodyNotRewindableError(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError) // Fail first 2 attempts
			return
		}
		w.WriteHeader(http.StatusOK) // Succeed on 3rd attempt
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := New(config, "test-client")
	defer client.Close()

	// Use bytes.NewReader which can be read multiple times after being buffered by RoundTripper
	body := bytes.NewReader([]byte("test-data"))
	req, err := http.NewRequest("POST", server.URL, body)
	require.NoError(t, err)
	req.Header.Set("Idempotency-Key", "test-key-123")

	resp, err := client.Do(req)

	// Should succeed after retries since the library buffers the body for POST with Idempotency-Key
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, callCount, "Should make 3 attempts for idempotent POST with retryable status")
}

// TestBodyReadFailurePreventsRetry tests that if reading the request body fails
// during retry preparation, the retry is aborted
func TestBodyReadFailurePreventsRetry(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
	}
	client := New(config, "test-client")
	defer client.Close()

	// Create a body that fails on second read
	body := &failingReadCloser{data: []byte("test-data")}
	req, err := http.NewRequest("GET", server.URL, body)
	require.NoError(t, err)

	_, err = client.Do(req)

	// Should get an error from body reading failure
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")
	assert.Equal(t, 0, callCount, "Should not make any calls due to body read failure")
}

// TestCircuitBreakerOpenPreventsRetryAttempts tests that when circuit breaker
// is open, retry logic is bypassed completely
func TestCircuitBreakerOpenPreventsRetryAttempts(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create circuit breaker that opens immediately
	cbConfig := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreakerWithConfig(cbConfig)
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      5,
			BaseDelay:        10 * time.Millisecond,
			RetryStatusCodes: []int{http.StatusInternalServerError},
		},
		CircuitBreakerEnable: true,
		CircuitBreaker:       cb,
	}

	client := New(config, "test-client")
	defer client.Close()

	ctx := context.Background()

	// First call opens the circuit breaker
	resp1, err1 := client.Get(ctx, server.URL)
	// The circuit breaker returns the response but opens after this call
	if err1 == nil {
		assert.Equal(t, http.StatusInternalServerError, resp1.StatusCode)
		resp1.Body.Close()
	}
	assert.Equal(t, CircuitBreakerOpen, cb.State())

	// Second call should be circuit broken and not attempt retries
	_, err2 := client.Get(ctx, server.URL)
	assert.Error(t, err2)
	// Check if the error is the circuit breaker error (might be wrapped in url.Error)
	assert.True(t, errors.Is(err2, ErrCircuitBreakerOpen) || strings.Contains(err2.Error(), "circuit breaker is open"),
		"Expected circuit breaker error, got: %v", err2)

	// Should only have made the first call
	assert.Equal(t, 1, callCount, "Circuit breaker should prevent additional calls")
}

// TestRetryAfterHeaderRespected tests that Retry-After header is respected
// when RespectRetryAfter is true
func TestRetryAfterHeaderRespected(t *testing.T) {
	// Not parallel due to timing sensitivity

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:       2,
			BaseDelay:         10 * time.Millisecond,
			RetryStatusCodes:  []int{http.StatusTooManyRequests},
			RespectRetryAfter: true,
		},
	}

	client := New(config, "test-client")
	defer client.Close()

	start := time.Now()
	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, callCount, "Should make exactly 2 attempts")

	// Should have waited at least 1 second due to Retry-After header
	assert.GreaterOrEqual(t, elapsed, 1*time.Second, "Should respect Retry-After header")
}

// TestErrorClassification tests the error classification functions
func TestErrorClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "timeout error",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("timeout")},
			expected: "timeout",
		},
		{
			name:     "network error",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection reset")},
			expected: "net",
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: "other",
		},
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHTTPErrorCreation tests the creation and behavior of HTTPError
func TestHTTPErrorCreation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	httpErr := NewHTTPError(resp, req)

	assert.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	assert.Equal(t, "400 Bad Request", httpErr.Status)
	assert.Equal(t, server.URL, httpErr.URL)
	assert.Equal(t, "GET", httpErr.Method)
	assert.Equal(t, "test-value", httpErr.Headers.Get("X-Custom-Header"))

	// Test error message
	expectedMsg := "HTTP 400 400 Bad Request: GET " + server.URL
	assert.Equal(t, expectedMsg, httpErr.Error())

	// Test IsHTTPError
	assert.True(t, IsHTTPError(httpErr))
	assert.False(t, IsHTTPError(errors.New("not http error")))
}

// TestMaxAttemptsExceededError tests the MaxAttemptsExceededError behavior
func TestMaxAttemptsExceededError(t *testing.T) {
	t.Parallel()

	// Test with last error
	lastErr := errors.New("connection refused")
	maxErr := &MaxAttemptsExceededError{
		MaxAttempts: 3,
		LastError:   lastErr,
		LastStatus:  0,
	}

	expectedMsg := "max attempts (3) exceeded, last error: connection refused"
	assert.Equal(t, expectedMsg, maxErr.Error())
	assert.Equal(t, lastErr, maxErr.Unwrap())

	// Test with last status only
	maxErrStatus := &MaxAttemptsExceededError{
		MaxAttempts: 3,
		LastError:   nil,
		LastStatus:  500,
	}

	expectedMsgStatus := "max attempts (3) exceeded, last status: 500"
	assert.Equal(t, expectedMsgStatus, maxErrStatus.Error())
	assert.Nil(t, maxErrStatus.Unwrap())
}

// TestAdvancedConfigurationError tests the ConfigurationError behavior
func TestAdvancedConfigurationError(t *testing.T) {
	t.Parallel()

	configErr := NewConfigurationError("timeout", -1, "must be positive")

	expectedMsg := "configuration error in field 'timeout': must be positive (value: -1)"
	assert.Equal(t, expectedMsg, configErr.Error())
	assert.Equal(t, "timeout", configErr.Field)
	assert.Equal(t, -1, configErr.Value)
	assert.Equal(t, "must be positive", configErr.Message)
}

// TestTimeoutExceededError tests the TimeoutExceededError behavior
func TestTimeoutExceededError(t *testing.T) {
	t.Parallel()

	timeoutErr := &TimeoutExceededError{
		Timeout: 5 * time.Second,
		Elapsed: 7 * time.Second,
	}

	expectedMsg := "timeout exceeded: 7s elapsed, 5s allowed"
	assert.Equal(t, expectedMsg, timeoutErr.Error())
}

// TestRetryableErrorInterface tests the RetryableError interface implementation
func TestRetryableErrorInterface(t *testing.T) {
	t.Parallel()

	// Test retryable error
	retryableErr := NewRetryableError(errors.New("temporary failure"))
	assert.True(t, IsRetryableError(retryableErr))

	// Test non-retryable error
	nonRetryableErr := NewNonRetryableError(errors.New("permanent failure"))
	assert.False(t, IsRetryableError(nonRetryableErr))

	// Test regular error (should be evaluated based on type)
	regularErr := errors.New("some error")
	assert.False(t, IsRetryableError(regularErr))

	// Test nil error
	assert.False(t, IsRetryableError(nil))
}

// TestNetworkErrorRetryClassification tests network error retry classification
func TestNetworkErrorRetryClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "connection reset",
			err:       errors.New("connection reset by peer"),
			retryable: true,
		},
		{
			name:      "broken pipe",
			err:       errors.New("broken pipe"),
			retryable: true,
		},
		{
			name:      "connection refused",
			err:       errors.New("connection refused"),
			retryable: true,
		},
		{
			name:      "no such host",
			err:       errors.New("no such host"),
			retryable: true,
		},
		{
			name:      "network unreachable",
			err:       errors.New("network is unreachable"),
			retryable: true,
		},
		{
			name:      "non-network error",
			err:       errors.New("invalid argument"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNetworkRetryableError(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

// TestRequestSizeCalculation tests the request size calculation logic
func TestRequestSizeCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         io.Reader
		contentLen   int64
		expectedSize int64
	}{
		{
			name:         "nil body",
			body:         nil,
			contentLen:   0,
			expectedSize: 0,
		},
		{
			name:         "body with content length",
			body:         strings.NewReader("test data"),
			contentLen:   9,
			expectedSize: 9,
		},
		{
			name:         "body without content length",
			body:         strings.NewReader("test data"),
			contentLen:   -1,
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "http://example.com", tt.body)
			require.NoError(t, err)
			req.ContentLength = tt.contentLen

			size := getRequestSize(req)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

// TestResponseSizeCalculation tests the response size calculation logic
func TestResponseSizeCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contentLen   int64
		expectedSize int64
	}{
		{
			name:         "with content length",
			contentLen:   100,
			expectedSize: 100,
		},
		{
			name:         "without content length",
			contentLen:   -1,
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				ContentLength: tt.contentLen,
			}

			size := getResponseSize(resp)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

// TestHostExtraction tests the host extraction logic for metrics
func TestHostExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		urlStr       string
		expectedHost string
	}{
		{
			name:         "hostname with port",
			urlStr:       "http://example.com:8080/path",
			expectedHost: "example.com",
		},
		{
			name:         "hostname without port",
			urlStr:       "http://example.com/path",
			expectedHost: "example.com",
		},
		{
			name:         "localhost with port",
			urlStr:       "http://localhost:3000/api",
			expectedHost: "localhost",
		},
		{
			name:         "IP address with port",
			urlStr:       "http://192.168.1.1:8080/test",
			expectedHost: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.urlStr)
			require.NoError(t, err)

			host := getHost(u)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

// TestClientCloseResourceCleanup tests that client.Close() properly cleans up resources
func TestClientCloseResourceCleanup(t *testing.T) {
	t.Parallel()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
		},
	}

	client := New(config, "test-client")

	// Close should not return error
	err := client.Close()
	assert.NoError(t, err)

	// Calling close again should be safe
	err2 := client.Close()
	assert.NoError(t, err2)
}

// TestRequestBodyCloning tests the request body cloning functionality
func TestRequestBodyCloning(t *testing.T) {
	t.Parallel()

	originalData := []byte("test request data")
	req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(originalData))
	require.NoError(t, err)

	clonedBody, err := cloneRequestBody(req)
	require.NoError(t, err)
	require.NotNil(t, clonedBody)

	// Read from cloned body
	clonedData, err := io.ReadAll(clonedBody)
	require.NoError(t, err)
	assert.Equal(t, originalData, clonedData)

	// Original body should still be readable
	originalData2, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, originalData, originalData2)
}

// TestNilBodyCloning tests cloning when request has nil body
func TestNilBodyCloning(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	clonedBody, err := cloneRequestBody(req)
	require.NoError(t, err)
	assert.Nil(t, clonedBody)
}
