# Quick Start

This section will help you quickly start using the HTTP client package.

## Installation

The package is available via GitHub:

```bash
go get github.com/rurick/http-client
```

## Basic Usage

### Simple HTTP Client

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
    
    // Read response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response: %s\n", body)
}
```

### POST Request with JSON

```go
func createUser(client *httpclient.Client) error {
    userData := `{
        "name": "John Doe",
        "email": "john@example.com"
    }`
    
    resp, err := client.Post(
        context.Background(),
        "https://api.example.com/users",
        "application/json",
        strings.NewReader(userData),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 201 {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    
    return nil
}
```

### Using Context and Timeout

```go
func fetchWithTimeout() error {
    client := httpclient.New(httpclient.Config{
        Timeout: 10 * time.Second,
    }, "api-client")
    defer client.Close()
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    resp, err := client.Get(ctx, "https://slow-api.example.com/data")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Configuration with Retry

### Basic Retry Settings

```go
func createRetryClient() *httpclient.Client {
    config := httpclient.Config{
        Timeout:       30 * time.Second,
        PerTryTimeout: 5 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    5 * time.Second,
            Jitter:      0.2,
        },
    }
    
    return httpclient.New(config, "retry-client")
}
```

### Idempotent Operations

```go
func updateResource(client *httpclient.Client, id string, data string) error {
    // PUT requests are automatically retried
    resp, err := client.Put(
        context.Background(),
        fmt.Sprintf("https://api.example.com/resources/%s", id),
        "application/json",
        strings.NewReader(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### POST with Idempotency-Key

```go
func createPayment(client *httpclient.Client, paymentData string) error {
    req, err := http.NewRequestWithContext(
        context.Background(),
        "POST",
        "https://api.payment.com/payments",
        strings.NewReader(paymentData),
    )
    if err != nil {
        return err
    }
    
    // Adding Idempotency-Key allows retrying POST requests
    req.Header.Set("Idempotency-Key", "payment-12345")
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Circuit Breaker (Brief)

### Enable by Default

```go
client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true, // if instance not set, SimpleCircuitBreaker is used
}, "my-service")
```

### Custom Thresholds and Handler

```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 2,
    SuccessThreshold: 1,
    Timeout:          5 * time.Second,
    OnStateChange: func(from, to httpclient.CircuitBreakerState) { log.Printf("CB: %s -> %s", from, to) },
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "orders-client")
```

## Error Handling

### Error Type Checking

```go
func handleErrors(client *httpclient.Client) {
    resp, err := client.Get(context.Background(), "https://api.example.com/data")
    if err != nil {
        // Check for errors after retry attempts exhausted
        if retryableErr, ok := err.(*httpclient.RetryableError); ok {
            log.Printf("Request failed after %d attempts: %v", 
                retryableErr.Attempts, retryableErr.Err)
            return
        }
        
        // Errors that cannot be retried
        if nonRetryableErr, ok := err.(*httpclient.NonRetryableError); ok {
            log.Printf("Non-retryable error: %v", nonRetryableErr.Err)
            return
        }
        
        // Other errors
        log.Printf("General error: %v", err)
        return
    }
    defer resp.Body.Close()
    
    // Check status code
    if resp.StatusCode >= 400 {
        log.Printf("HTTP error: %d", resp.StatusCode)
        return
    }
}
```

## Monitoring and Observability

### Enable Tracing

```go
func createTracedClient() *httpclient.Client {
    config := httpclient.Config{
        TracingEnabled: true,
        Timeout:        15 * time.Second,
    }
    
    return httpclient.New(config, "traced-service")
}
```

### Metrics are Automatically Collected

The package automatically collects Prometheus metrics:
- Request count
- Latency
- Errors and retry attempts
- Request/response sizes
- Active connections

No additional configuration required!

## Common Patterns

### Microservice Client

```go
type UserService struct {
    client *httpclient.Client
}

func NewUserService() *UserService {
    config := httpclient.Config{
        Timeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 2,
            BaseDelay:   50 * time.Millisecond,
            MaxDelay:    1 * time.Second,
        },
        TracingEnabled: true,
    }
    
    return &UserService{
        client: httpclient.New(config, "user-service"),
    }
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    resp, err := s.client.Get(ctx, fmt.Sprintf("/users/%s", id))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }
    
    return &user, nil
}

func (s *UserService) Close() error {
    return s.client.Close()
}
```

### External API Client

```go
func createExternalAPIClient() *httpclient.Client {
    config := httpclient.Config{
        Timeout:       30 * time.Second,
        PerTryTimeout: 10 * time.Second,
        RetryConfig: httpclient.RetryConfig{
            MaxAttempts: 5,
            BaseDelay:   200 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Jitter:      0.3,
        },
        TracingEnabled: true,
        
        // Custom transport if needed
        Transport: &http.Transport{
            MaxIdleConns:       100,
            IdleConnTimeout:    90 * time.Second,
            DisableCompression: false,
        },
    }
    
    return httpclient.New(config, "external-api")
}
```

## Next Steps

After mastering basic usage, explore:

- [Configuration](configuration.md) - Detailed client settings
- [Metrics](metrics.md) - Monitoring and alerting
- [Testing](testing.md) - Mock utilities and test servers
- [Best Practices](best-practices.md) - Production recommendations

## Frequently Asked Questions

**Q: How to configure custom headers for all requests?**

A: Use a custom Transport or add headers to each request via http.Request.

**Q: Can I disable retry for a specific request?**

A: Set MaxAttempts = 1 in the configuration or create a separate client.

**Q: How to log all HTTP requests?**

A: Enable TracingEnabled: true and configure OpenTelemetry logging export.

More answers in the [Troubleshooting](troubleshooting.md) section.
