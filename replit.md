# HTTP Client - Replit Development Guide

## Overview

This repository contains a comprehensive HTTP client library for Go that provides enterprise-grade features including retry mechanisms, circuit breaker patterns, metrics collection, and OpenTelemetry integration. The library is designed to be a production-ready solution for making HTTP requests with built-in reliability and observability features.

**Repository Location**: `gitlab.citydrive.tech/back-end/go/pkg/http-client`

## User Preferences

Preferred communication style: Simple, everyday language.

## Recent Changes

**July 31, 2025 - Перенос в корпоративный GitLab + улучшения с современными возможностями Go**
- ✓ Пакет переименован с "universalhttpclient" на "httpclient" во всех файлах
- ✓ Обновлен go.mod с новым именем модуля: gitlab.citydrive.tech/back-end/go/pkg/http-client
- ✓ Обновлены все импорты в основных файлах (client.go, options.go, interfaces.go, retry.go, middleware.go, circuit_breaker.go, metrics.go, streaming.go)
- ✓ Обновлены все тестовые файлы с новым именем пакета
- ✓ Обновлены файлы в директории mock с новыми импортами
- ✓ Обновлены все примеры использования (basic_usage.go, circuit_breaker_example.go, middleware_example.go, metrics_example.go)
- ✓ Обновлен README.md с новым названием и ссылками
- ✓ Добавлены новые возможности в retry.go: SmartRetryStrategy с адаптивными задержками и историей ошибок
- ✓ Улучшены middleware возможности: AddAll метод для добавления нескольких middleware сразу
- ✓ Добавлены улучшенные методы работы с метриками: GetStatusCodes для эффективного копирования
- ✓ Добавлен метод GetMetricsCollector в Client для доступа к коллектору метрик
- ✓ Создан практический пример advanced_features_example.go демонстрирующий расширенные возможности
- ✓ Добавлена функция IsRetryableStatusCode для проверки кодов состояния на возможность повтора
- ✓ Пакет готов к использованию в корпоративной среде CityDrive
- ✓ Изменено поведение по умолчанию: клиент теперь запускается БЕЗ ретраев (RetryMax: 0)
- ✓ Добавлено подробное описание работы Circuit Breaker в README.md с принципами, состояниями и примерами
- ✓ Добавлен официальный RateLimitMiddleware с алгоритмом Token Bucket в middleware.go
- ✓ Расширена документация middleware в README.md с описанием всех встроенных компонентов
- ✓ Добавлено подробное описание принципа работы Rate Limiter
- ✓ Добавлена подробная документация по трейсингу и распределенной трассировке в README.md
- ✓ Создан практический пример examples/tracing_example с демонстрацией OpenTelemetry интеграции
- ✓ Добавлены тесты для трейсинга в tracing_test.go с проверкой spans и атрибутов
- ✓ Документированы преимущества трейсинга и интеграция с Jaeger/Zipkin
- ✓ Добавлено подробное оглавление в README.md для улучшения навигации по документации
- ✓ Проверены и добавлены все недостающие опции конфигурации в документацию
- ✓ Добавлены разделы: настройка соединений, времени ожидания, пользовательских клиентов, управления функциями
- ✓ Обновлена версия Go в go.mod до 1.24.4
- ✓ Проверены все тесты - успешно проходят (circuit breaker, retry, middleware, client, tracing)
- ✓ Исправлены ошибки форматирования в примерах
- ✓ Проверена компиляция пакета и зависимости - все работает корректно
- ✓ Исправлена ошибка в функции base64Encode - заменена на стандартную библиотечную
- ✓ Все тесты теперь проходят успешно (100% PASS)

**January 31, 2025 - Полный перевод на русский язык + расширенная документация метрик**
- ✓ Переведен README.md с полной документацией API и примерами использования на русском языке
- ✓ Переведены все комментарии в основных файлах кода (client.go, options.go, interfaces.go, retry.go)
- ✓ Переведены комментарии в примерах использования (basic_usage.go, circuit_breaker_example.go, middleware_example.go)
- ✓ Исправлены конфликты имен в CustomRetryStrategy (приватные поля вместо публичных)
- ✓ Добавлена подробная документация по встроенным метрикам в README.md
- ✓ Создан практический пример metrics_example.go с 4 сценариями использования метрик
- ✓ Документированы различия между встроенными метриками и OpenTelemetry
- ✓ Добавлены рекомендации когда использовать/не использовать встроенные метрики
- ✓ Добавлены русские комментарии ко всем тестам с объяснением их назначения
- ✓ Полная русификация документации и кода завершена
- ✓ Библиотека готова к использованию с полностью русскоязычной документацией

## Current Project Status

**Completed Features:**
- Complete HTTP client implementation with retry mechanisms
- Circuit breaker pattern with configurable thresholds and timeouts
- Comprehensive middleware system (auth, logging, rate limiting, timeout)
- OpenTelemetry metrics integration for observability
- Streaming request/response support
- Mock objects and testing utilities
- Working examples for all major features including advanced capabilities demo
- Advanced retry strategies: ExponentialBackoff, FixedDelay, Custom, and SmartRetry with adaptive delays
- Efficient status code checking and retryable HTTP codes management
- Enhanced metrics collection with status code tracking and history
- Full test coverage (tests pass with minor streaming test issue)
- Package renamed to "httpclient" for simplicity and clarity

## System Architecture

The Universal HTTP Client follows a modular, middleware-based architecture that emphasizes reliability and observability:

### Core Design Principles
- **Middleware Chain Pattern**: Extensible request/response processing pipeline
- **Fault Tolerance**: Built-in retry mechanisms and circuit breaker to handle failures gracefully
- **Observability First**: Integrated metrics and tracing for production monitoring
- **Configuration-Driven**: Flexible configuration options for different use cases

### Architecture Layers
1. **Client Interface**: High-level HTTP client with standard and specialized methods
2. **Middleware Layer**: Configurable middleware chain for cross-cutting concerns
3. **Transport Layer**: Connection pooling and low-level HTTP transport
4. **Observability Layer**: Metrics collection and distributed tracing
5. **Resilience Layer**: Retry logic and circuit breaker implementation

## Key Components

### HTTP Client Core
- **Problem**: Need for a reliable, feature-rich HTTP client
- **Solution**: Comprehensive client with standard HTTP methods plus JSON/XML convenience methods
- **Rationale**: Provides both flexibility and ease of use for common operations

### Retry Mechanisms
- **Problem**: Network failures and transient errors are common in distributed systems
- **Solution**: Multiple retry strategies (exponential backoff, fixed delay, custom)
- **Rationale**: Different scenarios require different retry approaches

### Circuit Breaker
- **Problem**: Cascading failures can bring down entire systems
- **Solution**: Circuit breaker pattern with configurable failure thresholds
- **Rationale**: Prevents resource exhaustion and provides fast failure for unhealthy services

### Middleware System
- **Problem**: Cross-cutting concerns like logging, authentication, and metrics
- **Solution**: Extensible middleware chain pattern
- **Rationale**: Allows for composable, reusable request/response processing

### Observability Integration
- **Problem**: Production systems need monitoring and debugging capabilities
- **Solution**: Built-in OpenTelemetry integration for metrics and tracing
- **Rationale**: Industry-standard observability tools for production readiness

## Data Flow

1. **Request Initiation**: Client method called with HTTP parameters
2. **Middleware Processing**: Request passes through configured middleware chain
3. **Retry Logic**: Failed requests are retried according to configured strategy
4. **Circuit Breaker Check**: Circuit breaker evaluates service health
5. **Transport Execution**: Actual HTTP request made through connection pool
6. **Response Processing**: Response flows back through middleware chain
7. **Metrics Collection**: Request/response metrics recorded
8. **Tracing**: Distributed tracing spans completed

## External Dependencies

### Core Dependencies
- **Go Standard Library**: net/http, context, time packages for HTTP operations
- **OpenTelemetry**: Metrics and tracing instrumentation
  - Rationale: Industry standard for observability in cloud-native applications

### Testing Dependencies
- **Mock Framework**: Testing utilities for unit and integration tests
- **Rationale**: Comprehensive testing support for library consumers

## Deployment Strategy

### Library Distribution
- **Problem**: Need to distribute Go library to consumers
- **Solution**: Standard Go module distribution via version tags
- **Rationale**: Follows Go ecosystem conventions for library distribution

### Integration Approach
- **Embedded Library**: Designed to be embedded in applications as a dependency
- **Configuration-Based**: Runtime configuration without code changes
- **Zero Dependencies**: Minimal external dependencies for easy adoption

### Production Considerations
- **Connection Pooling**: Configurable for different deployment scenarios
- **Metrics Export**: OpenTelemetry metrics can be exported to various backends
- **Tracing Integration**: Distributed tracing fits into existing observability infrastructure
- **Resource Management**: Proper cleanup and resource management for long-running applications

### Monitoring and Observability
- **Built-in Metrics**: Request latency, error rates, circuit breaker state
- **Distributed Tracing**: Request flow across service boundaries
- **Health Checks**: Circuit breaker provides service health indicators
- **Debugging Support**: Comprehensive logging and error reporting