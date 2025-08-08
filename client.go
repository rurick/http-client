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

// Client представляет HTTP клиент с автоматическими метриками и retry механизмом
type Client struct {
	httpClient *http.Client
	config     Config
	metrics    *Metrics
	tracer     *Tracer
	name       string
}

// New создаёт новый HTTP клиент с указанной конфигурацией
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

// Get выполняет GET запрос
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// Post выполняет POST запрос
func (c *Client) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.httpClient.Do(req)
}

// Put выполняет PUT запрос
func (c *Client) Put(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.httpClient.Do(req)
}

// Delete выполняет DELETE запрос
func (c *Client) Delete(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// Head выполняет HEAD запрос
func (c *Client) Head(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// Do выполняет HTTP запрос
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// PostForm выполняет POST запрос с form data
func (c *Client) PostForm(ctx context.Context, url string, data url.Values) (*http.Response, error) {
	return c.Post(ctx, url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// GetConfig возвращает конфигурацию клиента
func (c *Client) GetConfig() Config {
	return c.config
}

// Close освобождает ресурсы клиента
func (c *Client) Close() error {
	if c.metrics != nil {
		return c.metrics.Close()
	}
	return nil
}
