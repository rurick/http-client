package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_New(t *testing.T) {
	config := Config{
		Timeout:       10 * time.Second,
		PerTryTimeout: 3 * time.Second,
	}

	client := New(config, "test-client")

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.config.Timeout != 10*time.Second {
		t.Errorf("expected timeout to be 10s, got %v", client.config.Timeout)
	}

	if client.config.PerTryTimeout != 3*time.Second {
		t.Errorf("expected per-try timeout to be 3s, got %v", client.config.PerTryTimeout)
	}
}

func TestClient_NewWithDefaults(t *testing.T) {
	client := New(Config{}, "test-client")

	if client.config.Timeout != 5*time.Second {
		t.Errorf("expected default timeout to be 5s, got %v", client.config.Timeout)
	}

	if client.config.PerTryTimeout != 2*time.Second {
		t.Errorf("expected default per-try timeout to be 2s, got %v", client.config.PerTryTimeout)
	}

	if client.config.RetryEnabled {
		t.Error("expected retry to be disabled by default")
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	client := New(Config{}, "test-client")

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != "test response" {
		t.Errorf("expected 'test response', got %s", string(body))
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		if string(body) != "test data" {
			t.Errorf("expected 'test data', got %s", string(body))
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))
	defer server.Close()

	client := New(Config{}, "test-client")

	ctx := context.Background()
	resp, err := client.Post(ctx, server.URL, "application/json", strings.NewReader("test data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestClient_WithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			RetryMethods:      []string{http.MethodGet},
			RetryStatusCodes:  []int{500},
			RespectRetryAfter: false,
		},
	}

	client := New(config, "test-client")

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(Config{}, "test-client")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)
	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}

	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestClient_Close(t *testing.T) {
	client := New(Config{}, "test-client")

	err := client.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

func TestClient_GetConfig(t *testing.T) {
	originalConfig := Config{
		Timeout:       10 * time.Second,
		PerTryTimeout: 3 * time.Second,
		RetryEnabled:  true,
	}

	client := New(originalConfig, "test-client")
	retrievedConfig := client.GetConfig()

	if retrievedConfig.Timeout != originalConfig.Timeout {
		t.Errorf("expected timeout %v, got %v", originalConfig.Timeout, retrievedConfig.Timeout)
	}

	if retrievedConfig.RetryEnabled != originalConfig.RetryEnabled {
		t.Errorf("expected retry enabled %v, got %v", originalConfig.RetryEnabled, retrievedConfig.RetryEnabled)
	}
}
