package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
	fmt.Println("=== HTTP Client Tracing Example ===")

	// Настройка OpenTelemetry трейсинга для демо
	setupTracing()

	// Создание HTTP клиента с трейсингом
	client, err := httpclient.NewClient()
	if err != nil {
		log.Fatal("Ошибка создания клиента:", err)
	}

	// Пример 1: Простой запрос с трейсингом
	simpleTracingExample(client)

	// Пример 2: Вложенные spans
	nestedSpansExample(client)

	// Пример 3: Трейсинг с ошибками
	errorTracingExample(client)

	fmt.Println("\nВсе traces отправлены в stdout. В продакшене используйте Jaeger/Zipkin экспортеры.")
}

// setupTracing настраивает OpenTelemetry для демонстрации
func setupTracing() {
	// Создаем stdout экспортер для демо
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal("Ошибка создания trace экспортера:", err)
	}

	// Создаем trace provider
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("tracing-example"),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)

	// Устанавливаем глобальный trace provider
	otel.SetTracerProvider(tp)
}

// simpleTracingExample демонстрирует базовый трейсинг
func simpleTracingExample(client httpclient.HTTPClient) {
	fmt.Println("\n1. Простой трейсинг HTTP запроса")

	ctx := context.Background()
	tracer := otel.Tracer("example")

	// Создаем родительский span
	ctx, span := tracer.Start(ctx, "simple_http_request")
	defer span.End()

	// HTTP клиент автоматически создаст дочерний span
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		span.RecordError(err)
		fmt.Printf("Ошибка запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Успешный запрос: %s\n", resp.Status)
	span.SetAttributes(attribute.String("response.status", resp.Status))
}

// nestedSpansExample показывает работу с вложенными spans
func nestedSpansExample(client httpclient.HTTPClient) {
	fmt.Println("\n2. Вложенные spans для множественных запросов")

	ctx := context.Background()
	tracer := otel.Tracer("example")

	// Родительский span для бизнес-логики
	ctx, mainSpan := tracer.Start(ctx, "fetch_user_data")
	defer mainSpan.End()

	// Span для получения основной информации пользователя
	ctx, userSpan := tracer.Start(ctx, "get_user_info")
	resp1, err := client.Get("https://httpbin.org/json")
	if err == nil {
		defer resp1.Body.Close()
		fmt.Printf("Получены данные пользователя: %s\n", resp1.Status)
	}
	userSpan.End()

	// Span для получения настроек пользователя
	ctx, settingsSpan := tracer.Start(ctx, "get_user_settings")
	resp2, err := client.Get("https://httpbin.org/headers")
	if err == nil {
		defer resp2.Body.Close()
		fmt.Printf("Получены настройки пользователя: %s\n", resp2.Status)
	}
	settingsSpan.End()

	mainSpan.SetAttributes(
		attribute.Bool("user_data.success", true),
		attribute.Int("requests.count", 2),
	)
}

// errorTracingExample демонстрирует трейсинг с ошибками
func errorTracingExample(client httpclient.HTTPClient) {
	fmt.Println("\n3. Трейсинг с обработкой ошибок")

	ctx := context.Background()
	tracer := otel.Tracer("example")

	ctx, span := tracer.Start(ctx, "error_handling_example")
	defer span.End()

	// Запрос к несуществующему URL для демонстрации ошибки
	resp, err := client.Get("https://httpbin.org/status/500")
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.type", "http_error"))
		fmt.Printf("Ожидаемая ошибка: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("Неожиданный успех: %s\n", resp.Status)
	}

	// Добавляем дополнительные атрибуты для отладки
	span.SetAttributes(
		attribute.String("request.url", "https://httpbin.org/status/500"),
		attribute.String("test.scenario", "error_handling"),
	)
}
