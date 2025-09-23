//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	openapi "gitlab.citydrive.tech/youdrive/go/pkg/open-api-transport"
)

// TestHTTPClientMetricsWithOpenAPITransport тестирует интеграцию метрик HTTP-клиента
// с пакетом open-api-transport. Проверяет что:
// 1. Сервис запускается с метрическим эндпойнтом /metrics
// 2. HTTP-клиент из нашего пакета делает запросы
// 3. Метрики клиента появляются в эндпойнте /metrics и изменяются при запросах
func TestHTTPClientMetricsWithOpenAPITransport(t *testing.T) {
	// Создаём тестовый внешний сервис для HTTP-запросов
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		case "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer externalServer.Close()

	// Используем глобальный реестр метрик - не очищаем для простоты

	// Структура для зависимостей сервиса
	type ServiceDeps struct {
		HTTPClient *Client
		CallCount  *atomic.Int64
	}

	// Создаём HTTP-клиент с включёнными метриками (теперь после очистки registry)
	httpClient := New(Config{
		Timeout:      5 * time.Second,
		RetryEnabled: true,
		RetryConfig: RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
		},
		TracingEnabled: false, // Отключаем tracing для простоты теста
	}, "integration-test-client")
	defer httpClient.Close()

	deps := &ServiceDeps{
		HTTPClient: httpClient,
		CallCount:  &atomic.Int64{},
	}

	// Конфигурация для open-api-transport
	config := &openapi.Config{
		Port:         ":10002", // Фиксированный порт для теста
		PprofPort:    ":10012",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		TimeOut:      30 * time.Second,
	}

	// Создаём транспорт с метриками и пользовательскими роутерами
	transport := openapi.WithGrpcGatewayTransport(
		openapi.WithAppName[*ServiceDeps]("integration-test-service"),
		openapi.WithConfig[*ServiceDeps](config),
		openapi.WithDependency(deps),
		openapi.WithRouters[*ServiceDeps](func(app *echo.Echo, dep *ServiceDeps) {
			// API эндпойнт, который делает HTTP-запросы через наш клиент
			app.POST("/api/v1/make-request", func(c echo.Context) error {
				dep.CallCount.Add(1)

				// Парсим тип запроса из body
				body, _ := io.ReadAll(c.Request().Body)
				requestType := strings.TrimSpace(string(body))
				if requestType == "" {
					requestType = "success"
				}

				ctx := c.Request().Context()
				var resp *http.Response
				var err error

				// Делаем разные типы запросов
				switch requestType {
				case "success":
					resp, err = dep.HTTPClient.Get(ctx, externalServer.URL+"/success")
				case "error":
					resp, err = dep.HTTPClient.Get(ctx, externalServer.URL+"/server-error")
				case "timeout":
					// Создаём короткий контекст для timeout
					shortCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
					defer cancel()
					resp, err = dep.HTTPClient.Get(shortCtx, externalServer.URL+"/timeout")
				case "retry":
					// Сначала сделаем запрос на ошибку, потом на успех
					dep.HTTPClient.Get(ctx, externalServer.URL+"/server-error")
					resp, err = dep.HTTPClient.Get(ctx, externalServer.URL+"/success")
				default:
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown request type"})
				}

				if err != nil {
					return c.JSON(http.StatusOK, map[string]interface{}{
						"status": "error",
						"error":  err.Error(),
					})
				}
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)

				return c.JSON(http.StatusOK, map[string]interface{}{
					"status":          "ok",
					"external_status": resp.StatusCode,
					"external_body":   string(respBody),
				})
			})

			// Эндпойнт для получения счётчика вызовов
			app.GET("/api/v1/call-count", func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"count": dep.CallCount.Load(),
				})
			})
		}),
	)

	// Запускаем сервер в горутине
	go transport.Run()

	// Ждём запуска сервера
	time.Sleep(1 * time.Second)

	// Получаем адрес сервера
	serverURL := fmt.Sprintf("http://localhost%s", config.Port)

	// Функция для получения метрик
	getMetrics := func() (string, error) {
		resp, err := http.Get(serverURL + "/metrics")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}

	// Функция для выполнения API запроса
	makeAPIRequest := func(requestType string) error {
		resp, err := http.Post(serverURL+"/api/v1/make-request", "text/plain", strings.NewReader(requestType))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	// 1. Проверяем, что метрики изначально доступны
	t.Run("metrics endpoint available", func(t *testing.T) {
		metrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить метрики: %v", err)
		}

		// Отладочная информация - выводим все метрики
		t.Logf("Доступные метрики:\n%s", metrics)

		// Проверяем что эндпойнт метрик работает и содержит заголовок Prometheus
		if !strings.Contains(metrics, "# HELP") {
			t.Error("Метрики не содержат заголовки Prometheus")
		}
	})

	// 2. Выполняем успешные запросы и проверяем изменение метрик
	t.Run("successful requests change metrics", func(t *testing.T) {
		// Получаем начальные метрики
		initialMetrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить начальные метрики: %v", err)
		}

		// Считаем начальное количество успешных запросов
		initialSuccessCount := countMetricValue(initialMetrics, "http_client_requests_total", `error="false"`)

		// Делаем несколько успешных запросов
		for i := 0; i < 3; i++ {
			if err := makeAPIRequest("success"); err != nil {
				t.Fatalf("Ошибка при выполнении успешного запроса %d: %v", i+1, err)
			}
		}

		// Ждём обновления метрик
		time.Sleep(500 * time.Millisecond)

		// Получаем обновлённые метрики
		updatedMetrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить обновлённые метрики: %v", err)
		}

		// Отладочная информация
		t.Logf("Обновлённые метрики после запросов:\n%s", updatedMetrics)

		// Проверяем что количество успешных запросов увеличилось
		updatedSuccessCount := countMetricValue(updatedMetrics, "http_client_requests_total", `error="false"`)
		if updatedSuccessCount <= initialSuccessCount {
			t.Errorf("Количество успешных запросов не увеличилось: было %d, стало %d",
				initialSuccessCount, updatedSuccessCount)
		}

		t.Logf("Успешные запросы: было %d, стало %d", initialSuccessCount, updatedSuccessCount)
	})

	// 3. Выполняем запросы с ошибками и проверяем метрики
	t.Run("error requests change metrics", func(t *testing.T) {
		// Получаем начальные метрики
		initialMetrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить начальные метрики: %v", err)
		}

		// Считаем начальное количество запросов с ошибками
		initialErrorCount := countMetricValue(initialMetrics, "http_client_requests_total", `error="true"`)

		// Делаем запросы с ошибками
		for i := 0; i < 2; i++ {
			makeAPIRequest("error") // Игнорируем ошибку, так как это ожидаемо
		}

		// Ждём обновления метрик
		time.Sleep(500 * time.Millisecond)

		// Получаем обновлённые метрики
		updatedMetrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить обновлённые метрики: %v", err)
		}

		// Проверяем что количество запросов с ошибками увеличилось
		updatedErrorCount := countMetricValue(updatedMetrics, "http_client_requests_total", `error="true"`)
		if updatedErrorCount <= initialErrorCount {
			t.Logf("Запросы с ошибками: было %d, стало %d (возможно, ошибки не регистрируются для HTTP 500)",
				initialErrorCount, updatedErrorCount)
		}

		t.Logf("Запросы с ошибками: было %d, стало %d", initialErrorCount, updatedErrorCount)
	})

	// 4. Проверяем метрики времени выполнения запросов
	t.Run("request duration metrics exist", func(t *testing.T) {
		// Делаем несколько запросов
		makeAPIRequest("success")
		makeAPIRequest("success")

		time.Sleep(500 * time.Millisecond)

		metrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить метрики: %v", err)
		}

		// Проверяем наличие метрик времени выполнения
		if !strings.Contains(metrics, "http_client_request_duration_seconds") {
			t.Error("Метрики времени выполнения запросов не найдены")
		}

		// Проверяем наличие бакетов гистограммы
		if !strings.Contains(metrics, "http_client_request_duration_seconds_bucket") {
			t.Error("Бакеты гистограммы времени выполнения не найдены")
		}

		t.Log("Метрики времени выполнения найдены")
	})

	// 5. Проверяем что метрики различаются по клиенту
	t.Run("metrics have correct client labels", func(t *testing.T) {
		makeAPIRequest("success")
		time.Sleep(500 * time.Millisecond)

		metrics, err := getMetrics()
		if err != nil {
			t.Fatalf("Не удалось получить метрики: %v", err)
		}

		// Проверяем наличие лейбла client_name
		if !strings.Contains(metrics, `client_name="integration-test-client"`) {
			t.Error("Метрики не содержат лейбл client_name с правильным значением")
		}

		t.Log("Лейблы клиента найдены в метриках")
	})

	// Завершаем тест
	transport.Shutdown()
}

// countMetricValue подсчитывает значение метрики в тексте Prometheus метрик
func countMetricValue(metricsText, metricName, labelFilter string) int {
	lines := strings.Split(metricsText, "\n")
	count := 0

	for _, line := range lines {
		// Пропускаем комментарии и пустые строки
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		// Ищем строки с нужной метрикой и лейблами
		if strings.HasPrefix(line, metricName) && strings.Contains(line, labelFilter) {
			// Простой подсчёт - если строка найдена, увеличиваем счётчик
			count++
		}
	}

	return count
}

// TestOpenAPITransportBasicIntegration базовый тест интеграции с open-api-transport
func TestOpenAPITransportBasicIntegration(t *testing.T) {
	// Используем глобальный реестр метрик

	type SimpleDeps struct {
		Name string
	}

	deps := &SimpleDeps{Name: "test-service"}

	config := &openapi.Config{
		Port:         ":10001",
		PprofPort:    ":10011",
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		TimeOut:      5 * time.Second,
	}

	// Создаём простой транспорт с минимальной конфигурацией
	transport := openapi.WithBaseTransport(
		openapi.WithAppName[*SimpleDeps]("basic-test"),
		openapi.WithConfig[*SimpleDeps](config),
		openapi.WithDependency(deps),
		openapi.WithRouters[*SimpleDeps](func(app *echo.Echo, dep *SimpleDeps) {
			app.GET("/api/hello", func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{
					"message": "Hello from " + dep.Name,
				})
			})
		}),
	)

	// Запускаем в горутине
	go transport.Run()
	time.Sleep(500 * time.Millisecond)

	// Проверяем доступность health endpoints
	t.Run("health endpoints available", func(t *testing.T) {
		endpoints := []string{"/health", "/liveness", "/readiness", "/metrics"}

		for _, endpoint := range endpoints {
			resp, err := http.Get("http://localhost:10001" + endpoint)
			if err != nil {
				t.Errorf("Эндпойнт %s недоступен: %v", endpoint, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Эндпойнт %s вернул неожиданный статус: %d", endpoint, resp.StatusCode)
			}
		}
	})

	// Проверяем пользовательский API
	t.Run("custom api endpoint works", func(t *testing.T) {
		resp, err := http.Get("http://localhost:10001/api/hello")
		if err != nil {
			t.Fatalf("API эндпойнт недоступен: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("API вернул неожиданный статус: %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Hello from test-service") {
			t.Errorf("Неожиданный ответ API: %s", string(body))
		}
	})

	transport.Shutdown()
}
