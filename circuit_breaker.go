package httpclient

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"
)

var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// CircuitBreaker определяет интерфейс автоматического выключателя
type CircuitBreaker interface {
	Execute(fn func() (*http.Response, error)) (*http.Response, error)
	State() CircuitBreakerState
	Reset()
}

// CircuitBreakerState представляет состояние автоматического выключателя
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

// SimpleCircuitBreaker реализует базовый паттерн автоматического выключателя
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

// CircuitBreakerConfig содержит конфигурацию для автоматического выключателя
type CircuitBreakerConfig struct {
	FailStatusCodes  []int         // Коды статуса, при которых считается ошибкой (по умолчанию 5xx и 429)
	FailureThreshold int           // Количество ошибок до открытия
	SuccessThreshold int           // Количество успешных попыток для закрытия из полуустановленного состояния
	Timeout          time.Duration // Время ожидания перед переходом в полуустановленное состояние
	OnStateChange    func(from, to CircuitBreakerState)
}

type strictReadCloser struct {
	reader *bytes.Reader
	closed bool
}

func (s *strictReadCloser) Read(p []byte) (int, error) {
	if s.closed {
		return 0, errors.New("http: read on closed response body")
	}
	return s.reader.Read(p)
}

func (s *strictReadCloser) Close() error {
	s.closed = true
	return nil
}

func newStrictReadCloser(b []byte) io.ReadCloser {
	return &strictReadCloser{reader: bytes.NewReader(b)}
}

// NewSimpleCircuitBreaker создает новый автоматический выключатель с настройками по умолчанию
func NewSimpleCircuitBreaker() *SimpleCircuitBreaker {
	return NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailStatusCodes: nil,
		OnStateChange: func(from, to CircuitBreakerState) {
		},
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          60 * time.Second,
	})
}

// NewCircuitBreakerWithConfig создает новый автоматический выключатель с пользовательской конфигурацией
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

// Execute выполняет функцию через автоматический выключатель
func (cb *SimpleCircuitBreaker) Execute(fn func() (*http.Response, error)) (*http.Response, error) {
	if !cb.canExecute() {
		return cb.cloneHttpResponse(cb.lastFailResponse), ErrCircuitBreakerOpen
	}

	resp, err := fn()

	cb.recordResult(resp, err)

	return resp, err
}

func (cb *SimpleCircuitBreaker) cloneHttpResponse(resp *http.Response) *http.Response {
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

	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			// Восстанавливаем оригинальное тело для вызывающего
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			// В клон кладем «строгое» тело, которое после Close() не даст читать
			clone.Body = newStrictReadCloser(bodyBytes)
			// Опционально: установить точную длину
			clone.ContentLength = int64(len(bodyBytes))
		} else {
			// Не удалось прочитать — оставляем клон без тела
			clone.Body = nil
		}
	}

	return clone
}

// State возвращает текущее состояние автоматического выключателя
func (cb *SimpleCircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset вручную сбрасывает автоматический выключатель в закрытое состояние
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

// canExecute определяет, позволяет ли автоматический выключатель выполнение
func (cb *SimpleCircuitBreaker) canExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Проверяем, следует ли перейти в полуустановленное состояние
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

// recordResult записывает результат выполнения
func (cb *SimpleCircuitBreaker) recordResult(resp *http.Response, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isSuccess := cb.isSuccess(resp, err)

	if !isSuccess {
		cb.lastFailResponse = cb.cloneHttpResponse(resp)
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

// isSuccess определяет, считается ли комбинация ответа/ошибки успешной
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
	return resp.StatusCode < 500
}

// setState изменяет состояние автоматического выключателя и вызывает callback, если он установлен
func (cb *SimpleCircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChangeCallback != nil && oldState != newState {
		cb.onStateChangeCallback(oldState, newState)
	}
}

// CircuitBreakerMiddleware оборачивает автоматический выключатель как middleware
type CircuitBreakerMiddleware struct {
	circuitBreaker CircuitBreaker
}

// NewCircuitBreakerMiddleware создает новый middleware для автоматического выключателя
func NewCircuitBreakerMiddleware(cb CircuitBreaker) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		circuitBreaker: cb,
	}
}

// Process реализует интерфейс Middleware
func (cbm *CircuitBreakerMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	return cbm.circuitBreaker.Execute(func() (*http.Response, error) {
		return next(req)
	})
}
