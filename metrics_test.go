package httpclient

import (
	"context"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	// Тест работает с уже зарегистрированными метриками
	// (они могли быть созданы в предыдущих тестах)
	
	metrics := NewMetrics("testhttpclient")

	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}
	
	if !metrics.enabled {
		t.Error("expected metrics to be enabled by default")
	}
	
	if metrics.clientName != "testhttpclient" {
		t.Errorf("expected clientName to be 'testhttpclient', got %s", metrics.clientName)
	}
	
	// Проверяем что глобальные метрики инициализированы
	if globalMetrics == nil {
		t.Error("expected global metrics to be initialized")
	}
}

func TestNewDisabledMetrics(t *testing.T) {
	metrics := NewDisabledMetrics("disabled-client")
	
	if metrics == nil {
		t.Fatal("expected metrics to be created")
	}
	
	if metrics.enabled {
		t.Error("expected metrics to be disabled")
	}
	
	if metrics.clientName != "disabled-client" {
		t.Errorf("expected clientName to be 'disabled-client', got %s", metrics.clientName)
	}
}

func TestMetrics_RecordRequest(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест записи метрики запроса - не должно паниковать
	metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	metrics.RecordRequest(ctx, "POST", "api.example.com", "500", true, true)
}

func TestMetricsDisabled_NoOp(t *testing.T) {
	metrics := NewDisabledMetrics("disabled")
	ctx := context.Background()

	// Все операции должны быть no-op и не паниковать
	metrics.RecordRequest(ctx, "GET", "example.com", "200", false, false)
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	metrics.RecordRetry(ctx, "status", "GET", "example.com")
	metrics.IncrementInflight(ctx, "GET", "example.com")
	metrics.DecrementInflight(ctx, "GET", "example.com")
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")
	
	err := metrics.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

func TestMetrics_RecordDuration(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест записи метрики длительности - не должно паниковать
	metrics.RecordDuration(ctx, 0.5, "GET", "example.com", "200", 1)
	metrics.RecordDuration(ctx, 1.2, "POST", "api.example.com", "500", 2)
}

func TestMetrics_RecordRetry(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест записи метрики retry - не должно паниковать
	metrics.RecordRetry(ctx, "status", "GET", "example.com")
	metrics.RecordRetry(ctx, "timeout", "POST", "api.example.com")
}

func TestMetrics_RecordRequestSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест записи метрики размера запроса - не должно паниковать
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")
	metrics.RecordRequestSize(ctx, 0, "GET", "api.example.com")
}

func TestMetrics_RecordResponseSize(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест записи метрики размера ответа - не должно паниковать
	metrics.RecordResponseSize(ctx, 2048, "GET", "example.com", "200")
	metrics.RecordResponseSize(ctx, 512, "POST", "api.example.com", "500")
}

func TestMetrics_Close(t *testing.T) {
	metrics := NewMetrics("testhttpclient")

	err := metrics.Close()
	if err != nil {
		t.Errorf("unexpected error during close: %v", err)
	}
}

// Интеграционный тест с использованием Prometheus метрик
func TestMetrics_Integration(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Симулируем последовательность вызовов метрик как в реальном HTTP запросе

	// 1. Увеличиваем счётчик активных запросов
	metrics.IncrementInflight(ctx, "POST", "example.com")

	// 2. Записываем размер запроса
	metrics.RecordRequestSize(ctx, 1024, "POST", "example.com")

	// 3. Записываем метрику запроса (первая попытка)
	metrics.RecordRequest(ctx, "POST", "example.com", "500", false, true)
	metrics.RecordDuration(ctx, 0.5, "POST", "example.com", "500", 1)

	// 4. Записываем retry
	metrics.RecordRetry(ctx, "status", "POST", "example.com")

	// 5. Записываем метрику запроса (retry попытка)
	metrics.RecordRequest(ctx, "POST", "example.com", "200", true, false)
	metrics.RecordDuration(ctx, 0.3, "POST", "example.com", "200", 2)

	// 6. Записываем размер ответа
	metrics.RecordResponseSize(ctx, 512, "POST", "example.com", "200")

	// 7. Уменьшаем счётчик активных запросов
	metrics.DecrementInflight(ctx, "POST", "example.com")

	// Если дошли до сюда без паники, тест пройден
}

func TestMetrics_EdgeCases(t *testing.T) {
	metrics := NewMetrics("testhttpclient")
	ctx := context.Background()

	// Тест с пустыми значениями
	metrics.RecordRequest(ctx, "", "", "", false, false)
	metrics.RecordDuration(ctx, 0, "", "", "", 0)
	metrics.RecordRetry(ctx, "", "", "")
	metrics.IncrementInflight(ctx, "", "")
	metrics.DecrementInflight(ctx, "", "")
	metrics.RecordRequestSize(ctx, 0, "", "")
	metrics.RecordResponseSize(ctx, 0, "", "", "")

	// Тест с очень большими значениями
	metrics.RecordDuration(ctx, 999999.999, "GET", "example.com", "200", 1)
	metrics.RecordRequestSize(ctx, 1<<60, "POST", "example.com")
	metrics.RecordResponseSize(ctx, 1<<60, "GET", "example.com", "200")

	// Тест работы с inflight метриками
	metrics.IncrementInflight(ctx, "GET", "example.com")
	metrics.DecrementInflight(ctx, "GET", "example.com")
}

// TestGlobalMetricsInitialization проверяет что множественные клиенты работают с одними метриками
func TestGlobalMetricsInitialization(t *testing.T) {
	// Метрики уже могут быть инициализированы предыдущими тестами
	
	// Сохраняем ссылку на текущие метрики
	currentMetrics := globalMetrics
	
	// Клиент 1
	metrics1 := NewMetrics("client-1")
	if globalMetrics == nil {
		t.Error("expected global metrics to be available")
	}
	
	// Клиент 2 должен использовать те же метрики
	metrics2 := NewMetrics("client-2")
	if globalMetrics != currentMetrics && currentMetrics != nil {
		t.Error("global metrics should remain the same between clients")
	}
	
	// Оба клиента используют одни и те же глобальные метрики
	if !metrics1.enabled || !metrics2.enabled {
		t.Error("both clients should have metrics enabled")
	}
	
	if metrics1.clientName != "client-1" {
		t.Errorf("expected client-1 name, got %s", metrics1.clientName)
	}
	
	if metrics2.clientName != "client-2" {
		t.Errorf("expected client-2 name, got %s", metrics2.clientName)
	}
}
