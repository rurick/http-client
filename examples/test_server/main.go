package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

//go:embed index.html
var indexHTMLFS embed.FS

// TestRequest структура для входящих запросов от формы
type TestRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
}

// TestResponse структура для ответов
type TestResponse struct {
	Status     string            `json:"status"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Duration   string            `json:"duration"`
	Error      string            `json:"error,omitempty"`
}

// TestServer представляет тестовый HTTP сервер
type TestServer struct {
	client    *httpclient.Client
	logger    *zap.Logger
	startTime time.Time
}

// NewTestServer создает новый тестовый сервер
func NewTestServer() *TestServer {
	logger, _ := zap.NewDevelopment()

	// Создаем Prometheus exporter для OpenTelemetry
	exporter, err := prometheus.New()
	if err != nil {
		logger.Fatal("Ошибка создания Prometheus exporter", zap.Error(err))
	}

	// Настраиваем OpenTelemetry metric provider
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	// Создаем HTTP клиент с включенными метриками
	client, err := httpclient.NewClient(
		httpclient.WithMetrics(true),
		httpclient.WithMetricsMeterName("test-server-http-client"),
		httpclient.WithTimeout(30*time.Second),
	)
	if err != nil {
		logger.Fatal("Ошибка создания HTTP клиента", zap.Error(err))
	}

	return &TestServer{
		client:    client,
		logger:    logger,
		startTime: time.Now(),
	}
}

// handleIndex обрабатывает главную страницу
func (ts *TestServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	content, err := indexHTMLFS.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Ошибка загрузки страницы", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// handleAPITest обрабатывает тестовые запросы
func (ts *TestServer) handleAPITest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения запроса", http.StatusBadRequest)
		return
	}

	var testReq TestRequest
	if err := json.Unmarshal(body, &testReq); err != nil {
		http.Error(w, "Некорректный JSON", http.StatusBadRequest)
		return
	}

	// Выполняем запрос через наш HTTP клиент
	response := ts.executeRequest(testReq)

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// executeRequest выполняет HTTP запрос через наш клиент
func (ts *TestServer) executeRequest(testReq TestRequest) TestResponse {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Создаем HTTP запрос
	var reqBody io.Reader
	if testReq.Body != "" {
		reqBody = strings.NewReader(testReq.Body)
	}

	req, err := http.NewRequestWithContext(ctx, testReq.Method, testReq.URL, reqBody)
	if err != nil {
		return TestResponse{
			Status:   "error",
			Error:    fmt.Sprintf("Ошибка создания запроса: %v", err),
			Duration: time.Since(start).String(),
		}
	}

	// Если есть тело запроса, устанавливаем Content-Type
	if testReq.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Выполняем запрос
	resp, err := ts.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return TestResponse{
			Status:   "error",
			Error:    fmt.Sprintf("Ошибка выполнения запроса: %v", err),
			Duration: duration.String(),
		}
	}
	defer resp.Body.Close()

	// Читаем ответ
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TestResponse{
			Status:     "error",
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("Ошибка чтения ответа: %v", err),
			Duration:   duration.String(),
		}
	}

	// Собираем заголовки
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return TestResponse{
		Status:     "success",
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(respBody),
		Duration:   duration.String(),
	}
}

// setupRoutes настраивает маршруты сервера
func (ts *TestServer) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Главная страница
	mux.HandleFunc("/", ts.handleIndex)

	// API для тестирования запросов
	mux.HandleFunc("/api/test", ts.handleAPITest)

	// Prometheus метрики
	mux.Handle("/metrics", promhttp.Handler())

	return mux
}

func main() {
	// Создаем тестовый сервер
	server := NewTestServer()
	defer server.logger.Sync()

	// Настраиваем маршруты
	mux := server.setupRoutes()

	// Создаем HTTP сервер
	httpServer := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: mux,
	}

	// Запускаем сервер в горутине
	go func() {
		server.logger.Info("Запуск тестового HTTP сервера",
			zap.String("host", "0.0.0.0"),
			zap.Int("port", 8080),
			zap.String("metrics", "/metrics"))

		fmt.Println("🚀 Тестовый сервер запущен на http://0.0.0.0:8080")
		fmt.Println("📊 Метрики доступны по адресу: http://0.0.0.0:8080/metrics")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.logger.Fatal("Ошибка запуска сервера", zap.Error(err))
		}
	}()

	// Ожидаем сигнал для корректного завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	server.logger.Info("Завершение работы сервера...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		server.logger.Fatal("Ошибка завершения сервера", zap.Error(err))
	}

	server.logger.Info("Сервер успешно завершен")
}
