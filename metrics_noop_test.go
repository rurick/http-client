package httpclient

import (
	"context"
	"testing"
)

// TestNoopMetricsProvider tests creation and operation of NoopMetricsProvider
func TestNoopMetricsProvider(t *testing.T) {
	provider := NewNoopMetricsProvider()

	if provider == nil {
		t.Fatal("expected noop provider to be created")
	}
}

// TestNoopMetricsProvider_AllMethods tests that all NoopMetricsProvider methods do not panic
func TestNoopMetricsProvider_AllMethods(t *testing.T) {
	provider := NewNoopMetricsProvider()
	ctx := context.Background()

	// All methods should be no-op and not panic
	provider.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	provider.RecordRequest(ctx, "POST", "api.example.com", "/api/create", "500", true, true)

	provider.RecordDuration(ctx, 0.5, "GET", "example.com", "/api/test", "200", 1)
	provider.RecordDuration(ctx, 1.2, "POST", "api.example.com", "/api/create", "500", 2)

	provider.RecordRetry(ctx, "status", "GET", "example.com", "/api/test")
	provider.RecordRetry(ctx, "timeout", "POST", "api.example.com", "/api/create")

	provider.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/create")
	provider.RecordRequestSize(ctx, 0, "GET", "api.example.com", "/api/test")

	provider.RecordResponseSize(ctx, 2048, "GET", "example.com", "/api/test", "200")
	provider.RecordResponseSize(ctx, 512, "POST", "api.example.com", "/api/create", "500")

	provider.InflightInc(ctx, "GET", "example.com", "/api/test")
	provider.InflightInc(ctx, "POST", "api.example.com", "/api/create")
	provider.InflightDec(ctx, "GET", "example.com", "/api/test")
	provider.InflightDec(ctx, "POST", "api.example.com", "/api/create")

	// If we reached here without panic, the test passed
}

// TestNoopMetricsProvider_Close tests closing NoopMetricsProvider
func TestNoopMetricsProvider_Close(t *testing.T) {
	provider := NewNoopMetricsProvider()

	err := provider.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// TestNoopMetricsProvider_EdgeCases tests edge cases
func TestNoopMetricsProvider_EdgeCases(t *testing.T) {
	provider := NewNoopMetricsProvider()
	ctx := context.Background()

	// Test with empty values
	provider.RecordRequest(ctx, "", "", "", "", false, false)
	provider.RecordDuration(ctx, 0, "", "", "", "", 0)
	provider.RecordRetry(ctx, "", "", "", "")
	provider.InflightInc(ctx, "", "", "")
	provider.InflightDec(ctx, "", "", "")
	provider.RecordRequestSize(ctx, 0, "", "", "")
	provider.RecordResponseSize(ctx, 0, "", "", "", "")

	// Test with very large values
	provider.RecordDuration(ctx, 999999.999, "GET", "example.com", "/api/test", "200", 1)
	provider.RecordRequestSize(ctx, 1<<60, "POST", "example.com", "/api/create")
	provider.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "/api/test", "200")

	// Test with negative values (for inflight)
	provider.InflightDec(ctx, "GET", "example.com", "/api/test") // decrement without increment

	// If we reached here without panic, the test passed
}

// TestMetricsWithDisabledBackend tests integration with disabled metrics
func TestMetricsWithDisabledBackend(t *testing.T) {
	// Create a client with disabled metrics
	enabled := false
	client := New(Config{
		MetricsEnabled: &enabled,
	}, "disabled-metrics-client")
	defer client.Close()

	// Check that metrics are initialized but disabled
	if client.metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}

	if client.metrics.enabled {
		t.Error("expected metrics to be disabled")
	}

	if client.metrics.clientName != "disabled-metrics-client" {
		t.Errorf("expected clientName to be 'disabled-metrics-client', got %s", client.metrics.clientName)
	}

	// Test that metric calls do not panic
	ctx := context.Background()
	client.metrics.RecordRequest(ctx, "GET", "example.com", "/test", "200", false, false)
	client.metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "/test", "200", 1)

	// If we reached here without panic, the test passed
}
