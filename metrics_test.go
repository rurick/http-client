package httpclient

import (
	"context"
	"testing"
)

// TestNewMetrics tests creation of Metrics
func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics("testclient")
	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}

	if !metrics.enabled {
		t.Error("expected metrics to be enabled by default")
	}

	if metrics.clientName != "testclient" {
		t.Errorf("expected clientName to be 'testclient', got %s", metrics.clientName)
	}
}

// TestNewDisabledMetrics tests creation of disabled metrics
func TestNewDisabledMetrics(t *testing.T) {
	metrics := NewDisabledMetrics("disabledclient")
	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}

	if metrics.enabled {
		t.Error("expected metrics to be disabled")
	}

	if metrics.clientName != "disabledclient" {
		t.Errorf("expected clientName to be 'disabledclient', got %s", metrics.clientName)
	}
}

// TestNewMetricsWithProvider tests creation with custom provider
func TestNewMetricsWithProvider(t *testing.T) {
	provider := NewNoopMetricsProvider()
	metrics := NewMetricsWithProvider("testclient", provider)

	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}
}

func TestMetrics_RecordRequest(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording request metric - should not panic
	metrics.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	metrics.RecordRequest(ctx, "POST", "api.example.com", "/api/create", "500", true, true)
}

func TestMetrics_DisabledOperations(t *testing.T) {
	metrics := NewDisabledMetrics("testhttpclient")
	ctx := context.Background()

	// All operations should be no-op and not panic
	metrics.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "/api/test", "200", 1)
	metrics.RecordRetry(ctx, "status", "GET", "example.com", "/api/test")
	metrics.IncrementInflight(ctx, "GET", "example.com", "/api/test")
	metrics.DecrementInflight(ctx, "GET", "example.com", "/api/test")
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/create")
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "/api/test", "200")

	// If we reached here without panic, the test passed
}

func TestMetrics_RecordDuration(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording duration metric - should not panic
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "/api/test", "200", 1)
	metrics.RecordDuration(ctx, 1.2, "POST", "api.example.com", "/api/create", "500", 2)
}

func TestMetrics_RecordRetry(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording retry metric - should not panic
	metrics.RecordRetry(ctx, "status", "GET", "example.com", "/api/test")
	metrics.RecordRetry(ctx, "timeout", "POST", "api.example.com", "/api/create")
}

func TestMetrics_RecordRequestSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording request size metric - should not panic
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/create")
	metrics.RecordRequestSize(ctx, 0, "GET", "api.example.com", "/api/test")
}

func TestMetrics_RecordResponseSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording response size metric - should not panic
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "/api/test", "200")
	metrics.RecordResponseSize(ctx, 512, "POST", "api.example.com", "/api/create", "500")
}

func TestMetrics_Close(t *testing.T) {
	metrics := NewMetrics("testhttpclient")

	err := metrics.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// TestMetrics_CompleteScenario tests a typical metrics recording scenario
func TestMetrics_CompleteScenario(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Simulate a complete request lifecycle with retry
	// 1. Increment inflight
	metrics.IncrementInflight(ctx, "POST", "example.com", "/api/create")

	// 2. Record request size
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/create")

	// 3. Record request metric (first attempt)
	metrics.RecordRequest(ctx, "POST", "example.com", "/api/create", "500", false, true)
	metrics.RecordDuration(ctx, 0.5, "POST", "example.com", "/api/create", "500", 1)

	// 4. Record retry
	metrics.RecordRetry(ctx, "status", "POST", "example.com", "/api/create")

	// 5. Record request metric (retry attempt)
	metrics.RecordRequest(ctx, "POST", "example.com", "/api/create", "200", true, false)
	metrics.RecordDuration(ctx, 0.3, "POST", "example.com", "/api/create", "200", 2)

	// 6. Record response size
	metrics.RecordResponseSize(ctx, 512, "POST", "example.com", "/api/create", "200")

	// 7. Decrement inflight
	metrics.DecrementInflight(ctx, "POST", "example.com", "/api/create")

	// If we reached here without panic, the test passed
}

func TestMetrics_EdgeCases(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test with empty values
	metrics.RecordRequest(ctx, "", "", "", "", false, false)
	metrics.RecordDuration(ctx, 0, "", "", "", "", 0)
	metrics.RecordRetry(ctx, "", "", "", "")
	metrics.IncrementInflight(ctx, "", "", "")
	metrics.DecrementInflight(ctx, "", "", "")
	metrics.RecordRequestSize(ctx, 0, "", "", "")
	metrics.RecordResponseSize(ctx, 0, "", "", "", "")

	// Test with very large values
	metrics.RecordDuration(ctx, 999999.999, "GET", "example.com", "/api/test", "200", 1)
	metrics.RecordRequestSize(ctx, 1<<60, "POST", "example.com", "/api/create")
	metrics.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "/api/test", "200")

	// If we reached here without panic, the test passed
}
