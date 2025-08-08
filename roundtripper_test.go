package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// mockRoundTripper имитирует http.RoundTripper для тестирования
type mockRoundTripper struct {
	responses []*http.Response
	errors    []error
	callCount int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	defer func() { m.callCount++ }()

	if m.callCount >= len(m.responses) && m.callCount >= len(m.errors) {
		return nil, errors.New("no more responses configured")
	}

	var err error
	if m.callCount < len(m.errors) {
		err = m.errors[m.callCount]
	}

	var resp *http.Response
	if m.callCount < len(m.responses) {
		resp = m.responses[m.callCount]
	}

	return resp, err
}

func (m *mockRoundTripper) reset() {
	m.callCount = 0
}

func TestRoundTripper_SuccessfulRequest(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}

	mock := &mockRoundTripper{
		responses: []*http.Response{resp},
	}

	config := Config{
		Transport:      mock,
		RetryEnabled:   false,
		TracingEnabled: false,
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(context.Background())

	result, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", result.StatusCode)
	}

	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}
}

func TestRoundTripper_RetryOnServerError(t *testing.T) {
	responses := []*http.Response{
		{StatusCode: 500, Header: make(http.Header)},
		{StatusCode: 500, Header: make(http.Header)},
		{StatusCode: 200, Header: make(http.Header)},
	}

	mock := &mockRoundTripper{responses: responses}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:       3,
			BaseDelay:         1 * time.Millisecond,
			MaxDelay:          10 * time.Millisecond,
			RetryMethods:      []string{"GET"},
			RetryStatusCodes:  []int{500},
			RespectRetryAfter: false,
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(context.Background())

	result, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected final status code 200, got %d", result.StatusCode)
	}

	if mock.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", mock.callCount)
	}
}

func TestRoundTripper_RespectRetryAfter(t *testing.T) {
	retryAfterResp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"1"}},
	}

	responses := []*http.Response{retryAfterResp, {StatusCode: 200, Header: make(http.Header)}}
	mock := &mockRoundTripper{responses: responses}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:       2,
			BaseDelay:         10 * time.Millisecond,
			MaxDelay:          2 * time.Second,
			RetryMethods:      []string{"GET"},
			RetryStatusCodes:  []int{429},
			RespectRetryAfter: true,
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(context.Background())

	start := time.Now()
	result, err := rt.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected final status code 200, got %d", result.StatusCode)
	}

	// Проверяем, что была задержка близкая к 1 секунде
	if elapsed < 900*time.Millisecond || elapsed > 1100*time.Millisecond {
		t.Errorf("expected delay around 1s due to Retry-After, got %v", elapsed)
	}
}

func TestRoundTripper_NonRetryableMethod(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Header:     make(http.Header),
	}

	mock := &mockRoundTripper{responses: []*http.Response{resp}}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			RetryMethods:     []string{"GET"}, // POST не включён
			RetryStatusCodes: []int{500},
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	req = req.WithContext(context.Background())

	result, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 500 {
		t.Errorf("expected status code 500, got %d", result.StatusCode)
	}

	// POST не должен ретраиться
	if mock.callCount != 1 {
		t.Errorf("expected 1 call (no retry for POST), got %d", mock.callCount)
	}
}

func TestRoundTripper_ContextCancellation(t *testing.T) {
	mock := &mockRoundTripper{
		responses: []*http.Response{
			{StatusCode: 500, Header: make(http.Header)},
			{StatusCode: 200, Header: make(http.Header)},
		},
	}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      2,
			BaseDelay:        100 * time.Millisecond,
			RetryMethods:     []string{"GET"},
			RetryStatusCodes: []int{500},
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(ctx)

	// Отменяем контекст через 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context cancellation error, got: %v", err)
	}
}

func TestRoundTripper_NetworkError(t *testing.T) {
	networkErr := &mockNetworkError{temporary: true}

	mock := &mockRoundTripper{
		errors: []error{networkErr, nil},
		responses: []*http.Response{
			nil,
			{StatusCode: 200, Header: make(http.Header)},
		},
	}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:  2,
			BaseDelay:    1 * time.Millisecond,
			RetryMethods: []string{"GET"},
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(context.Background())

	result, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", result.StatusCode)
	}

	if mock.callCount != 2 {
		t.Errorf("expected 2 calls (1 error + 1 success), got %d", mock.callCount)
	}
}

func TestRoundTripper_MaxAttemptsExceeded(t *testing.T) {
	responses := []*http.Response{
		{StatusCode: 500, Header: make(http.Header)},
		{StatusCode: 500, Header: make(http.Header)},
		{StatusCode: 500, Header: make(http.Header)},
	}

	mock := &mockRoundTripper{responses: responses}

	config := Config{
		Transport:    mock,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts:      3,
			BaseDelay:        1 * time.Millisecond,
			RetryMethods:     []string{"GET"},
			RetryStatusCodes: []int{500},
		},
	}.withDefaults()

	rt := &RoundTripper{
		base:    mock,
		config:  config,
		metrics: NewMetrics("testhttpclient"),
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = req.WithContext(context.Background())

	result, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Должен вернуть последний ответ с ошибкой
	if result.StatusCode != 500 {
		t.Errorf("expected status code 500, got %d", result.StatusCode)
	}

	if mock.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", mock.callCount)
	}
}

// Вспомогательные типы для тестирования

type mockNetworkError struct {
	temporary bool
	timeout   bool
}

func (e *mockNetworkError) Error() string {
	return "mock network error"
}

func (e *mockNetworkError) Temporary() bool {
	return e.temporary
}

func (e *mockNetworkError) Timeout() bool {
	return e.timeout
}

func TestGetHost(t *testing.T) {
	testCases := []struct {
		url      string
		expected string
	}{
		{"http://example.com", "example.com"},
		{"http://example.com:8080", "example.com"},
		{"https://api.example.com", "api.example.com"},
		{"https://api.example.com:443", "api.example.com"},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			result := getHost(u)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestShouldRetryStatus(t *testing.T) {
	retryableStatuses := []int{429, 500, 502, 503, 504, 599}
	nonRetryableStatuses := []int{200, 201, 400, 401, 403, 404, 410}

	for _, status := range retryableStatuses {
		if !shouldRetryStatus(status) {
			t.Errorf("status %d should be retryable", status)
		}
	}

	for _, status := range nonRetryableStatuses {
		if shouldRetryStatus(status) {
			t.Errorf("status %d should not be retryable", status)
		}
	}
}
