# HTTP Client Package Documentation

Welcome to the HTTP client package documentation - a comprehensive solution for HTTP requests with automatic retry mechanisms, built-in Prometheus metrics, and idempotency policies.

## Contents

- [Quick Start](quick-start.md) - Usage examples and first steps
- [Configuration](configuration.md) - Complete configuration documentation
- [Circuit Breaker](circuit-breaker.md) - Automatic circuit breaker and protection against cascading failures
- [Rate Limiter](rate-limiter.md) - Request rate management with Token Bucket algorithm
- [Metrics](metrics.md) - Metrics description and PromQL queries
- [OpenTelemetry Metrics](opentelemetry-metrics.md) - OpenTelemetry integration
- [Testing](testing.md) - Utilities and test examples
- [API Reference](api-reference.md) - Complete description of all functions
- [Best Practices](best-practices.md) - Usage recommendations
- [Troubleshooting](troubleshooting.md) - Common problem solving
- [Examples](examples.md) - Ready code snippets

## Key Features

### 🔄 Smart Retry Mechanisms
- Exponential backoff with jitter
- Automatic detection of idempotent methods
- Idempotency-Key support for POST/PATCH requests
- Configurable timeouts and number of attempts

### 📊 Automatic Metrics
- Support for Prometheus (prometheus/client_golang v1.22.0) and OpenTelemetry
- 6 metric types: requests, durations, retries, sizes, inflight
- Configurable metrics providers (Prometheus/OpenTelemetry/Noop)
- Ready PromQL queries and alerts

### 🔍 Observability
### 🛡️ Circuit Breaker
- Built-in support for automatic circuit breaker
- Configurable error thresholds and recovery timeout
- Returns last unsuccessful response when open
- Open Circuit Breaker does not initiate retry
- Full integration with OpenTelemetry tracing
- Automatic span creation for each request
- Context propagation between services
- Detailed error logging

### 🧪 Testing Utilities
- TestServer for integration tests
- MockRoundTripper for unit tests
- Helpers for condition checking with timeout
- Collectors for metrics testing

## Quick Start

```go
package main

import (
    "context"
    "log"
    httpclient "github.com/rurick/http-client"
)

func main() {
    // Create client with default settings
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    // GET request
    resp, err := client.Get(context.Background(), "https://api.example.com/users")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
}
```

## Configuration with retry

```go
config := httpclient.Config{
    Timeout:       30 * time.Second,
    PerTryTimeout: 5 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    10 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled: true,
}

client := httpclient.New(config, "payment-service")
```

## Configuration with OpenTelemetry metrics

```go
// Main OpenTelemetry configuration
config := httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendOpenTelemetry,
    // Can specify custom MeterProvider
    // OTelMeterProvider: customMeterProvider,
}

client := httpclient.New(config, "otel-service")

// Disable metrics
config = httpclient.Config{
    MetricsBackend: httpclient.MetricsBackendNone,
}
client = httpclient.New(config, "no-metrics-service")
```

## Available Metrics

1. **http_client_requests_total** - Total number of requests
2. **http_client_request_duration_seconds** - Request duration
3. **http_client_retries_total** - Number of retry attempts
4. **http_client_inflight_requests** - Current active requests
5. **http_client_request_size_bytes** - Request size
6. **http_client_response_size_bytes** - Response size

## Ready Alerts

```yaml
# High error rate
- alert: HTTPClientHighErrorRate
  expr: |
    (sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
     sum(rate(http_client_requests_total[5m])) by (host)) > 0.05
  for: 2m

# High latency
- alert: HTTPClientHighLatency  
  expr: |
    histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 2
  for: 5m
```

## Package Status

✅ **Production Ready**
- All components implemented and tested
- Code compiles without errors
- Test coverage 61.7%+
- Complete documentation and examples
- Integration tests for metrics
- Mock utilities for unit tests

## Documentation Files

- [`quick-start.md`](quick-start.md) - Quick start with examples
- [`configuration.md`](configuration.md) - Detailed configuration documentation
- [`circuit-breaker.md`](circuit-breaker.md) - Detailed Circuit Breaker documentation
- [`rate-limiter.md`](rate-limiter.md) - Detailed Rate Limiter guide
- [`metrics.md`](metrics.md) - Metrics, PromQL queries and alerts
- [`opentelemetry-metrics.md`](opentelemetry-metrics.md) - OpenTelemetry metrics integration
- [`api-reference.md`](api-reference.md) - Complete API reference
- [`best-practices.md`](best-practices.md) - Best usage practices
- [`testing.md`](testing.md) - Testing guide
- [`examples.md`](examples.md) - Practical code examples
- [`troubleshooting.md`](troubleshooting.md) - Problem solving

## Usage in Projects

```go
// For internal APIs
client := httpclient.New(httpclient.Config{
    Timeout: 5 * time.Second,
    RetryConfig: httpclient.RetryConfig{MaxAttempts: 2},
}, "internal-service")

// For external APIs
client := httpclient.New(httpclient.Config{
    Timeout: 30 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   200 * time.Millisecond,
        MaxDelay:    10 * time.Second,
    },
    TracingEnabled: true,
}, "external-api")
```

See detailed examples and complete documentation in the corresponding sections above.

## Additional Resources

### PromQL Examples for Monitoring

```promql
# Request rate
rate(http_client_requests_total[5m])

# Error percentage
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) / 
sum(rate(http_client_requests_total[5m])) by (host) * 100

# 95th percentile latency
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Retry rate
sum(rate(http_client_retries_total[5m])) by (host, reason)
```

### Recommended Alert Settings

- **Error percentage** > 5% for 2 minutes
- **95th percentile latency** > 2 seconds for 5 minutes  
- **Retry rate** > 1 request/sec for 2 minutes
- **Active requests** > 100 for 1 minute

### Troubleshooting

Common problems and solutions:

1. **High error rate**
   - Check target service availability
   - Increase timeouts if needed
   - Check network connectivity

2. **High latency**
   - Check target service performance
   - Consider increasing PerTryTimeout
   - Check network delays

3. **Many retries**
   - Check target service stability
   - Consider reducing MaxAttempts
   - Check retry reasons in metrics

### Support and Feedback

The package is ready for production use. For questions and suggestions, contact the development team.
