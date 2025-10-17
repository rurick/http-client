package httpclient

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestOpenTelemetryMetricsProvider тестирует создание провайдера OpenTelemetry метрик
func TestOpenTelemetryMetricsProvider(t *testing.T) {
	provider := NewOpenTelemetryMetricsProvider("test-client", nil)
	
	if provider == nil {
		t.Fatal("expected provider to be created")
	}
	
	if provider.clientName != "test-client" {
		t.Errorf("expected clientName to be 'test-client', got %s", provider.clientName)
	}
	
	if provider.inst == nil {
		t.Error("expected instruments to be initialized")
	}
}

// TestOpenTelemetryMetricsProvider_WithCustomMeterProvider тестирует создание с кастомным MeterProvider
func TestOpenTelemetryMetricsProvider_WithCustomMeterProvider(t *testing.T) {
	// Создаём тестовый MeterProvider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("custom-client", meterProvider)
	
	if provider == nil {
		t.Fatal("expected provider to be created")
	}
	
	if provider.clientName != "custom-client" {
		t.Errorf("expected clientName to be 'custom-client', got %s", provider.clientName)
	}
}

// TestOpenTelemetryMetricsProvider_RecordRequest тестирует запись метрик запросов
func TestOpenTelemetryMetricsProvider_RecordRequest(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()
	
	// Записываем несколько метрик
	provider.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	provider.RecordRequest(ctx, "POST", "api.example.com", "500", true, true)
	
	// Проверяем что вызовы не вызывают панику
	// (детальная проверка значений может быть сложной из-за внутренней структуры OpenTelemetry)
}

// TestOpenTelemetryMetricsProvider_RecordDuration тестирует запись метрик длительности
func TestOpenTelemetryMetricsProvider_RecordDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()
	
	// Записываем метрики длительности
	provider.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	provider.RecordDuration(ctx, 1.2, "POST", "api.example.com", "500", 2)
	
	// Проверяем что вызовы не вызывают панику
}

// TestOpenTelemetryMetricsProvider_RecordRetry тестирует запись метрик повторных попыток
func TestOpenTelemetryMetricsProvider_RecordRetry(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()
	
	// Записываем метрики retry
	provider.RecordRetry(ctx, "status", "GET", "example.com")
	provider.RecordRetry(ctx, "timeout", "POST", "api.example.com")
	
	// Проверяем что вызовы не вызывают панику
}

// TestOpenTelemetryMetricsProvider_RecordSizes тестирует запись метрик размеров
func TestOpenTelemetryMetricsProvider_RecordSizes(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()
	
	// Записываем метрики размеров
	provider.RecordRequestSize(ctx, 1024, "POST", "example.com")
	provider.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")
	
	// Проверяем что вызовы не вызывают панику
}

// TestOpenTelemetryMetricsProvider_Inflight тестирует метрики активных запросов
func TestOpenTelemetryMetricsProvider_Inflight(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("test-client", meterProvider)
	ctx := context.Background()
	
	// Тестируем инкремент/декремент активных запросов
	provider.InflightInc(ctx, "GET", "example.com")
	provider.InflightInc(ctx, "POST", "api.example.com")
	provider.InflightDec(ctx, "GET", "example.com")
	provider.InflightDec(ctx, "POST", "api.example.com")
	
	// Проверяем что вызовы не вызывают панику
}

// TestOpenTelemetryMetricsProvider_Close тестирует закрытие провайдера
func TestOpenTelemetryMetricsProvider_Close(t *testing.T) {
	provider := NewOpenTelemetryMetricsProvider("test-client", nil)
	
	err := provider.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// TestOpenTelemetryMetricsProvider_Integration интеграционный тест с реальной последовательностью вызовов
func TestOpenTelemetryMetricsProvider_Integration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("integration-test", meterProvider)
	ctx := context.Background()
	
	// Симулируем последовательность вызовов метрик как в реальном HTTP запросе
	
	// 1. Увеличиваем счётчик активных запросов
	provider.InflightInc(ctx, "POST", "example.com")
	
	// 2. Записываем размер запроса
	provider.RecordRequestSize(ctx, 1024, "POST", "example.com")
	
	// 3. Записываем метрику запроса (первая попытка)
	provider.RecordRequest(ctx, "POST", "example.com", "500", false, true)
	provider.RecordDuration(ctx, 0.5, "POST", "example.com", "500", 1)
	
	// 4. Записываем retry
	provider.RecordRetry(ctx, "status", "POST", "example.com")
	
	// 5. Записываем метрику запроса (retry попытка)
	provider.RecordRequest(ctx, "POST", "example.com", "200", true, false)
	provider.RecordDuration(ctx, 0.3, "POST", "example.com", "200", 2)
	
	// 6. Записываем размер ответа
	provider.RecordResponseSize(ctx, 512, "POST", "example.com", "200")
	
	// 7. Уменьшаем счётчик активных запросов
	provider.InflightDec(ctx, "POST", "example.com")
	
	// Если дошли до сюда без паники, тест пройден
}

// TestOpenTelemetryMetricsProvider_EdgeCases тестирует граничные случаи
func TestOpenTelemetryMetricsProvider_EdgeCases(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	provider := NewOpenTelemetryMetricsProvider("edge-cases", meterProvider)
	ctx := context.Background()
	
	// Тест с пустыми значениями
	provider.RecordRequest(ctx, "", "", "", false, false)
	provider.RecordDuration(ctx, 0, "", "", "", 0)
	provider.RecordRetry(ctx, "", "", "")
	provider.InflightInc(ctx, "", "")
	provider.InflightDec(ctx, "", "")
	provider.RecordRequestSize(ctx, 0, "", "")
	provider.RecordResponseSize(ctx, 0, "", "", "")
	
	// Тест с очень большими значениями
	provider.RecordDuration(ctx, 999999.999, "GET", "example.com", "200", 1)
	provider.RecordRequestSize(ctx, 1<<60, "POST", "example.com")
	provider.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "200")
	
	// Тест с отрицательными значениями (для inflight)
	provider.InflightDec(ctx, "GET", "example.com") // декремент без инкремента
	
	// Если дошли до сюда без паники, тест пройден
}

// TestOpenTelemetryMetricsProvider_MultipleClients тестирует работу с несколькими клиентами
func TestOpenTelemetryMetricsProvider_MultipleClients(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	// Создаём несколько провайдеров с одним MeterProvider
	provider1 := NewOpenTelemetryMetricsProvider("client-1", meterProvider)
	provider2 := NewOpenTelemetryMetricsProvider("client-2", meterProvider)
	
	ctx := context.Background()
	
	// Проверяем что у каждого провайдера свой clientName
	if provider1.clientName != "client-1" {
		t.Errorf("expected client-1 name, got %s", provider1.clientName)
	}
	if provider2.clientName != "client-2" {
		t.Errorf("expected client-2 name, got %s", provider2.clientName)
	}
	
	// Записываем метрики от разных клиентов
	provider1.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	provider2.RecordRequest(ctx, "POST", "api.example.com", "201", false, false)
	
	// Проверяем что оба используют одни инструменты (кеширование работает)
	if provider1.inst != provider2.inst {
		t.Error("providers should share the same instruments when using the same MeterProvider")
	}
}

// TestMetricsWithOpenTelemetryBackend тестирует интеграцию через Config
func TestMetricsWithOpenTelemetryBackend(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer meterProvider.Shutdown(context.Background())
	
	// Создаём клиент с OpenTelemetry бэкендом
	client := New(Config{
		MetricsBackend:    MetricsBackendOpenTelemetry,
		OTelMeterProvider: meterProvider,
	}, "test-otel-client")
	defer client.Close()
	
	// Проверяем что метрики инициализированы
	if client.metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}
	
	if !client.metrics.enabled {
		t.Error("expected metrics to be enabled")
	}
	
	if client.metrics.clientName != "test-otel-client" {
		t.Errorf("expected clientName to be 'test-otel-client', got %s", client.metrics.clientName)
	}
	
	// Тестируем запись метрик через клиент
	ctx := context.Background()
	client.metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	client.metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	
	// Если дошли до сюда без паники, тест пройден
}