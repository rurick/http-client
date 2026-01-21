package httpclient

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Constants for classifying retry reasons.
const (
	RetryReasonTimeout = "timeout"
	RetryReasonNetwork = "net"
)

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
	retryableErrors := []string{
		"connection reset",
		"broken pipe",
		"connection refused",
		"no such host",
		"network is unreachable",
		"connection timed out",
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(errStr, retryableErr) {
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
	timeoutErrors := []string{
		"timeout",
		"deadline exceeded",
		"context deadline exceeded",
		"request timeout",
	}

	for _, timeoutErr := range timeoutErrors {
		if strings.Contains(errStr, timeoutErr) {
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
