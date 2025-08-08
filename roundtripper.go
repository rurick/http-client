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
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RoundTripper реализует http.RoundTripper с автоматическими метриками и retry
type RoundTripper struct {
	base    http.RoundTripper
	config  Config
	metrics *Metrics
	tracer  *Tracer
}

// RoundTrip выполняет HTTP запрос с автоматическими метриками и retry
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Создаём span для трассировки (если включено)
	var span trace.Span
	if rt.tracer != nil {
		ctx, span = rt.tracer.StartSpan(ctx, fmt.Sprintf("HTTP %s", req.Method))
		defer span.End()

		// Добавляем атрибуты к span
		span.SetAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL.String()),
			attribute.String("http.host", req.URL.Host),
		)
	}

	// Обновляем контекст в запросе
	req = req.WithContext(ctx)

	// Получаем хост для метрик
	host := getHost(req.URL)

	// Увеличиваем счётчик активных запросов
	rt.metrics.IncrementInflight(ctx, req.Method, host)
	defer rt.metrics.DecrementInflight(ctx, req.Method, host)

	// Записываем размер запроса
	requestSize := getRequestSize(req)
	rt.metrics.RecordRequestSize(ctx, requestSize, req.Method, host)

	startTime := time.Now()

	var lastResponse *http.Response
	var lastError error

	// Определяем максимальное количество попыток
	maxAttempts := 1
	if rt.config.RetryEnabled {
		maxAttempts = rt.config.RetryConfig.MaxAttempts
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Создаём контекст с per-try timeout
		attemptCtx, cancel := context.WithTimeout(ctx, rt.config.PerTryTimeout)
		attemptReq := req.WithContext(attemptCtx)

		// Клонируем тело запроса для повторных попыток
		if attempt > 1 && req.Body != nil {
			clonedBody, err := cloneRequestBody(req)
			if err != nil {
				cancel()
				return nil, fmt.Errorf("failed to clone request body for retry: %w", err)
			}
			attemptReq.Body = clonedBody
		}

		// Выполняем запрос
		resp, err := rt.base.RoundTrip(attemptReq)
		cancel()

		duration := time.Since(startTime)
		isRetry := attempt > 1

		// Записываем метрики для этой попытки
		status := 0
		isError := err != nil
		if resp != nil {
			status = resp.StatusCode
		}

		rt.metrics.RecordRequest(ctx, req.Method, host, strconv.Itoa(status), isRetry, isError)
		rt.metrics.RecordDuration(ctx, duration.Seconds(), req.Method, host, strconv.Itoa(status), attempt)

		// Записываем размер ответа
		if resp != nil {
			responseSize := getResponseSize(resp)
			rt.metrics.RecordResponseSize(ctx, responseSize, req.Method, host, strconv.Itoa(status))
		}

		// Обновляем атрибуты span
		if span != nil {
			span.SetAttributes(
				attribute.Int("http.status_code", status),
				attribute.Int("http.attempt", attempt),
				attribute.Bool("http.retry", isRetry),
				attribute.Bool("http.error", isError),
				attribute.Float64("http.duration_seconds", duration.Seconds()),
			)
		}

		lastResponse = resp
		lastError = err

		// Если запрос успешен или retry отключён, возвращаем результат
		if !rt.config.RetryEnabled || err == nil && !shouldRetryStatus(status) {
			return resp, err
		}

		// Проверяем, стоит ли повторять запрос
		if attempt >= maxAttempts {
			break
		}

		// Определяем причину retry
		retryReason := getRetryReason(err, status)
		if retryReason == "" {
			// Не подходит под политику retry
			break
		}

		// Проверяем метод запроса
		if !rt.config.RetryConfig.isMethodRetryable(req.Method) {
			break
		}

		// Записываем метрику retry
		rt.metrics.RecordRetry(ctx, retryReason, req.Method, host)

		// Вычисляем задержку перед следующей попыткой
		delay := rt.calculateRetryDelay(attempt, resp)

		// Проверяем, что задержка не превышает оставшееся время общего таймаута
		if deadline, ok := ctx.Deadline(); ok {
			remainingTime := time.Until(deadline)
			if delay >= remainingTime {
				break // Недостаточно времени для retry
			}
		}

		// Ждём перед следующей попыткой
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Продолжаем к следующей попытке
		}

		startTime = time.Now() // Сбрасываем время для следующей попытки
	}

	return lastResponse, lastError
}

// calculateRetryDelay вычисляет задержку перед следующей попыткой
func (rt *RoundTripper) calculateRetryDelay(attempt int, resp *http.Response) time.Duration {
	config := rt.config.RetryConfig

	// Проверяем заголовок Retry-After
	if config.RespectRetryAfter && resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				return time.Duration(seconds) * time.Second
			}
			if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
				return time.Until(t)
			}
		}
	}

	// Используем exponential backoff с full jitter
	return CalculateBackoffDelay(attempt, config.BaseDelay, config.MaxDelay, config.Jitter)
}

// shouldRetryStatus проверяет, стоит ли повторять запрос для данного статуса
func shouldRetryStatus(status int) bool {
	return status == 429 || (status >= 500 && status <= 599)
}

// getRetryReason определяет причину retry
func getRetryReason(err error, status int) string {
	if err != nil {
		if isNetworkError(err) {
			return "net"
		}
		if isTimeoutError(err) {
			return "timeout"
		}
		return ""
	}

	if shouldRetryStatus(status) {
		return "status"
	}

	return ""
}

// isNetworkError проверяет, является ли ошибка сетевой
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем различные типы сетевых ошибок
	var netErr net.Error
	if ok := errors.As(err, &netErr); ok {
		return netErr.Temporary() || strings.Contains(err.Error(), "connection reset")
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

	var netErr net.Error
	if ok := errors.As(err, &netErr); ok && netErr.Timeout() {
		return true
	}

	var urlErr *url.Error
	if ok := errors.As(err, &urlErr); ok {
		return isTimeoutError(urlErr.Err)
	}

	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
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

// cloneRequestBody клонирует тело запроса для повторных попыток
func cloneRequestBody(req *http.Request) (io.ReadCloser, error) {
	if req.Body == nil {
		return nil, nil
	}

	// Читаем тело в память
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	// Восстанавливаем оригинальное тело
	req.Body = io.NopCloser(bytes.NewReader(body))

	// Возвращаем клон
	return io.NopCloser(bytes.NewReader(body)), nil
}
