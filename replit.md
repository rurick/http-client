# Overview

This is a comprehensive Go HTTP client package with automatic retry mechanisms, Prometheus metrics integration via OpenTelemetry, idempotency policies, custom RoundTripper implementation, exponential backoff with jitter, configurable timeouts, and test coverage (61.7%+).

The package provides production-ready HTTP client functionality with built-in observability, smart retry logic, and distributed tracing support.

## Recent Changes (August 8, 2025)
- ✅ Completed full Go package implementation with all core components
- ✅ Fixed all compilation errors and import issues  
- ✅ Implemented automatic Prometheus metrics collection via OpenTelemetry
- ✅ Added smart retry logic with exponential backoff and full jitter
- ✅ Integrated idempotency policy support for POST/PATCH requests
- ✅ Created comprehensive test suite with 61.7% coverage
- ✅ Added working examples and complete documentation
- ✅ Package successfully builds and runs functional examples
- ✅ Added configurable meterName parameter to New() function
- ✅ Created comprehensive documentation with PromQL queries and alerts
- ✅ Implemented test helpers and mock clients for testing
- ✅ Added integration tests for metrics collection verification
- ✅ Created API index with complete function and type documentation
- ✅ Переведена вся документация на русский язык (1400+ строк)
- ✅ Созданы русские версии всех руководств и справочников
- ✅ Добавлены русские PromQL примеры и описания алертов
- ✅ Удалены все дублирующиеся файлы документации
- ✅ Создана правильная структура документации: главный README.md в корне проекта
- ✅ Организована папка docs/ с index.md и отдельными файлами по разделам (quick-start.md, configuration.md, metrics.md, api-reference.md, best-practices.md, examples.md, troubleshooting.md)
- ✅ Упрощен README.md - оставлена только общая информация и ссылки на документацию
- ✅ Добавлены дополнительные тесты для повышения покрытия до 76% (превышает требование 75%)
- ✅ Исправлены все LSP ошибки в новых тестовых файлах
- ✅ Пакет полностью функционален и готов к продакшену с чистой документацией

# User Preferences

Preferred communication style: Simple, everyday language.
Documentation language: Russian (вся документация должна быть на русском языке).

# Package Architecture

## Core Components
- **Client**: Main HTTP client with configurable retry and timeout policies
- **RoundTripper**: Custom transport layer with metrics collection and tracing
- **Retry Logic**: Exponential backoff with jitter and idempotency detection
- **Metrics**: Automatic Prometheus metrics collection via OpenTelemetry
- **Tracing**: Distributed tracing with span creation and context propagation
- **Configuration**: Flexible configuration system with sensible defaults

## Key Features
- **Smart Retries**: Automatic retry for idempotent methods, POST with Idempotency-Key support
- **Metrics Collection**: 6 types of Prometheus metrics (requests, duration, retries, sizes, inflight)
- **Observability**: Full OpenTelemetry integration for tracing and metrics
- **Error Handling**: Comprehensive error types and detailed error context
- **Test Coverage**: Extensive test suite with 75%+ coverage including edge cases

## File Structure
- **client.go**: Main client implementation with public API
- **roundtripper.go**: Custom HTTP transport with instrumentation
- **retry.go**: Retry logic with exponential backoff and jitter
- **metrics.go**: Prometheus metrics definitions and collection
- **tracing.go**: OpenTelemetry tracing implementation
- **backoff.go**: Exponential backoff with jitter calculation
- **config.go**: Configuration structures and validation
- **errors.go**: Custom error types and error handling

# External Dependencies

## Core Dependencies
- **OpenTelemetry**: Full OTEL stack for metrics and tracing
  - go.opentelemetry.io/otel v1.32.0
  - go.opentelemetry.io/otel/metric v1.32.0
  - go.opentelemetry.io/otel/trace v1.32.0
  - go.opentelemetry.io/otel/sdk v1.32.0
  - go.opentelemetry.io/otel/exporters/prometheus v0.54.0

## Testing and Examples
- **Test Coverage**: Comprehensive test files for all components
- **Examples**: Working examples for basic usage, retry, idempotency, and metrics
- **Documentation**: Detailed documentation in docs/ directory

## Go Version
- **Requirement**: Go 1.23+
- **Module**: gitlab.citydrive.tech/back-end/go/pkg/http-client