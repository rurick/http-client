package httpclient

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics содержит конфигурацию метрик для конкретного HTTP клиента.
type Metrics struct {
	clientName string
	enabled    bool
	provider   MetricsProvider
}

// NewMetrics создаёт новый экземпляр метрик с Prometheus провайдером по умолчанию.
func NewMetrics(meterName string) *Metrics {
	provider := NewPrometheusMetricsProvider(meterName, nil)
	return &Metrics{
		clientName: meterName,
		enabled:    true,
		provider:   provider,
	}
}

// NewDisabledMetrics создаёт экземпляр метрик с выключенным сбором.
func NewDisabledMetrics(meterName string) *Metrics {
	return &Metrics{
		clientName: meterName,
		enabled:    false,
		provider:   NewNoopMetricsProvider(),
	}
}

// NewMetricsWithProvider создаёт экземпляр метрик с указанным провайдером.
// Используется внутренне клиентом для выбора провайдера.
func NewMetricsWithProvider(meterName string, provider MetricsProvider) *Metrics {
	// Метрики считаются включенными, если провайдер не noop
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

// RecordRequest записывает метрики для запроса.
func (m *Metrics) RecordRequest(ctx context.Context, method, host, status string, retry, hasError bool) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRequest(ctx, method, host, status, retry, hasError)
}

// RecordDuration записывает длительность запроса.
func (m *Metrics) RecordDuration(ctx context.Context, duration float64, method, host, status string, attempt int) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordDuration(ctx, duration, method, host, status, attempt)
}

// RecordRetry записывает метрику retry.
func (m *Metrics) RecordRetry(ctx context.Context, reason, method, host string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRetry(ctx, reason, method, host)
}

// RecordRequestSize записывает размер запроса.
func (m *Metrics) RecordRequestSize(ctx context.Context, size int64, method, host string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordRequestSize(ctx, size, method, host)
}

// RecordResponseSize записывает размер ответа.
func (m *Metrics) RecordResponseSize(ctx context.Context, size int64, method, host, status string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.RecordResponseSize(ctx, size, method, host, status)
}

// IncrementInflight увеличивает счётчик активных запросов.
func (m *Metrics) IncrementInflight(ctx context.Context, method, host string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.InflightInc(ctx, method, host)
}

// DecrementInflight уменьшает счётчик активных запросов.
func (m *Metrics) DecrementInflight(ctx context.Context, method, host string) {
	if !m.enabled || m.provider == nil {
		return
	}
	m.provider.InflightDec(ctx, method, host)
}

// Close освобождает ресурсы метрик.
func (m *Metrics) Close() error {
	if m.provider != nil {
		return m.provider.Close()
	}
	return nil
}

// GetDefaultMetricsRegistry возвращает глобальный Prometheus DefaultGatherer.
// Сохраняется для обратной совместимости.
func GetDefaultMetricsRegistry() prometheus.Gatherer {
	return prometheus.DefaultGatherer
}
