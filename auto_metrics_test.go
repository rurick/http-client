package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TestAutoMetricsRegistration проверяет автоматическую регистрацию метрик.
func TestAutoMetricsRegistration(t *testing.T) {
	// Создаём HTTP сервер для тестов
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Создаём первый клиент - должен инициализировать метрики
	client1 := New(Config{}, "test-client-1")
	defer client1.Close()

	// Создаём второй клиент - метрики уже должны быть инициализированы
	client2 := New(Config{}, "test-client-2")
	defer client2.Close()

	// Делаем запросы от обоих клиентов
	ctx := context.Background()
	
	resp, err := client1.Get(ctx, server.URL+"/test1")
	if err != nil {
		t.Fatalf("Ошибка запроса client1: %v", err)
	}
	resp.Body.Close()

	resp, err = client2.Get(ctx, server.URL+"/test2")
	if err != nil {
		t.Fatalf("Ошибка запроса client2: %v", err)
	}
	resp.Body.Close()

	// Проверяем что метрики зарегистрированы в DefaultRegistry
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Ошибка получения метрик: %v", err)
	}

	// Ищем наши метрики
	foundMetrics := make(map[string]bool)
	expectedMetrics := []string{
		"http_client_requests_total",
		"http_client_request_duration_seconds", 
		"http_client_retries_total",
		"http_client_inflight_requests",
		"http_client_request_size_bytes",
		"http_client_response_size_bytes",
	}

	for _, mf := range metricFamilies {
		name := mf.GetName()
		for _, expected := range expectedMetrics {
			if name == expected {
				foundMetrics[expected] = true
				
				// Проверяем что есть метрики для обоих клиентов
				hasClient1 := false
				hasClient2 := false
				
				for _, metric := range mf.GetMetric() {
					for _, label := range metric.GetLabel() {
						if label.GetName() == "client_name" {
							value := label.GetValue()
							if value == "test-client-1" {
								hasClient1 = true
							} else if value == "test-client-2" {
								hasClient2 = true
							}
						}
					}
				}
				
				if !hasClient1 {
					t.Errorf("Метрика %s не содержит данные для test-client-1", name)
				}
				if !hasClient2 {
					t.Errorf("Метрика %s не содержит данные для test-client-2", name)
				}
			}
		}
	}

	// Проверяем что все ожидаемые метрики найдены
	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("Метрика %s не найдена в DefaultRegistry", expected)
		}
	}
}

// TestMetricsDisabled проверяет что метрики можно отключить.
func TestMetricsDisabled(t *testing.T) {
	// Сбрасываем глобальные метрики для чистого теста
	// (в реальном коде это невозможно, но для теста можем)
	oldMetrics := globalMetrics
	oldOnce := globalMetricsOnce
	defer func() {
		globalMetrics = oldMetrics
		globalMetricsOnce = oldOnce
	}()
	
	// Создаём клиент с отключенными метриками
	disabled := false
	client := New(Config{
		MetricsEnabled: &disabled,
	}, "disabled-client")
	defer client.Close()

	// Проверяем что метрики отключены
	if client.metrics.enabled {
		t.Error("Метрики должны быть отключены")
	}
	
	// Проверяем что globalMetrics не инициализированы
	if globalMetrics != nil {
		t.Error("Глобальные метрики не должны быть инициализированы при отключенных метриках")
	}
}

// TestDefaultMetricsRegistry проверяет функцию GetDefaultMetricsRegistry.
func TestDefaultMetricsRegistry(t *testing.T) {
	registry := GetDefaultMetricsRegistry()
	if registry == nil {
		t.Error("GetDefaultMetricsRegistry вернул nil")
	}
	
	// Проверяем что это именно DefaultGatherer
	if registry != prometheus.DefaultGatherer {
		t.Error("GetDefaultMetricsRegistry должен возвращать prometheus.DefaultGatherer")
	}
}

// TestPromHttpIntegration проверяет интеграцию с promhttp.Handler().
func TestPromHttpIntegration(t *testing.T) {
	// Создаём клиент для генерации метрик
	client := New(Config{}, "integration-test")
	defer client.Close()
	
	// Делаем запрос
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Ошибка запроса: %v", err)
	}
	resp.Body.Close()
	
	// Создаём HTTP обработчик метрик
	handler := promhttp.Handler()
	
	// Делаем запрос к /metrics
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatalf("Ошибка создания запроса: %v", err)
	}
	
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", recorder.Code)
	}
	
	body := recorder.Body.String()
	
	// Проверяем что в ответе есть наши метрики
	expectedStrings := []string{
		"http_client_requests_total",
		"client_name=\"integration-test\"",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("В ответе /metrics не найдена строка: %s", expected)
		}
	}
}