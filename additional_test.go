package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestClientPut(t *testing.T) {
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

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClientDelete(t *testing.T) {
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

	if resp.StatusCode != 204 {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestClientHead(t *testing.T) {
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

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Length") != "123" {
		t.Errorf("Expected Content-Length 123, got %s", resp.Header.Get("Content-Length"))
	}
}

func TestConfigurationError(t *testing.T) {
	err := NewConfigurationError("timeout", -1, "must be positive")
	expected := "configuration error in field 'timeout': must be positive (value: -1)"

	if err.Error() != expected {
		t.Errorf("Expected %s, got %s", expected, err.Error())
	}
}

func TestClientWithTracingEnabled(t *testing.T) {
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

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestClientWithCustomTransport(t *testing.T) {
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
	if string(body) != "custom response" {
		t.Errorf("Expected 'custom response', got %s", string(body))
	}
}

func TestClientWithMaxResponseBytes(t *testing.T) {
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
	if len(body) == 0 {
		t.Error("Response body should not be empty")
	}
}

func TestClientWithRetryEnabled(t *testing.T) {
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

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 after retry, got %d", resp.StatusCode)
	}

	if server.GetRequestCount() != 2 {
		t.Errorf("Expected 2 requests, got %d", server.GetRequestCount())
	}
}

func TestRetryConfigValidation(t *testing.T) {
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
			result := tt.config.withDefaults()

			// Basic validation that defaults are applied
			if result.MaxAttempts == 0 {
				t.Error("MaxAttempts should have default value")
			}
			if result.BaseDelay == 0 {
				t.Error("BaseDelay should have default value")
			}
		})
	}
}

func TestTracerMethods(t *testing.T) {
	tracer := NewTracer()

	ctx := context.Background()
	newCtx, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	// Test SpanFromContext
	retrievedSpan := tracer.SpanFromContext(newCtx)
	if retrievedSpan == nil {
		t.Error("SpanFromContext returned nil")
	}
}
