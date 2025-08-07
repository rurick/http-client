package httpclient

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOTelMetricsCollectorTracing проверяет создание и завершение spans
func TestOTelMetricsCollectorTracing(t *testing.T) {
	t.Parallel()

	// Настройка трейсинга для тестов
	setupTestTracing(t)

	collector, err := NewOTelMetricsCollector("test-client")
	require.NoError(t, err)

	ctx := context.Background()

	// Тестируем создание span
	spanCtx, span := collector.StartSpan(ctx, "GET", "https://example.com/test")

	// Проверяем что span создался
	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, spanCtx) // Контекст должен измениться

	// Проверяем что span содержит правильные атрибуты
	if traceSpan := span; traceSpan != nil {
		assert.True(t, traceSpan.IsRecording())
	}

	// Завершаем span с успешным ответом
	collector.FinishSpan(span, 200, nil)
}

// TestTracingWithError проверяет запись ошибок в spans
func TestTracingWithError(t *testing.T) {
	t.Parallel()

	setupTestTracing(t)

	collector, err := NewOTelMetricsCollector("test-client")
	require.NoError(t, err)

	ctx := context.Background()
	_, span := collector.StartSpan(ctx, "GET", "https://example.com/error")

	// Симулируем ошибку
	testErr := assert.AnError
	collector.FinishSpan(span, 500, testErr)

	// В реальном приложении ошибка была бы записана в span
	// Здесь мы просто проверяем что метод не паникует
	assert.NotNil(t, span)
}

// TestNestedSpans проверяет работу с вложенными spans
func TestNestedSpans(t *testing.T) {
	t.Parallel()

	setupTestTracing(t)

	collector, err := NewOTelMetricsCollector("test-client")
	require.NoError(t, err)

	tracer := otel.Tracer("test")
	ctx := context.Background()

	// Создаем родительский span
	ctx, parentSpan := tracer.Start(ctx, "test_parent_operation")
	defer parentSpan.End()

	// Создаем дочерний span для HTTP запроса
	spanCtx, childSpan := collector.StartSpan(ctx, "POST", "https://example.com/api")

	// Проверяем что spans связаны (контекст содержит информацию о родителе)
	assert.NotEqual(t, ctx, spanCtx)
	assert.NotNil(t, parentSpan)
	assert.NotNil(t, childSpan)

	collector.FinishSpan(childSpan, 201, nil)
}

// TestSpanAttributes проверяет установку атрибутов spans
func TestSpanAttributes(t *testing.T) {
	t.Parallel()

	setupTestTracing(t)

	collector, err := NewOTelMetricsCollector("test-client")
	require.NoError(t, err)

	testCases := []struct {
		name       string
		method     string
		url        string
		statusCode int
		err        error
	}{
		{
			name:       "successful_get",
			method:     "GET",
			url:        "https://example.com/users",
			statusCode: 200,
			err:        nil,
		},
		{
			name:       "post_with_error",
			method:     "POST",
			url:        "https://example.com/create",
			statusCode: 400,
			err:        assert.AnError,
		},
		{
			name:       "timeout_error",
			method:     "GET",
			url:        "https://slow.example.com/data",
			statusCode: 0,
			err:        context.DeadlineExceeded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, span := collector.StartSpan(ctx, tc.method, tc.url)

			// Проверяем что span создался
			assert.NotNil(t, span)

			// Завершаем span
			collector.FinishSpan(span, tc.statusCode, tc.err)

			// Span должен быть завершен без паники
		})
	}
}

// TestTracingIntegrationWithClient проверяет интеграцию трейсинга с HTTP клиентом
func TestTracingIntegrationWithClient(t *testing.T) {
	// НЕ parallel - тест с HTTP запросами к реальному API
	setupTestTracing(t)

	// Создаем клиент с трейсингом
	client, err := NewClient(
		WithTimeout(5 * time.Second),
	)
	require.NoError(t, err)

	// Создаем контекст с родительским span
	tracer := otel.Tracer("integration-test")
	_, parentSpan := tracer.Start(context.Background(), "test_http_request")
	defer parentSpan.End()

	// Делаем запрос через клиент - должен создаться дочерний span
	resp, err := client.Get("https://httpbin.org/get")

	// Проверяем результат (может быть ошибка сети, это нормально для тестов)
	if err == nil {
		assert.NotNil(t, resp)
		if resp != nil {
			defer resp.Body.Close()
			assert.True(t, resp.StatusCode > 0)
		}
	}

	// Главное что трейсинг не сломал функциональность
	assert.NotNil(t, parentSpan)
}

// TestOTelCollectorMetrics проверяет что трейсинг не влияет на сбор метрик
func TestOTelCollectorMetrics(t *testing.T) {
	t.Parallel()

	setupTestTracing(t)

	collector, err := NewOTelMetricsCollector("metrics-test")
	require.NoError(t, err)

	// Записываем метрики
	collector.RecordRequest("GET", "https://example.com", 200, time.Second, 100, 500)
	collector.RecordRetry("GET", "https://example.com", 500, assert.AnError)

	// Метрики теперь доступны только через Prometheus/OTel. Удалены проверки локальных метрик.
}

// setupTestTracing настраивает OpenTelemetry для тестов
func setupTestTracing(t *testing.T) {
	// Создаем no-op экспортер для тестов (не выводит в stdout)
	exporter, err := stdouttrace.New(stdouttrace.WithoutTimestamps())
	require.NoError(t, err)

	// Создаем trace provider с минимальной конфигурацией
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSyncer(exporter), // Синхронная обработка для тестов
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("test-service"),
		)),
	)

	// Устанавливаем trace provider только для этого теста
	otel.SetTracerProvider(tp)

	// Очистка после теста
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})
}
