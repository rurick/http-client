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

// contextAwareBody оборачивает http.Response.Body для отложенной отмены context
// до закрытия body, предотвращая ошибки "context canceled" во время чтения body
type contextAwareBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

// Close закрывает базовое body и отменяет связанный context
func (c *contextAwareBody) Close() error {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
	return c.ReadCloser.Close()
}

// retryContext содержит контекст для выполнения retry
type retryContext struct {
	ctx          context.Context
	originalReq  *http.Request
	originalBody []byte
	host         string
	span         trace.Span
	startTime    time.Time
	maxAttempts  int
}

// RoundTripper реализует http.RoundTripper с автоматическими метриками и retry
type RoundTripper struct {
	base    http.RoundTripper
	config  Config
	metrics *Metrics
	tracer  *Tracer
}

// RoundTrip выполняет HTTP запрос с автоматическими метриками и retry
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := rt.setupTracing(req)
	if span != nil {
		defer span.End()
	}
	req = req.WithContext(ctx)
	host := getHost(req.URL)

	// Управляем метриками активных запросов
	rt.metrics.IncrementInflight(ctx, req.Method, host)
	defer rt.metrics.DecrementInflight(ctx, req.Method, host)

	// Записываем размер запроса
	requestSize := getRequestSize(req)
	rt.metrics.RecordRequestSize(ctx, requestSize, req.Method, host)

	// Подготавливаем тело запроса для retry
	originalBody, err := rt.prepareRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Выполняем цикл попыток
	retryCtx := &retryContext{
		ctx:          ctx,
		originalReq:  req,
		originalBody: originalBody,
		host:         host,
		span:         span,
		startTime:    time.Now(),
		maxAttempts:  rt.getMaxAttempts(),
	}

	return rt.executeWithRetry(retryCtx)
}

// calculateRetryDelay вычисляет задержку перед следующей попыткой.
func (rt *RoundTripper) calculateRetryDelay(attempt int, resp *http.Response) time.Duration {
	config := rt.config.RetryConfig

	// Проверяем заголовок Retry-After
	if delay := rt.parseRetryAfterHeader(config, resp); delay > 0 {
		return delay
	}

	// Используем exponential backoff с full jitter
	return CalculateBackoffDelay(attempt, config.BaseDelay, config.MaxDelay, config.Jitter)
}

// parseRetryAfterHeader парсит заголовок Retry-After.
func (rt *RoundTripper) parseRetryAfterHeader(config RetryConfig, resp *http.Response) time.Duration {
	if !config.RespectRetryAfter || resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Пытаемся парсить как число секунд
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Пытаемся парсить как дату
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// getRetryReasonWithConfig аналогичен getRetryReason, но использует политику статусов из RetryConfig
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

// doTransport выполняет реальный HTTP-запрос, опционально через CircuitBreaker
func (rt *RoundTripper) doTransport(req *http.Request) (*http.Response, error) {
	if rt.config.CircuitBreakerEnable && rt.config.CircuitBreaker != nil {
		return rt.config.CircuitBreaker.Execute(func() (*http.Response, error) {
			return rt.base.RoundTrip(req)
		})
	}
	return rt.base.RoundTrip(req)
}

// shouldRetryAttempt принимает решение о повторе попытки и возвращает причину
func shouldRetryAttempt(
	cfg Config, req *http.Request, attempt, maxAttempts int, err error, status int, deadline time.Time,
) (bool, string) {
	if !cfg.RetryEnabled {
		return false, ""
	}

	// Не ретраим, если вышли по открытому CircuitBreaker
	if errors.Is(err, ErrCircuitBreakerOpen) {
		return false, ""
	}

	// По статусу — используем политику из RetryConfig
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

// recordAttemptMetrics логирует метрики одной попытки
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

// recordRetry логирует метрику повторной попытки
func (rt *RoundTripper) recordRetry(ctx context.Context, reason, method, host string) {
	rt.metrics.RecordRetry(ctx, reason, method, host)
}

// isNetworkError проверяет, является ли ошибка сетевой
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем различные типы сетевых ошибок
	var netErr net.Error
	if ok := errors.As(err, &netErr); ok {
		// Таймауты считаем сетевыми ошибками
		if netErr.Timeout() {
			return true
		}
		return strings.Contains(err.Error(), "connection reset")
	}

	// Проверяем URL ошибки
	var urlErr *url.Error
	if ok := errors.As(err, &urlErr); ok {
		return isNetworkError(urlErr.Err)
	}

	// Проверяем на connection reset
	return strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "connection refused")
}

// isTimeoutError проверяет, является ли ошибка таймаутом
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Проверяем, является ли это нашей детализированной ошибкой тайм-аута
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

	// Проверяем текст ошибки на наличие ключевых слов тайм-аута
	errorMsg := err.Error()
	return strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "deadline exceeded") ||
		strings.Contains(errorMsg, "Client.Timeout exceeded") ||
		strings.Contains(errorMsg, "request canceled while waiting for connection")
}

// getHost извлекает хост из URL для метрик
func getHost(u *url.URL) string {
	if u.Port() != "" {
		return u.Hostname()
	}
	return u.Host
}

// getRequestSize вычисляет размер запроса
func getRequestSize(req *http.Request) int64 {
	if req.Body == nil {
		return 0
	}

	// Пытаемся получить размер из Content-Length
	if req.ContentLength >= 0 {
		return req.ContentLength
	}

	return 0
}

// getResponseSize вычисляет размер ответа
func getResponseSize(resp *http.Response) int64 {
	if resp.ContentLength >= 0 {
		return resp.ContentLength
	}
	return 0
}

// setupTracing настраивает трассировку для запроса
func (rt *RoundTripper) setupTracing(req *http.Request) (context.Context, trace.Span) {
	ctx := req.Context()

	// Создаём span для трассировки (если включено)
	if rt.tracer == nil {
		return ctx, nil
	}

	ctx, span := rt.tracer.StartSpan(ctx, fmt.Sprintf("HTTP %s", req.Method))

	// Добавляем атрибуты к span
	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.host", req.URL.Host),
	)

	return ctx, span
}

// prepareRequestBody подготавливает тело запроса для retry
func (rt *RoundTripper) prepareRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || !rt.config.RetryEnabled {
		return nil, nil
	}

	originalBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close() // Игнорируем ошибку при закрытии

	// Восстанавливаем для первого запроса
	req.Body = io.NopCloser(bytes.NewReader(originalBody))
	return originalBody, nil
}

// getMaxAttempts возвращает максимальное количество попыток
func (rt *RoundTripper) getMaxAttempts() int {
	if rt.config.RetryEnabled {
		return rt.config.RetryConfig.MaxAttempts
	}
	return 1
}

// executeWithRetry выполняет HTTP запрос с retry
func (rt *RoundTripper) executeWithRetry(retryCtx *retryContext) (*http.Response, error) {
	var lastResponse *http.Response
	var lastError error

	for attempt := 1; attempt <= retryCtx.maxAttempts; attempt++ {
		resp, err := rt.executeSingleAttempt(retryCtx, attempt)
		lastResponse = resp
		lastError = err

		// Проверяем, нужно ли повторять
		if !rt.shouldRetryResponse(retryCtx, attempt, resp, err) {
			return resp, err
		}

		// Ждём перед следующей попыткой
		if !rt.waitForRetry(retryCtx, attempt, resp) {
			return lastResponse, lastError
		}
	}

	return lastResponse, lastError
}

// executeSingleAttempt выполняет одну попытку HTTP запроса
func (rt *RoundTripper) executeSingleAttempt(retryCtx *retryContext, attempt int) (*http.Response, error) {
	// Создаём контекст с per-try timeout
	attemptCtx, cancel := context.WithTimeout(retryCtx.ctx, rt.config.PerTryTimeout)
	attemptReq := retryCtx.originalReq.WithContext(attemptCtx)

	// Восстанавливаем тело запроса для повторных попыток
	if attempt > 1 && retryCtx.originalBody != nil {
		attemptReq.Body = io.NopCloser(bytes.NewReader(retryCtx.originalBody))
	}

	// Запоминаем время начала попытки для точного измерения
	attemptStart := time.Now()

	// Выполняем запрос
	resp, err := rt.doTransport(attemptReq)

	// Если произошла ошибка тайм-аута, заменяем её на детализированную
	if err != nil {
		err = rt.enhanceTimeoutError(err, attemptReq, rt.config, attempt, retryCtx.maxAttempts, time.Since(attemptStart))
	}

	// Обрабатываем тело ответа
	resp = rt.wrapResponseBody(resp, err, cancel)

	// Записываем метрики и обновляем tracing
	rt.recordAttemptResults(retryCtx, attempt, resp, err)

	return resp, err
}

// wrapResponseBody оборачивает тело ответа для управления context
func (rt *RoundTripper) wrapResponseBody(resp *http.Response, err error, cancel context.CancelFunc) *http.Response {
	if err == nil && resp != nil && resp.Body != nil {
		resp.Body = &contextAwareBody{
			ReadCloser: resp.Body,
			cancel:     cancel,
		}
	} else {
		cancel() // Отменяем context, если нет body или есть ошибка
	}
	return resp
}

// recordAttemptResults записывает метрики и обновляет tracing
func (rt *RoundTripper) recordAttemptResults(retryCtx *retryContext, attempt int, resp *http.Response, err error) {
	duration := time.Since(retryCtx.startTime)
	isRetry := attempt > 1
	status := 0
	isError := err != nil
	if resp != nil {
		status = resp.StatusCode
	}

	// Записываем метрики
	rt.recordAttemptMetrics(
		retryCtx.ctx, retryCtx.originalReq.Method, retryCtx.host, resp, status, attempt, isRetry, isError, duration,
	)

	// Обновляем span
	rt.updateSpan(retryCtx.span, status, attempt, isRetry, isError, duration)

	// Сбрасываем время для следующей попытки
	retryCtx.startTime = time.Now()
}

// updateSpan обновляет атрибуты span
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

// shouldRetryResponse проверяет, нужно ли повторять запрос
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

// waitForRetry ждёт перед следующей попыткой
func (rt *RoundTripper) waitForRetry(retryCtx *retryContext, attempt int, resp *http.Response) bool {
	// Вычисляем задержку
	delay := rt.calculateRetryDelay(attempt, resp)

	// Проверяем, что задержка не превышает оставшееся время
	if deadline, ok := retryCtx.ctx.Deadline(); ok {
		remainingTime := time.Until(deadline)
		if delay >= remainingTime {
			return false // Недостаточно времени
		}
	}

	// Ждём
	select {
	case <-retryCtx.ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}

// enhanceTimeoutError улучшает ошибки тайм-аута, добавляя детальный контекст
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

	// Определяем тип тайм-аута
	timeoutType := rt.determineTimeoutType(err, config, elapsed)

	// Создаём детализированную ошибку
	return NewTimeoutError(req, config, attempt, maxAttempts, elapsed, timeoutType, err)
}

// determineTimeoutType определяет тип тайм-аута на основе ошибки и конфигурации
func (rt *RoundTripper) determineTimeoutType(err error, config Config, elapsed time.Duration) string {
	errorMsg := err.Error()

	// Проверяем, что это context deadline exceeded
	if strings.Contains(errorMsg, "context deadline exceeded") {
		// Если elapsed время близко к per-try timeout, это per-try timeout
		if elapsed >= config.PerTryTimeout-100*time.Millisecond &&
			elapsed <= config.PerTryTimeout+100*time.Millisecond {
			return "per-try"
		}

		// Если elapsed время близко к общему timeout, это overall timeout
		if elapsed >= config.Timeout-500*time.Millisecond &&
			elapsed <= config.Timeout+500*time.Millisecond {
			return "overall"
		}

		// Иначе это внешний context timeout
		return "context"
	}

	// Другие типы тайм-аутов
	if strings.Contains(errorMsg, "timeout") {
		return "network"
	}

	return "unknown"
}
