package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientGetMetricsCollectorOption проверяет получение коллектора метрик
func TestClientGetMetricsCollectorOption(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	require.NoError(t, err)

	collector := client.GetMetricsCollector()
	assert.NotNil(t, collector)
}

// TestClientGetOptionsMethod проверяет получение опций клиента
func TestClientGetOptionsMethod(t *testing.T) {
	t.Parallel()

	client, err := NewClient(
		WithTimeout(5*time.Second),
		WithRetryMax(3),
	)
	require.NoError(t, err)

	options := client.GetOptions()
	assert.Equal(t, 5*time.Second, options.Timeout)
	assert.Equal(t, 3, options.RetryMax)
}

// TestOptionsWithRetryStrategyConfig проверяет опцию WithRetryStrategy
func TestOptionsWithRetryStrategyConfig(t *testing.T) {
	t.Parallel()

	strategy := NewFixedDelayStrategy(3, time.Second)

	client, err := NewClient(WithRetryStrategy(strategy))
	require.NoError(t, err)

	options := client.GetOptions()
	assert.NotNil(t, options.RetryStrategy)
}

// TestOptionsWithRetryWaitConfig проверяет опцию WithRetryWait
func TestOptionsWithRetryWaitConfig(t *testing.T) {
	t.Parallel()

	minWait := time.Second
	maxWait := 2 * time.Second

	client, err := NewClient(WithRetryWait(minWait, maxWait))
	require.NoError(t, err)

	options := client.GetOptions()
	assert.Equal(t, minWait, options.RetryWaitMin)
	assert.Equal(t, maxWait, options.RetryWaitMax)
}

// TestOptionsWithTracingConfig проверяет опцию WithTracing
func TestOptionsWithTracingConfig(t *testing.T) {
	t.Parallel()

	client, err := NewClient(WithTracing(true))
	require.NoError(t, err)

	options := client.GetOptions()
	assert.True(t, options.TracingEnabled)
}

// TestOptionsWithHTTPClientConfig проверяет опцию WithHTTPClient
func TestOptionsWithHTTPClientConfig(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{Timeout: 10 * time.Second}

	client, err := NewClient(WithHTTPClient(customClient))
	require.NoError(t, err)

	options := client.GetOptions()
	assert.Equal(t, customClient, options.HTTPClient)
}
