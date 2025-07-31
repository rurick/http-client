# Overview

HTTP Клиент is a comprehensive HTTP client library for Go applications designed for production environments. The library provides a robust, reliable solution for making HTTP requests with built-in resilience mechanisms, observability features, and extensive configuration options. It includes automatic retry strategies, circuit breaker patterns, middleware system, metrics collection, OpenTelemetry integration, and comprehensive testing utilities.

The library supports all standard HTTP methods, JSON/XML handling, streaming for large data processing, and comes with built-in authentication, logging, and rate limiting middleware. It's designed to handle production workloads with features like connection pooling, distributed tracing, and comprehensive error handling.

## Recent Changes

### 2025-01-31: Added CtxHTTPClient Interface
- ✓ Added new CtxHTTPClient interface with context-aware HTTP methods
- ✓ Implemented DoCtx, GetCtx, PostCtx, PostFormCtx, HeadCtx methods
- ✓ All methods support context cancellation and timeout management
- ✓ Extended ExtendedHTTPClient to embed CtxHTTPClient
- ✓ Updated mock clients to support new context methods
- ✓ Created comprehensive test suite for context functionality
- ✓ Added documentation for context methods usage patterns
- ✓ **Updated all documentation to recommend context methods as preferred approach**
- ✓ Added warnings and best practices in README, quick-start, and API docs
- ✓ **Created comprehensive connection pool documentation**
- ✓ **Created detailed default settings documentation**
- ✓ Updated navigation and cross-references between documentation files

### 2025-01-31: Modernized Type System and Go 1.23 Features
- ✓ Updated to Go 1.23 with modern toolchain support
- ✓ Replaced all interface{} with modern 'any' type (Go 1.18+)  
- ✓ Added slices package usage for modern slice operations
- ✓ Implemented slices.Clone, slices.Concat, slices.ContainsFunc
- ✓ Added WithMultipleMiddleware using modern slices operations
- ✓ Updated interfaces, client implementation, and mock objects
- ✓ Updated all test files and examples
- ✓ Added comprehensive test coverage (73.6% -> 85%+)
- ✓ Created options_test.go and middleware_test_comprehensive.go
- ✓ Fixed all compilation errors and test conflicts
- ✓ All tests passing with improved coverage
- ✓ Code formatted and all tests passing
- ✓ Project now uses modern Go syntax throughout

### 2025-01-31: Added Comprehensive Prometheus Metrics Documentation
- ✓ Created detailed docs/metrics.md with all Prometheus metric types
- ✓ Documented all metric names, types, labels, and bucket configurations
- ✓ Added metric examples with realistic values and PromQL queries
- ✓ Included Grafana dashboard recommendations and alert configurations
- ✓ Added performance troubleshooting guide using metrics
- ✓ Updated documentation navigation to highlight metrics section

### 2025-01-31: Added WithMetricsName() Method for Custom Metric Prefixes
- ✓ Implemented WithMetricsName(string) method for custom Prometheus metric prefixes
- ✓ Added MetricsPrefix field to ClientOptions with default "httpclient"
- ✓ Added empty string validation with automatic fallback to default
- ✓ Created comprehensive tests covering normal usage, edge cases, and combinations
- ✓ Updated all metrics documentation to show {prefix} placeholder format
- ✓ Added examples for different services (API Gateway, User Service, Payment)
- ✓ Updated PromQL queries and alerts to use configurable prefixes

### 2025-01-31: Refactored Metrics Constants
- ✓ Extracted all metric names into constants in metrics.go file
- ✓ Added 16 metric constants covering all Prometheus metric types
- ✓ Updated OpenTelemetry metric creation to use constants
- ✓ Created comprehensive tests for metric constants validation
- ✓ Added documentation section explaining constants usage
- ✓ Ensured naming follows Prometheus conventions (_total, _seconds, _bytes)

### 2025-01-31: Fixed MetricsName Implementation  
- ✓ Renamed MetricsPrefix to MetricsName throughout the codebase
- ✓ Fixed client.go to use options.MetricsName instead of options.MetricsPrefix
- ✓ Updated NewOTelMetricsCollector to use MetricsName for meter/tracer identification
- ✓ Removed prefix concatenation from metric names (prefix only used in otel.Meter/Tracer)
- ✓ Updated documentation to reflect standard metric names without prefixes
- ✓ All tests passing with proper OpenTelemetry instrument naming

# User Preferences

Preferred communication style: Simple, everyday language.

# System Architecture

## Core HTTP Client Architecture
The system is built around a layered architecture with multiple interfaces:
- **HTTPClient**: Base interface providing standard HTTP methods (GET, POST, PUT, PATCH, DELETE, HEAD)
- **CtxHTTPClient**: Context-aware interface with timeout and cancellation support for all HTTP methods  
- **ExtendedHTTPClient**: Extended interface adding JSON/XML methods, streaming capabilities, and context-aware operations
- **Client Implementation**: Main client struct that implements all interfaces with configurable options

## Reliability Mechanisms
**Retry Strategies**: Multiple retry patterns including exponential backoff, fixed delay, adaptive (SmartRetryStrategy), and custom strategies. Retries are disabled by default and must be explicitly enabled.

**Circuit Breaker**: Implements the circuit breaker pattern with three states (Closed, Open, Half-Open) to prevent cascading failures. Configurable failure thresholds, timeout periods, and request limits for recovery testing.

**Timeout Management**: Comprehensive timeout configuration including overall request timeouts, connection timeouts, and idle connection timeouts. Enhanced with context-based timeout management.

**Context Support**: Full context integration for request cancellation, timeout management, and distributed tracing propagation through CtxHTTPClient interface.

## Middleware System
Extensible middleware chain for request/response processing:
- **Authentication Middleware**: Basic Auth, Bearer Token, and API Key authentication
- **Logging Middleware**: Integration with zap logger for detailed operation logging
- **Rate Limiting Middleware**: Token bucket algorithm implementation for request throttling
- **Custom Middleware**: Interface for implementing custom request/response processors

## Observability and Monitoring
**Built-in Metrics**: Internal metrics collection without external dependencies, tracking request counts, success/failure rates, latency statistics, and data transfer volumes.

**OpenTelemetry Integration**: Automatic span creation for HTTP requests, distributed tracing support, and metrics export capabilities for integration with monitoring systems.

**Logging**: Comprehensive logging of all HTTP operations including request details, response information, timing, and error conditions.

## Data Processing
**JSON/XML Support**: Specialized methods for JSON and XML request/response handling with automatic serialization/deserialization.

**Streaming Support**: StreamResponse interface for handling large requests and responses without loading all data into memory, suitable for file transfers and large API responses.

**Context Propagation**: All methods support context for timeout management, cancellation, and tracing information propagation.

## Testing Framework
**Mock Objects**: Comprehensive MockHTTPClient implementation using testify/mock for unit testing, including support for context methods.

**Test Utilities**: Helper functions and utilities for testing HTTP client integrations, including response builders and request matchers.

**Context Testing**: Specialized tests for context cancellation, timeout behavior, and error handling patterns.

# External Dependencies

## Core Dependencies
- **Standard Go HTTP**: Built on top of Go's standard `net/http` package
- **Context Support**: Full Go context integration for request cancellation and timeouts

## Third-party Libraries
- **Zap Logger**: Integration with Uber's zap logging library for structured logging
- **OpenTelemetry**: Complete OpenTelemetry integration for tracing and metrics export
- **Testify Mock**: Mock framework for testing utilities and test helpers

## Optional Integrations
- **Custom HTTP Clients**: Support for bringing your own `http.Client` with custom transport configurations
- **TLS Configuration**: Support for custom TLS settings and certificate handling
- **Monitoring Systems**: OpenTelemetry exporters for various monitoring platforms (Prometheus, Jaeger, etc.)

## Configuration Options
The client supports extensive configuration through functional options pattern, allowing fine-tuning of connection pools, retry behaviors, circuit breaker parameters, middleware chains, and observability settings.