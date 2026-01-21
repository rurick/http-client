# Enhanced Timeout Errors

## Overview

We've added enhanced timeout error handling to the HTTP client. Now instead of a standard `context deadline exceeded` error with minimal information, the client provides detailed error messages with context and practical recommendations.

## Problem

### Before Enhancement
```
"level": "ERROR",
"message": "request failed", 
"error": "Post \"https://openapi.nalog.ru:8090/open-api/AuthService/0.1\": context deadline exceeded"
```

From this error it's impossible to understand:
- What timeouts were configured
- On which attempt the failure occurred
- Whether retry is enabled
- What exactly needs to be fixed

### After Enhancement
```
"level": "ERROR",
"message": "request failed",
"error": "timeout error: POST https://openapi.nalog.ru:8090/open-api/AuthService/0.1 (host: openapi.nalog.ru) failed after 5s on attempt 1/1. Timeout config: overall=5s, per-try=2s, retry=false. Type: overall. Suggestions: [increase overall timeout (current: 5s) enable retry for resilience to temporary failures]"
```

## Implementation

### New `TimeoutError` Type

```go
type TimeoutError struct {
    // Basic request information
    Method   string
    URL      string  
    Host     string
    
    // Timeout information
    Timeout       time.Duration // Overall timeout
    PerTryTimeout time.Duration // Per-attempt timeout
    Elapsed       time.Duration // Execution time until error
    
    // Retry context
    Attempt     int  // Attempt number on which timeout occurred
    MaxAttempts int  // Maximum number of attempts  
    RetryEnabled bool // Whether retry was enabled
    
    // Additional context
    TimeoutType string // Timeout type: "overall", "per-try", "context"
    OriginalErr error  // Original error
    
    // Solution suggestions
    Suggestions []string
}
```

### Timeout Types

1. **"overall"** - overall timeout exceeded (`Config.Timeout`)
2. **"per-try"** - per-attempt timeout exceeded (`Config.PerTryTimeout`) 
3. **"context"** - timeout was set in external context
4. **"network"** - network timeout (not related to client settings)

### Automatic Suggestions

The system analyzes configuration and error conditions, generating practical recommendations:

- **For overall timeout**: "increase overall timeout (current: 5s)"
- **For per-try timeout**: "increase per-try timeout (current: 2s)" 
- **For exhausted attempts**: "increase number of attempts (current: 3)"
- **If retry disabled**: "enable retry for resilience to temporary failures"
- **For slow services**: "check availability and performance of remote service"

## Usage

### Programmatic Handling

```go
resp, err := client.Post(ctx, url, body)
if err != nil {
    // Check if this is a detailed timeout error
    var timeoutErr *httpclient.TimeoutError
    if errors.As(err, &timeoutErr) {
        log.Printf("Timeout during %s:", operation)
        log.Printf("  URL: %s", timeoutErr.URL)
        log.Printf("  Attempt: %d/%d", timeoutErr.Attempt, timeoutErr.MaxAttempts)
        log.Printf("  Execution time: %v", timeoutErr.Elapsed)
        log.Printf("  Type: %s", timeoutErr.TimeoutType)
        
        // Programmatically handle different timeout types
        switch timeoutErr.TimeoutType {
        case "overall":
            log.Printf("  → Recommendation: increase overall timeout from %v", timeoutErr.Timeout)
        case "per-try":
            log.Printf("  → Recommendation: increase per-try timeout from %v", timeoutErr.PerTryTimeout)
        case "context":
            log.Printf("  → Recommendation: check context settings in calling code")
        }
        return
    }
    
    // Handle other error types as usual
    log.Printf("Error: %v", err)
}
```

### Recommended Configuration for Slow APIs

For working with slow external APIs (e.g., FNS API) it's recommended:

```go
config := httpclient.Config{
    // Increased timeouts
    Timeout:       60 * time.Second, // 1 minute overall
    PerTryTimeout: 20 * time.Second, // 20 seconds per attempt
    
    // Aggressive retry for stability
    RetryEnabled: true,
    RetryConfig: httpclient.RetryConfig{
        MaxAttempts:       4,    // 4 attempts
        BaseDelay:        500 * time.Millisecond,
        MaxDelay:         15 * time.Second,
        Jitter:           0.3,   // 30% jitter
        RespectRetryAfter: true,
        
        // Additional statuses for retry
        RetryStatusCodes: []int{408, 429, 500, 502, 503, 504, 520, 521, 522, 524},
    },
    
    TracingEnabled: true,
}

client := httpclient.New(config, "external-api-client")
```

## Important Features

### Backward Compatibility

- Non-timeout errors remain unchanged
- Existing code continues to work without changes
- New functionality is only available when explicitly checking error type

### Testing

Comprehensive tests added, covering:

- ✅ Detailed error messages
- ✅ Automatic fix suggestions
- ✅ Various timeout types
- ✅ Handling of non-timeout errors (remain unchanged)
- ✅ Real usage scenarios with FNS API
- ✅ Integration tests with RoundTripper

### Performance

- Minimal performance impact
- Detailed errors are only created on timeouts
- No additional overhead for successful requests

## Usage Examples

See `examples/enhanced_timeout_errors/main.go` for complete examples demonstrating the new functionality.

## Conclusion

This implementation significantly improves timeout problem diagnostics, providing developers with all necessary information for quickly solving problems and optimizing HTTP client settings.
