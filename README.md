# HTTP Client Package

Comprehensive Go HTTP client with automatic retry mechanisms, OpenTelemetry metrics, and distributed tracing.

## Key Features

- **Smart retries** with exponential backoff and jitter
- **Built-in OpenTelemetry metrics** (with Prometheus support)
- **Distributed tracing** via OpenTelemetry
- **Circuit Breaker** for protection against cascading failures
- **Built-in Rate Limiter** with Token Bucket algorithm
- **Idempotency policies** for safe POST/PATCH retries
- **Configurable timeouts** and backoff strategies
- **Testing utilities** for unit and integration tests

## Quick Start

```go
package main

import (
    "context"
    httpclient "github.com/rurick/http-client"
)

func main() {
    client := httpclient.New(httpclient.Config{}, "my-service")
    defer client.Close()
    
    // Simple GET request
    resp, err := client.Get(context.Background(), "https://api.example.com/data")
    if err != nil {
        // handle error
    }
    defer resp.Body.Close()
    
    // GET with headers via new options
    resp, err = client.Get(context.Background(), "https://api.example.com/users",
        httpclient.WithHeaders(map[string]string{
            "Authorization": "Bearer your-token",
            "Accept": "application/json",
        }))
    if err != nil {
        return
    }
    defer resp.Body.Close()
    
    // POST with JSON body
    user := map[string]interface{}{
        "name": "John Doe",
        "email": "john@example.com",
    }
    resp, err = client.Post(context.Background(), "https://api.example.com/users", nil,
        httpclient.WithJSONBody(user),
        httpclient.WithBearerToken("your-token"))
    if err != nil {
        return
    }
    defer resp.Body.Close()

    // POST with JSON body as string
	userString := `{"name": "John Doe","email": "john@example.com"}`
	resp, err = client.Post(context.Background(), "https://api.example.com/users", nil,
		httpclient.WithJSONBody(userString),
		httpclient.WithBearerToken("your-token"))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Using Rate Limiter
	clientWithRateLimit := httpclient.New(httpclient.Config{
		RateLimiterEnabled: true,
		RateLimiterConfig: httpclient.RateLimiterConfig{
			RequestsPerSecond: 5.0, // 5 requests per second
			BurstCapacity:     10,  // up to 10 requests at once
		},
	}, "rate-limited-service")
	defer clientWithRateLimit.Close()
}
```

## Documentation

**Full documentation:** [docs/index.md](docs/index.md)

**Main sections:**
- [Quick Start](docs/quick-start.md) - Usage examples  
- [Configuration](docs/configuration.md) - Client settings
- [Metrics](docs/metrics.md) - Monitoring and alerts
- [API Reference](docs/api-reference.md) - Complete function descriptions
- [Best Practices](docs/best-practices.md) - Recommendations
- [Testing](docs/testing.md) - Utilities and examples
- [Troubleshooting](docs/troubleshooting.md) - Problem solving
- [Examples](docs/examples.md) - Ready code snippets

