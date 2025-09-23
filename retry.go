package httpclient

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Константы для классификации причин retry.
const (
	RetryReasonTimeout = "timeout"
	RetryReasonNetwork = "net"
)

// RetryableError интерфейс для ошибок, которые можно повторить.
type RetryableError interface {
	error
	Retryable() bool
}

// retryableError обёртка для ошибок, которые можно повторить.
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

// NewRetryableError создаёт новую ошибку, которую можно повторить.
func NewRetryableError(err error) error {
	return &retryableError{
		err:       err,
		retryable: true,
	}
}

// NewNonRetryableError создаёт новую ошибку, которую нельзя повторить.
func NewNonRetryableError(err error) error {
	return &retryableError{
		err:       err,
		retryable: false,
	}
}

// IsRetryableError проверяет, можно ли повторить ошибку.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var retryableErr RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable()
	}

	// Проверяем стандартные типы ошибок
	return isNetworkRetryableError(err) || isTimeoutRetryableError(err)
}

// isNetworkRetryableError проверяет, является ли сетевая ошибка повторяемой.
func isNetworkRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем net.Error (таймауты можно повторять)
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Проверяем таймауты как повторяемые ошибки
		if netErr.Timeout() {
			return true
		}
	}

	// Проверяем url.Error
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isNetworkRetryableError(urlErr.Err)
	}

	// Проверяем специфические сетевые ошибки (заменя deprecated Temporary())
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

// isTimeoutRetryableError проверяет, является ли ошибка таймаута повторяемой.
func isTimeoutRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем net.Error на таймаут
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Проверяем url.Error
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeoutRetryableError(urlErr.Err)
	}

	errStr := err.Error()

	// Проверяем строки, связанные с таймаутом
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

// ClassifyError классифицирует ошибку для целей retry.
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
