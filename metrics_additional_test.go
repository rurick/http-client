package httpclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsRecordCircuitBreakerStateMethod проверяет запись состояния circuit breaker
func TestMetricsRecordCircuitBreakerStateMethod(t *testing.T) {
	t.Parallel()

	collector, err := NewOTelMetricsCollector("test")
	require.NoError(t, err)

	// Метод не должен паниковать
	collector.RecordCircuitBreakerState(CircuitBreakerClosed)
	collector.RecordCircuitBreakerState(CircuitBreakerOpen)
	collector.RecordCircuitBreakerState(CircuitBreakerHalfOpen)
}

// TestMetricsGetStatusCodesMethod проверяет получение статус кодов
func TestMetricsGetStatusCodesMethod(t *testing.T) {
	t.Parallel()

	collector, err := NewOTelMetricsCollector("test")
	require.NoError(t, err)

	// Записываем метрики
	collector.RecordRequest("GET", "http://example.com", 200, time.Millisecond*100, 100, 200)
	collector.RecordRequest("POST", "http://example.com", 404, time.Millisecond*200, 150, 50)

	metrics := collector.GetMetrics()

	// Проверяем общие метрики
	assert.Equal(t, int64(2), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.SuccessfulReqs) // 200 OK
	assert.Equal(t, int64(1), metrics.FailedRequests) // 404 Not Found
}
