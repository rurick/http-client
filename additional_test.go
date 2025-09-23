package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Хелпер для создания клиентского объекта
func newTestClient(t *testing.T, config Config, name string) *Client {
	client := New(config, name)
	t.Cleanup(func() {
		client.Close()
	})
	return client
}

// Хелпер для выполнения HTTP-запроса и проверки базовых условий
func executeRequestAndCheckStatus(
	t *testing.T,
	client *Client,
	method,
	url string,
	headers map[string]string,
	body io.Reader) *http.Response {
	resp, err := callHTTPMethod(client, method, url, headers, body)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.NotNil(t, resp, "Response should not be nil")
	return resp
}

// Хелпер для вызова методов HTTP-клиента
func callHTTPMethod(client *Client, method, url string, headers map[string]string, body io.Reader) (*http.Response, error) {
	switch method {
	case "HEAD":
		return client.Head(context.Background(), url)
	case "GET":
		return client.Get(context.Background(), url)
	case "POST":
		return client.Post(context.Background(), url, body, WithContentType(headers["Content-Type"]))
	case "PUT":
		return client.Put(context.Background(), url, body, WithContentType(headers["Content-Type"]))
	case "DELETE":
		return client.Delete(context.Background(), url)
	default:
		return nil, nil
	}
}

// Хелпер для проверки содержимого тела ответа
func assertResponseBody(t *testing.T, resp *http.Response, expected string) {
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, expected, string(body), "Response body mismatch")
}

// Основные тесты

func TestClientPost(t *testing.T) {
	server := NewTestServer(
		TestResponse{
			StatusCode: 201,
			Body:       `{"id": 123}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		},
	)
	defer server.Close()

	client := newTestClient(t, Config{}, "test-post")

	body := strings.NewReader(`{"name": "test"}`)
	resp := executeRequestAndCheckStatus(t, client, "POST", server.URL, map[string]string{"Content-Type": "application/json"}, body)

	assert.Equal(t, 201, resp.StatusCode, "Expected status code 201")
	assertResponseBody(t, resp, `{"id": 123}`)
}

func TestClientPut(t *testing.T) {
	t.Parallel()
	server := NewTestServer(
		TestResponse{
			StatusCode: 200,
			Body:       `{"updated": true}`,
		},
	)
	defer server.Close()

	client := newTestClient(t, Config{}, "test-put")

	body := strings.NewReader(`{"name": "updated"}`)
	resp := executeRequestAndCheckStatus(t, client, "PUT", server.URL, map[string]string{"Content-Type": "application/json"}, body)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assertResponseBody(t, resp, `{"updated": true}`)
}

func TestClientDelete(t *testing.T) {
	t.Parallel()
	server := NewTestServer(
		TestResponse{StatusCode: 204},
	)
	defer server.Close()

	client := newTestClient(t, Config{}, "test-delete")

	resp := executeRequestAndCheckStatus(t, client, "DELETE", server.URL, nil, nil)

	assert.Equal(t, 204, resp.StatusCode, "Expected status code 204")
}

func TestClientHead(t *testing.T) {
	t.Parallel()
	testResp := TestResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Length": "123"},
	}
	server := NewTestServer(
		testResp,
	)
	defer server.Close()

	client := newTestClient(t, Config{}, "test-head")

	resp := executeRequestAndCheckStatus(t, client, "HEAD", server.URL, testResp.Headers, nil)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assert.Equal(t, int64(123), resp.ContentLength, "Expected Content-Length 123")
}

func TestConfigurationError(t *testing.T) {
	t.Parallel()
	err := NewConfigurationError("timeout", -1, "must be positive")
	expected := "configuration error in field 'timeout': must be positive (value: -1)"
	assert.Equal(t, expected, err.Error())
}

func TestClientWithTracingEnabled(t *testing.T) {
	t.Parallel()
	server := NewTestServer(
		TestResponse{StatusCode: 200, Body: "OK"},
	)
	defer server.Close()

	config := Config{
		TracingEnabled: true,
	}
	client := newTestClient(t, config, "test-tracing")

	resp := executeRequestAndCheckStatus(t, client, "GET", server.URL, nil, nil)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assertResponseBody(t, resp, "OK")
}

func TestClientWithCustomTransport(t *testing.T) {
	t.Parallel()
	mock := NewMockRoundTripper()
	mock.AddResponse(&http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("custom response"))),
	})

	config := Config{
		Transport: mock,
	}
	client := newTestClient(t, config, "test-custom-transport")

	resp := executeRequestAndCheckStatus(t, client, "GET", "http://example.com", nil, nil)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assertResponseBody(t, resp, "custom response")
}

func TestClientWithMaxResponseBytes(t *testing.T) {
	t.Parallel()
	maxBytes := int64(10)
	config := Config{
		MaxResponseBytes: &maxBytes,
	}
	testResp := TestResponse{
		StatusCode: 200,
		Body:       "this is a short response",
	}
	server := NewTestServer(
		testResp,
	)
	defer server.Close()

	client := newTestClient(t, config, "test-max-bytes")

	resp := executeRequestAndCheckStatus(t, client, "GET", server.URL, nil, strings.NewReader(testResp.Body))

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assertResponseBody(t, resp, "this is a short response")

	// Проверяем, что тело ответа не превышает максимальный размер в 10 байт
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "Failed to read response body")
	assert.LessOrEqual(t, int64(len(body)), maxBytes, "Response body should not exceed max bytes limit")
}

func TestClientWithRetryEnabled(t *testing.T) {
	t.Parallel()
	server := NewTestServer()
	server.AddResponse(TestResponse{StatusCode: 500})
	server.AddResponse(TestResponse{StatusCode: 200, Body: "success"})
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   10 * time.Millisecond,
		},
	}
	client := newTestClient(t, config, "test-retry-enabled")

	resp := executeRequestAndCheckStatus(t, client, "GET", server.URL, nil, nil)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200 after retry")
	assert.Equal(t, server.GetRequestCount(), 2, "Expected 2 requests")
}

func TestRetryConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		config   RetryConfig
		expected RetryConfig
	}{
		{
			name: "with retry methods",
			config: RetryConfig{
				RetryMethods: []string{"GET", "PUT", "DELETE"},
			},
			expected: RetryConfig{
				RetryMethods:      []string{"GET", "PUT", "DELETE"},
				MaxAttempts:       3,                              // default
				BaseDelay:         100 * time.Millisecond,         // default
				MaxDelay:          2 * time.Second,                // default
				Jitter:            0.2,                            // default
				RetryStatusCodes:  []int{429, 500, 502, 503, 504}, // default
				RespectRetryAfter: true,                           // default
			},
		},
		{
			name: "with retry status codes",
			config: RetryConfig{
				RetryStatusCodes: []int{500, 502, 503},
			},
			expected: RetryConfig{
				RetryStatusCodes:  []int{500, 502, 503},
				MaxAttempts:       3,                                                                                                // default
				BaseDelay:         100 * time.Millisecond,                                                                           // default
				MaxDelay:          2 * time.Second,                                                                                  // default
				Jitter:            0.2,                                                                                              // default
				RetryMethods:      []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete}, // default
				RespectRetryAfter: true,                                                                                             // default
			},
		},
		{
			name: "with respect retry after",
			config: RetryConfig{
				RespectRetryAfter: true,
			},
			expected: RetryConfig{
				RespectRetryAfter: true,
				MaxAttempts:       3,                                                                                                // default
				BaseDelay:         100 * time.Millisecond,                                                                           // default
				MaxDelay:          2 * time.Second,                                                                                  // default
				Jitter:            0.2,                                                                                              // default
				RetryMethods:      []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete}, // default
				RetryStatusCodes:  []int{429, 500, 502, 503, 504},                                                                   // default
			},
		},
		{
			name: "custom max attempts and base delay",
			config: RetryConfig{
				MaxAttempts: 5,
				BaseDelay:   200 * time.Millisecond,
			},
			expected: RetryConfig{
				MaxAttempts:       5,
				BaseDelay:         200 * time.Millisecond,
				MaxDelay:          2 * time.Second,                                                                                  // default
				Jitter:            0.2,                                                                                              // default
				RetryMethods:      []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete}, // default
				RetryStatusCodes:  []int{429, 500, 502, 503, 504},                                                                   // default
				RespectRetryAfter: true,                                                                                             // default
			},
		},
		{
			name: "full custom configuration",
			config: RetryConfig{
				MaxAttempts:      4,
				BaseDelay:        150 * time.Millisecond,
				MaxDelay:         2 * time.Second,
				Jitter:           0.1,
				RetryMethods:     []string{"POST"},
				RetryStatusCodes: []int{429},
			},
			expected: RetryConfig{
				MaxAttempts:       4,
				BaseDelay:         150 * time.Millisecond,
				MaxDelay:          2 * time.Second,
				Jitter:            0.1,
				RetryMethods:      []string{"POST"},
				RetryStatusCodes:  []int{429},
				RespectRetryAfter: true, // default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.config.withDefaults()

			assert.Equal(t, tt.expected.MaxAttempts, result.MaxAttempts, "MaxAttempts mismatch")
			assert.Equal(t, tt.expected.BaseDelay, result.BaseDelay, "BaseDelay mismatch")
			assert.Equal(t, tt.expected.MaxDelay, result.MaxDelay, "MaxDelay mismatch")
			assert.Equal(t, tt.expected.Jitter, result.Jitter, "Jitter mismatch")
			assert.ElementsMatch(t, tt.expected.RetryMethods, result.RetryMethods, "RetryMethods mismatch")
			assert.ElementsMatch(t, tt.expected.RetryStatusCodes, result.RetryStatusCodes, "RetryStatusCodes mismatch")
			assert.Equal(t, tt.expected.RespectRetryAfter, result.RespectRetryAfter, "RespectRetryAfter mismatch")
		})
	}
}

func TestTracerMethods(t *testing.T) {
	t.Parallel()
	tracer := NewTracer()

	ctx := context.Background()
	newCtx, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	// Test SpanFromContext
	retrievedSpan := tracer.SpanFromContext(newCtx)
	assert.NotNil(t, retrievedSpan, "SpanFromContext should return a non-nil span")
}
