# Metrics and Monitoring

The HTTP client automatically collects metrics via OpenTelemetry (with Prometheus backend support) for full observability of your HTTP requests.

## Available Metrics

### 1. http_client_requests_total (Counter)
Tracks the total number of HTTP requests.

**Labels:**
- `method`: HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
- `host`: Target host (example.com)
- `status`: HTTP status code (200, 404, 500, etc.)
- `retry`: Whether this was a retry attempt (true/false)
- `error`: Whether the request resulted in an error (true/false)

```promql
# Total number of requests
http_client_requests_total

# Requests by method
http_client_requests_total{method="GET"}

# Successful requests
http_client_requests_total{error="false"}
```

### 2. http_client_request_duration_seconds (Histogram)
Measures request duration in seconds.

**Labels:**
- `method`: HTTP method
- `host`: Target host
- `status`: HTTP status code
- `attempt`: Attempt number (1, 2, 3, etc.)

**Buckets:** `0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 7, 10, 13, 16, 20, 25, 30, 40, 50, 60`

```promql
# 95th percentile latency
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le))

# Average latency
rate(http_client_request_duration_seconds_sum[5m]) / rate(http_client_request_duration_seconds_count[5m])
```

### 3. http_client_retries_total (Counter)
Counts retry attempts with reason details.

**Labels:**
- `reason`: Retry reason (status_code, network_error, timeout, connection_error)
- `method`: HTTP method
- `host`: Target host

```promql
# Retry rate
rate(http_client_retries_total[5m])

# Retries by reason
sum(rate(http_client_retries_total[5m])) by (reason)
```

### 4. http_client_inflight_requests (UpDownCounter)
Current number of active requests.

**Labels:**
- `host`: Target host

```promql
# Current active requests
http_client_inflight_requests

# Maximum over period
max_over_time(http_client_inflight_requests[5m])
```

### 5. http_client_request_size_bytes (Histogram)
Request body size in bytes.

**Labels:**
- `method`: HTTP method
- `host`: Target host

**Buckets:** `256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216`

```promql
# 95th percentile request size
histogram_quantile(0.95, sum(rate(http_client_request_size_bytes_bucket[5m])) by (le))
```

### 6. http_client_response_size_bytes (Histogram)
Response body size in bytes.

**Labels:**
- `method`: HTTP method
- `host`: Target host
- `status`: HTTP status code

**Buckets:** Same as request size

```promql
# 95th percentile response size
histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le))
```

## PromQL Queries

### Basic Performance Metrics

#### Request Rate (RPS)
```promql
# Requests per second
sum(rate(http_client_requests_total[5m]))

# RPS by services
sum(rate(http_client_requests_total[5m])) by (host)

# RPS by methods
sum(rate(http_client_requests_total[5m])) by (method)
```

#### Error Percentage
```promql
# Overall error percentage
sum(rate(http_client_requests_total{error="true"}[5m])) / 
sum(rate(http_client_requests_total[5m])) * 100

# Error percentage by services
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) / 
sum(rate(http_client_requests_total[5m])) by (host) * 100

# HTTP error percentage (4xx, 5xx)
sum(rate(http_client_requests_total{status=~"[45].."}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host) * 100
```

#### Latency Analysis
```promql
# 50th, 95th, 99th percentiles
histogram_quantile(0.50, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
histogram_quantile(0.99, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Average latency
sum(rate(http_client_request_duration_seconds_sum[5m])) by (host) /
sum(rate(http_client_request_duration_seconds_count[5m])) by (host)

# Latency by status codes
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, status))
```

### Retry Behavior Analysis

#### Retry Statistics
```promql
# Retry rate
sum(rate(http_client_retries_total[5m])) by (host, reason)

# Percentage of requests with retries
sum(rate(http_client_requests_total{retry="true"}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host) * 100

# Retry success rate
sum(rate(http_client_requests_total{retry="true", error="false"}[5m])) by (host) /
sum(rate(http_client_retries_total[5m])) by (host) * 100
```

#### Top Retry Reasons
```promql
# Most frequent retry reasons
topk(5, sum(rate(http_client_retries_total[5m])) by (reason))

# Retries by services
topk(10, sum(rate(http_client_retries_total[5m])) by (host))
```

### Load Analysis

#### Active Connections
```promql
# Current active requests
http_client_inflight_requests

# Peak load over hour
max_over_time(http_client_inflight_requests[1h])

# Average load
avg_over_time(http_client_inflight_requests[5m])
```

#### Size Analysis
```promql
# Average request size
rate(http_client_request_size_bytes_sum[5m]) / rate(http_client_request_size_bytes_count[5m])

# Average response size
rate(http_client_response_size_bytes_sum[5m]) / rate(http_client_response_size_bytes_count[5m])

# Top "heaviest" endpoints
topk(10, histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le, host)))
```

### Dashboard Queries

#### SLI Metrics
```promql
# Availability (99.9% target)
sum(rate(http_client_requests_total{error="false"}[5m])) /
sum(rate(http_client_requests_total[5m])) * 100

# Latency SLI (95% requests < 500ms)
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le))

# Throughput
sum(rate(http_client_requests_total[5m]))
```

## Alert Rules

### Critical Alerts

#### High Error Rate
```yaml
groups:
- name: httpclient.critical
  rules:
  - alert: HTTPClientHighErrorRate
    expr: |
      (
        sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
        sum(rate(http_client_requests_total[5m])) by (host)
      ) > 0.05
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "High HTTP client error rate"
      description: "{{ $labels.host }} has {{ $value | humanizePercentage }} errors in the last 5 minutes"
```

#### Critically High Latency
```yaml
  - alert: HTTPClientCriticalLatency
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 5
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Critically high HTTP client latency"
      description: "{{ $labels.host }} has 95th percentile latency of {{ $value }}s"
```

### Warnings

#### Elevated Latency
```yaml
- name: httpclient.warnings
  rules:
  - alert: HTTPClientHighLatency
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host)) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Elevated HTTP client latency"
      description: "{{ $labels.host }} has 95th percentile latency of {{ $value }}s over 5 minutes"
```

#### Excessive Retries
```yaml
  - alert: HTTPClientExcessiveRetries
    expr: |
      sum(rate(http_client_retries_total[5m])) by (host) > 1
    for: 3m
    labels:
      severity: warning
    annotations:
      summary: "High HTTP client retry rate"
      description: "{{ $labels.host }} makes {{ $value }} retries/sec over the last 5 minutes"
```

#### Many Active Requests
```yaml
  - alert: HTTPClientHighInflight
    expr: |
      http_client_inflight_requests > 100
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Many concurrent HTTP requests"
      description: "{{ $labels.host }} has {{ $value }} concurrent requests"
```

### Informational Alerts

#### Unusually Large Responses
```yaml
- name: httpclient.info
  rules:
  - alert: HTTPClientLargeResponses
    expr: |
      histogram_quantile(0.95, sum(rate(http_client_response_size_bytes_bucket[5m])) by (le, host)) > 10485760 # 10MB
    for: 10m
    labels:
      severity: info
    annotations:
      summary: "Large HTTP responses"
      description: "{{ $labels.host }} returns responses of size {{ $value | humanizeBytes }}"
```

## Grafana Dashboard

### Main Panels

#### Overview Panel
```promql
# Requests per second
sum(rate(http_client_requests_total[5m])) by (host)

# Error rate
sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
sum(rate(http_client_requests_total[5m])) by (host)

# 95th percentile latency
histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))

# Active requests
http_client_inflight_requests
```

#### Detailed Analytics
```promql
# Requests by method
sum(rate(http_client_requests_total[5m])) by (method)

# Status code distribution
sum(rate(http_client_requests_total[5m])) by (status)

# Retry analysis
sum(rate(http_client_retries_total[5m])) by (reason)

# Size distribution
histogram_quantile(0.95, sum(rate(http_client_request_size_bytes_bucket[5m])) by (le))
```

### Recording Rules

Use recording rules for performance optimization:

```yaml
groups:
- name: httpclient.recording
  interval: 30s
  rules:
  - record: httpclient:request_rate
    expr: sum(rate(http_client_requests_total[5m])) by (host)
    
  - record: httpclient:error_rate
    expr: |
      sum(rate(http_client_requests_total{error="true"}[5m])) by (host) /
      sum(rate(http_client_requests_total[5m])) by (host)
    
  - record: httpclient:latency_p95
    expr: histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket[5m])) by (le, host))
    
  - record: httpclient:retry_rate
    expr: sum(rate(http_client_retries_total[5m])) by (host)
```

## Using Metrics in Code

Metrics are collected automatically, but you can access them:

```go
// Metrics are available through the client (internal API)
// Usually no direct interaction is required

client := httpclient.New(config, "my-service")
defer client.Close()

// All metrics are collected automatically when executing requests
resp, err := client.Get(ctx, "https://api.example.com/data")
```

## Using Metrics in Code

```go
import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    httpclient "github.com/rurick/http-client"
)

// Metrics are created automatically when creating the client
client := httpclient.New(httpclient.Config{}, "my-service")
defer client.Close()

// Create HTTP endpoint for metrics - metrics are automatically registered
http.Handle("/metrics", promhttp.Handler())

// Metrics are collected automatically when executing requests
resp, err := client.Get(ctx, "https://api.example.com/data")
```

## Metrics Troubleshooting

### Metrics Not Appearing
1. Check metric registration in prometheus.DefaultRegistry
2. Ensure HTTP endpoint /metrics is configured correctly
3. Check that the client is making requests

### Unexpected Values
1. Check labels in PromQL queries
2. Verify time intervals are correct
3. Check filters by host/method

### Performance
1. Use recording rules for frequently used queries
2. Optimize PromQL query execution time
3. Configure appropriate retention policy