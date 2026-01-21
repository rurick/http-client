package httpclient

import "context"

// NoopMetricsProvider is a provider that does nothing.
// Used when metrics are disabled in configuration.
type NoopMetricsProvider struct{}

// NewNoopMetricsProvider creates a new NoopMetricsProvider instance.
func NewNoopMetricsProvider() *NoopMetricsProvider {
	return &NoopMetricsProvider{}
}

// RecordRequest does nothing.
func (n *NoopMetricsProvider) RecordRequest(_ context.Context, _, _, _ string, _, _ bool) {}

// RecordDuration does nothing.
func (n *NoopMetricsProvider) RecordDuration(_ context.Context, _ float64, _, _, _ string, _ int) {}

// RecordRetry does nothing.
func (n *NoopMetricsProvider) RecordRetry(_ context.Context, _, _, _ string) {}

// RecordRequestSize does nothing.
func (n *NoopMetricsProvider) RecordRequestSize(_ context.Context, _ int64, _, _ string) {}

// RecordResponseSize does nothing.
func (n *NoopMetricsProvider) RecordResponseSize(_ context.Context, _ int64, _, _, _ string) {}

// InflightInc does nothing.
func (n *NoopMetricsProvider) InflightInc(_ context.Context, _, _ string) {}

// InflightDec does nothing.
func (n *NoopMetricsProvider) InflightDec(_ context.Context, _, _ string) {}

// Close returns nil.
func (n *NoopMetricsProvider) Close() error {
	return nil
}