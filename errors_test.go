package httpclient

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPErrorType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      *HTTPError
		expected string
	}{
		{
			name: "basic HTTP error",
			err: &HTTPError{
				StatusCode: 404,
				Status:     "Not Found",
				Method:     "GET",
				URL:        "https://api.example.com/users",
			},
			expected: "HTTP 404 Not Found: GET https://api.example.com/users",
		},
		{
			name: "500 error",
			err: &HTTPError{
				StatusCode: 500,
				Status:     "Internal Server Error",
				Method:     "POST",
				URL:        "https://api.example.com/create",
			},
			expected: "HTTP 500 Internal Server Error: POST https://api.example.com/create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.err.Error(), "MaxAttemptsExceededError.Error() returned unexpected result")
		})
	}
}

func TestMaxAttemptsExceededErrorType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      *MaxAttemptsExceededError
		expected string
	}{
		{
			name: "with last error",
			err: &MaxAttemptsExceededError{
				MaxAttempts: 3,
				LastError:   errors.New("network timeout"),
			},
			expected: "max attempts (3) exceeded, last error: network timeout",
		},
		{
			name: "with status code",
			err: &MaxAttemptsExceededError{
				MaxAttempts: 5,
				LastStatus:  502,
			},
			expected: "max attempts (5) exceeded, last status: 502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.err.Error(), "MaxAttemptsExceededError.Error() returned unexpected result")
		})
	}
}

func TestTimeoutExceededErrorType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      *TimeoutExceededError
		expected string
	}{
		{
			name: "basic timeout error",
			err: &TimeoutExceededError{
				Timeout: 5 * time.Second,
				Elapsed: 6 * time.Second,
			},
			expected: "timeout exceeded: 6s elapsed, 5s allowed",
		},
		{
			name: "milliseconds",
			err: &TimeoutExceededError{
				Timeout: 500 * time.Millisecond,
				Elapsed: 600 * time.Millisecond,
			},
			expected: "timeout exceeded: 600ms elapsed, 500ms allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.err.Error(), "TimeoutExceededError.Error() returned unexpected result")
		})
	}
}

func TestIsHTTPError(t *testing.T) {
	t.Parallel()
	httpErr := &HTTPError{StatusCode: 404}
	regularErr := errors.New("regular error")

	if !IsHTTPError(httpErr) {
		t.Error("IsHTTPError should return true for HTTPError")
	}

	if IsHTTPError(regularErr) {
		t.Error("IsHTTPError should return false for regular error")
	}

	if IsHTTPError(nil) {
		t.Error("IsHTTPError should return false for nil")
	}
}

func TestNewHTTPError(t *testing.T) {
	t.Parallel()
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/api"},
	}

	resp := &http.Response{
		StatusCode: 404,
		Status:     "Not Found",
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	httpErr := NewHTTPError(resp, req)

	if httpErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", httpErr.StatusCode)
	}
	if httpErr.Status != "Not Found" {
		t.Errorf("Status = %s, want 'Not Found'", httpErr.Status)
	}
	if httpErr.Method != "GET" {
		t.Errorf("Method = %s, want 'GET'", httpErr.Method)
	}
	if httpErr.URL != "https://example.com/api" {
		t.Errorf("URL = %s, want 'https://example.com/api'", httpErr.URL)
	}
}

func TestMaxAttemptsExceededErrorUnwrap(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("network error")
	maxErr := &MaxAttemptsExceededError{
		MaxAttempts: 3,
		LastError:   originalErr,
	}

	unwrapped := maxErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}

	// Test with no last error
	maxErrNoLast := &MaxAttemptsExceededError{
		MaxAttempts: 3,
		LastStatus:  500,
	}

	unwrappedNil := maxErrNoLast.Unwrap()
	if unwrappedNil != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrappedNil)
	}
}
