package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestServer предоставляет моковый HTTP сервер для тестирования
type TestServer struct {
	*httptest.Server
	mu              sync.RWMutex
	responses       []TestResponse
	currentResponse int
	RequestLog      []TestRequest
}

// TestResponse описывает ответ тестового сервера
type TestResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	Delay      time.Duration
}

// TestRequest логирует информацию о запросе
type TestRequest struct {
	Method     string
	URL        string
	Headers    map[string]string
	Body       string
	Timestamp  time.Time
	RemoteAddr string
}

// NewTestServer создаёт новый тестовый сервер
func NewTestServer(responses ...TestResponse) *TestServer {
	ts := &TestServer{
		responses:  responses,
		RequestLog: make([]TestRequest, 0),
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

// NewTestServerTLS создаёт новый тестовый HTTPS сервер
func NewTestServerTLS(responses ...TestResponse) *TestServer {
	ts := &TestServer{
		responses:  responses,
		RequestLog: make([]TestRequest, 0),
	}

	ts.Server = httptest.NewTLSServer(http.HandlerFunc(ts.handler))
	return ts
}

// handler обрабатывает HTTP запросы
func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Логируем запрос
	bodyBytes := make([]byte, 0)
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	ts.RequestLog = append(ts.RequestLog, TestRequest{
		Method:     r.Method,
		URL:        r.URL.String(),
		Headers:    headers,
		Body:       string(bodyBytes),
		Timestamp:  time.Now(),
		RemoteAddr: r.RemoteAddr,
	})

	// Получаем текущий ответ
	if len(ts.responses) == 0 {
		// Дефолтный ответ если не настроен
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
		return
	}

	responseIndex := ts.currentResponse
	if responseIndex >= len(ts.responses) {
		responseIndex = len(ts.responses) - 1 // используем последний ответ
	}

	response := ts.responses[responseIndex]
	ts.currentResponse++

	// Добавляем задержку если указана
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}

	// Устанавливаем заголовки
	for k, v := range response.Headers {
		w.Header().Set(k, v)
	}

	// Устанавливаем статус код
	w.WriteHeader(response.StatusCode)

	// Отправляем тело ответа
	if response.Body != "" {
		w.Write([]byte(response.Body))
	}
}

// Reset сбрасывает состояние сервера
func (ts *TestServer) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.currentResponse = 0
	ts.RequestLog = ts.RequestLog[:0]
}

// GetRequestCount возвращает количество полученных запросов
func (ts *TestServer) GetRequestCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.RequestLog)
}

// GetLastRequest возвращает последний полученный запрос
func (ts *TestServer) GetLastRequest() *TestRequest {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if len(ts.RequestLog) == 0 {
		return nil
	}
	return &ts.RequestLog[len(ts.RequestLog)-1]
}

// AddResponse добавляет новый ответ в очередь
func (ts *TestServer) AddResponse(response TestResponse) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.responses = append(ts.responses, response)
}

// MockRoundTripper предоставляет моковый RoundTripper для unit тестов
type MockRoundTripper struct {
	mu        sync.RWMutex
	responses []*http.Response
	errors    []error
	callCount int
	requests  []*http.Request
}

// NewMockRoundTripper создаёт новый моковый RoundTripper
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		responses: make([]*http.Response, 0),
		errors:    make([]error, 0),
		requests:  make([]*http.Request, 0),
	}
}

// RoundTrip реализует http.RoundTripper интерфейс
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Сохраняем запрос для проверки
	m.requests = append(m.requests, req)

	defer func() { m.callCount++ }()

	// Проверяем есть ли ошибка для этого вызова
	if m.callCount < len(m.errors) && m.errors[m.callCount] != nil {
		return nil, m.errors[m.callCount]
	}

	// Проверяем есть ли ответ для этого вызова
	if m.callCount < len(m.responses) && m.responses[m.callCount] != nil {
		return m.responses[m.callCount], nil
	}

	// Дефолтный ответ
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"status": "ok"}`)),
		Request:    req,
	}, nil
}

// AddResponse добавляет моковый ответ
func (m *MockRoundTripper) AddResponse(resp *http.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, resp)
}

// AddError добавляет ошибку для следующего вызова
func (m *MockRoundTripper) AddError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, err)
}

// GetCallCount возвращает количество вызовов RoundTrip
func (m *MockRoundTripper) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

// GetRequests возвращает все полученные запросы
func (m *MockRoundTripper) GetRequests() []*http.Request {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*http.Request(nil), m.requests...)
}

// Reset сбрасывает состояние мока
func (m *MockRoundTripper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = m.responses[:0]
	m.errors = m.errors[:0]
	m.requests = m.requests[:0]
	m.callCount = 0
}

// MetricsCollector для тестирования метрик
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]interface{}
}

// NewMetricsCollector создаёт новый коллектор метрик
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]interface{}),
	}
}

// Record записывает метрику
func (mc *MetricsCollector) Record(name string, value interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics[name] = value
}

// Get возвращает значение метрики
func (mc *MetricsCollector) Get(name string) interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.metrics[name]
}

// GetAll возвращает все метрики
func (mc *MetricsCollector) GetAll() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}

// Reset сбрасывает все метрики
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make(map[string]interface{})
}

// CreateTestHTTPResponse создаёт HTTP ответ для тестов
func CreateTestHTTPResponse(statusCode int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	for k, v := range headers {
		resp.Header.Set(k, v)
	}

	return resp
}

// WaitForCondition ожидает выполнения условия с таймаутом
func WaitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// AssertEventuallyTrue проверяет что условие станет истинным в течение таймаута
func AssertEventuallyTrue(t testing.TB, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	if !WaitForCondition(timeout, condition) {
		t.Fatalf("Condition was not met within %v: %s", timeout, message)
	}
}
