package httpclient

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Constants for classifying retry reasons.
const (
	RetryReasonTimeout    = "timeout"
	RetryReasonNetwork    = "net"
	RetryReasonPreConnect = "pre-connect"
)

// preConnectErrorStrings contains error substrings indicating TCP-level failures
// where the connection was never established or failed before any HTTP data
// could be transmitted. Safe for retry regardless of HTTP method.
var preConnectErrorStrings = []string{
	"connection refused",
	"connection reset",
	"broken pipe",
	"no such host",
	"network is unreachable",
	"connection timed out",
}

// timeoutErrorStrings contains error substrings indicating timeout failures.
var timeoutErrorStrings = []string{
	"timeout",
	"deadline exceeded",
	"context deadline exceeded",
	"request timeout",
}

// RetryableError interface for errors that can be retried.
type RetryableError interface {
	error
	Retryable() bool
}

// retryableError wrapper for errors that can be retried.
type retryableError struct {
	err       error
	retryable bool
}

func (re *retryableError) Error() string {
	return re.err.Error()
}

func (re *retryableError) Retryable() bool {
	return re.retryable
}

func (re *retryableError) Unwrap() error {
	return re.err
}

// NewRetryableError creates a new error that can be retried.
func NewRetryableError(err error) error {
	return &retryableError{
		err:       err,
		retryable: true,
	}
}

// NewNonRetryableError creates a new error that cannot be retried.
func NewNonRetryableError(err error) error {
	return &retryableError{
		err:       err,
		retryable: false,
	}
}

// IsRetryableError checks if an error can be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var retryableErr RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable()
	}

	// Check standard error types
	return isNetworkRetryableError(err) || isTimeoutRetryableError(err)
}

// isNetworkRetryableError checks if a network error is retryable.
func isNetworkRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check net.Error (timeouts can be retried)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Check timeouts as retryable errors
		if netErr.Timeout() {
			return true
		}
	}

	// Check url.Error
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isNetworkRetryableError(urlErr.Err)
	}

	// Check specific network errors (replaces deprecated Temporary())
	errStr := err.Error()
	for _, s := range preConnectErrorStrings {
		if strings.Contains(errStr, s) {
			return true
		}
	}

	return false
}

// isTimeoutRetryableError checks if a timeout error is retryable.
func isTimeoutRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check net.Error for timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check url.Error
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeoutRetryableError(urlErr.Err)
	}

	errStr := err.Error()

	// Check timeout-related strings
	for _, s := range timeoutErrorStrings {
		if strings.Contains(errStr, s) {
			return true
		}
	}

	return false
}

// ClassifyError classifies an error for retry purposes.
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}

	if isTimeoutRetryableError(err) {
		return RetryReasonTimeout
	}

	if isNetworkRetryableError(err) {
		return RetryReasonNetwork
	}

	return "other"
}

// isPreConnectError checks if an error occurred before the HTTP request was sent.
// These are TCP-level errors where the connection was never established or failed
// before any HTTP data could be transmitted, making retry safe for any HTTP method.
func isPreConnectError(err error) bool {
	if err == nil {
		return false
	}

	// Unwrap url.Error
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isPreConnectError(urlErr.Err)
	}

	errStr := err.Error()
	for _, s := range preConnectErrorStrings {
		if strings.Contains(errStr, s) {
			return true
		}
	}

	return false
}
