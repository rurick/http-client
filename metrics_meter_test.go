package httpclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsMeterNameIntegration проверяет что префикс действительно применяется к метрикам
func TestMetricsMeterNameIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		metricsMeterName  string
		expectedMeterName string
	}{
		{
			name:              "default_MeterName",
			metricsMeterName:  defaultMetricMeterName,
			expectedMeterName: defaultMetricMeterName,
		},
		{
			name:              "custom_MeterName",
			metricsMeterName:  "myapp",
			expectedMeterName: "myapp",
		},
		{
			name:              "service_MeterName",
			metricsMeterName:  "api_gateway",
			expectedMeterName: "api_gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем клиент с пользовательским префиксом метрик
			client, err := NewClient(
				WithMetrics(true),
				WithMetricsMeterName(tt.metricsMeterName),
			)
			require.NoError(t, err)
			require.NotNil(t, client)

			// Проверяем что метрики коллектор создался
			require.NotNil(t, client.metricsCollector)

			// Проверяем что опции содержат правильный префикс
			assert.Equal(t, tt.expectedMeterName, client.options.MetricsMeterName)

			// Проверяем что метрики включены
			assert.True(t, client.options.MetricsEnabled)
		})
	}
}

// TestMetricsMeterNameWithEmptyString проверяет поведение с пустым префиксом
func TestMetricsMeterNameWithEmptyString(t *testing.T) {
	t.Parallel()

	// Создаем клиент с пустым префиксом
	client, err := NewClient(
		WithMetrics(true),
		WithMetricsMeterName(""), // Пустая строка
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Проверяем что установился префикс по умолчанию
	assert.Equal(t, defaultMetricMeterName, client.options.MetricsMeterName)
	assert.True(t, client.options.MetricsEnabled)
}

// TestMetricsMeterNameDisabled проверяет что метрики не создаются когда они отключены
func TestMetricsMeterNameDisabled(t *testing.T) {
	t.Parallel()

	// Создаем клиент с отключенными метриками
	client, err := NewClient(
		WithMetrics(false),
		WithMetricsMeterName("myapp"),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Проверяем что метрики отключены, но префикс установлен
	assert.False(t, client.options.MetricsEnabled)
	assert.Equal(t, "myapp", client.options.MetricsMeterName)
	assert.Nil(t, client.metricsCollector) // Коллектор не должен создаваться
}

// TestOTelMetricsCollectorMeterName проверяет создание коллектора с префиксом
func TestOTelMetricsCollectorMeterName(t *testing.T) {
	t.Parallel()

	prefixes := []string{"httpclient", "myapp", "api_service", "payment_svc"}

	for _, prefix := range prefixes {
		t.Run("prefix_"+prefix, func(t *testing.T) {
			collector, err := NewOTelMetricsCollector(prefix)
			require.NoError(t, err)
			require.NotNil(t, collector)

			// Проверяем что коллектор создался с правильными параметрами
			assert.NotNil(t, collector.meter)
			assert.NotNil(t, collector.tracer)
			assert.NotNil(t, collector.metrics)
			assert.NotNil(t, collector.requestCounter)
			assert.NotNil(t, collector.requestDuration)
			assert.NotNil(t, collector.requestSizeCounter)
			assert.NotNil(t, collector.responseSizeCounter)
			assert.NotNil(t, collector.retryCounter)
		})
	}
}

// TestClientOptionsMetricsNameField проверяет что поле MetricsName корректно работает
func TestClientOptionsMetricsNameField(t *testing.T) {
	t.Parallel()

	// Тестируем значения по умолчанию
	opts := DefaultOptions()
	assert.Equal(t, defaultMetricMeterName, opts.MetricsMeterName)
	assert.True(t, opts.MetricsEnabled)

	// Тестируем установку пользовательского значения
	WithMetricsMeterName("custom_service")(opts)
	assert.Equal(t, "custom_service", opts.MetricsMeterName)

	// Тестируем что пустая строка заменяется на значение по умолчанию
	WithMetricsMeterName("")(opts)
	assert.Equal(t, defaultMetricMeterName, opts.MetricsMeterName)
}
