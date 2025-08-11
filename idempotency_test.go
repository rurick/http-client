package httpclient

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestIdempotencyRetryLogic(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			MaxDelay:         10 * time.Millisecond,
			RetryMethods:     []string{"GET", "HEAD", "PUT", "DELETE", "OPTIONS"},
			RetryStatusCodes: []int{500, 502, 503},
		},
	}

	ctx := context.Background()

	t.Run("GET_always_retryable", func(t *testing.T) {
		server := NewTestServer(
			TestResponse{StatusCode: 500, Body: "error"},
			TestResponse{StatusCode: 200, Body: "success"},
		)
		defer server.Close()

		client := New(config, "test-get")
		defer client.Close()

		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if server.GetRequestCount() != 2 {
			t.Errorf("expected 2 requests (1 failure + 1 retry), got %d", server.GetRequestCount())
		}
	})

	t.Run("POST_without_idempotency_key_not_retryable", func(t *testing.T) {
		server := NewTestServer(
			TestResponse{StatusCode: 500, Body: "error"},
			TestResponse{StatusCode: 200, Body: "success"},
		)
		defer server.Close()

		client := New(config, "test-post-no-key")
		defer client.Close()

		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
		}

		// POST без Idempotency-Key не должен повторяться, поэтому ожидаем ошибку или 500
		if server.GetRequestCount() != 1 {
			t.Errorf("expected 1 request (no retry for POST without Idempotency-Key), got %d", server.GetRequestCount())
		}
	})

	t.Run("POST_with_idempotency_key_retryable", func(t *testing.T) {
		server := NewTestServer(
			TestResponse{StatusCode: 503, Body: "service unavailable"},
			TestResponse{StatusCode: 201, Body: "created"},
		)
		defer server.Close()

		client := New(config, "test-post-with-key")
		defer client.Close()

		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "payment-operation-12345")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		if server.GetRequestCount() != 2 {
			t.Errorf("expected 2 requests (1 failure + 1 retry with Idempotency-Key), got %d", server.GetRequestCount())
		}
	})

	t.Run("PATCH_with_idempotency_key_retryable", func(t *testing.T) {
		server := NewTestServer(
			TestResponse{StatusCode: 502, Body: "bad gateway"},
			TestResponse{StatusCode: 200, Body: "updated"},
		)
		defer server.Close()

		client := New(config, "test-patch-with-key")
		defer client.Close()

		req, _ := http.NewRequestWithContext(ctx, "PATCH", server.URL, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "update-operation-67890")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if server.GetRequestCount() != 2 {
			t.Errorf("expected 2 requests (1 failure + 1 retry with Idempotency-Key), got %d", server.GetRequestCount())
		}
	})

	t.Run("PUT_always_retryable", func(t *testing.T) {
		server := NewTestServer(
			TestResponse{StatusCode: 503, Body: "service unavailable"},
			TestResponse{StatusCode: 200, Body: "replaced"},
		)
		defer server.Close()

		client := New(config, "test-put")
		defer client.Close()

		req, _ := http.NewRequestWithContext(ctx, "PUT", server.URL, nil)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		if server.GetRequestCount() != 2 {
			t.Errorf("expected 2 requests (1 failure + 1 retry for PUT), got %d", server.GetRequestCount())
		}
	})
}

func TestIdempotencyKeyValidation(t *testing.T) {
	config := RetryConfig{
		RetryMethods: []string{"GET", "PUT", "DELETE"},
	}

	testCases := []struct {
		name          string
		method        string
		hasIdempKey   bool
		expectedRetry bool
	}{
		{"GET without key", "GET", false, true},
		{"GET with key", "GET", true, true},
		{"POST without key", "POST", false, false},
		{"POST with key", "POST", true, true},
		{"PATCH without key", "PATCH", false, false},
		{"PATCH with key", "PATCH", true, true},
		{"PUT without key", "PUT", false, true},
		{"PUT with key", "PUT", true, true},
		{"DELETE without key", "DELETE", false, true},
		{"DELETE with key", "DELETE", true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, "https://example.com", nil)
			if tc.hasIdempKey {
				req.Header.Set("Idempotency-Key", "test-key-123")
			}

			result := config.isRequestRetryable(req)
			if result != tc.expectedRetry {
				t.Errorf("expected retry=%v for %s (idempKey=%v), got %v",
					tc.expectedRetry, tc.method, tc.hasIdempKey, result)
			}
		})
	}
}

func TestRetryPreservesLastResponseStatus(t *testing.T) {
	config := Config{
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			MaxDelay:         10 * time.Millisecond,
			RetryMethods:     []string{"GET", "PUT", "DELETE"},
			RetryStatusCodes: []int{500, 502, 503, 429},
		},
	}

	ctx := context.Background()

	t.Run("failed_retries_return_last_status", func(t *testing.T) {
		// Сервер возвращает разные ошибки, последняя 502
		server := NewTestServer(
			TestResponse{StatusCode: 500, Body: "internal server error"},
			TestResponse{StatusCode: 503, Body: "service unavailable"},
			TestResponse{StatusCode: 502, Body: "bad gateway"},
		)
		defer server.Close()

		client := New(config, "test-last-status")
		defer client.Close()

		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Fatalf("expected response despite retries, got error: %v", err)
		}
		defer resp.Body.Close()

		// Должен вернуть последний статус код (502)
		if resp.StatusCode != 502 {
			t.Errorf("expected final status 502 (last attempt), got %d", resp.StatusCode)
		}

		// Проверяем что все 3 попытки были сделаны
		if server.GetRequestCount() != 3 {
			t.Errorf("expected 3 requests, got %d", server.GetRequestCount())
		}
	})

	t.Run("successful_retry_returns_success_status", func(t *testing.T) {
		// Первые попытки ошибки, последняя успех
		server := NewTestServer(
			TestResponse{StatusCode: 503, Body: "service unavailable"},
			TestResponse{StatusCode: 500, Body: "internal server error"},
			TestResponse{StatusCode: 200, Body: "success"},
		)
		defer server.Close()

		client := New(config, "test-success-status")
		defer client.Close()

		resp, err := client.Get(ctx, server.URL)
		if err != nil {
			t.Fatalf("expected success after retries, got error: %v", err)
		}
		defer resp.Body.Close()

		// Должен вернуть успешный статус код (200)
		if resp.StatusCode != 200 {
			t.Errorf("expected final status 200 (successful retry), got %d", resp.StatusCode)
		}

		// Проверяем что все 3 попытки были сделаны
		if server.GetRequestCount() != 3 {
			t.Errorf("expected 3 requests, got %d", server.GetRequestCount())
		}
	})
}
