// Package httpclient предоставляет HTTP клиент с автоматическим сбором метрик,
// настраиваемыми механизмами retry и интеграцией с OpenTelemetry.
package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client представляет HTTP клиент с автоматическими метриками и retry механизмом.
type Client struct {
	httpClient *http.Client
	config     Config
	metrics    *Metrics
	tracer     *Tracer
	name       string
}

// New создаёт новый HTTP клиент с указанной конфигурацией.
func New(config Config, meterName string) *Client {
	// Применяем значения по умолчанию
	config = config.withDefaults()

	// Устанавливаем имя метера по умолчанию если не задано
	if meterName == "" {
		meterName = "http-client"
	}

	// Инициализируем метрики
	metrics := NewMetrics(meterName)

	// Инициализируем трассировку (опционально)
	var tracer *Tracer
	if config.TracingEnabled {
		tracer = NewTracer()
	}

	// Создаём кастомный RoundTripper
	rt := &RoundTripper{
		base:    config.Transport,
		config:  config,
		metrics: metrics,
		tracer:  tracer,
	}

	// Создаём HTTP клиент
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

// Get выполняет GET запрос.
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Post выполняет POST запрос.
func (c *Client) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Put выполняет PUT запрос.
func (c *Client) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Delete выполняет DELETE запрос.
func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Head выполняет HEAD запрос.
func (c *Client) Head(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Patch выполняет PATCH запрос.
func (c *Client) Patch(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	if err != nil {
		return nil, err
	}
	applyOptions(req, opts)
	return c.httpClient.Do(req)
}

// Do выполняет HTTP запрос.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// PostForm выполняет POST запрос с form data.
func (c *Client) PostForm(ctx context.Context, url string, data url.Values) (*http.Response, error) {
	return c.Post(ctx, url, strings.NewReader(data.Encode()), WithContentType("application/x-www-form-urlencoded"))
}

// GetConfig возвращает конфигурацию клиента.
func (c *Client) GetConfig() Config {
	return c.config
}

// Close освобождает ресурсы клиента.
func (c *Client) Close() error {
	if c.metrics != nil {
		return c.metrics.Close()
	}
	return nil
}

// GetWithHeaders выполняет GET запрос с заголовками.
func (c *Client) GetWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Get(ctx, url, WithHeaders(headers))
}

// PostWithHeaders выполняет POST запрос с заголовками.
func (c *Client) PostWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Post(ctx, url, body, WithHeaders(headers))
}

// PutWithHeaders выполняет PUT запрос с заголовками.
func (c *Client) PutWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Put(ctx, url, body, WithHeaders(headers))
}

// DeleteWithHeaders выполняет DELETE запрос с заголовками.
func (c *Client) DeleteWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Delete(ctx, url, WithHeaders(headers))
}

// HeadWithHeaders выполняет HEAD запрос с заголовками.
func (c *Client) HeadWithHeaders(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.Head(ctx, url, WithHeaders(headers))
}

// PatchWithHeaders выполняет PATCH запрос с заголовками.
func (c *Client) PatchWithHeaders(
	ctx context.Context, url string, body io.Reader, headers map[string]string,
) (*http.Response, error) {
	return c.Patch(ctx, url, body, WithHeaders(headers))
}
