package httpclient

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// HTTPError represents an HTTP error with additional information.
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
	Method     string
	Body       []byte
	Headers    http.Header
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d %s: %s %s", e.StatusCode, e.Status, e.Method, e.URL)
}

// IsHTTPError checks if an error is an HTTP error.
func IsHTTPError(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr)
}

// NewHTTPError creates a new HTTP error.
func NewHTTPError(resp *http.Response, req *http.Request) *HTTPError {
	return &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		URL:        req.URL.String(),
		Method:     req.Method,
		Headers:    resp.Header,
	}
}

// MaxAttemptsExceededError represents an error for exceeding maximum number of attempts.
type MaxAttemptsExceededError struct {
	MaxAttempts int
	LastError   error
	LastStatus  int
}

// Error implements the error interface.
func (e *MaxAttemptsExceededError) Error() string {
	if e.LastError != nil {
		return fmt.Sprintf("max attempts (%d) exceeded, last error: %v", e.MaxAttempts, e.LastError)
	}
	return fmt.Sprintf("max attempts (%d) exceeded, last status: %d", e.MaxAttempts, e.LastStatus)
}

// Unwrap returns the last error for errors.Unwrap support.
func (e *MaxAttemptsExceededError) Unwrap() error {
	return e.LastError
}

// TimeoutExceededError represents a timeout exceeded error.
type TimeoutExceededError struct {
	Timeout time.Duration
	Elapsed time.Duration
}

// Error implements the error interface.
func (e *TimeoutExceededError) Error() string {
	return fmt.Sprintf("timeout exceeded: %v elapsed, %v allowed", e.Elapsed, e.Timeout)
}

// ConfigurationError represents a configuration error.
type ConfigurationError struct {
	Field   string
	Value   interface{}
	Message string
}

// Error implements the error interface.
func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// NewConfigurationError creates a new configuration error.
func NewConfigurationError(field string, value interface{}, message string) *ConfigurationError {
	return &ConfigurationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// TimeoutError represents a detailed timeout error with context.
type TimeoutError struct {
	// Basic request information
	Method string
	URL    string
	Host   string
	// Timeout information
	Timeout       time.Duration // Overall timeout
	PerTryTimeout time.Duration // Per-attempt timeout
	Elapsed       time.Duration // Execution time until error
	// Retry context
	Attempt      int  // Attempt number on which timeout occurred
	MaxAttempts  int  // Maximum number of attempts
	RetryEnabled bool // Whether retry was enabled
	// Additional context
	TimeoutType string // Timeout type: "overall", "per-try", "context"
	OriginalErr error  // Original error
	// Solution suggestions
	Suggestions []string
}

// Error implements the error interface with detailed message.
func (e *TimeoutError) Error() string {
	var suggestions string
	if len(e.Suggestions) > 0 {
		suggestions = fmt.Sprintf(" Suggestions: %v", e.Suggestions)
	}

	return fmt.Sprintf(
		"timeout error: %s %s (host: %s) failed after %v on attempt %d/%d. "+
			"Timeout config: overall=%v, per-try=%v, retry=%t. Type: %s.%s",
		e.Method, e.URL, e.Host, e.Elapsed, e.Attempt, e.MaxAttempts,
		e.Timeout, e.PerTryTimeout, e.RetryEnabled, e.TimeoutType, suggestions,
	)
}

// Unwrap returns the original error for errors.Unwrap support.
func (e *TimeoutError) Unwrap() error {
	return e.OriginalErr
}

// NewTimeoutError creates a detailed timeout error.
func NewTimeoutError(
	req *http.Request,
	config Config,
	attempt, maxAttempts int,
	elapsed time.Duration,
	timeoutType string,
	originalErr error,
) *TimeoutError {
	host := getHost(req.URL)

	// Generate suggestions for solving the problem
	suggestions := generateTimeoutSuggestions(config, elapsed, timeoutType, attempt, maxAttempts)

	return &TimeoutError{
		Method:        req.Method,
		URL:           req.URL.String(),
		Host:          host,
		Timeout:       config.Timeout,
		PerTryTimeout: config.PerTryTimeout,
		Elapsed:       elapsed,
		Attempt:       attempt,
		MaxAttempts:   maxAttempts,
		RetryEnabled:  config.RetryEnabled,
		TimeoutType:   timeoutType,
		OriginalErr:   originalErr,
		Suggestions:   suggestions,
	}
}

// generateTimeoutSuggestions generates suggestions for solving timeout problems.
func generateTimeoutSuggestions(
	config Config,
	elapsed time.Duration,
	timeoutType string,
	attempt, maxAttempts int,
) []string {
	var suggestions []string

	switch timeoutType {
	case "overall":
		if elapsed >= config.Timeout {
			suggestions = append(suggestions, fmt.Sprintf("increase overall timeout (current: %v)", config.Timeout))
		}
		if !config.RetryEnabled {
			suggestions = append(suggestions, "enable retry for resilience to temporary failures")
		}

	case "per-try":
		if elapsed >= config.PerTryTimeout {
			suggestions = append(suggestions, fmt.Sprintf("increase per-try timeout (current: %v)", config.PerTryTimeout))
		}
		if attempt < maxAttempts {
			suggestions = append(suggestions, "retry attempts continue")
		}

	case "context":
		suggestions = append(suggestions, "timeout was set in context.WithTimeout() or context.WithDeadline()")
		suggestions = append(suggestions, "check context settings in calling code")
	}

	// General suggestions
	if config.RetryEnabled && attempt >= maxAttempts {
		suggestions = append(suggestions, fmt.Sprintf("increase number of attempts (current: %d)", maxAttempts))
	}

	if elapsed > 10*time.Second {
		suggestions = append(suggestions, "check availability and performance of remote service")
	}

	return suggestions
}
