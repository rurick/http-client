package httpclient

import (
	"context"
	"testing"
)

// TestNoopMetricsProvider тестирует создание и работу NoopMetricsProvider
func TestNoopMetricsProvider(t *testing.T) {
	provider := NewNoopMetricsProvider()
	
	if provider == nil {
		t.Fatal("expected noop provider to be created")
	}
}

// TestNoopMetricsProvider_AllMethods тестирует что все методы NoopMetricsProvider не вызывают панику
func TestNoopMetricsProvider_AllMethods(t *testing.T) {
	provider := NewNoopMetricsProvider()
	ctx := context.Background()
	
	// Все методы должны быть no-op и не паниковать
	provider.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	provider.RecordRequest(ctx, "POST", "api.example.com", "500", true, true)
	
	provider.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	provider.RecordDuration(ctx, 1.2, "POST", "api.example.com", "500", 2)
	
	provider.RecordRetry(ctx, "status", "GET", "example.com")
	provider.RecordRetry(ctx, "timeout", "POST", "api.example.com")
	
	provider.RecordRequestSize(ctx, 1024, "POST", "example.com")
	provider.RecordRequestSize(ctx, 0, "GET", "api.example.com")
	
	provider.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")
	provider.RecordResponseSize(ctx, 512, "POST", "api.example.com", "500")
	
	provider.InflightInc(ctx, "GET", "example.com")
	provider.InflightInc(ctx, "POST", "api.example.com")
	provider.InflightDec(ctx, "GET", "example.com")
	provider.InflightDec(ctx, "POST", "api.example.com")
	
	// Если дошли до сюда без паники, тест пройден
}

// TestNoopMetricsProvider_Close тестирует закрытие NoopMetricsProvider
func TestNoopMetricsProvider_Close(t *testing.T) {
	provider := NewNoopMetricsProvider()
	
	err := provider.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// TestNoopMetricsProvider_EdgeCases тестирует граничные случаи
func TestNoopMetricsProvider_EdgeCases(t *testing.T) {
	provider := NewNoopMetricsProvider()
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

// TestMetricsWithDisabledBackend тестирует интеграцию с отключенными метриками
func TestMetricsWithDisabledBackend(t *testing.T) {
	// Создаём клиент с отключенными метриками
	enabled := false
	client := New(Config{
		MetricsEnabled: &enabled,
	}, "disabled-metrics-client")
	defer client.Close()
	
	// Проверяем что метрики инициализированы но отключены
	if client.metrics == nil {
		t.Fatal("expected metrics to be initialized")
	}
	
	if client.metrics.enabled {
		t.Error("expected metrics to be disabled")
	}
	
	if client.metrics.clientName != "disabled-metrics-client" {
		t.Errorf("expected clientName to be 'disabled-metrics-client', got %s", client.metrics.clientName)
	}
	
	// Тестируем что вызовы метрик не паникуют
	ctx := context.Background()
	client.metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	client.metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	
	// Если дошли до сюда без паники, тест пройден
}