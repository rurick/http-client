package httpclient

import "context"

// NoopMetricsProvider - провайдер, который ничего не делает.
// Используется когда метрики отключены в конфигурации.
type NoopMetricsProvider struct{}

// NewNoopMetricsProvider создает новый экземпляр NoopMetricsProvider.
func NewNoopMetricsProvider() *NoopMetricsProvider {
	return &NoopMetricsProvider{}
}

// RecordRequest ничего не делает.
func (n *NoopMetricsProvider) RecordRequest(_ context.Context, _, _, _ string, _, _ bool) {}

// RecordDuration ничего не делает.
func (n *NoopMetricsProvider) RecordDuration(_ context.Context, _ float64, _, _, _ string, _ int) {}

// RecordRetry ничего не делает.
func (n *NoopMetricsProvider) RecordRetry(_ context.Context, _, _, _ string) {}

// RecordRequestSize ничего не делает.
func (n *NoopMetricsProvider) RecordRequestSize(_ context.Context, _ int64, _, _ string) {}

// RecordResponseSize ничего не делает.
func (n *NoopMetricsProvider) RecordResponseSize(_ context.Context, _ int64, _, _, _ string) {}

// InflightInc ничего не делает.
func (n *NoopMetricsProvider) InflightInc(_ context.Context, _, _ string) {}

// InflightDec ничего не делает.
func (n *NoopMetricsProvider) InflightDec(_ context.Context, _, _ string) {}

// Close возвращает nil.
func (n *NoopMetricsProvider) Close() error {
	return nil
}