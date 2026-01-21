// Package httpclient provides an HTTP client with automatic metrics collection,
// configurable retry mechanisms, and OpenTelemetry integration.
package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client represents an HTTP client with automatic metrics and retry mechanism.
type Client struct {
	httpClient *http.Client
	config     Config
	metrics    *Metrics
	tracer     *Tracer
	name       string
}

// New creates a new HTTP client with the specified configuration.
func New(config Config, meterName string) *Client {
	// Apply default values
	config = config.withDefaults()

	// Set default meter name if not provided
	if meterName == "" {
		meterName = "http-client"
	}

	// Initialize metrics
	var metrics *Metrics
	if config.MetricsEnabled == nil || *config.MetricsEnabled {
		// Select metrics provider based on configuration
		var provider MetricsProvider
		switch config.MetricsBackend {
		case MetricsBackendOpenTelemetry:
			provider = NewOpenTelemetryMetricsProvider(meterName, config.OTelMeterProvider)
		default: // Prometheus by default
			provider = NewPrometheusMetricsProvider(meterName, config.PrometheusRegisterer)
		}
		metrics = NewMetricsWithProvider(meterName, provider)
	} else {
		metrics = NewMetricsWithProvider(meterName, NewNoopMetricsProvider())
	}

	// Initialize tracing (optional)
	var tracer *Tracer
	if config.TracingEnabled {
		tracer = NewTracer()
	}

	// Build RoundTripper chain from bottom to top
	transport := config.Transport

	// Add Rate Limiter if enabled
	if config.RateLimiterEnabled {
		transport = NewRateLimiterRoundTripper(transport, config.RateLimiterConfig)
	}

	// Circuit Breaker is integrated in RoundTripper.doTransport(), no need to modify transport

	// Create custom RoundTripper (retry + metrics + tracing)
	rt := &RoundTripper{
		base:    transport,
		config:  config,
		metrics: metrics,
		tracer:  tracer,
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: rt,
		Timeout:   config.Timeout,
	}

	return &Client{
		httpClient: httpClient,
		config:     config,
		metrics:    metrics,
		tracer:     tracer,
		name:       meterName,
	}
}

// Get executes a GET request.
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Post executes a POST request.
func (c *Client) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Put executes a PUT request.
func (c *Client) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Delete executes a DELETE request.
func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Head executes a HEAD request.
func (c *Client) Head(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Patch executes a PATCH request.
func (c *Client) Patch(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Do executes an HTTP request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// PostForm executes a POST request with form data.
func (c *Client) PostForm(ctx context.Context, url string, data url.Values) (*http.Response, error) {
	return c.Post(ctx, url, strings.NewReader(data.Encode()), WithContentType("application/x-www-form-urlencoded"))
}

// GetConfig returns the client configuration.
func (c *Client) GetConfig() Config {
	return c.config
}

// Close releases client resources.
func (c *Client) Close() error {
	if c.metrics != nil {
		return c.metrics.Close()
	}
	return nil
}

// GetWithHeaders executes a GET request with headers.
func (c *Client) GetWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Get(ctx, url, WithHeaders(headers))
}

// PostWithHeaders executes a POST request with headers.
func (c *Client) PostWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Post(ctx, url, body, WithHeaders(headers))
}

// PutWithHeaders executes a PUT request with headers.
func (c *Client) PutWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Put(ctx, url, body, WithHeaders(headers))
}

// DeleteWithHeaders executes a DELETE request with headers.
func (c *Client) DeleteWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Delete(ctx, url, WithHeaders(headers))
}

// HeadWithHeaders executes a HEAD request with headers.
func (c *Client) HeadWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Head(ctx, url, WithHeaders(headers))
}

// PatchWithHeaders executes a PATCH request with headers.
func (c *Client) PatchWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Patch(ctx, url, body, WithHeaders(headers))
}
