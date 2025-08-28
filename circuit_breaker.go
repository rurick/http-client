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

// CircuitBreaker определяет интерфейс автоматического выключателя.
type CircuitBreaker interface {
	Execute(fn func() (*http.Response, error)) (*http.Response, error)
	State() CircuitBreakerState
	Reset()
}

// CircuitBreakerState представляет состояние автоматического выключателя.
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

// SimpleCircuitBreaker реализует базовый паттерн автоматического выключателя.
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

// CircuitBreakerConfig содержит конфигурацию для автоматического выключателя.
type CircuitBreakerConfig struct {
	FailStatusCodes  []int         // Коды статуса, при которых считается ошибкой (по умолчанию 5xx и 429)
	FailureThreshold int           // Количество ошибок до открытия
	SuccessThreshold int           // Количество успешных попыток для закрытия из полуустановленного состояния
	Timeout          time.Duration // Время ожидания перед переходом в полуустановленное состояние
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

// NewSimpleCircuitBreaker создает новый автоматический выключатель с настройками по умолчанию.
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker {
	return NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailStatusCodes: nil,
		OnStateChange: func(_, _ CircuitBreakerState) {
			// Пустой обработчик по умолчанию
		},
		FailureThreshold: defaultFailureThreshold,
		SuccessThreshold: defaultSuccessThreshold,
		Timeout:          defaultCircuitTimeout,
	})
}

// NewCircuitBreakerWithConfig создает новый автоматический выключатель с пользовательской конфигурацией.
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

// Execute выполняет функцию через автоматический выключатель.
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
		resp.Body.Close() // Всегда закрываем оригинальное body после чтения
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

// State возвращает текущее состояние автоматического выключателя.
func (cb *SimpleCircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset вручную сбрасывает автоматический выключатель в закрытое состояние.
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
		// Проверяем, следует ли перейти в полуустановленное состояние
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

// recordResult записывает результат выполнения.
func (cb *SimpleCircuitBreaker) recordResult(resp *http.Response, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isSuccess := cb.isSuccess(resp, err)

	// Store a clone of the failed response for later use
	if !isSuccess && resp != nil {
		// Закрываем предыдущий сохранённый ответ чтобы избежать утечки памяти
		if cb.lastFailResponse != nil && cb.lastFailResponse.Body != nil {
			cb.lastFailResponse.Body.Close()
		}
		// Clone the response before storing it to avoid sharing mutable state
		// Важно: исходный resp.Body будет закрыт внутри safeCloneResponse
		clonedResp := cb.safeCloneResponse(resp)
		// Сохраняем клонированный response независимо от наличия body
		cb.lastFailResponse = clonedResp
	}

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
		// Всегда закрываем оригинальное body после чтения для предотвращения утечек
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Логируем ошибку закрытия, но продолжаем работу
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
			// Восстанавливаем оригинальный body как пустой для consistency
			resp.Body = io.NopCloser(strings.NewReader(""))
		}
	} else {
		// Original had no body
		clone.Body = nil
	}

	return clone
}

// isSuccess определяет, считается ли комбинация ответа/ошибки успешной.
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

// setState изменяет состояние автоматического выключателя и вызывает callback, если он установлен.
func (cb *SimpleCircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChangeCallback != nil && oldState != newState {
		cb.onStateChangeCallback(oldState, newState)
	}
}

// CircuitBreakerMiddleware оборачивает автоматический выключатель как middleware.
type CircuitBreakerMiddleware struct {
	circuitBreaker CircuitBreaker
}

// NewCircuitBreakerMiddleware создает новый middleware для автоматического выключателя.
func NewCircuitBreakerMiddleware(cb CircuitBreaker) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		circuitBreaker: cb,
	}
}

// Process реализует интерфейс Middleware.
func (cbm *CircuitBreakerMiddleware) Process(
	req *http.Request,
	next func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	return cbm.circuitBreaker.Execute(func() (*http.Response, error) {
		return next(req)
	})
}
