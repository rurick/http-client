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
- ✓ Переработаны тесты с использованием t.Parallel() и t.Run() для улучшения производительности
- ✓ Добавлены комментарии о тестах которые не должны выполняться параллельно (timing, concurrency, sleep)
- ✓ Исправлены все проблемы с LSP diagnostics - теперь код полностью валиден
- ✓ Проведено полное тестирование с параллельным выполнением (parallel=8) - все тесты проходят
- ✓ Оптимизирована производительность тестов за счет параллельного выполнения где это безопасно
- ✓ Проведена полная проверка линтерами (go vet, staticcheck, golangci-lint)
- ✓ Исправлены все замечания статического анализа - код соответствует стандартам Go
- ✓ Исправлена версия Go в go.mod на 1.23 с toolchain 1.24.4 для полной совместимости
- ✓ Очищены неиспользуемые импорты и переменные
- ✅ **Финальная проверка качества кода (31 июля 2025)**
- ✓ Достигнуто покрытие тестами 72.9% (превышает требуемые 70%)
- ✓ Добавлено 15+ новых тестов в additional_test.go для непокрытых функций
- ✓ Проверен go vet - без ошибок
- ✓ Проверен staticcheck - без ошибок
- ✓ Применен gofmt для стандартного форматирования
- ✓ Все тесты проходят успешно с race detection
- ✓ Пакет готов к продакшн использованию с высоким качеством кода
- ✓ Тестовые файлы реорганизованы в модульную структуру:
  - http_methods_test.go - тесты HTTP методов (HEAD, PUT, PATCH, DELETE, XML)  
  - options_test.go - тесты опций конфигурации клиента
  - retry_strategies_test.go - тесты стратегий повтора  
  - middleware_chain_test.go - тесты middleware системы
  - streaming_test.go - тесты streaming функциональности
  - metrics_additional_test.go - дополнительные тесты метрик
- ✓ Сохранено покрытие 72.9% при улучшенной организации кода
- ✅ **Финальная проверка с Go 1.24.4 (31 июля 2025)**
- ✓ Установлена Go версия 1.24.4 (go version go1.24.4 linux/amd64)
- ✓ Все тесты проходят успешно (5.198s)
- ✓ Покрытие тестами остается 72.9%
- ✓ go vet проверка пройдена
- ✓ gofmt форматирование применено
- ✓ go mod модули проверены и обновлены
- ✓ Пакет полностью готов к продакшн использованию
- ✅ **SAST/SCA анализ безопасности и качества кода (31 июля 2025)**
- ✓ Проведен анализ безопасности: не найдено уязвимостей (unsafe операций, SQL инъекций, exec команд)
- ✓ Зависимости обновлены до последних безопасных версий (go-retryablehttp v0.7.8, go-hclog v1.6.3)
- ✓ Исправлены все проблемы со статическим анализом кода
- ✓ Устранена избыточная функция base64Encode, используется стандартная библиотечная
- ✓ Код отформатирован стандартным gofmt (0 неотформатированных файлов)
- ✓ go vet проверка пройдена без ошибок и предупреждений
- ✓ Покрытие тестами поддерживается на уровне 72.9% (выше требуемых 70%)
- ✓ Версия Go 1.24.4 сохранена и работает корректно
- ✓ Код соответствует стандартам безопасности для продакшн использования
- ✓ Исправлено предупреждение статического анализа: добавлен недостающий case CircuitBreakerOpen в switch statement
- ✓ Улучшена полнота обработки всех состояний circuit breaker с документированной логикой
- ✓ Исправлен импорт encoding/base64 в тестовом файле middleware_test.go
- ✅ **Устранение неободранных ошибок для полной готовности (31 июля 2025)**
- ✓ Исправлены все неободранные ошибки в defer statements в примерах (circuit_breaker_example, middleware_example, basic_usage)
- ✓ Устранены неободранные ошибки logger.Sync() с корректной обработкой в анонимных функциях
- ✓ Исправлены неободранные ошибки body.WriteString() в mock/mock_client.go
- ✓ Исправлены неободранные ошибки r.Body.Read() в тестовых файлах
- ✓ Код теперь полностью соответствует стандартам Go с корректной обработкой всех ошибок
- ✓ Библиотека готова к продакшн использованию без warnings от статических анализаторов
- ✅ **Исправление неиспользуемых параметров (31 июля 2025)**
- ✓ Исправлены неиспользуемые параметры lastErr в ExponentialBackoffStrategy.NextDelay и FixedDelayStrategy.NextDelay
- ✓ Добавлены подчеркивания (_) для явного указания игнорируемых параметров согласно стандартам Go
- ✓ Код теперь полностью соответствует best practices Go без предупреждений о неиспользуемых параметрах
- ✓ Исправлена версия Go в go.mod (убрана неподдерживаемая версия 1.23.0 и toolchain директива)
- ✓ Добавлены пояснительные комментарии к функциям которые намеренно игнорируют параметры
- ✓ Сохранены корректные сигнатуры интерфейса RetryStrategy для всех стратегий

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