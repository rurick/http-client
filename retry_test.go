package httpclient

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTemporaryError struct {
	msg       string
	temporary bool
	timeout   bool
}

func (e *mockTemporaryError) Error() string {
	return e.msg
}

func (e *mockTemporaryError) Temporary() bool {
	return e.temporary
}

func (e *mockTemporaryError) Timeout() bool {
	return e.timeout
}

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "temporary network error",
			err:      &mockTemporaryError{msg: "temporary error", temporary: true},
			expected: true,
		},
		{
			name:     "timeout error",
			err:      &mockTemporaryError{msg: "timeout error", timeout: true},
			expected: true,
		},
		{
			name:     "connection reset error",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe error",
			err:      errors.New("broken pipe"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("invalid request"),
			expected: false,
		},
		{
			name:     "retryable error interface",
			err:      NewRetryableError(errors.New("custom retryable")),
			expected: true,
		},
		{
			name:     "non-retryable error interface",
			err:      NewNonRetryableError(errors.New("custom non-retryable")),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, IsRetryableError(tc.err), "IsRetryableError returned unexpected result")
		})
	}
}

func TestIsNetworkRetryableError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "temporary network error",
			err:      &mockTemporaryError{msg: "network error", temporary: true},
			expected: true,
		},
		{
			name:     "non-temporary network error",
			err:      &mockTemporaryError{msg: "network error", temporary: false},
			expected: false,
		},
		{
			name:     "connection reset error",
			err:      errors.New("read tcp 127.0.0.1:8080->127.0.0.1:54321: connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe error",
			err:      errors.New("write tcp 127.0.0.1:8080->127.0.0.1:54321: broken pipe"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      errors.New("dial tcp 127.0.0.1:8080: connection refused"),
			expected: true,
		},
		{
			name:     "no such host error",
			err:      errors.New("dial tcp: lookup nonexistent.example.com: no such host"),
			expected: true,
		},
		{
			name:     "network unreachable error",
			err:      errors.New("dial tcp 192.0.2.1:80: network is unreachable"),
			expected: true,
		},
		{
			name:     "url error wrapping network error",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection reset")},
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isNetworkRetryableError(tc.err)
			assert.Equal(t, tc.expected, result, "IsNetworkRetryableError returned unexpected result")
		})
	}
}

func TestIsTimeoutRetryableError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout network error",
			err:      &mockTemporaryError{msg: "timeout", timeout: true},
			expected: true,
		},
		{
			name:     "non-timeout network error",
			err:      &mockTemporaryError{msg: "other error", timeout: false},
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "request timeout",
			err:      errors.New("net/http: request timeout"),
			expected: true,
		},
		{
			name:     "dial timeout",
			err:      errors.New("dial tcp 127.0.0.1:8080: i/o timeout"),
			expected: true,
		},
		{
			name:     "url error wrapping timeout",
			err:      &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("timeout")},
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, isTimeoutRetryableError(tc.err), "isTimeoutRetryableError returned unexpected result")
		})
	}
}

func TestClassifyError(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "timeout error",
			err:      &mockTemporaryError{msg: "timeout", timeout: true},
			expected: "timeout",
		},
		{
			name:     "network error",
			err:      &mockTemporaryError{msg: "connection error", temporary: true},
			expected: "net",
		},
		{
			name:     "connection reset error",
			err:      errors.New("connection reset by peer"),
			expected: "net",
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: "timeout",
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: "other",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, ClassifyError(tc.err), "ClassifyError returned unexpected result")
		})
	}
}

func TestNewRetryableError(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("original error")
	retryableErr := NewRetryableError(originalErr)

	assert.NotNil(t, retryableErr)
	assert.Equal(t, "original error", retryableErr.Error(), "Error message not as expected")
	assert.True(t, IsRetryableError(retryableErr), "expected error to be retryable")
	// Проверяем unwrapping
	assert.ErrorIs(t, retryableErr, originalErr, "expected error to wrap original error")
}

func TestNewNonRetryableError(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("original error")
	nonRetryableErr := NewNonRetryableError(originalErr)

	assert.NotNil(t, nonRetryableErr)
	assert.Equal(t, "original error", nonRetryableErr.Error(), "Error message not as expected")
	assert.EqualError(t, nonRetryableErr, "original error")
	assert.False(t, IsRetryableError(nonRetryableErr), "expected error to be non-retryable")

	// Проверяем unwrapping
	assert.ErrorIs(t, nonRetryableErr, originalErr, "expected error to wrap original error")

}
