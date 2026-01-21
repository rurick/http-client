# Configuration

The HTTP client package offers comprehensive configuration capabilities for various use cases.

## Config Structure

```go
type Config struct {
    Timeout         time.Duration    // Overall request timeout
    PerTryTimeout   time.Duration    // Timeout per attempt
    RetryEnabled    bool             // Enable/disable retry mechanism  
    RetryConfig     RetryConfig      // Retry configuration
    TracingEnabled  bool             // Enable OpenTelemetry tracing
    Transport       http.RoundTripper // Custom transport
    CircuitBreakerEnable bool        // Enable Circuit Breaker
    CircuitBreaker       httpclient.CircuitBreaker // Circuit Breaker instance
	RateLimiterEnabled bool         // RateLimiterEnabled enables/disables rate limiting
	RateLimiterConfig RateLimiterConfig // RateLimiterConfig rate limiter configuration
}
```

## Configuration Parameters

### Timeout (Overall Timeout)
- **Type:** `time.Duration`
- **Default:** `5 * time.Second`
- **Description:** Maximum wait time for the entire request, including all retry attempts

```go
config := httpclient.Config{
    Timeout: 30 * time.Second, // Overall limit 30 seconds
}
```

### PerTryTimeout (Attempt Timeout)
- **Type:** `time.Duration`
- **Default:** `2 * time.Second`
- **Description:** Maximum wait time for a single request attempt

```go
config := httpclient.Config{
    PerTryTimeout: 5 * time.Second, // Each attempt up to 5 seconds
}
```

### RetryEnabled (Enable/Disable Retry Mechanism)
- **Type:** `bool`
- **Default:** `false`
- **Description:** Enables/disables retry mechanism

```go
config := httpclient.Config{
	RetryEnabled: true,
}

client := httpclient.New(config, "httpclient")
```

### TracingEnabled (Enable Tracing)
- **Type:** `bool`
- **Default:** `false`
- **Description:** Enables OpenTelemetry span creation for each request

```go
config := httpclient.Config{
    TracingEnabled: true, // Enable tracing
}
```

### Transport (Custom Transport)
- **Type:** `http.RoundTripper`
- **Default:** `http.DefaultTransport`
- **Description:** Allows configuring a custom HTTP transport

```go
config := httpclient.Config{
    Transport: &http.Transport{
        MaxIdleConns:       100,
        IdleConnTimeout:    90 * time.Second,
        DisableCompression: false,
    },
}
    RateLimiterEnabled bool                // Enable Rate Limiter
    RateLimiterConfig  RateLimiterConfig   // Rate Limiter Configuration
```

## Rate Limiter Configuration

Rate Limiter implements the Token Bucket algorithm to limit outgoing request frequency. This helps comply with external service API limits and protect against overload.

### RateLimiter Interface

```go
type RateLimiter interface {
    // Allow checks if a request can be executed immediately
    Allow() bool

    // Wait blocks execution until permission for a request is received
    Wait(ctx context.Context) error
}
```

**Methods:**
- `Allow()` - Non-blocking token availability check. Returns `true` if request can be executed immediately
- `Wait(ctx)` - Blocks execution until token appears. Respects context for cancellation

### TokenBucketLimiter Implementation

```go
type TokenBucketLimiter struct {
    rate     float64    // tokens per second
    capacity int        // maximum bucket capacity
    tokens   float64    // current number of tokens
    lastTime time.Time  // last update time
    mu       sync.Mutex // concurrent access protection
}
```

**Creation:**
```go
limiter := httpclient.NewTokenBucketLimiter(
    5.0,  // rate: 5 requests per second
    10,   // capacity: bucket for 10 tokens
)
```

### Rate Limiter Architecture

Rate Limiter is implemented as middleware in the RoundTripper chain. It executes before the main retry and metrics mechanism, but after Circuit Breaker.

```
HTTP Request
    ↓
Circuit Breaker (optional)
    ↓
Rate Limiter (optional) ← NEW COMPONENT
    ↓
RoundTripper (retry + metrics + tracing)
    ↓
Base HTTP Transport
    ↓
Network / External Service
```

#### Architecture Features:

1. **Middleware pattern**: Rate Limiter doesn't modify existing logic, but adds a new layer
2. **Optionality**: Fully optional component, enabled only when RateLimiterEnabled: true
3. **Positioning**: Correctly positioned in chain - limits before retry, but after circuit breaker
4. **Independence**: Doesn't affect metrics, tracing, or retry logic

### Token Bucket Algorithm

Rate Limiter uses the Token Bucket algorithm ("Token Bucket"):

#### Working Principle:
1. Bucket has a certain capacity (BurstCapacity)
2. Tokens are added at a constant rate (RequestsPerSecond)
3. Each request consumes 1 token
4. If no tokens available - request waits for them to appear

#### Advantages:
- **Burst traffic**: Allows short bursts of requests
- **Smooth limiting**: Smooth limiting without abrupt rejections
- **Predictable**: Predictable behavior and latency
- **Wait strategy**: Automatic waiting instead of rejection

### RateLimiterConfig Structure

```go
type RateLimiterConfig struct {
    RequestsPerSecond float64 // Maximum number of requests per second
    BurstCapacity     int     // Bucket size for peak requests
}
```

### RateLimiterEnabled (Enable Rate Limiter)
- **Type:** `bool`
- **Default:** `false`
- **Description:** Enables/disables rate limiting for all requests

```go
config := httpclient.Config{
    RateLimiterEnabled: true, // Enable rate limiting
}
```

### RequestsPerSecond (Requests Per Second)
- **Type:** `float64`
- **Default:** `10.0`
- **Description:** Maximum sustainable request rate. Tokens are added to bucket at this rate

```go
RateLimiterConfig{
    RequestsPerSecond: 5.0, // 5 requests per second
}
```

### BurstCapacity (Bucket Size)
- **Type:** `int`
- **Default:** equals `RequestsPerSecond`
- **Description:** Maximum number of tokens in bucket. Allows peak requests beyond sustainable rate

```go
RateLimiterConfig{
    RequestsPerSecond: 10.0,
    BurstCapacity:     20, // Can immediately make up to 20 requests
}
```


## Retry Configuration

### RetryConfig Structure

```go
type RetryConfig struct {
    MaxAttempts int           // Maximum number of attempts
    BaseDelay   time.Duration // Base delay for backoff
    MaxDelay    time.Duration // Maximum delay
    Jitter      float64       // Jitter factor (0.0-1.0)
    RetryMethods []string     // list of HTTP methods for retry
    RetryStatusCodes []int   // list of HTTP status codes for retry
    RespectRetryAfter bool    // respect Retry-After header
}
```

### MaxAttempts (Maximum Attempts)
- **Type:** `int`
- **Default:** `3` (1 initial + 2 retries)
- **Description:** Total number of attempts (including the initial one)

```go
RetryConfig{
    MaxAttempts: 3, // 1 initial + 2 retries
}
```

### BaseDelay (Base Delay)
- **Type:** `time.Duration`
- **Default:** `100 * time.Millisecond`
- **Description:** Initial delay for exponential backoff

```go
RetryConfig{
    BaseDelay: 200 * time.Millisecond, // Start with 200ms
}
```

### MaxDelay (Maximum Delay)
- **Type:** `time.Duration`
- **Default:** `2 * time.Second`
- **Description:** Maximum delay between attempts

```go
RetryConfig{
    MaxDelay: 10 * time.Second, // No more than 10 seconds
}
```

### Jitter
- **Type:** `float64`
- **Range:** `0.0 - 1.0`
- **Default:** `0.2`
- **Description:** Random delay deviation to prevent thundering herd

```go
RetryConfig{
    Jitter: 0.3, // ±30% random deviation
}
```

### RetryMethods (HTTP Methods for Retry)
- **Type:** `[]string`
- **Default:** `["GET", "HEAD", "OPTIONS", "PUT", "DELETE"]`
- **Description:** List of HTTP methods for which retry will be performed. By default, only idempotent methods are included. POST and PATCH will only be retried if the `Idempotency-Key` header is present

```go
RetryConfig{
    RetryMethods: []string{"GET", "POST", "PUT"}, // Custom method list
}
```

### RetryStatusCodes (HTTP Status Codes for Retry)
- **Type:** `[]int`
- **Default:** `[429, 500, 502, 503, 504]`
- **Description:** List of HTTP status codes for which retry will be performed. Includes temporary server errors and rate limiting

```go
RetryConfig{
    RetryStatusCodes: []int{429, 500, 502, 503}, // Exclude 504 Gateway Timeout
}
```

### RespectRetryAfter (Respect Retry-After Header)
- **Type:** `bool`
- **Default:** `true`
- **Description:** When `true`, the client will respect the `Retry-After` header in server responses and wait for the specified time before retrying. Takes priority over the standard backoff algorithm

```go
RetryConfig{
    RespectRetryAfter: false, // Ignore Retry-After, use only backoff
}
```

## Rate Limiter Usage Examples

### Limiting for External APIs

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 5.0,  // API allows 5 RPS
        BurstCapacity:     10,   // Can make batch up to 10 requests
    },
}

client := httpclient.New(config, "external-api-client")
```

### High-Frequency Requests with Burst Support

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 100.0, // 100 RPS globally
        BurstCapacity:     50,    // Conservative burst
    },
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 3,
    },
}

client := httpclient.New(config, "high-throughput-client")
// Global limiter for all client requests
```

### Conservative Configuration for Reliability

```go
config := httpclient.Config{
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 1.0,  // Very conservative
        BurstCapacity:     1,    // No peaks
    },
    Timeout:       30 * time.Second,
    PerTryTimeout: 10 * time.Second,
}

client := httpclient.New(config, "conservative-client")
```

### Custom Rate Limiter

You can create your own `RateLimiter` implementation for special cases:

```go
// Create custom limiter directly
limiter := httpclient.NewTokenBucketLimiter(50.0, 100)

// Use in client via Config
// IMPORTANT: when using custom RateLimiter,
// RateLimiterConfig is ignored
config := httpclient.Config{
    RateLimiterEnabled: true,
    // RateLimiterConfig is not used if RateLimiter is set
}

// Custom limiter is set via internal mechanisms
// (see source code for implementation details)
client := httpclient.New(config, "custom-limiter-client")
```

**When to use custom limiter:**
- Need specific limiting logic (e.g., different limits for different endpoints)
- Integration with external rate limiting system required
- Need special algorithm (not Token Bucket)

**Implementing Your Own RateLimiter:**
```go
type MyCustomLimiter struct {
    // your logic
}

func (m *MyCustomLimiter) Allow() bool {
    // implement non-blocking check
    return true
}

func (m *MyCustomLimiter) Wait(ctx context.Context) error {
    // implement blocking wait
    return nil
}
```

## Default Values

```go
// Automatically applied when creating client
defaultConfig := Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 2 * time.Second,
    RetryConfig: RetryConfig{
        MaxAttempts: 3,        // 1 initial + 2 retries
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    2 * time.Second,
        Jitter:      0.2,
    },
    TracingEnabled:     false,
    RateLimiterEnabled: false, // Rate limiter disabled by default
    Transport:          http.DefaultTransport,
}
```

## Configuration Examples

### Fast Internal Services

```go
config := httpclient.Config{
    Timeout:       5 * time.Second,
    PerTryTimeout: 1 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   50 * time.Millisecond,
        MaxDelay:    500 * time.Millisecond,
        Jitter:      0.1,
    },
}

client := httpclient.New(config, "internal-api")
```

### External APIs (Requiring Reliability)

```go
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
    RateLimiterEnabled: true,
    RateLimiterConfig: httpclient.RateLimiterConfig{
        RequestsPerSecond: 10.0, // Respect API limits
        BurstCapacity:     15,   // Small burst
    },
}

client := httpclient.New(config, "external-api")
```

### Critical Payment Services

```go
config := httpclient.Config{
    Timeout:       60 * time.Second,
    PerTryTimeout: 15 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 7,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Jitter:      0.25,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        50,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

client := httpclient.New(config, "payment-service")
```

### High-Performance API Gateway

```go
config := httpclient.Config{
    Timeout:       10 * time.Second,
    PerTryTimeout: 3 * time.Second,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: 2,
        BaseDelay:   25 * time.Millisecond,
        MaxDelay:    1 * time.Second,
        Jitter:      0.1,
    },
    TracingEnabled: true,
    Transport: &http.Transport{
        MaxIdleConns:        200,
        MaxIdleConnsPerHost: 50,
        IdleConnTimeout:     60 * time.Second,
    },
}

client := httpclient.New(config, "api-gateway")
```

## Advanced Transport Settings

### Connection Pool Configuration

```go
transport := &http.Transport{
    // Overall connection pool
    MaxIdleConns:        100,
    
    // Connections per host
    MaxIdleConnsPerHost: 10,
    
    // Idle connection lifetime
    IdleConnTimeout:     90 * time.Second,
    
    // TLS timeouts
    TLSHandshakeTimeout: 10 * time.Second,
    
    // TCP timeouts
    DialTimeout:         5 * time.Second,
    
    // Keep-alive
    KeepAlive:           30 * time.Second,
    
    // Disable compression
    DisableCompression:  false,
    
    // Read buffer size
    ReadBufferSize:      4096,
    
    // Write buffer size
    WriteBufferSize:     4096,
}

config := httpclient.Config{
    Transport: transport,
}
```

### TLS Configuration

```go
tlsConfig := &tls.Config{
    // Certificate verification
    InsecureSkipVerify: false,
    
    // Minimum TLS version
    MinVersion: tls.VersionTLS12,
    
    // Preferred cipher suites
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
}

transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

config := httpclient.Config{
    Transport: transport,
}
```

## Configuration Validation

The package automatically validates configuration:

```go
// Invalid values will be corrected
config := httpclient.Config{
    Timeout:       -1 * time.Second,  // Will be set to default
    PerTryTimeout: 0,                 // Will be set to default
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts: -5,              // Will be set to 1
        Jitter:      2.0,             // Will be limited to 1.0
    },
}

client := httpclient.New(config, "service") // Works with corrected values
```

## Getting Current Configuration

```go
client := httpclient.New(config, "service")

// Get active configuration
currentConfig := client.GetConfig()

fmt.Printf("Timeout: %v\n", currentConfig.Timeout)
fmt.Printf("Max Attempts: %d\n", currentConfig.RetryConfig.MaxAttempts)
```

## Configuration Recommendations

### By Service Type

| Service Type | Timeout | PerTryTimeout | MaxAttempts | BaseDelay |
|-------------|---------|---------------|-------------|-----------|
| Internal API | 5s | 1s | 2 | 50ms |
| External API | 30s | 10s | 5 | 200ms |
| Databases | 10s | 3s | 3 | 100ms |
| Payments | 60s | 15s | 7 | 500ms |
| File Upload | 300s | 60s | 3 | 1s |

### By SLA Requirements

- **99.9% SLA:** MaxAttempts = 3-5, aggressive timeouts
- **99.95% SLA:** MaxAttempts = 5-7, moderate timeouts  
- **99.99% SLA:** MaxAttempts = 7-10, conservative timeouts

### By Network Environment

- **Internal network:** Low jitter (0.1), fast timeouts
- **Public internet:** High jitter (0.3), long timeouts
- **Mobile networks:** Very high jitter (0.5), very long timeouts

## Configuration Debugging

Enable tracing for debugging:

```go
config := httpclient.Config{
    TracingEnabled: true,
    // ... other settings
}
```

This helps see:
- Actual request execution time
- Number of retry attempts
- Error causes
- Backoff strategy effectiveness

## Status Code Preservation Logic

The HTTP client correctly preserves and returns response status codes depending on retry logic results:

### Successful Retry
When retry completes successfully, the successful response status code is returned:
```go
// Sequence: 500 → 503 → 200
// Result: StatusCode = 200 (success)
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 200 {
    // Got successful response after retry
}
```

### Retry Attempts Exhausted
When all retry attempts are exhausted, the last attempt's status code is returned:
```go
// Sequence: 500 → 503 → 502
// Result: StatusCode = 502 (last error)
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 502 {
    // All retries exhausted, last status returned
}
```

### No Retry
When retry is not applied, the original status code is returned:
```go
// 400 Bad Request (not retryable)
// Result: StatusCode = 400 (original error)  
resp, err := client.Get(ctx, url)
if err == nil && resp.StatusCode == 400 {
    // Original status preserved
}
```

### Mixed Status Codes
The client correctly handles various status code combinations:
```go
// Example: 502 → 429 → 201
// Result: StatusCode = 201 (final success)

// Example: 429 → 429 → 429 
// Result: StatusCode = 429 (last of exhausted attempts)
```

**Guarantees:**
- ✅ Last attempt status code is always preserved
- ✅ Successful responses take priority over errors
- ✅ Client errors (4xx) don't affect retry logic
- ✅ Server errors (5xx) and 429 are correctly handled