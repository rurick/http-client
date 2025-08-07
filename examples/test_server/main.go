package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

//go:embed index.html
var indexHTMLFS embed.FS

// Config содержит настройки сервера
type Config struct {
	Port            int    `json:"port"`
	Host            string `json:"host"`
	MetricsEndpoint string `json:"metrics_endpoint"`
}

// RequestData структура для входящих запросов
type RequestData struct {
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// ResponseData структура для ответов
type ResponseData struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Echo      map[string]interface{} `json:"echo,omitempty"`
}

// TestServer представляет тестовый HTTP сервер
type TestServer struct {
	config           *Config
	client           *httpclient.Client
	logger           *zap.Logger
	requestCount     int
	metricsExporter  *prometheus.Exporter
	requestCounter   metric.Int64Counter
	latencyHistogram metric.Float64Histogram
	startTime        time.Time
}

// NewTestServer создает новый тестовый сервер
func NewTestServer(config *Config) *TestServer {
	logger, _ := zap.NewDevelopment()

	// Создаем Prometheus exporter для OpenTelemetry
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal("Ошибка создания Prometheus exporter:", err)
	}

	// Настраиваем OpenTelemetry metric provider
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	// Получаем meter для создания метрик
	//meter := otel.Meter("test_server")
	//
	//// Создаем метрики
	//requestCounter, err := meter.Int64Counter(
	//	"test_server_requests_total",
	//	metric.WithDescription("Общее количество запросов к тестовому серверу"),
	//)
	//if err != nil {
	//	log.Fatal("Ошибка создания счетчика запросов:", err)
	//}
	//
	//latencyHistogram, err := meter.Float64Histogram(
	//	"test_server_request_duration_seconds",
	//	metric.WithDescription("Время обработки запросов в секундах"),
	//	metric.WithUnit("s"),
	//)
	if err != nil {
		log.Fatal("Ошибка создания гистограммы латентности:", err)
	}

	// Создаем HTTP клиент с метриками
	client, err := httpclient.NewClient(
		httpclient.WithTimeout(30*time.Second),
		httpclient.WithRetryMax(3),
		httpclient.WithLogger(logger),
		httpclient.WithMetricsMeterName("testserver"),
	)
	if err != nil {
		log.Fatal("Ошибка создания HTTP клиента:", err)
	}

	return &TestServer{
		config:          config,
		client:          client,
		logger:          logger,
		metricsExporter: exporter,
		//requestCounter:   requestCounter,
		//latencyHistogram: latencyHistogram,
		startTime: time.Now(),
	}
}

// handleIndex возвращает HTML страницу для тестирования
func (ts *TestServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Загружаем шаблон из embed.FS
	content, err := indexHTMLFS.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Ошибка загрузки index.html", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("index").Parse(string(content))
	if err != nil {
		http.Error(w, "Ошибка парсинга шаблона", http.StatusInternalServerError)
		return
	}

	data := struct {
		Port int
	}{
		Port: ts.config.Port,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := tmpl.Execute(w, data); err != nil {
		ts.logger.Error("Ошибка рендеринга шаблона", zap.Error(err))
	}
}

// handleTest обрабатывает тестовые GET/POST запросы
func (ts *TestServer) handleTest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	ts.requestCount++

	// Инкрементируем счетчик запросов в OpenTelemetry
	ts.requestCounter.Add(r.Context(), 1,
		metric.WithAttributes(
		// Можно добавить лейблы, например method
		),
	)

	ts.logger.Info("Получен тестовый запрос",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("request_count", ts.requestCount),
	)

	var requestData RequestData
	var response ResponseData

	switch r.Method {
	case http.MethodGet:
		message := r.URL.Query().Get("message")
		if message == "" {
			message = "GET запрос получен"
		}

		response = ResponseData{
			Status:    "success",
			Message:   message,
			Timestamp: time.Now(),
		}

	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Ошибка парсинга JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		response = ResponseData{
			Status:    "success",
			Message:   fmt.Sprintf("POST запрос получен: %s", requestData.Message),
			Timestamp: time.Now(),
			Echo:      requestData.Data,
		}

	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Записываем латентность в OpenTelemetry
	duration := time.Since(startTime).Seconds()
	ts.latencyHistogram.Record(r.Context(), duration,
		metric.WithAttributes(
		// Можно добавить лейблы, например status_code
		),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleEcho возвращает параметры запроса
func (ts *TestServer) handleEcho(w http.ResponseWriter, r *http.Request) {
	params := make(map[string]interface{})

	// Собираем все параметры запроса
	for key, values := range r.URL.Query() {
		if len(values) == 1 {
			params[key] = values[0]
		} else {
			params[key] = values
		}
	}

	response := ResponseData{
		Status:    "success",
		Message:   "Эхо параметров запроса",
		Timestamp: time.Now(),
		Echo:      params,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStatus возвращает статус сервера
func (ts *TestServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Используем стандартный Prometheus handler для вывода метрик
	promhttp.Handler().ServeHTTP(w, r)
}

// handleMetrics serves Prometheus metrics via promhttp
func (ts *TestServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

// setupRoutes настраивает маршруты сервера
func (ts *TestServer) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", ts.handleIndex)
	mux.HandleFunc("/api/test", ts.handleTest)
	mux.HandleFunc("/api/echo", ts.handleEcho)
	mux.HandleFunc("/api/status", ts.handleStatus)
	mux.HandleFunc(ts.config.MetricsEndpoint, ts.handleMetrics)

	return mux
}

// Start запускает сервер
func (ts *TestServer) Start() error {
	mux := ts.setupRoutes()

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", ts.config.Host, ts.config.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ts.logger.Info("Получен сигнал завершения, останавливаем сервер...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			ts.logger.Error("Ошибка остановки сервера", zap.Error(err))
		}
	}()

	ts.logger.Info("Запуск тестового HTTP сервера",
		zap.String("host", ts.config.Host),
		zap.Int("port", ts.config.Port),
		zap.String("metrics", ts.config.MetricsEndpoint),
	)

	fmt.Printf("🚀 Тестовый сервер запущен на http://%s:%d\n", ts.config.Host, ts.config.Port)
	fmt.Printf("📊 Метрики доступны по адресу: http://%s:%d%s\n", ts.config.Host, ts.config.Port, ts.config.MetricsEndpoint)

	return server.ListenAndServe()
}

func main() {
	// Конфигурация по умолчанию
	config := &Config{
		Port:            8080,
		Host:            "0.0.0.0",
		MetricsEndpoint: "/metrics",
	}

	// Создаем и запускаем сервер
	server := NewTestServer(config)

	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatal("Ошибка запуска сервера:", err)
	}

	fmt.Println("Сервер остановлен")
}
