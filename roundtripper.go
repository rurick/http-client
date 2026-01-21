package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// contextAwareBody wraps http.Response.Body for deferred context cancellation
// until body is closed, preventing "context canceled" errors during body reading.
type contextAwareBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

// Close closes the underlying body and cancels the associated context.
func (c *contextAwareBody) Close() error {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
	return c.ReadCloser.Close()
}

// retryContext contains context for retry execution.
type retryContext struct {
	ctx            context.Context
	originalReq    *http.Request
	originalBody   []byte
	originalLength int64 // Store original ContentLength
	host           string
	span           trace.Span
	startTime      time.Time
	maxAttempts    int
}

// RoundTripper implements http.RoundTripper with automatic metrics and retry.
type RoundTripper struct {
	base    http.RoundTripper
	config  Config
	metrics *Metrics
	tracer  *Tracer
}

// RoundTrip executes an HTTP request with automatic metrics and retry.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := rt.setupTracing(req)
	if span != nil {
		defer span.End()
	}
	req = req.WithContext(ctx)
	host := getHost(req.URL)

	// Manage active request metrics
	rt.metrics.IncrementInflight(ctx, req.Method, host)
	defer rt.metrics.DecrementInflight(ctx, req.Method, host)

	// Record request size
	requestSize := getRequestSize(req)
	rt.metrics.RecordRequestSize(ctx, requestSize, req.Method, host)

	// Prepare request body for retry
	originalBody, err := rt.prepareRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Execute retry loop
	retryCtx := &retryContext{
		ctx:            ctx,
		originalReq:    req,
		originalBody:   originalBody,
		originalLength: req.ContentLength, // Store original ContentLength
		host:           host,
		span:           span,
		startTime:      time.Now(),
		maxAttempts:    rt.getMaxAttempts(),
	}

	return rt.executeWithRetry(retryCtx)
}

// calculateRetryDelay calculates the delay before the next attempt.
func (rt *RoundTripper) calculateRetryDelay(attempt int, resp *http.Response) time.Duration {
	config := rt.config.RetryConfig

	// Check Retry-After header
	if delay := rt.parseRetryAfterHeader(config, resp); delay > 0 {
		return delay
	}

	// Use exponential backoff with full jitter
	return CalculateBackoffDelay(attempt, config.BaseDelay, config.MaxDelay, config.Jitter)
}

// parseRetryAfterHeader parses the Retry-After header.
func (rt *RoundTripper) parseRetryAfterHeader(config RetryConfig, resp *http.Response) time.Duration {
	if !config.RespectRetryAfter || resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try to parse as number of seconds
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as date
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// getRetryReasonWithConfig is similar to getRetryReason, but uses status policy from RetryConfig.
func getRetryReasonWithConfig(cfg RetryConfig, err error, status int) string {
	if err != nil {
		if isNetworkError(err) {
			return RetryReasonNetwork
		}
		if isTimeoutError(err) {
			return RetryReasonTimeout
		}
		return ""
	}

	if cfg.isStatusRetryable(status) {
		return "status"
	}

	return ""
}

// doTransport executes the actual HTTP request, optionally through CircuitBreaker.
func (rt *RoundTripper) doTransport(req *http.Request) (*http.Response, error) {
	if rt.config.CircuitBreakerEnable && rt.config.CircuitBreaker != nil {
		return rt.config.CircuitBreaker.Execute(func() (*http.Response, error) {
			return rt.base.RoundTrip(req)
		})
	}
	return rt.base.RoundTrip(req)
}

// shouldRetryAttempt makes a decision about retrying an attempt and returns the reason.
func shouldRetryAttempt(
	cfg Config, req *http.Request, attempt, maxAttempts int, err error, status int, deadline time.Time,
) (bool, string) {
	if !cfg.RetryEnabled {
		return false, ""
	}

	// Don't retry if we exited due to open CircuitBreaker
	if errors.Is(err, ErrCircuitBreakerOpen) {
		return false, ""
	}

	// By status — use policy from RetryConfig
	if err == nil && !cfg.RetryConfig.isStatusRetryable(status) {
		return false, ""
	}

	if attempt >= maxAttempts {
		return false, ""
	}

	if !cfg.RetryConfig.isRequestRetryable(req) {
		return false, ""
	}

	if !deadline.IsZero() && time.Until(deadline) <= 0 {
		return false, ""
	}

	reason := getRetryReasonWithConfig(cfg.RetryConfig, err, status)
	if reason == "" {
		return false, ""
	}
	return true, reason
}

// recordAttemptMetrics logs metrics for a single attempt.
func (rt *RoundTripper) recordAttemptMetrics(
	ctx context.Context, method, host string, resp *http.Response, status int, attempt int,
	isRetry bool, isError bool, duration time.Duration,
) {
	rt.metrics.RecordRequest(ctx, method, host, strconv.Itoa(status), isRetry, isError)
	rt.metrics.RecordDuration(ctx, duration.Seconds(), method, host, strconv.Itoa(status), attempt)
	if resp != nil {
		responseSize := getResponseSize(resp)
		rt.metrics.RecordResponseSize(ctx, responseSize, method, host, strconv.Itoa(status))
	}
}

// recordRetry logs a retry metric.
func (rt *RoundTripper) recordRetry(ctx context.Context, reason, method, host string) {
	rt.metrics.RecordRetry(ctx, reason, method, host)
}

// isNetworkError checks if an error is a network error.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check various types of network errors
	var netErr net.Error
	if ok := errors.As(err, &netErr); ok {
		// Timeouts are considered network errors
		if netErr.Timeout() {
			return true
		}
	}

	// Check URL errors
	var urlErr *url.Error
	if ok := errors.As(err, &urlErr); ok {
		return isNetworkError(urlErr.Err)
	}

	// Check for connection reset
	return strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "connection refused")
}

// isTimeoutError checks if an error is a timeout.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Check if this is our detailed timeout error
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return true
	}

	var netErr net.Error
	if ok := errors.As(err, &netErr); ok && netErr.Timeout() {
		return true
	}

	var urlErr *url.Error
	if ok := errors.As(err, &urlErr); ok {
		return isTimeoutError(urlErr.Err)
	}

	// Check error message for timeout keywords
	errorMsg := err.Error()
	return strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "deadline exceeded") ||
		strings.Contains(errorMsg, "Client.Timeout exceeded") ||
		strings.Contains(errorMsg, "request canceled while waiting for connection")
}

// getHost extracts the host from URL for metrics.
func getHost(u *url.URL) string {
	if u.Port() != "" {
		return u.Hostname()
	}
	return u.Host
}

// getRequestSize calculates the request size.
func getRequestSize(req *http.Request) int64 {
	if req.Body == nil {
		return 0
	}

	// Try to get size from Content-Length
	if req.ContentLength >= 0 {
		return req.ContentLength
	}

	return 0
}

// getResponseSize calculates the response size.
func getResponseSize(resp *http.Response) int64 {
	if resp.ContentLength >= 0 {
		return resp.ContentLength
	}
	return 0
}

// setupTracing sets up tracing for the request.
func (rt *RoundTripper) setupTracing(req *http.Request) (context.Context, trace.Span) {
	ctx := req.Context()

	// Create span for tracing (if enabled)
	if rt.tracer == nil {
		return ctx, nil
	}

	ctx, span := rt.tracer.StartSpan(ctx, fmt.Sprintf("HTTP %s", req.Method))

	// Add attributes to span
	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.host", req.URL.Host),
	)

	return ctx, span
}

// prepareRequestBody prepares the request body for retry.
func (rt *RoundTripper) prepareRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || !rt.config.RetryEnabled {
		// No body to prepare or retry disabled
		return nil, nil
	}

	originalBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close() // Ignore error on close

	// Restore for first request
	req.Body = io.NopCloser(bytes.NewReader(originalBody))
	return originalBody, nil
}

// getMaxAttempts returns the maximum number of attempts.
func (rt *RoundTripper) getMaxAttempts() int {
	if rt.config.RetryEnabled {
		return rt.config.RetryConfig.MaxAttempts
	}
	return 1
}

// executeWithRetry executes an HTTP request with retry.
func (rt *RoundTripper) executeWithRetry(retryCtx *retryContext) (*http.Response, error) {
	var lastResponse *http.Response
	var lastError error

	for attempt := 1; attempt <= retryCtx.maxAttempts; attempt++ {
		resp, err := rt.executeSingleAttempt(retryCtx, attempt)
		lastResponse = resp
		lastError = err

		// Check if we need to retry
		if !rt.shouldRetryResponse(retryCtx, attempt, resp, err) {
			return resp, err
		}

		// Wait before next attempt
		if !rt.waitForRetry(retryCtx, attempt, resp) {
			return lastResponse, lastError
		}
	}

	return lastResponse, lastError
}

// executeSingleAttempt executes a single HTTP request attempt.
func (rt *RoundTripper) executeSingleAttempt(retryCtx *retryContext, attempt int) (*http.Response, error) {
	// Create context with per-try timeout
	attemptCtx, cancel := context.WithTimeout(retryCtx.ctx, rt.config.PerTryTimeout)
	attemptReq := retryCtx.originalReq.WithContext(attemptCtx)

	// Restore request body for retry attempts
	if attempt > 1 {
		// CONTENTLENGTH RESTORATION: Critically important!
		// On retry, always need to restore original ContentLength,
		// even for empty bodies (where originalBody may be []byte{})
		attemptReq.ContentLength = retryCtx.originalLength

		if len(retryCtx.originalBody) > 0 {
			attemptReq.Body = io.NopCloser(bytes.NewReader(retryCtx.originalBody))
		} else if retryCtx.originalLength == 0 {
			// For empty bodies set nil body
			attemptReq.Body = nil
		}
	}

	// Remember attempt start time for accurate measurement
	attemptStart := time.Now()

	// Execute request
	resp, err := rt.doTransport(attemptReq)

	// If timeout error occurred, replace it with detailed one
	if err != nil {
		err = rt.enhanceTimeoutError(err, attemptReq, rt.config, attempt, retryCtx.maxAttempts, time.Since(attemptStart))
	}

	// Handle response body
	resp = rt.wrapResponseBody(resp, err, cancel)

	// Record metrics and update tracing
	rt.recordAttemptResults(retryCtx, attempt, resp, err)

	return resp, err
}

// wrapResponseBody wraps the response body for context management.
func (rt *RoundTripper) wrapResponseBody(resp *http.Response, err error, cancel context.CancelFunc) *http.Response {
	if err == nil && resp != nil && resp.Body != nil {
		resp.Body = &contextAwareBody{
			ReadCloser: resp.Body,
			cancel:     cancel,
		}
	} else {
		cancel() // Cancel context if no body or error occurred
	}
	return resp
}

// recordAttemptResults records metrics and updates tracing.
func (rt *RoundTripper) recordAttemptResults(retryCtx *retryContext, attempt int, resp *http.Response, err error) {
	duration := time.Since(retryCtx.startTime)
	isRetry := attempt > 1
	status := 0
	isError := err != nil
	if resp != nil {
		status = resp.StatusCode
	}

	// Record metrics
	rt.recordAttemptMetrics(
		retryCtx.ctx, retryCtx.originalReq.Method, retryCtx.host, resp, status, attempt, isRetry, isError, duration,
	)

	// Update span
	rt.updateSpan(retryCtx.span, status, attempt, isRetry, isError, duration)

	// Reset time for next attempt
	retryCtx.startTime = time.Now()
}

// updateSpan updates span attributes.
func (rt *RoundTripper) updateSpan(
	span trace.Span, status, attempt int, isRetry, isError bool, duration time.Duration,
) {
	if span != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", status),
			attribute.Int("http.attempt", attempt),
			attribute.Bool("http.retry", isRetry),
			attribute.Bool("http.error", isError),
			attribute.Float64("http.duration_seconds", duration.Seconds()),
		)
	}
}

// shouldRetryResponse checks if the request should be retried.
func (rt *RoundTripper) shouldRetryResponse(retryCtx *retryContext, attempt int, resp *http.Response, err error) bool {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}

	deadline, _ := retryCtx.ctx.Deadline()
	shouldRetry, retryReason := shouldRetryAttempt(
		rt.config, retryCtx.originalReq, attempt, retryCtx.maxAttempts, err, status, deadline,
	)

	if shouldRetry {
		rt.recordRetry(retryCtx.ctx, retryReason, retryCtx.originalReq.Method, retryCtx.host)
	}

	return shouldRetry
}

// waitForRetry waits before the next attempt.
func (rt *RoundTripper) waitForRetry(retryCtx *retryContext, attempt int, resp *http.Response) bool {
	// Calculate delay
	delay := rt.calculateRetryDelay(attempt, resp)

	// Check that delay doesn't exceed remaining time
	if deadline, ok := retryCtx.ctx.Deadline(); ok {
		remainingTime := time.Until(deadline)
		if delay >= remainingTime {
			return false // Not enough time
		}
	}

	// Wait
	select {
	case <-retryCtx.ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}

// enhanceTimeoutError enhances timeout errors by adding detailed context.
func (rt *RoundTripper) enhanceTimeoutError(
	err error,
	req *http.Request,
	config Config,
	attempt, maxAttempts int,
	elapsed time.Duration,
) error {
	if err == nil || !isTimeoutError(err) {
		return err
	}

	// Determine timeout type
	timeoutType := rt.determineTimeoutType(err, config, elapsed)

	// Create detailed error
	return NewTimeoutError(req, config, attempt, maxAttempts, elapsed, timeoutType, err)
}

// determineTimeoutType determines the timeout type based on error and configuration.
func (rt *RoundTripper) determineTimeoutType(err error, config Config, elapsed time.Duration) string {
	errorMsg := err.Error()

	// Check if this is context deadline exceeded
	if strings.Contains(errorMsg, "context deadline exceeded") {
		// If elapsed time is close to per-try timeout, this is per-try timeout
		if elapsed >= config.PerTryTimeout-100*time.Millisecond &&
			elapsed <= config.PerTryTimeout+100*time.Millisecond {
			return "per-try"
		}

		// If elapsed time is close to overall timeout, this is overall timeout
		if elapsed >= config.Timeout-500*time.Millisecond &&
			elapsed <= config.Timeout+500*time.Millisecond {
			return "overall"
		}

		// Otherwise this is external context timeout
		return "context"
	}

	// Other timeout types
	if strings.Contains(errorMsg, "timeout") {
		return "network"
	}

	return "unknown"
}
