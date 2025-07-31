package mock

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

// TestServer provides utilities for testing HTTP clients
type TestServer struct {
	*httptest.Server
	requests []*http.Request
	mu       sync.Mutex
}

// NewTestServer creates a new test server
func NewTestServer(handler http.HandlerFunc) *TestServer {
	ts := &TestServer{}

	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		ts.requests = append(ts.requests, r)
		ts.mu.Unlock()

		if handler != nil {
			handler(w, r)
		}
	}))

	return ts
}

// GetRequests returns all recorded requests
func (ts *TestServer) GetRequests() []*http.Request {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	requests := make([]*http.Request, len(ts.requests))
	copy(requests, ts.requests)
	return requests
}

// GetLastRequest returns the last recorded request
func (ts *TestServer) GetLastRequest() *http.Request {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if len(ts.requests) == 0 {
		return nil
	}
	return ts.requests[len(ts.requests)-1]
}

// Reset clears all recorded requests
func (ts *TestServer) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.requests = nil
}

// ClientTestSuite provides common test utilities for HTTP clients
type ClientTestSuite struct {
	Client httpclient.ExtendedHTTPClient
	Server *TestServer
	T      *testing.T
}

// NewClientTestSuite creates a new client test suite
func NewClientTestSuite(t *testing.T, client httpclient.ExtendedHTTPClient) *ClientTestSuite {
	return &ClientTestSuite{
		Client: client,
		T:      t,
	}
}

// WithTestServer sets up a test server for the suite
func (cts *ClientTestSuite) WithTestServer(handler http.HandlerFunc) *ClientTestSuite {
	cts.Server = NewTestServer(handler)
	return cts
}

// AssertRequestCount asserts the number of requests made
func (cts *ClientTestSuite) AssertRequestCount(expected int) {
	if cts.Server != nil {
		requests := cts.Server.GetRequests()
		assert.Len(cts.T, requests, expected, "Expected %d requests, got %d", expected, len(requests))
	}
}

// AssertLastRequestMethod asserts the method of the last request
func (cts *ClientTestSuite) AssertLastRequestMethod(expectedMethod string) {
	if cts.Server != nil {
		lastReq := cts.Server.GetLastRequest()
		require.NotNil(cts.T, lastReq, "No requests recorded")
		assert.Equal(cts.T, expectedMethod, lastReq.Method, "Expected method %s, got %s", expectedMethod, lastReq.Method)
	}
}

// AssertLastRequestHeader asserts a header value in the last request
func (cts *ClientTestSuite) AssertLastRequestHeader(headerName, expectedValue string) {
	if cts.Server != nil {
		lastReq := cts.Server.GetLastRequest()
		require.NotNil(cts.T, lastReq, "No requests recorded")
		actualValue := lastReq.Header.Get(headerName)
		assert.Equal(cts.T, expectedValue, actualValue, "Expected header %s: %s, got %s", headerName, expectedValue, actualValue)
	}
}

// AssertLastRequestBody asserts the body content of the last request
func (cts *ClientTestSuite) AssertLastRequestBody(expectedBody string) {
	if cts.Server != nil {
		lastReq := cts.Server.GetLastRequest()
		require.NotNil(cts.T, lastReq, "No requests recorded")

		if lastReq.Body != nil {
			bodyBytes, err := io.ReadAll(lastReq.Body)
			require.NoError(cts.T, err, "Failed to read request body")
			actualBody := string(bodyBytes)
			assert.Equal(cts.T, expectedBody, actualBody, "Expected body %s, got %s", expectedBody, actualBody)
		} else {
			assert.Empty(cts.T, expectedBody, "Expected empty body but got %s", expectedBody)
		}
	}
}

// Cleanup cleans up test resources
func (cts *ClientTestSuite) Cleanup() {
	if cts.Server != nil {
		cts.Server.Close()
	}
}

// MockResponseBuilder helps build mock HTTP responses
type MockResponseBuilder struct {
	statusCode int
	headers    http.Header
	body       []byte
}

// NewMockResponse creates a new mock response builder
func NewMockResponse() *MockResponseBuilder {
	return &MockResponseBuilder{
		statusCode: http.StatusOK,
		headers:    make(http.Header),
	}
}

// WithStatus sets the status code
func (mrb *MockResponseBuilder) WithStatus(statusCode int) *MockResponseBuilder {
	mrb.statusCode = statusCode
	return mrb
}

// WithHeader adds a header
func (mrb *MockResponseBuilder) WithHeader(key, value string) *MockResponseBuilder {
	mrb.headers.Set(key, value)
	return mrb
}

// WithBody sets the response body
func (mrb *MockResponseBuilder) WithBody(body string) *MockResponseBuilder {
	mrb.body = []byte(body)
	return mrb
}

// WithJSONBody sets a JSON response body
func (mrb *MockResponseBuilder) WithJSONBody(data any) *MockResponseBuilder {
	jsonBytes, _ := json.Marshal(data)
	mrb.body = jsonBytes
	mrb.headers.Set("Content-Type", "application/json")
	return mrb
}

// WithXMLBody sets an XML response body
func (mrb *MockResponseBuilder) WithXMLBody(data any) *MockResponseBuilder {
	xmlBytes, _ := xml.Marshal(data)
	mrb.body = xmlBytes
	mrb.headers.Set("Content-Type", "application/xml")
	return mrb
}

// Build creates the HTTP response
func (mrb *MockResponseBuilder) Build() *http.Response {
	return &http.Response{
		StatusCode:    mrb.statusCode,
		Status:        http.StatusText(mrb.statusCode),
		Header:        mrb.headers,
		Body:          io.NopCloser(bytes.NewReader(mrb.body)),
		ContentLength: int64(len(mrb.body)),
	}
}

// RetryTestHelper provides utilities for testing retry behavior
type RetryTestHelper struct {
	attemptCount int
	responses    []*http.Response
	errors       []error
	delays       []time.Duration
}

// NewRetryTestHelper creates a new retry test helper
func NewRetryTestHelper() *RetryTestHelper {
	return &RetryTestHelper{}
}

// AddAttempt adds an attempt with response and error
func (rth *RetryTestHelper) AddAttempt(resp *http.Response, err error, delay time.Duration) {
	rth.responses = append(rth.responses, resp)
	rth.errors = append(rth.errors, err)
	rth.delays = append(rth.delays, delay)
}

// Handler returns an HTTP handler that simulates retry behavior
func (rth *RetryTestHelper) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rth.attemptCount < len(rth.responses) {
			resp := rth.responses[rth.attemptCount]
			err := rth.errors[rth.attemptCount]
			delay := rth.delays[rth.attemptCount]

			// Simulate delay
			if delay > 0 {
				time.Sleep(delay)
			}

			// Simulate error by closing connection
			if err != nil {
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, _ := hj.Hijack()
					conn.Close()
					return
				}
			}

			// Send response
			if resp != nil {
				for key, values := range resp.Header {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(resp.StatusCode)
				if resp.Body != nil {
					io.Copy(w, resp.Body)
				}
			}

			rth.attemptCount++
		}
	}
}

// GetAttemptCount returns the number of attempts made
func (rth *RetryTestHelper) GetAttemptCount() int {
	return rth.attemptCount
}

// Reset resets the attempt counter
func (rth *RetryTestHelper) Reset() {
	rth.attemptCount = 0
}

// CircuitBreakerTestHelper provides utilities for testing circuit breaker behavior
type CircuitBreakerTestHelper struct {
	failureCount int
	successCount int
	state        httpclient.CircuitBreakerState
}

// NewCircuitBreakerTestHelper creates a new circuit breaker test helper
func NewCircuitBreakerTestHelper() *CircuitBreakerTestHelper {
	return &CircuitBreakerTestHelper{
		state: httpclient.CircuitBreakerClosed,
	}
}

// SimulateFailures simulates a number of failures
func (cbth *CircuitBreakerTestHelper) SimulateFailures(count int) {
	cbth.failureCount += count
}

// SimulateSuccesses simulates a number of successes
func (cbth *CircuitBreakerTestHelper) SimulateSuccesses(count int) {
	cbth.successCount += count
}

// SetState sets the circuit breaker state
func (cbth *CircuitBreakerTestHelper) SetState(state httpclient.CircuitBreakerState) {
	cbth.state = state
}

// GetFailureCount returns the failure count
func (cbth *CircuitBreakerTestHelper) GetFailureCount() int {
	return cbth.failureCount
}

// GetSuccessCount returns the success count
func (cbth *CircuitBreakerTestHelper) GetSuccessCount() int {
	return cbth.successCount
}

// GetState returns the current state
func (cbth *CircuitBreakerTestHelper) GetState() httpclient.CircuitBreakerState {
	return cbth.state
}
