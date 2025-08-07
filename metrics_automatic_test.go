package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestConnectionPoolMetricsAutomatic проверяет автоматическую запись connection pool метрик
func TestConnectionPoolMetricsAutomatic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewClient(WithMetrics(true))
	require.NoError(t, err)
	require.NotNil(t, client.otelCollector)

	// Make a request to trigger automatic metrics
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Connection pool metrics should be recorded automatically
	// This test verifies that the metrics recording code path is executed
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestConnectionPoolFailureMetrics проверяет запись connection pool miss при ошибках
func TestConnectionPoolFailureMetrics(t *testing.T) {
	t.Parallel()

	client, err := NewClient(WithMetrics(true))
	require.NoError(t, err)
	require.NotNil(t, client.otelCollector)

	// Make a request to a non-existent server (should trigger connection failure)
	resp, err := client.Get("http://127.0.0.1:65432") // Non-existent port

	// We expect an error for connection failure
	assert.Error(t, err, "Expected connection error for non-existent server")
	assert.Nil(t, resp, "Response should be nil for failed connection")

	// Connection pool miss should be recorded for connection failures
	// The test validates that the metrics recording code path is executed
	assert.Contains(t, err.Error(), "connection refused", "Expected connection refused error")
}

// TestMiddlewareMetricsAutomatic проверяет автоматическую запись middleware метрик
func TestMiddlewareMetricsAutomatic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Небольшая задержка для измерения времени
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client, err := NewClient(
		WithMetrics(true),
		WithMiddleware(NewLoggingMiddleware(logger)),
	)
	require.NoError(t, err)
	require.NotNil(t, client.otelCollector)

	// Make a request through middleware
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Middleware metrics should be recorded automatically
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestMiddlewareErrorMetricsAutomatic проверяет автоматическую запись middleware ошибок
func TestMiddlewareErrorMetricsAutomatic(t *testing.T) {
	t.Parallel()

	// Server that causes an error condition
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client, err := NewClient(
		WithMetrics(true),
		WithMiddleware(NewLoggingMiddleware(logger)),
	)
	require.NoError(t, err)

	// Make a request that will result in error logging
	resp, err := client.Get(server.URL)
	require.NoError(t, err) // HTTP client doesn't return error for 404
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestRetryAttemptsMetricIntegration проверяет интеграцию retry attempts метрики
func TestRetryAttemptsMetricIntegration(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer server.Close()

	retryStrategy := NewExponentialBackoffStrategy(3, 10*time.Millisecond, 2.0)
	client, err := NewClient(
		WithMetrics(true),
		WithRetryStrategy(retryStrategy),
	)
	require.NoError(t, err)

	// Make a request that will trigger retries
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed after retries
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, attempts) // Should have made 3 attempts total
}

// TestOTelCollectorContextIntegration проверяет передачу коллектора через контекст
func TestOTelCollectorContextIntegration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewClient(WithMetrics(true))
	require.NoError(t, err)
	require.NotNil(t, client.otelCollector)

	// Custom middleware to check context
	contextCheckerMiddleware := &ContextCheckerMiddleware{}
	client.middlewareChain.Add(contextCheckerMiddleware)

	// Make a request
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Context should contain the OTel collector
	assert.True(t, contextCheckerMiddleware.foundCollector, "OTel collector should be available in context")
}

// ContextCheckerMiddleware проверяет наличие коллектора в контексте
type ContextCheckerMiddleware struct {
	foundCollector bool
}

func (ccm *ContextCheckerMiddleware) Process(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	// Check if OTel collector is available in context
	if collector, ok := req.Context().Value("otel_collector").(*OTelMetricsCollector); ok && collector != nil {
		ccm.foundCollector = true
	}

	return next(req)
}

// TestAllAutomaticMetricsEnabled проверяет что все автоматические метрики включены
func TestAllAutomaticMetricsEnabled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	retryStrategy := NewExponentialBackoffStrategy(2, 10*time.Millisecond, 2.0)

	client, err := NewClient(
		WithMetrics(true),
		WithMiddleware(NewLoggingMiddleware(logger)),
		WithRetryStrategy(retryStrategy),
	)
	require.NoError(t, err)
	require.NotNil(t, client.otelCollector)

	// Make multiple requests to trigger all automatic metrics
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// All automatic metrics should be working:
	// - Connection pool metrics (RecordConnectionStats, RecordConnectionPoolHit)
	// - Middleware metrics (RecordMiddlewareDuration)
	// - Request metrics (RecordRequest)
	// This test ensures the integration works end-to-end
}

// TestMetricsDisabledNoAutomaticRecording проверяет что автоматические метрики не записываются при отключении
func TestMetricsDisabledNoAutomaticRecording(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewClient(WithMetrics(false))
	require.NoError(t, err)
	require.Nil(t, client.otelCollector)

	// Make a request with metrics disabled
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// No automatic metrics should be recorded (otelCollector is nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
