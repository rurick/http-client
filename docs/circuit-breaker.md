# Circuit Breaker

Circuit Breaker protects the system from cascading failures by automatically "disabling" problematic services and quickly returning an error without waiting for timeouts.

## States

1. **Closed**: all requests are executed. On errors, the failure counter increases.
2. **Open**: requests are not sent. Returns the last unsuccessful response (clone) and `ErrCircuitBreakerOpen` error.
3. **Half-Open**: single "probe" requests. Successes close the breaker, failures return it to open state.

## Enabling and Basic Usage

```go
config := httpclient.Config{
    CircuitBreakerEnable: true, // enable CB
    // CircuitBreaker: nil     // not set — SimpleCircuitBreaker with defaults will be used
}

client := httpclient.New(config, "my-service")
defer client.Close()

resp, err := client.Get(ctx, "https://api.example.com")
```

If `CircuitBreakerEnable == true` and `CircuitBreaker == nil`, `SimpleCircuitBreaker` is automatically created with default settings.

## Custom Configuration

```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 3,              // how many failures before opening
    SuccessThreshold: 1,               // how many successes to close from Half-Open
    Timeout:          10 * time.Second, // pause before transitioning to Half-Open
    FailStatusCodes:  []int{429, 500, 502, 503}, // optional: what counts as failure
    OnStateChange: func(from, to httpclient.CircuitBreakerState) {
        // your logger/metrics
    },
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "my-service")
```

## What Counts as Success/Failure

- Failure: any transport error, `nil` response, or HTTP status from `FailStatusCodes`.
- If `FailStatusCodes == nil`, failures are considered `429` and any `5xx`. Other statuses (including `4xx`, except `429`) are considered success.

## Default Values (SimpleCircuitBreaker)

```text
FailureThreshold: 5
SuccessThreshold: 3
Timeout:          60s
FailStatusCodes:  nil   // means: 429 and >=500 are considered failures
```

## Behavior with Retries

- Circuit Breaker is applied to each attempt.
- `ErrCircuitBreakerOpen` error is not a reason for retry and terminates attempts.

## Getting State and Reset

```go
state := cb.State() // httpclient.CircuitBreakerClosed/Open/HalfOpen
cb.Reset()          // forcibly close the breaker
```

## Observability

- Use `OnStateChange` for logging and metrics of breaker state.
- HTTP client metrics continue to work as usual (requests/durations/retries).

## Example

```go
cb := httpclient.NewCircuitBreakerWithConfig(httpclient.CircuitBreakerConfig{
    FailureThreshold: 2,
    SuccessThreshold: 1,
    Timeout:          5 * time.Second,
})

client := httpclient.New(httpclient.Config{
    CircuitBreakerEnable: true,
    CircuitBreaker:       cb,
}, "orders-client")

resp, err := client.Get(ctx, "https://service.internal/orders/123")
// when breaker is open, a clone of the last unsuccessful response and ErrCircuitBreakerOpen will be returned
```

## Best Practices

1. Adjust thresholds for the service (too low — false positives, too high — late response).
2. Log state transitions via `OnStateChange` and monitor consequences in client metrics.
3. For UX, provide a fallback if the breaker is open (cache/prepared response).
