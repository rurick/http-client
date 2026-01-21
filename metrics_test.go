package httpclient

import (
	"context"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	// Test works with already registered metrics
	// (they might have been created in previous tests)

	metrics := NewMetrics("testhttpclient")

	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}

	if !metrics.enabled {
		t.Error("expected metrics to be enabled by default")
	}

	if metrics.clientName != "testhttpclient" {
		t.Errorf("expected clientName to be 'testhttpclient', got %s", metrics.clientName)
	}

	// Metrics are now encapsulated in the provider
	if metrics.provider == nil {
		t.Error("expected metrics provider to be initialized")
	}
}

func TestNewDisabledMetrics(t *testing.T) {
	metrics := NewDisabledMetrics("disabled-client")

	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}

	if metrics.enabled {
		t.Error("expected metrics to be disabled")
	}

	if metrics.clientName != "disabled-client" {
		t.Errorf("expected clientName to be 'disabled-client', got %s", metrics.clientName)
	}
}

func TestMetrics_RecordRequest(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording request metric - should not panic
	metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	metrics.RecordRequest(ctx, "POST", "api.example.com", "500", true, true)
}

func TestMetricsDisabled_NoOp(t *testing.T) {
	metrics := NewDisabledMetrics("disabled")
	ctx := context.Background()

	// All operations should be no-op and not panic
	metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	metrics.RecordRetry(ctx, "status", "GET", "example.com")
	metrics.IncrementInflight(ctx, "GET", "example.com")
	metrics.DecrementInflight(ctx, "GET", "example.com")
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")

	err := metrics.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

func TestMetrics_RecordDuration(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording duration metric - should not panic
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	metrics.RecordDuration(ctx, 1.2, "POST", "api.example.com", "500", 2)
}

func TestMetrics_RecordRetry(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording retry metric - should not panic
	metrics.RecordRetry(ctx, "status", "GET", "example.com")
	metrics.RecordRetry(ctx, "timeout", "POST", "api.example.com")
}

func TestMetrics_RecordRequestSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording request size metric - should not panic
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")
	metrics.RecordRequestSize(ctx, 0, "GET", "api.example.com")
}

func TestMetrics_RecordResponseSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test recording response size metric - should not panic
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")
	metrics.RecordResponseSize(ctx, 512, "POST", "api.example.com", "500")
}

func TestMetrics_Close(t *testing.T) {
	metrics := NewMetrics("testhttpclient")

	err := metrics.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// Integration test using Prometheus metrics
func TestMetrics_Integration(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Simulate a sequence of metric calls as in a real HTTP request

	// 1. Increment active requests counter
	metrics.IncrementInflight(ctx, "POST", "example.com")

	// 2. Record request size
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")

	// 3. Record request metric (first attempt)
	metrics.RecordRequest(ctx, "POST", "example.com", "500", false, true)
	metrics.RecordDuration(ctx, 0.5, "POST", "example.com", "500", 1)

	// 4. Record retry
	metrics.RecordRetry(ctx, "status", "POST", "example.com")

	// 5. Record request metric (retry attempt)
	metrics.RecordRequest(ctx, "POST", "example.com", "200", true, false)
	metrics.RecordDuration(ctx, 0.3, "POST", "example.com", "200", 2)

	// 6. Record response size
	metrics.RecordResponseSize(ctx, 512, "POST", "example.com", "200")

	// 7. Decrement active requests counter
	metrics.DecrementInflight(ctx, "POST", "example.com")

	// If we reached here without panic, the test passed
}

func TestMetrics_EdgeCases(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Test with empty values
	metrics.RecordRequest(ctx, "", "", "", false, false)
	metrics.RecordDuration(ctx, 0, "", "", "", 0)
	metrics.RecordRetry(ctx, "", "", "")
	metrics.IncrementInflight(ctx, "", "")
	metrics.DecrementInflight(ctx, "", "")
	metrics.RecordRequestSize(ctx, 0, "", "")
	metrics.RecordResponseSize(ctx, 0, "", "", "")

	// Test with very large values
	metrics.RecordDuration(ctx, 999999.999, "GET", "example.com", "200", 1)
	metrics.RecordRequestSize(ctx, 1<<60, "POST", "example.com")
	metrics.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "200")

	// Test inflight metrics
	metrics.IncrementInflight(ctx, "GET", "example.com")
	metrics.DecrementInflight(ctx, "GET", "example.com")
}

// TestPrometheusMetricsMultipleClients checks that multiple clients with Prometheus work correctly
func TestPrometheusMetricsMultipleClients(t *testing.T) {
	// Client 1
	metrics1 := NewMetrics("client-1")
	if metrics1.provider == nil {
		t.Error("expected metrics provider to be available")
	}

	// Клиент 2
	metrics2 := NewMetrics("client-2")
	if metrics2.provider == nil {
		t.Error("expected metrics provider to be available")
	}

	// Оба клиента должны быть включены
	if !metrics1.enabled || !metrics2.enabled {
		t.Error("both clients should have metrics enabled")
	}

	if metrics1.clientName != "client-1" {
		t.Errorf("expected client-1 name, got %s", metrics1.clientName)
	}

	if metrics2.clientName != "client-2" {
		t.Errorf("expected client-2 name, got %s", metrics2.clientName)
	}
}
