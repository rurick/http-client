# Метрики тестирование с retry и circuit breaker

Данный документ описывает новые тесты, которые проверяют корректность записи метрик при использовании retry политик и circuit breaker.

## Добавленные тесты

### `TestMetricsWithRetryPolicy`
**Цель:** Проверить что при retry политике все метрики duration и count записываются корректно для каждой попытки.

**Проверяет:**
- `http_client_requests_total` - счетчик увеличивается для каждой попытки с правильным статусом
- `http_client_request_duration_seconds` - duration записывается для каждого attempt (1, 2, 3)
- `http_client_retries_total` - записываются метрики retry с правильным количеством

### `TestMetricsWithCircuitBreaker`
**Цель:** Проверить что метрики записываются корректно при работе circuit breaker, включая cached ответы.

**Проверяет:**
- Метрики записываются для реальных запросов до открытия CB
- Метрики записываются для cached ответов когда CB открыт
- Duration метрики записываются даже для cached ответов
- Cached запросы НЕ доходят до реального сервера

### `TestMetricsWithRetryAndCircuitBreaker`
**Цель:** Проверить комбинированный сценарий retry + circuit breaker.

**Проверяет:**
- Все retry попытки записываются в метрики
- Duration записывается для каждой попытки
- Retry метрики правильно учитывают неудачные попытки

### `TestMetricsWithIdempotentRetry`
**Цель:** Проверить метрики для идемпотентных POST запросов с Idempotency-Key заголовком.

**Проверяет:**
- POST запросы с Idempotency-Key могут повторяться
- Метрики count и duration записываются для каждой попытки POST
- Атрибуты method правильно указывают "POST"

## Ключевые проверки

### 1. Count метрики
- Каждая попытка (включая retry) должна быть записана как отдельная метрика
- Метрики должны иметь правильные атрибуты (method, host, status)
- Circuit breaker cached ответы тоже должны записываться

### 2. Duration метрики  
- Duration должен записываться для каждой попытки
- Атрибут `attempt` должен правильно отражать номер попытки (1, 2, 3...)
- Cached ответы через circuit breaker должны иметь duration (пусть и минимальный)

### 3. Retry метрики
- Должны записываться только для фактических retry (не для первой попытки)
- Должны иметь правильный `reason` атрибут (status, timeout, net)

## Вспомогательные функции

Файл содержит утилитарные функции для извлечения данных из OpenTelemetry метрик:

- `extractMetricsMap()` - извлекает карту метрик по именам
- `getCounterSum()` - получает общую сумму счетчика
- `getCounterSumByAttribute()` - группирует сумму по атрибутам 
- `getHistogramTotalCount()` - общее количество записей в гистограмме
- `getHistogramCountByAttribute()` - группирует записи гистограммы по атрибутам

### Поддержка integer атрибутов

Функция `getHistogramCountByAttribute()` поддерживает как строковые, так и целочисленные атрибуты (например, `attempt` как int64).

## Запуск тестов

```bash
# Запуск всех новых метрик тестов
go test -v -run "TestMetricsWithRetry|TestMetricsWithCircuit|TestMetricsWithIdempotent"

# Запуск всех метрик тестов  
go test -v -run "TestMetrics"

# Запуск конкретного теста
go test -v -run "TestMetricsWithRetryPolicy"
```

## Результат

Эти тесты гарантируют что:
1. ✅ Метрики duration записываются для каждой попытки независимо от retry политики
2. ✅ Метрики count правильно отражают все попытки включая retry
3. ✅ Circuit breaker не нарушает запись метрик
4. ✅ Cached ответы circuit breaker тоже записываются в метрики
5. ✅ Идемпотентные запросы с retry правильно отслеживаются
