package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Additional tests to push coverage over 75%

func TestBackoffCalculationLogic(t *testing.T) {
	// Test general backoff behavior without relying on internal functions
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   50 * time.Millisecond,
			MaxDelay:    500 * time.Millisecond,
			Jitter:      0.1,
		},
	}

	client := New(config, "test-backoff")
	defer client.Close()

	server := NewTestServer()
	server.AddResponse(TestResponse{StatusCode: 500})
	server.AddResponse(TestResponse{StatusCode: 500})
	server.AddResponse(TestResponse{StatusCode: 200, Body: "success"})
	defer server.Close()

	start := time.Now()
	resp, err := client.Get(context.Background(), server.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected final status 200, got %d", resp.StatusCode)
	}

	// Should have taken some time due to backoff delays
	if elapsed < 50*time.Millisecond {
		t.Errorf("Expected some delay due to backoff, got %v", elapsed)
	}

	if server.GetRequestCount() != 3 {
		t.Errorf("Expected 3 requests, got %d", server.GetRequestCount())
	}
}

func TestClientDoWithContext(t *testing.T) {
	server := NewTestServer(
		TestResponse{StatusCode: 200, Body: "OK"},
	)
	defer server.Close()

	client := New(Config{}, "test-do-context")
	defer client.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestConfigWithCustomRetryMethods(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			RetryMethods: []string{"POST", "PATCH"},
		},
	}

	result := config.withDefaults()

	if !result.RetryEnabled {
		t.Error("RetryEnabled should be true")
	}

	if len(result.RetryConfig.RetryMethods) != 2 {
		t.Errorf("Expected 2 retry methods, got %d", len(result.RetryConfig.RetryMethods))
	}
}

func TestConfigWithCustomRetryStatusCodes(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			RetryStatusCodes: []int{500, 502, 503},
		},
	}

	result := config.withDefaults()

	if len(result.RetryConfig.RetryStatusCodes) != 3 {
		t.Errorf("Expected 3 retry status codes, got %d", len(result.RetryConfig.RetryStatusCodes))
	}
}

func TestConfigWithRespectRetryAfter(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			RespectRetryAfter: true,
		},
	}

	result := config.withDefaults()

	if !result.RetryConfig.RespectRetryAfter {
		t.Error("RespectRetryAfter should be true")
	}
}

func TestHTTPErrorWithBody(t *testing.T) {
	httpErr := &HTTPError{
		StatusCode: 400,
		Status:     "Bad Request",
		Method:     "POST",
		URL:        "https://api.example.com/users",
		Body:       []byte(`{"error": "validation failed"}`),
	}

	expected := "HTTP 400 Bad Request: POST https://api.example.com/users"
	if httpErr.Error() != expected {
		t.Errorf("Expected %s, got %s", expected, httpErr.Error())
	}

	if len(httpErr.Body) == 0 {
		t.Error("Body should not be empty")
	}
}

func TestMaxAttemptsExceededErrorWithBothErrorAndStatus(t *testing.T) {
	err := &MaxAttemptsExceededError{
		MaxAttempts: 3,
		LastError:   fmt.Errorf("network timeout"),
		LastStatus:  502,
	}

	// When both LastError and LastStatus are present, LastError takes precedence
	expected := "max attempts (3) exceeded, last error: network timeout"
	if err.Error() != expected {
		t.Errorf("Expected %s, got %s", expected, err.Error())
	}
}

func TestClientWithAllMethodsAndHeaders(t *testing.T) {
	server := NewTestServer(
		TestResponse{StatusCode: 200, Body: "OK"},
	)
	defer server.Close()

	client := New(Config{}, "test-all-methods")
	defer client.Close()

	// Test GET with custom headers
	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	req.Header.Set("User-Agent", "test-client/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Test POST with body and headers
	req, _ = http.NewRequestWithContext(
		context.Background(),
		"POST",
		server.URL,
		strings.NewReader(`{"test": "data"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestMockRoundTripperEdgeCases(t *testing.T) {
	mock := NewMockRoundTripper()

	// Test with no responses configured
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := mock.RoundTrip(req)

	if err == nil || resp != nil {
		t.Error("Expected error when no responses configured")
	}

	// Test adding multiple responses
	mock.AddResponse(&http.Response{StatusCode: 200})
	mock.AddResponse(&http.Response{StatusCode: 201})
	mock.AddResponse(&http.Response{StatusCode: 202})

	// First call
	resp, err = mock.RoundTrip(req)
	if err != nil || resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Second call
	resp, err = mock.RoundTrip(req)
	if err != nil || resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Third call
	resp, err = mock.RoundTrip(req)
	if err != nil || resp.StatusCode != 202 {
		t.Errorf("Expected status 202, got %d", resp.StatusCode)
	}

	// Test call count
	if mock.GetCallCount() != 3 {
		t.Errorf("Expected 3 calls, got %d", mock.GetCallCount())
	}

	// Test requests tracking
	requests := mock.GetRequests()
	if len(requests) != 3 {
		t.Errorf("Expected 3 tracked requests, got %d", len(requests))
	}
}

func TestTestServerEdgeCases(t *testing.T) {
	server := NewTestServer()

	// Add multiple responses
	server.AddResponse(TestResponse{StatusCode: 200, Body: "first"})
	server.AddResponse(TestResponse{StatusCode: 201, Body: "second"})

	client := New(Config{}, "test-server-edge")
	defer client.Close()

	// First request
	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Second request
	resp, err = client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Check request count
	if server.GetRequestCount() != 2 {
		t.Errorf("Expected 2 requests, got %d", server.GetRequestCount())
	}

	// Get last request
	lastReq := server.GetLastRequest()
	if lastReq == nil {
		t.Error("GetLastRequest should not return nil")
	}

	server.Close()
}
