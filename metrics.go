package httpclient

import "context"

// Metrics contains metrics configuration for a specific HTTP client.
type Metrics struct {
	clientName string
	enabled    bool
	provider   MetricsProvider
}

// NewMetrics creates a new metrics instance with Prometheus provider by default.
func NewMetrics(meterName string) *Metrics {
	provider := NewPrometheusMetricsProvider(meterName, nil)
	return &Metrics{
		clientName: meterName,
		enabled:    true,
		provider:   provider,
	}
}

// NewDisabledMetrics creates a metrics instance with collection disabled.
func NewDisabledMetrics(meterName string) *Metrics {
	return &Metrics{
		clientName: meterName,
		enabled:    false,
		provider:   NewNoopMetricsProvider(),
	}
}

// NewMetricsWithProvider creates a metrics instance with the specified provider.
// Used internally by the client to select a provider.
func NewMetricsWithProvider(meterName string, provider MetricsProvider) *Metrics {
	// Metrics are considered enabled if provider is not noop
	enabled := provider != nil
	if noop, ok := provider.(*NoopMetricsProvider); ok && noop != nil {
		enabled = false
	}
	return &Metrics{
		clientName: meterName,
		enabled:    enabled,
		provider:   provider,
	}
}

// RecordRequest records metrics for a request.
func (m *Metrics) RecordRequest(ctx context.Context, method, host, path, status string, retry, hasError bool) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRequest(ctx, method, host, path, status, retry, hasError)
}

// RecordDuration records request duration.
func (m *Metrics) RecordDuration(ctx context.Context, duration float64, method, host, path, status string, attempt int) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordDuration(ctx, duration, method, host, path, status, attempt)
}

// RecordRetry records a retry metric.
func (m *Metrics) RecordRetry(ctx context.Context, reason, method, host, path string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRetry(ctx, reason, method, host, path)
}

// RecordRequestSize records request size.
func (m *Metrics) RecordRequestSize(ctx context.Context, size int64, method, host, path string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRequestSize(ctx, size, method, host, path)
}

// RecordResponseSize records response size.
func (m *Metrics) RecordResponseSize(ctx context.Context, size int64, method, host, path, status string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordResponseSize(ctx, size, method, host, path, status)
}

// IncrementInflight increments the active requests counter.
func (m *Metrics) IncrementInflight(ctx context.Context, method, host, path string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.InflightInc(ctx, method, host, path)
}

// DecrementInflight decrements the active requests counter.
func (m *Metrics) DecrementInflight(ctx context.Context, method, host, path string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.InflightDec(ctx, method, host, path)
}

// Close releases metrics resources.
func (m *Metrics) Close() error {
	if m.provider != nil {
		return m.provider.Close()
	}
	return nil
}
