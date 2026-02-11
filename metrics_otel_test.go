package httpclient

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestOpenTelemetryMetricsProvider tests creation of OpenTelemetry metrics provider
func TestOpenTelemetryMetricsProvider(t *testing.T) {
	provider := NewOpenTelemetryMetricsProvider("test-client", nil)

	if provider == nil {
		t.Fatal("expected provider to be created")
	}

	if provider.clientName != "test-client" {
		t.Errorf("expected clientName to be 'test-client', got %s", provider.clientName)
	}

	if provider.inst == nil {
		t.Error("expected instruments to be initialized")
	}
}

// TestOpenTelemetryMetricsProvider_WithCustomMeterProvider tests creation with custom MeterProvider
func TestOpenTelemetryMetricsProvider_WithCustomMeterProvider(t *testing.T) {
	// Create a test MeterProvider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("custom-client", meterProvider)

	if provider == nil {
		t.Fatal("expected provider to be created")
	}

	if provider.clientName != "custom-client" {
		t.Errorf("expected clientName to be 'custom-client', got %s", provider.clientName)
	}
}

// TestOpenTelemetryMetricsProvider_RecordRequest tests recording request metrics
func TestOpenTelemetryMetricsProvider_RecordRequest(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()

	// Record several metrics
	provider.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	provider.RecordRequest(ctx, "POST", "api.example.com", "/api/test", "500", true, true)

	// Check that calls do not panic
	// (detailed value checking can be complex due to OpenTelemetry's internal structure)
}

// TestOpenTelemetryMetricsProvider_RecordDuration tests recording duration metrics
func TestOpenTelemetryMetricsProvider_RecordDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()

	// Record duration metrics
	provider.RecordDuration(ctx, 0.5, "GET", "example.com", "/api/test", "200", 1)
	provider.RecordDuration(ctx, 1.2, "POST", "api.example.com", "/api/test", "500", 2)

	// Check that calls do not panic
}

// TestOpenTelemetryMetricsProvider_RecordRetry tests recording retry metrics
func TestOpenTelemetryMetricsProvider_RecordRetry(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()

	// Record retry metrics
	provider.RecordRetry(ctx, "status", "GET", "example.com", "/api/test")
	provider.RecordRetry(ctx, "timeout", "POST", "api.example.com", "/api/test")

	// Check that calls do not panic
}

// TestOpenTelemetryMetricsProvider_RecordSizes tests recording size metrics
func TestOpenTelemetryMetricsProvider_RecordSizes(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()

	// Record size metrics
	provider.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/test")
	provider.RecordResponseSize(ctx, 2048, "GET", "example.com", "/api/test", "200")

	// Check that calls do not panic
}

// TestOpenTelemetryMetricsProvider_Inflight tests active requests metrics
func TestOpenTelemetryMetricsProvider_Inflight(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()

	// Test increment/decrement of active requests
	provider.InflightInc(ctx, "GET", "example.com", "/api/test")
	provider.InflightInc(ctx, "POST", "api.example.com", "/api/test")
	provider.InflightDec(ctx, "GET", "example.com", "/api/test")
	provider.InflightDec(ctx, "POST", "api.example.com", "/api/test")

	// Check that calls do not panic
}

// TestOpenTelemetryMetricsProvider_Close tests closing the provider
func TestOpenTelemetryMetricsProvider_Close(t *testing.T) {
	provider := NewOpenTelemetryMetricsProvider("test-client", nil)

	err := provider.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// TestOpenTelemetryMetricsProvider_Integration integration test with real sequence of calls
func TestOpenTelemetryMetricsProvider_Integration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("integration-test", meterProvider)
	ctx := context.Background()

	// Simulate a sequence of metric calls as in a real HTTP request

	// 1. Increment active requests counter
	provider.InflightInc(ctx, "POST", "example.com", "/api/test")

	// 2. Record request size
	provider.RecordRequestSize(ctx, 1024, "POST", "example.com", "/api/test")

	// 3. Record request metric (first attempt)
	provider.RecordRequest(ctx, "POST", "example.com", "/api/test", "500", false, true)
	provider.RecordDuration(ctx, 0.5, "POST", "example.com", "/api/test", "500", 1)

	// 4. Record retry
	provider.RecordRetry(ctx, "status", "POST", "example.com", "/api/test")

	// 5. Record request metric (retry attempt)
	provider.RecordRequest(ctx, "POST", "example.com", "/api/test", "200", true, false)
	provider.RecordDuration(ctx, 0.3, "POST", "example.com", "/api/test", "200", 2)

	// 6. Record response size
	provider.RecordResponseSize(ctx, 512, "POST", "example.com", "/api/test", "200")

	// 7. Decrement active requests counter
	provider.InflightDec(ctx, "POST", "example.com", "/api/test")

	// If we reached here without panic, the test passed
}

// TestOpenTelemetryMetricsProvider_EdgeCases tests edge cases
func TestOpenTelemetryMetricsProvider_EdgeCases(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	provider := NewOpenTelemetryMetricsProvider("edge-cases", meterProvider)
	ctx := context.Background()

	// Test with empty values
	provider.RecordRequest(ctx, "", "", "/api/test", "", false, false)
	provider.RecordDuration(ctx, 0, "", "", "/api/test", "", 0)
	provider.RecordRetry(ctx, "", "", "", "/api/test")
	provider.InflightInc(ctx, "", "", "/api/test")
	provider.InflightDec(ctx, "", "", "/api/test")
	provider.RecordRequestSize(ctx, 0, "", "", "/api/test")
	provider.RecordResponseSize(ctx, 0, "", "", "/api/test", "")

	// Test with very large values
	provider.RecordDuration(ctx, 999999.999, "GET", "example.com", "/api/test", "200", 1)
	provider.RecordRequestSize(ctx, 1<<60, "POST", "example.com", "/api/test")
	provider.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "/api/test", "200")

	// Test with negative values (for inflight)
	provider.InflightDec(ctx, "GET", "example.com", "/api/test") // decrement without increment

	// If we reached here without panic, the test passed
}

// TestOpenTelemetryMetricsProvider_MultipleClients tests working with multiple clients
func TestOpenTelemetryMetricsProvider_MultipleClients(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	// Create multiple providers with one MeterProvider
	provider1 := NewOpenTelemetryMetricsProvider("client-1", meterProvider)
	provider2 := NewOpenTelemetryMetricsProvider("client-2", meterProvider)

	ctx := context.Background()

	// Check that each provider has its own clientName
	if provider1.clientName != "client-1" {
		t.Errorf("expected client-1 name, got %s", provider1.clientName)
	}
	if provider2.clientName != "client-2" {
		t.Errorf("expected client-2 name, got %s", provider2.clientName)
	}

	// Record metrics from different clients
	provider1.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	provider2.RecordRequest(ctx, "POST", "api.example.com", "/api/test", "201", false, false)

	// Check that both use the same instruments (caching works)
	if provider1.inst != provider2.inst {
		t.Error("providers should share the same instruments when using the same MeterProvider")
	}
}

// TestMetricsWithOpenTelemetryBackend tests integration through Config
func TestMetricsWithOpenTelemetryBackend(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())

	// Create a client with OpenTelemetry backend
	client := New(Config{
		MetricsBackend:    MetricsBackendOpenTelemetry,
		OTelMeterProvider: meterProvider,
	}, "test-otel-client")
	defer client.Close()

	// Check that metrics are initialized
	if client.metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}

	if !client.metrics.enabled {
		t.Error("expected metrics to be enabled")
	}

	if client.metrics.clientName != "test-otel-client" {
		t.Errorf("expected clientName to be 'test-otel-client', got %s", client.metrics.clientName)
	}

	// Test recording metrics through the client
	ctx := context.Background()
	client.metrics.RecordRequest(ctx, "GET", "example.com", "/api/test", "200", false, false)
	client.metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "/api/test", "200", 1)

	// If we reached here without panic, the test passed
}
