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

// Additional tests to boost coverage to 75%+

func TestClientPost(t *testing.T) {
	server := NewTestServer(
		TestResponse{
			StatusCode: 201,
			Body:       `{"id": 123}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		},
	)
	defer server.Close()

	client := New(Config{}, "test-post")
	defer client.Close()

	body := strings.NewReader(`{"name": "test"}`)
	resp, err := client.Post(context.Background(), server.URL, "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 201, "Expected status code 201")
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

	client := New(Config{}, "test-put")
	defer client.Close()

	body := strings.NewReader(`{"name": "updated"}`)
	resp, err := client.Put(context.Background(), server.URL, "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 200, "Expected status code 200")
}

func TestClientDelete(t *testing.T) {
	t.Parallel()
	server := NewTestServer(
		TestResponse{StatusCode: 204},
	)
	defer server.Close()

	client := New(Config{}, "test-delete")
	defer client.Close()

	resp, err := client.Delete(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 204, "Expected status code 204")
}

func TestClientHead(t *testing.T) {
	t.Parallel()
	server := NewTestServer(
		TestResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Length": "123"},
		},
	)
	defer server.Close()

	client := New(Config{}, "test-head")
	defer client.Close()

	resp, err := client.Head(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 200, "Expected status code 200")
	assert.Equal(t, resp.ContentLength, int64(123), "Expected Content-Length 123")
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
	client := New(config, "test-tracing")
	defer client.Close()

	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 200, "Expected status code 200")
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
	client := New(config, "test-custom-transport")
	defer client.Close()

	resp, err := client.Get(context.Background(), "http://example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, string(body), "custom response", "Expected 'custom response'")
}

func TestClientWithMaxResponseBytes(t *testing.T) {
	t.Parallel()
	maxBytes := int64(100)
	config := Config{
		MaxResponseBytes: &maxBytes,
	}

	server := NewTestServer(
		TestResponse{
			StatusCode: 200,
			Body:       "this is a short response",
		},
	)
	defer server.Close()

	client := New(config, "test-max-bytes")
	defer client.Close()

	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// This test just validates the config is applied
	body, _ := io.ReadAll(resp.Body)
	assert.NotEqual(t, 0, len(body), "Response body should not be empty")
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

	client := New(config, "test-retry-enabled")
	defer client.Close()

	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, 200, "Expected status code 200 after retry")
	assert.Equal(t, server.GetRequestCount(), 2, "Expected 2 requests")
}

func TestRetryConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config RetryConfig
	}{
		{
			name: "with retry methods",
			config: RetryConfig{
				RetryMethods: []string{"GET", "PUT", "DELETE"},
			},
		},
		{
			name: "with retry status codes",
			config: RetryConfig{
				RetryStatusCodes: []int{500, 502, 503},
			},
		},
		{
			name: "with respect retry after",
			config: RetryConfig{
				RespectRetryAfter: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.config.withDefaults()

			// Basic validation that defaults are applied
			assert.NotEqual(t, result.MaxAttempts, 0, "MaxAttempts should have default value")
			assert.NotEqual(t, result.BaseDelay, 0, "BaseDelay should have default value")
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
