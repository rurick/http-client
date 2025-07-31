package httpclient

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// SimpleCircuitBreaker implements a basic circuit breaker pattern
type SimpleCircuitBreaker struct {
	mu                    sync.RWMutex
	state                 CircuitBreakerState
	failureCount          int
	successCount          int
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	lastFailureTime       time.Time
	onStateChangeCallback func(from, to CircuitBreakerState)
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening
	SuccessThreshold int           // Number of successes to close from half-open
	Timeout          time.Duration // Time to wait before going to half-open
	OnStateChange    func(from, to CircuitBreakerState)
}

// NewSimpleCircuitBreaker creates a new circuit breaker with default settings
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker {
	return NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          60 * time.Second,
	})
}

// NewCircuitBreakerWithConfig creates a new circuit breaker with custom configuration
func NewCircuitBreakerWithConfig(config CircuitBreakerConfig) *SimpleCircuitBreaker {
	return &SimpleCircuitBreaker{
		state:                 CircuitBreakerClosed,
		failureThreshold:      config.FailureThreshold,
		successThreshold:      config.SuccessThreshold,
		timeout:               config.Timeout,
		onStateChangeCallback: config.OnStateChange,
	}
}

// Execute runs the function through the circuit breaker
func (cb *SimpleCircuitBreaker) Execute(fn func() (*http.Response, error)) (*http.Response, error) {
	if !cb.canExecute() {
		return nil, ErrCircuitBreakerOpen
	}

	resp, err := fn()

	cb.recordResult(resp, err)

	return resp, err
}

// State returns the current state of the circuit breaker
func (cb *SimpleCircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker to closed state
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

// canExecute determines if the circuit breaker allows execution
func (cb *SimpleCircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.setState(CircuitBreakerHalfOpen)
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an execution
func (cb *SimpleCircuitBreaker) recordResult(resp *http.Response, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isSuccess := cb.isSuccess(resp, err)

	switch cb.state {
	case CircuitBreakerClosed:
		if isSuccess {
			cb.failureCount = 0
		} else {
			cb.failureCount++
			cb.lastFailureTime = time.Now()
			if cb.failureThreshold > 0 && cb.failureCount >= cb.failureThreshold {
				cb.setState(CircuitBreakerOpen)
			}
		}
	case CircuitBreakerOpen:
		// В состоянии Open мы не записываем результаты, так как запросы не выполняются
		// Переход в Half-Open происходит только по таймауту в canExecute()
	case CircuitBreakerHalfOpen:
		if isSuccess {
			cb.successCount++
			if cb.successCount >= cb.successThreshold {
				cb.setState(CircuitBreakerClosed)
				cb.failureCount = 0
				cb.successCount = 0
			}
		} else {
			cb.setState(CircuitBreakerOpen)
			cb.failureCount++
			cb.successCount = 0
			cb.lastFailureTime = time.Now()
		}
	}
}

// isSuccess determines if a response/error combination is considered successful
func (cb *SimpleCircuitBreaker) isSuccess(resp *http.Response, err error) bool {
	if err != nil {
		return false
	}

	if resp == nil {
		return false
	}

	// Consider 5xx status codes as failures
	return resp.StatusCode < 500
}

// setState changes the circuit breaker state and calls the callback if set
func (cb *SimpleCircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChangeCallback != nil && oldState != newState {
		cb.onStateChangeCallback(oldState, newState)
	}
}

// CircuitBreakerMiddleware wraps a circuit breaker as middleware
type CircuitBreakerMiddleware struct {
	circuitBreaker CircuitBreaker
}

// NewCircuitBreakerMiddleware creates a new circuit breaker middleware
func NewCircuitBreakerMiddleware(cb CircuitBreaker) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		circuitBreaker: cb,
	}
}

// Process implements the Middleware interface
func (cbm *CircuitBreakerMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	return cbm.circuitBreaker.Execute(func() (*http.Response, error) {
		return next(req)
	})
}
