package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDefaultOptions проверяет значения опций по умолчанию
func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	assert.Equal(t, 30*time.Second, opts.Timeout)
	assert.Equal(t, 100, opts.MaxIdleConns)
	assert.Equal(t, 10, opts.MaxConnsPerHost)
	assert.Equal(t, 0, opts.RetryMax)
	assert.Equal(t, 1*time.Second, opts.RetryWaitMin)
	assert.Equal(t, 10*time.Second, opts.RetryWaitMax)
	assert.Nil(t, opts.RetryStrategy)
	assert.True(t, opts.MetricsEnabled)
	assert.True(t, opts.TracingEnabled)
	assert.NotNil(t, opts.Logger)
	assert.Empty(t, opts.Middlewares)
}

// TestWithTimeout проверяет установку таймаута
func TestWithTimeout(t *testing.T) {
	opts := DefaultOptions()
	WithTimeout(45 * time.Second)(opts)

	assert.Equal(t, 45*time.Second, opts.Timeout)
}

// TestWithMaxIdleConns проверяет установку максимального количества неактивных соединений
func TestWithMaxIdleConns(t *testing.T) {
	opts := DefaultOptions()
	WithMaxIdleConns(200)(opts)

	assert.Equal(t, 200, opts.MaxIdleConns)
}

// TestWithMaxConnsPerHost проверяет установку максимального количества соединений на хост
func TestWithMaxConnsPerHost(t *testing.T) {
	opts := DefaultOptions()
	WithMaxConnsPerHost(25)(opts)

	assert.Equal(t, 25, opts.MaxConnsPerHost)
}

// TestWithRetryMax проверяет установку максимального количества повторов
func TestWithRetryMax(t *testing.T) {
	opts := DefaultOptions()
	WithRetryMax(5)(opts)

	assert.Equal(t, 5, opts.RetryMax)
}

// TestWithRetryWait проверяет установку времени ожидания повторов
func TestWithRetryWait(t *testing.T) {
	opts := DefaultOptions()
	WithRetryWait(500*time.Millisecond, 30*time.Second)(opts)

	assert.Equal(t, 500*time.Millisecond, opts.RetryWaitMin)
	assert.Equal(t, 30*time.Second, opts.RetryWaitMax)
}

// TestWithRetryStrategy проверяет установку стратегии повторов
func TestWithRetryStrategy(t *testing.T) {
	opts := DefaultOptions()
	strategy := NewExponentialBackoffStrategy(3, 1*time.Second, 30*time.Second)
	WithRetryStrategy(strategy)(opts)

	assert.Equal(t, strategy, opts.RetryStrategy)
	assert.Equal(t, 3, opts.RetryMax)
}

// TestWithCircuitBreaker проверяет установку circuit breaker
func TestWithCircuitBreaker(t *testing.T) {
	opts := DefaultOptions()
	cb := NewSimpleCircuitBreaker()
	WithCircuitBreaker(cb)(opts)

	assert.Equal(t, cb, opts.CircuitBreaker)
}

// TestWithMiddleware проверяет добавление middleware
func TestWithMiddleware(t *testing.T) {
	opts := DefaultOptions()
	middleware := NewHeaderMiddleware(map[string]string{"X-Test": "value"})
	WithMiddleware(middleware)(opts)

	assert.Len(t, opts.Middlewares, 1)
	assert.Equal(t, middleware, opts.Middlewares[0])
}

// TestWithMultipleMiddleware проверяет добавление нескольких middleware
func TestWithMultipleMiddleware(t *testing.T) {
	opts := DefaultOptions()
	middleware1 := NewHeaderMiddleware(map[string]string{"X-Test-1": "value1"})
	middleware2 := NewHeaderMiddleware(map[string]string{"X-Test-2": "value2"})

	WithMultipleMiddleware(middleware1, middleware2)(opts)

	assert.Len(t, opts.Middlewares, 2)
	assert.Equal(t, middleware1, opts.Middlewares[0])
	assert.Equal(t, middleware2, opts.Middlewares[1])
}

// TestCloneMiddlewares проверяет клонирование middleware
func TestCloneMiddlewares(t *testing.T) {
	opts := DefaultOptions()
	middleware := NewHeaderMiddleware(map[string]string{"X-Test": "value"})
	WithMiddleware(middleware)(opts)

	cloned := opts.CloneMiddlewares()

	assert.Len(t, cloned, 1)
	assert.Equal(t, middleware, cloned[0])

	// Проверяем что это копия, а не ссылка
	opts.Middlewares = append(opts.Middlewares, NewHeaderMiddleware(map[string]string{"X-Test-2": "value2"}))
	assert.Len(t, cloned, 1) // Клон не должен измениться
}

// TestHasMiddleware проверяет проверку наличия middleware
func TestHasMiddleware(t *testing.T) {
	opts := DefaultOptions()
	middleware := NewHeaderMiddleware(map[string]string{"X-Test": "value"})
	WithMiddleware(middleware)(opts)

	assert.True(t, opts.HasMiddleware(middleware))

	otherMiddleware := NewHeaderMiddleware(map[string]string{"X-Other": "value"})
	assert.False(t, opts.HasMiddleware(otherMiddleware))
}

// TestWithLogger проверяет установку логгера
func TestWithLogger(t *testing.T) {
	opts := DefaultOptions()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	WithLogger(logger)(opts)

	assert.Equal(t, logger, opts.Logger)
}

// TestWithMetrics проверяет включение/отключение метрик
func TestWithMetrics(t *testing.T) {
	opts := DefaultOptions()

	// Отключаем метрики
	WithMetrics(false)(opts)
	assert.False(t, opts.MetricsEnabled)

	// Включаем метрики
	WithMetrics(true)(opts)
	assert.True(t, opts.MetricsEnabled)
}

// TestWithMetricsName проверяет установку префикса метрик
func TestWithMetricsName(t *testing.T) {
	opts := DefaultOptions()

	// Устанавливаем пользовательский префикс
	WithMetricsMeterName("myapp")(opts)
	assert.Equal(t, "myapp", opts.MetricsMeterName)

	// Тестируем с другим префиксом
	WithMetricsMeterName("api_service")(opts)
	assert.Equal(t, "api_service", opts.MetricsMeterName)
}

// TestWithMetricsName_EmptyPrefix проверяет обработку пустого префикса
func TestWithMetricsName_EmptyPrefix(t *testing.T) {
	opts := DefaultOptions()

	// Пустой префикс должен установить значение по умолчанию
	WithMetricsMeterName("")(opts)
	assert.Equal(t, defaultMetricMeterName, opts.MetricsMeterName)

	// Проверяем что значение по умолчанию уже установлено
	assert.Equal(t, defaultMetricMeterName, DefaultOptions().MetricsMeterName)
}

// TestWithTracing проверяет включение/отключение трейсинга
func TestWithTracing(t *testing.T) {
	opts := DefaultOptions()

	// Отключаем трейсинг
	WithTracing(false)(opts)
	assert.False(t, opts.TracingEnabled)

	// Включаем трейсинг
	WithTracing(true)(opts)
	assert.True(t, opts.TracingEnabled)
}

// TestWithHTTPClient проверяет установку пользовательского HTTP клиента
func TestWithHTTPClient(t *testing.T) {
	opts := DefaultOptions()
	customClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	WithHTTPClient(customClient)(opts)

	assert.Equal(t, customClient, opts.HTTPClient)
}

// TestChainedOptions проверяет цепочку опций
func TestChainedOptions(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	middleware := NewHeaderMiddleware(map[string]string{"X-Test": "value"})

	opts := DefaultOptions()

	// Применяем несколько опций подряд
	WithTimeout(45 * time.Second)(opts)
	WithMaxIdleConns(200)(opts)
	WithRetryMax(3)(opts)
	WithLogger(logger)(opts)
	WithMiddleware(middleware)(opts)
	WithMetrics(false)(opts)

	// Проверяем что все опции применились
	assert.Equal(t, 45*time.Second, opts.Timeout)
	assert.Equal(t, 200, opts.MaxIdleConns)
	assert.Equal(t, 3, opts.RetryMax)
	assert.Equal(t, logger, opts.Logger)
	assert.Len(t, opts.Middlewares, 1)
	assert.False(t, opts.MetricsEnabled)
}

// TestMetricsNameInDefaultOptions проверяет что префикс метрик включен в значения по умолчанию
func TestMetricsNameInDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	assert.Equal(t, defaultMetricMeterName, opts.MetricsMeterName)
	assert.True(t, opts.MetricsEnabled)
	assert.True(t, opts.TracingEnabled)
}

// TestWithMetricsName_SpecialCharacters проверяет работу с специальными символами
func TestWithMetricsName_SpecialCharacters(t *testing.T) {
	opts := DefaultOptions()

	// Префикс с подчеркиваниями (стандарт Prometheus)
	WithMetricsMeterName("my_app_service")(opts)
	assert.Equal(t, "my_app_service", opts.MetricsMeterName)

	// Префикс с точками (может использоваться в неймспейсах)
	WithMetricsMeterName("com.example.service")(opts)
	assert.Equal(t, "com.example.service", opts.MetricsMeterName)
}

// TestCombinedMetricsOptions проверяет сочетание опций метрик
func TestCombinedMetricsOptions(t *testing.T) {
	opts := DefaultOptions()

	// Применяем несколько опций метрик
	WithMetrics(true)(opts)
	WithMetricsMeterName("custom_app")(opts)
	WithTracing(false)(opts)

	assert.True(t, opts.MetricsEnabled)
	assert.Equal(t, "custom_app", opts.MetricsMeterName)
	assert.False(t, opts.TracingEnabled)
}
