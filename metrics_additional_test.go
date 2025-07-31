package httpclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMetricConstants проверяет что все константы метрик определены правильно
func TestMetricConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"HTTP Requests Total", MetricHTTPRequestsTotal, "http_requests_total"},
		{"HTTP Request Duration", MetricHTTPRequestDuration, "http_request_duration_seconds"},
		{"HTTP Request Size", MetricHTTPRequestSize, "http_request_size_bytes"},
		{"HTTP Response Size", MetricHTTPResponseSize, "http_response_size_bytes"},
		{"HTTP Retries Total", MetricHTTPRetriesTotal, "http_retries_total"},
		{"HTTP Retry Attempts", MetricHTTPRetryAttempts, "http_retry_attempts"},
		{"Circuit Breaker State", MetricCircuitBreakerState, "circuit_breaker_state"},
		{"Circuit Breaker Failures", MetricCircuitBreakerFailures, "circuit_breaker_failures_total"},
		{"Circuit Breaker Successes", MetricCircuitBreakerSuccesses, "circuit_breaker_successes_total"},
		{"Circuit Breaker State Changes", MetricCircuitBreakerStateChanges, "circuit_breaker_state_changes_total"},
		{"HTTP Connections Active", MetricHTTPConnectionsActive, "http_connections_active"},
		{"HTTP Connections Idle", MetricHTTPConnectionsIdle, "http_connections_idle"},
		{"HTTP Connection Pool Hits", MetricHTTPConnectionPoolHits, "http_connection_pool_hits_total"},
		{"HTTP Connection Pool Misses", MetricHTTPConnectionPoolMisses, "http_connection_pool_misses_total"},
		{"Middleware Duration", MetricMiddlewareDuration, "middleware_duration_seconds"},
		{"Middleware Errors", MetricMiddlewareErrors, "middleware_errors_total"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

// TestMetricConstantsUniqueness проверяет что все константы метрик уникальны
func TestMetricConstantsUniqueness(t *testing.T) {
	metrics := []string{
		MetricHTTPRequestsTotal,
		MetricHTTPRequestDuration,
		MetricHTTPRequestSize,
		MetricHTTPResponseSize,
		MetricHTTPRetriesTotal,
		MetricHTTPRetryAttempts,
		MetricCircuitBreakerState,
		MetricCircuitBreakerFailures,
		MetricCircuitBreakerSuccesses,
		MetricCircuitBreakerStateChanges,
		MetricHTTPConnectionsActive,
		MetricHTTPConnectionsIdle,
		MetricHTTPConnectionPoolHits,
		MetricHTTPConnectionPoolMisses,
		MetricMiddlewareDuration,
		MetricMiddlewareErrors,
	}

	// Проверяем уникальность всех констант
	seen := make(map[string]bool)
	for _, metric := range metrics {
		assert.False(t, seen[metric], "Метрика %s не должна повторяться", metric)
		seen[metric] = true
	}

	// Проверяем что у нас есть все ожидаемые метрики
	assert.Len(t, metrics, 16, "Должно быть 16 констант метрик")
}

// TestMetricConstantsNaming проверяет соответствие именования констант Prometheus стандартам
func TestMetricConstantsNaming(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		hasTotal bool
		hasSeconds bool
		hasBytes bool
	}{
		{"HTTP Requests Total", MetricHTTPRequestsTotal, true, false, false},
		{"HTTP Request Duration", MetricHTTPRequestDuration, false, true, false},
		{"HTTP Request Size", MetricHTTPRequestSize, false, false, true},
		{"HTTP Response Size", MetricHTTPResponseSize, false, false, true},
		{"HTTP Retries Total", MetricHTTPRetriesTotal, true, false, false},
		{"Circuit Breaker Failures", MetricCircuitBreakerFailures, true, false, false},
		{"Circuit Breaker Successes", MetricCircuitBreakerSuccesses, true, false, false},
		{"Circuit Breaker State Changes", MetricCircuitBreakerStateChanges, true, false, false},
		{"HTTP Connection Pool Hits", MetricHTTPConnectionPoolHits, true, false, false},
		{"HTTP Connection Pool Misses", MetricHTTPConnectionPoolMisses, true, false, false},
		{"Middleware Duration", MetricMiddlewareDuration, false, true, false},
		{"Middleware Errors", MetricMiddlewareErrors, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasTotal {
				assert.Contains(t, tt.constant, "_total", "Counter метрика должна заканчиваться на _total")
			}
			if tt.hasSeconds {
				assert.Contains(t, tt.constant, "_seconds", "Метрика времени должна содержать _seconds")
			}
			if tt.hasBytes {
				assert.Contains(t, tt.constant, "_bytes", "Метрика размера должна содержать _bytes")
			}
		})
	}
}