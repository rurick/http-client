package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Constants for tests.
const (
	// Polling interval for WaitForCondition.
	pollIntervalMs = 10
)

// TestServer provides a mock HTTP server for testing.
type TestServer struct {
	*httptest.Server
	mu              sync.RWMutex
	responses       []TestResponse
	currentResponse int
	RequestLog      []TestRequest
}

// TestResponse describes a test server response.
type TestResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	Delay      time.Duration
}

// TestRequest logs request information.
type TestRequest struct {
	Method     string
	URL        string
	Headers    map[string]string
	Body       string
	Timestamp  time.Time
	RemoteAddr string
}

// NewTestServer creates a new test server.
func NewTestServer(responses ...TestResponse) *TestServer {
	ts := &TestServer{
		responses:  responses,
		RequestLog: make([]TestRequest, 0),
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

// NewTestServerTLS creates a new test HTTPS server.
func NewTestServerTLS(responses ...TestResponse) *TestServer {
	ts := &TestServer{
		responses:  responses,
		RequestLog: make([]TestRequest, 0),
	}

	ts.Server = httptest.NewTLSServer(http.HandlerFunc(ts.handler))
	return ts
}

// handler handles HTTP requests.
func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Log request
	bodyBytes := make([]byte, 0)
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		defer r.Body.Close() // Use defer for reliable closing
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

	// Get current response
	if len(ts.responses) == 0 {
		// Default response if not configured
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status": "ok"}`)); err != nil {
			// In test server write error is critical
			// Log it, but can't handle it within handler
			return
		}
		return
	}

	responseIndex := ts.currentResponse
	if responseIndex >= len(ts.responses) {
		responseIndex = len(ts.responses) - 1 // use last response
	}

	response := ts.responses[responseIndex]
	ts.currentResponse++

	// Add delay if specified
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}

	// Set headers
	for k, v := range response.Headers {
		w.Header().Set(k, v)
	}

	// Set status code
	w.WriteHeader(response.StatusCode)

	// Send response body
	if response.Body != "" {
		if _, err := w.Write([]byte(response.Body)); err != nil {
			// In test server write error is critical
			// Log it, but can't handle it within handler
			return
		}
	}
}

// Reset resets server state.
func (ts *TestServer) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.currentResponse = 0
	ts.responses = ts.responses[:0]
	ts.RequestLog = ts.RequestLog[:0]
}

// GetRequestCount returns the number of received requests.
func (ts *TestServer) GetRequestCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.RequestLog)
}

// GetLastRequest returns the last received request.
func (ts *TestServer) GetLastRequest() *TestRequest {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if len(ts.RequestLog) == 0 {
		return nil
	}
	return &ts.RequestLog[len(ts.RequestLog)-1]
}

// AddResponse adds a new response to the queue.
func (ts *TestServer) AddResponse(response TestResponse) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.responses = append(ts.responses, response)
}

// MockRoundTripper provides a mock RoundTripper for unit tests.
type MockRoundTripper struct {
	mu        sync.RWMutex
	responses []*http.Response
	errors    []error
	callCount int
	requests  []*http.Request
}

// NewMockRoundTripper creates a new mock RoundTripper.
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		responses: make([]*http.Response, 0),
		errors:    make([]error, 0),
		requests:  make([]*http.Request, 0),
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Save request for verification
	m.requests = append(m.requests, req)

	defer func() { m.callCount++ }()

	// Check if there's an error for this call
	if m.callCount < len(m.errors) && m.errors[m.callCount] != nil {
		return nil, m.errors[m.callCount]
	}

	// Check if there's a response for this call
	if m.callCount < len(m.responses) && m.responses[m.callCount] != nil {
		return m.responses[m.callCount], nil
	}

	// Default response
	const defaultOKStatus = 200
	return &http.Response{
		StatusCode: defaultOKStatus,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"status": "ok"}`)),
		Request:    req,
	}, nil
}

// AddResponse adds a mock response.
func (m *MockRoundTripper) AddResponse(resp *http.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// For test responses with body, ensure they will be properly handled.
	// In tests body should be managed by user, so don't close automatically.
	// But add response validity check.
	if resp == nil {
		return
	}

	// In test environment add response as is.
	// Note: if response.Body is not nil, it's assumed to be NopCloser
	// or another ReadCloser that's safe to use multiple times in tests.
	// To prevent linter warnings about unclosed body,
	// we clone body if necessary.
	if resp.Body != nil {
		// Read body for safe cloning
		bodyBytes, err := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			// In test environment log warning, but don't interrupt execution
			log.Printf("Warning: failed to close test response body: %v", closeErr)
		}

		// Restore body for use in mock
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		} else {
			// If failed to read, create empty body
			resp.Body = io.NopCloser(strings.NewReader(""))
		}
	}

	m.responses = append(m.responses, resp) //nolint:bodyclose // body is safely handled in mock context
}

// AddError adds an error for the next call.
func (m *MockRoundTripper) AddError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, err)
}

// GetCallCount returns the number of RoundTrip calls.
func (m *MockRoundTripper) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

// GetRequests returns all received requests.
func (m *MockRoundTripper) GetRequests() []*http.Request {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*http.Request(nil), m.requests...)
}

// Reset resets mock state.
func (m *MockRoundTripper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = m.responses[:0]
	m.errors = m.errors[:0]
	m.requests = m.requests[:0]
	m.callCount = 0
}

// MetricsCollector for testing metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]interface{}
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]interface{}),
	}
}

// Record records a metric.
func (mc *MetricsCollector) Record(name string, value interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics[name] = value
}

// Get returns the metric value.
func (mc *MetricsCollector) Get(name string) interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.metrics[name]
}

// GetAll returns all metrics.
func (mc *MetricsCollector) GetAll() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}

// Reset resets all metrics.
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make(map[string]interface{})
}

// CreateTestHTTPResponse creates an HTTP response for tests.
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

// WaitForCondition waits for a condition to be met with timeout.
func WaitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(pollIntervalMs * time.Millisecond)
	}
	return false
}

// AssertEventuallyTrue checks that a condition becomes true within timeout.
func AssertEventuallyTrue(t testing.TB, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	if !WaitForCondition(timeout, condition) {
		t.Fatalf("Condition was not met within %v: %s", timeout, message)
	}
}
