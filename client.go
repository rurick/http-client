package httpclient

import (
        "bytes"
        "context"
        "encoding/json"
        "encoding/xml"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "time"

        "github.com/hashicorp/go-retryablehttp"
        "go.opentelemetry.io/otel/trace"
        "go.uber.org/zap"
)

// Client - основной HTTP клиент с поддержкой повтора, метрик и промежуточного ПО
type Client struct {
        httpClient       *http.Client
        retryClient      *retryablehttp.Client
        options          *ClientOptions
        middlewareChain  *MiddlewareChain
        metricsCollector MetricsCollector
        logger           *zap.Logger
}

// NewClient создает новый HTTP клиент с заданными опциями
func NewClient(opts ...ClientOption) (*Client, error) {
        options := DefaultOptions()

        // Apply options
        for _, opt := range opts {
                opt(options)
        }

        client := &Client{
                options: options,
                logger:  options.Logger,
        }

        // Setup HTTP client
        if options.HTTPClient != nil {
                client.httpClient = options.HTTPClient
        } else {
                client.httpClient = &http.Client{
                        Timeout: options.Timeout,
                        Transport: &http.Transport{
                                MaxIdleConns:        options.MaxIdleConns,
                                MaxIdleConnsPerHost: options.MaxConnsPerHost,
                                IdleConnTimeout:     90 * time.Second,
                        },
                }
        }

        // Setup retry client
        if options.RetryClient != nil {
                client.retryClient = options.RetryClient
        } else {
                retryClient := retryablehttp.NewClient()
                retryClient.HTTPClient = client.httpClient
                retryClient.RetryMax = options.RetryMax
                retryClient.RetryWaitMin = options.RetryWaitMin
                retryClient.RetryWaitMax = options.RetryWaitMax
                retryClient.Logger = nil // Disable default logging

                // Set custom retry policy if strategy is provided
                if options.RetryStrategy != nil {
                        retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
                                return options.RetryStrategy.ShouldRetry(resp, err), nil
                        }
                        retryClient.Backoff = func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
                                return options.RetryStrategy.NextDelay(attemptNum, nil)
                        }
                }

                client.retryClient = retryClient
        }

        // Setup middleware chain
        middlewares := options.Middlewares

        // Add circuit breaker middleware if configured
        if options.CircuitBreaker != nil {
                middlewares = append([]Middleware{NewCircuitBreakerMiddleware(options.CircuitBreaker)}, middlewares...)
        }

        client.middlewareChain = NewMiddlewareChain(middlewares...)

        // Setup metrics collector
        if options.MetricsEnabled {
                collector, err := NewOTelMetricsCollector("http-client")
                if err != nil {
                        return nil, fmt.Errorf("failed to create metrics collector: %w", err)
                }
                client.metricsCollector = collector
        }

        return client, nil
}

// Do executes an HTTP request with retry, middleware, and metrics
func (c *Client) Do(req *http.Request) (*http.Response, error) {
        return c.DoWithContext(req.Context(), req)
}

// DoWithContext executes an HTTP request with context
func (c *Client) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
        req = req.WithContext(ctx)

        start := time.Now()
        var resp *http.Response
        var err error
        var requestSize, responseSize int64

        // Calculate request size
        if req.Body != nil {
                if req.ContentLength > 0 {
                        requestSize = req.ContentLength
                }
        }

        // Start tracing if enabled
        var span any
        if c.metricsCollector != nil && c.options.TracingEnabled {
                if collector, ok := c.metricsCollector.(*OTelMetricsCollector); ok {
                        ctx, span = collector.StartSpan(ctx, req.Method, req.URL.String())
                        req = req.WithContext(ctx)
                }
        }

        // Execute through middleware chain
        resp, err = c.middlewareChain.Execute(req, c.executeRequest)

        duration := time.Since(start)
        statusCode := 0

        if resp != nil {
                statusCode = resp.StatusCode
                if resp.ContentLength > 0 {
                        responseSize = resp.ContentLength
                }
        }

        // Record metrics
        if c.metricsCollector != nil {
                c.metricsCollector.RecordRequest(req.Method, req.URL.String(), statusCode, duration, requestSize, responseSize)
        }

        // Finish tracing
        if span != nil {
                if collector, ok := c.metricsCollector.(*OTelMetricsCollector); ok {
                        if traceSpan, ok := span.(trace.Span); ok {
                                collector.FinishSpan(traceSpan, statusCode, err)
                        }
                }
        }

        return resp, err
}

// executeRequest is the final handler that executes the HTTP request
func (c *Client) executeRequest(req *http.Request) (*http.Response, error) {
        // Convert to retryable request
        retryableReq, err := retryablehttp.FromRequest(req)
        if err != nil {
                return nil, fmt.Errorf("failed to create retryable request: %w", err)
        }

        return c.retryClient.Do(retryableReq)
}

// Get performs a GET request
func (c *Client) Get(url string) (*http.Response, error) {
        req, err := http.NewRequest(http.MethodGet, url, nil)
        if err != nil {
                return nil, err
        }
        return c.Do(req)
}

// Post performs a POST request
func (c *Client) Post(url, contentType string, body io.Reader) (*http.Response, error) {
        req, err := http.NewRequest(http.MethodPost, url, body)
        if err != nil {
                return nil, err
        }
        req.Header.Set("Content-Type", contentType)
        return c.Do(req)
}

// PostForm performs a POST request with form data
func (c *Client) PostForm(requestURL string, data map[string][]string) (*http.Response, error) {
        form := make(url.Values)
        for key, vals := range data {
                for _, val := range vals {
                        form.Add(key, val)
                }
        }
        return c.Post(requestURL, "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
}

// Head performs a HEAD request
func (c *Client) Head(url string) (*http.Response, error) {
        req, err := http.NewRequest(http.MethodHead, url, nil)
        if err != nil {
                return nil, err
        }
        return c.Do(req)
}

// GetJSON performs a GET request and decodes JSON response
func (c *Client) GetJSON(ctx context.Context, url string, result any) error {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
                return err
        }
        req.Header.Set("Accept", "application/json")

        resp, err := c.Do(req)
        if err != nil {
                return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 400 {
                return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
        }

        return json.NewDecoder(resp.Body).Decode(result)
}

// PostJSON performs a POST request with JSON body and decodes JSON response
func (c *Client) PostJSON(ctx context.Context, url string, body any, result any) error {
        return c.sendJSON(ctx, http.MethodPost, url, body, result)
}

// PutJSON performs a PUT request with JSON body and decodes JSON response
func (c *Client) PutJSON(ctx context.Context, url string, body any, result any) error {
        return c.sendJSON(ctx, http.MethodPut, url, body, result)
}

// PatchJSON performs a PATCH request with JSON body and decodes JSON response
func (c *Client) PatchJSON(ctx context.Context, url string, body any, result any) error {
        return c.sendJSON(ctx, http.MethodPatch, url, body, result)
}

// DeleteJSON performs a DELETE request and decodes JSON response
func (c *Client) DeleteJSON(ctx context.Context, url string, result any) error {
        req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
        if err != nil {
                return err
        }
        req.Header.Set("Accept", "application/json")

        resp, err := c.Do(req)
        if err != nil {
                return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 400 {
                return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
        }

        if result != nil {
                return json.NewDecoder(resp.Body).Decode(result)
        }

        return nil
}

// sendJSON is a helper method for sending JSON requests
func (c *Client) sendJSON(ctx context.Context, method, url string, body any, result any) error {
        var bodyReader io.Reader
        if body != nil {
                jsonBody, err := json.Marshal(body)
                if err != nil {
                        return fmt.Errorf("failed to marshal JSON body: %w", err)
                }
                bodyReader = bytes.NewBuffer(jsonBody)
        }

        req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
        if err != nil {
                return err
        }

        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Accept", "application/json")

        resp, err := c.Do(req)
        if err != nil {
                return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 400 {
                return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
        }

        if result != nil {
                return json.NewDecoder(resp.Body).Decode(result)
        }

        return nil
}

// GetXML performs a GET request and decodes XML response
func (c *Client) GetXML(ctx context.Context, url string, result any) error {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
                return err
        }
        req.Header.Set("Accept", "application/xml")

        resp, err := c.Do(req)
        if err != nil {
                return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 400 {
                return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
        }

        return xml.NewDecoder(resp.Body).Decode(result)
}

// PostXML performs a POST request with XML body and decodes XML response
func (c *Client) PostXML(ctx context.Context, url string, body any, result any) error {
        var bodyReader io.Reader
        if body != nil {
                xmlBody, err := xml.Marshal(body)
                if err != nil {
                        return fmt.Errorf("failed to marshal XML body: %w", err)
                }
                bodyReader = bytes.NewBuffer(xmlBody)
        }

        req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
        if err != nil {
                return err
        }

        req.Header.Set("Content-Type", "application/xml")
        req.Header.Set("Accept", "application/xml")

        resp, err := c.Do(req)
        if err != nil {
                return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 400 {
                return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
        }

        if result != nil {
                return xml.NewDecoder(resp.Body).Decode(result)
        }

        return nil
}

// Stream performs a streaming request
func (c *Client) Stream(ctx context.Context, req *http.Request) (StreamResponse, error) {
        req = req.WithContext(ctx)
        resp, err := c.Do(req)
        if err != nil {
                return nil, err
        }

        return NewStreamResponse(resp), nil
}

// DoCtx performs an HTTP request with context support
func (c *Client) DoCtx(ctx context.Context, req *http.Request) (*http.Response, error) {
        return c.Do(req.WithContext(ctx))
}

// GetCtx performs a GET request with context support
func (c *Client) GetCtx(ctx context.Context, url string) (*http.Response, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
                return nil, err
        }
        return c.Do(req)
}

// PostCtx performs a POST request with context support
func (c *Client) PostCtx(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
        if err != nil {
                return nil, err
        }
        req.Header.Set("Content-Type", contentType)
        return c.Do(req)
}

// PostFormCtx performs a POST request with form data and context support
func (c *Client) PostFormCtx(ctx context.Context, requestURL string, data map[string][]string) (*http.Response, error) {
        form := make(url.Values)
        for key, vals := range data {
                for _, val := range vals {
                        form.Add(key, val)
                }
        }
        return c.PostCtx(ctx, requestURL, "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
}

// HeadCtx performs a HEAD request with context support
func (c *Client) HeadCtx(ctx context.Context, url string) (*http.Response, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
        if err != nil {
                return nil, err
        }
        return c.Do(req)
}

// GetMetricsCollector возвращает коллектор метрик клиента
func (c *Client) GetMetricsCollector() MetricsCollector {
        return c.metricsCollector
}

// GetOptions возвращает копию настроек клиента
func (c *Client) GetOptions() *ClientOptions {
        return c.options
}

// GetMetrics returns the current metrics
func (c *Client) GetMetrics() *ClientMetrics {
        if c.metricsCollector != nil {
                return c.metricsCollector.GetMetrics()
        }
        return NewClientMetrics()
}
