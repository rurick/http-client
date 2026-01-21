package httpclient

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// CircuitBreaker defines the interface for a circuit breaker.
type CircuitBreaker interface {
	Execute(fn func() (*http.Response, error)) (*http.Response, error)
	State() CircuitBreakerState
	Reset()
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// SimpleCircuitBreaker implements the basic circuit breaker pattern.
type SimpleCircuitBreaker struct {
	mu                    sync.RWMutex
	state                 CircuitBreakerState
	lastFailResponse      *http.Response
	failStatuses          []int
	failureCount          int
	successCount          int
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	lastFailureTime       time.Time
	onStateChangeCallback func(from, to CircuitBreakerState)
}

// CircuitBreakerConfig contains configuration for a circuit breaker.
type CircuitBreakerConfig struct {
	FailStatusCodes  []int         // Status codes that are considered errors (default: 5xx and 429)
	FailureThreshold int           // Number of failures before opening
	SuccessThreshold int           // Number of successful attempts to close from half-open state
	Timeout          time.Duration // Wait time before transitioning to half-open state
	OnStateChange    func(from, to CircuitBreakerState)
}

type strictReadCloser struct {
	reader *bytes.Reader
	mu     sync.RWMutex
	closed bool
}

func (s *strictReadCloser) Read(p []byte) (int, error) {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return 0, errors.New("http: read on closed response body")
	}
	return s.reader.Read(p)
}

func (s *strictReadCloser) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func newStrictReadCloser(b []byte) io.ReadCloser {
	return &strictReadCloser{reader: bytes.NewReader(b)}
}

// NewSimpleCircuitBreaker creates a new circuit breaker with default settings.
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker {
	return NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailStatusCodes: nil,
		OnStateChange: func(_, _ CircuitBreakerState) {
			// Empty handler by default
		},
		FailureThreshold: defaultFailureThreshold,
		SuccessThreshold: defaultSuccessThreshold,
		Timeout:          defaultCircuitTimeout,
	})
}

// NewCircuitBreakerWithConfig creates a new circuit breaker with custom configuration.
func NewCircuitBreakerWithConfig(config CircuitBreakerConfig) *SimpleCircuitBreaker {
	return &SimpleCircuitBreaker{
		state:                 CircuitBreakerClosed,
		failStatuses:          config.FailStatusCodes,
		failureThreshold:      config.FailureThreshold,
		successThreshold:      config.SuccessThreshold,
		timeout:               config.Timeout,
		onStateChangeCallback: config.OnStateChange,
	}
}

// Execute executes a function through the circuit breaker.
func (cb *SimpleCircuitBreaker) Execute(fn func() (*http.Response, error)) (*http.Response, error) {
	// Check if we can execute and get the last fail response atomically
	canExec, lastFailResp := cb.canExecuteAndGetLastFailResponse()
	if !canExec {
		return cb.cloneHTTPResponse(lastFailResp), ErrCircuitBreakerOpen
	}

	resp, err := fn()

	cb.recordResult(resp, err)

	return resp, err
}

func (cb *SimpleCircuitBreaker) cloneHTTPResponse(resp *http.Response) *http.Response {
	if resp == nil {
		return nil
	}

	clone := &http.Response{
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           make(http.Header, len(resp.Header)),
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          make(http.Header, len(resp.Trailer)),
		Request:          resp.Request,
		TLS:              resp.TLS,
	}

	for k, v := range resp.Header {
		clone.Header[k] = v
	}
	for k, v := range resp.Trailer {
		clone.Trailer[k] = v
	}

	// Handle body cloning safely - avoid concurrent reading
	if resp.Body != nil {
		// Try to read the body, but handle the case where it might already be read
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close() // Always close original body after reading
		if err == nil && len(bodyBytes) > 0 {
			// Restore original body for the caller
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			// Clone gets its own copy of the body
			clone.Body = newStrictReadCloser(bodyBytes)
			clone.ContentLength = int64(len(bodyBytes))
		} else {
			// Body was already read, empty, or there was an error
			// Create an empty body for the clone to avoid nil pointer issues
			clone.Body = newStrictReadCloser(nil)
			clone.ContentLength = 0
		}
	}

	return clone
}

// State returns the current state of the circuit breaker.
func (cb *SimpleCircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker to closed state.
func (cb *SimpleCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = CircuitBreakerClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastFailureTime = time.Time{}

	if cb.onStateChangeCallback != nil && oldState != CircuitBreakerClosed {
		cb.onStateChangeCallback(oldState, CircuitBreakerClosed)
	}
}

// canExecuteAndGetLastFailResponse atomically checks if we can execute and gets the last fail response.
func (cb *SimpleCircuitBreaker) canExecuteAndGetLastFailResponse() (bool, *http.Response) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	var lastFailResp *http.Response
	if cb.lastFailResponse != nil {
		// Make a copy of the last fail response to avoid races
		lastFailResp = cb.lastFailResponse
	}

	switch cb.state {
	case CircuitBreakerClosed:
		return true, lastFailResp
	case CircuitBreakerOpen:
		// Check if we should transition to half-open state
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.setState(CircuitBreakerHalfOpen)
			return true, lastFailResp
		}
		return false, lastFailResp
	case CircuitBreakerHalfOpen:
		return true, lastFailResp
	default:
		return false, lastFailResp
	}
}

// recordResult records the execution result.
func (cb *SimpleCircuitBreaker) recordResult(resp *http.Response, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isSuccess := cb.isSuccess(resp, err)

	// Handle unsuccessful response
	if !isSuccess {
		cb.handleFailedResponse(resp)
	}

	// Update state based on current circuit breaker state
	cb.updateStateOnResult(isSuccess)
}

// handleFailedResponse handles an unsuccessful response by cloning it for later use.
func (cb *SimpleCircuitBreaker) handleFailedResponse(resp *http.Response) {
	if resp == nil {
		return
	}

	// Close previous saved response to avoid memory leaks
	cb.closeLastFailResponse()

	// Clone the response before storing it to avoid sharing mutable state
	// Important: original resp.Body will be closed inside safeCloneResponse
	clonedResp := cb.safeCloneResponse(resp) //nolint:bodyclose // body is properly closed inside safeCloneResponse
	// Save cloned response regardless of body presence
	cb.lastFailResponse = clonedResp
}

// closeLastFailResponse safely closes the previous saved response.
func (cb *SimpleCircuitBreaker) closeLastFailResponse() {
	if cb.lastFailResponse != nil && cb.lastFailResponse.Body != nil {
		if closeErr := cb.lastFailResponse.Body.Close(); closeErr != nil {
			log.Printf("Failed to close lastFailResponse body: %v", closeErr)
		}
	}
}

// updateStateOnResult updates the circuit breaker state based on request result.
func (cb *SimpleCircuitBreaker) updateStateOnResult(isSuccess bool) {
	switch cb.state {
	case CircuitBreakerClosed:
		cb.handleClosedState(isSuccess)
	case CircuitBreakerOpen:
		// In Open state we don't record results, as requests are not executed
		// Transition to Half-Open only happens by timeout in canExecute()
	case CircuitBreakerHalfOpen:
		cb.handleHalfOpenState(isSuccess)
	}
}

// handleClosedState handles the result in Closed state.
func (cb *SimpleCircuitBreaker) handleClosedState(isSuccess bool) {
	if isSuccess {
		cb.failureCount = 0
		return
	}

	// Handle unsuccessful result
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	// Check if we need to open the circuit breaker
	if cb.shouldOpenCircuit() {
		cb.setState(CircuitBreakerOpen)
	}
}

// handleHalfOpenState handles the result in Half-Open state.
func (cb *SimpleCircuitBreaker) handleHalfOpenState(isSuccess bool) {
	if isSuccess {
		cb.handleSuccessInHalfOpen()
	} else {
		cb.handleFailureInHalfOpen()
	}
}

// handleSuccessInHalfOpen handles a successful result in Half-Open state.
func (cb *SimpleCircuitBreaker) handleSuccessInHalfOpen() {
	cb.successCount++
	if cb.successCount >= cb.successThreshold {
		cb.setState(CircuitBreakerClosed)
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// handleFailureInHalfOpen handles an unsuccessful result in Half-Open state.
func (cb *SimpleCircuitBreaker) handleFailureInHalfOpen() {
	cb.setState(CircuitBreakerOpen)
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()
}

// shouldOpenCircuit determines if the circuit breaker should be opened.
func (cb *SimpleCircuitBreaker) shouldOpenCircuit() bool {
	return cb.failureThreshold > 0 && cb.failureCount >= cb.failureThreshold
}

// safeCloneResponse creates a safe clone of the HTTP response without concurrent body reading.
func (cb *SimpleCircuitBreaker) safeCloneResponse(resp *http.Response) *http.Response {
	if resp == nil {
		return nil
	}

	clone := &http.Response{
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           make(http.Header, len(resp.Header)),
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          make(http.Header, len(resp.Trailer)),
		Request:          resp.Request,
		TLS:              resp.TLS,
	}

	// Copy headers
	for k, v := range resp.Header {
		clone.Header[k] = append([]string(nil), v...)
	}
	for k, v := range resp.Trailer {
		clone.Trailer[k] = append([]string(nil), v...)
	}

	// Try to safely read and clone the body, but don't risk race conditions
	if resp.Body != nil {
		// Try to read the body safely
		bodyBytes, err := io.ReadAll(resp.Body)
		// Always close original body after reading to prevent leaks
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log close error, but continue
			log.Printf("Failed to close response body: %v", closeErr)
		}
		if err == nil && len(bodyBytes) > 0 {
			// Successfully read the body, restore it and clone it
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			clone.Body = newStrictReadCloser(bodyBytes)
			clone.ContentLength = int64(len(bodyBytes))
		} else {
			// Could not read body or body is empty, create empty body
			clone.Body = newStrictReadCloser(nil)
			clone.ContentLength = 0
			// Restore original body as empty for consistency
			resp.Body = io.NopCloser(strings.NewReader(""))
		}
	} else {
		// Original had no body
		clone.Body = nil
	}

	return clone
}

// isSuccess determines if the response/error combination is considered successful.
func (cb *SimpleCircuitBreaker) isSuccess(resp *http.Response, err error) bool {
	if err != nil {
		return false
	}

	if resp == nil {
		return false
	}

	if cb.failStatuses != nil {
		return !slices.Contains(cb.failStatuses, resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return false
	}
	const internalServerErrorThreshold = 500
	return resp.StatusCode < internalServerErrorThreshold
}

// setState changes the circuit breaker state and calls callback if set.
func (cb *SimpleCircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChangeCallback != nil && oldState != newState {
		cb.onStateChangeCallback(oldState, newState)
	}
}

// CircuitBreakerMiddleware wraps a circuit breaker as middleware.
type CircuitBreakerMiddleware struct {
	circuitBreaker CircuitBreaker
}

// NewCircuitBreakerMiddleware creates a new middleware for a circuit breaker.
func NewCircuitBreakerMiddleware(cb CircuitBreaker) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		circuitBreaker: cb,
	}
}

// Process implements the Middleware interface.
func (cbm *CircuitBreakerMiddleware) Process(
	req *http.Request,
	next func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	return cbm.circuitBreaker.Execute(func() (*http.Response, error) {
		return next(req)
	})
}
