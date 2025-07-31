# Автоматический выключатель (Circuit Breaker)

Circuit Breaker защищает систему от каскадных сбоев, автоматически "отключая" проблемные сервисы.

## Принцип работы

Circuit Breaker работает как электрический автомат - при превышении количества ошибок он "размыкает цепь" и начинает немедленно возвращать ошибки без выполнения реальных запросов.

### Состояния Circuit Breaker

1. **Closed (Закрыт)** - Нормальная работа
   - Все запросы проходят к сервису
   - Отслеживается количество ошибок
   
2. **Open (Открыт)** - Сервис недоступен  
   - Запросы блокируются немедленно
   - Возвращается ошибка без обращения к сервису
   - Экономятся ресурсы и время
   
3. **Half-Open (Полуоткрыт)** - Проверка восстановления
   - Пропускается ограниченное количество тестовых запросов
   - При успехе - переход в Closed
   - При неудаче - возврат в Open

## Базовая настройка

```go
// Простой circuit breaker с настройками по умолчанию
circuitBreaker := httpclient.NewSimpleCircuitBreaker()

client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(circuitBreaker),
)
```

## Расширенная конфигурация

```go
// Настраиваемый circuit breaker
circuitBreaker := httpclient.NewCircuitBreaker(
    5,                    // failureThreshold - количество ошибок для открытия
    10*time.Second,       // timeout - время ожидания перед переходом в half-open
    3,                    // maxRequests - максимум запросов в half-open состоянии
)

client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(circuitBreaker),
)
```

### Параметры конфигурации

- **failureThreshold**: Количество подряд идущих ошибок для открытия circuit breaker
- **timeout**: Время ожидания в открытом состоянии перед попыткой восстановления  
- **maxRequests**: Максимальное количество запросов в полуоткрытом состоянии

## Что считается неудачей

Circuit breaker считает запрос неудачным в следующих случаях:

- **Сетевые ошибки** (connection timeout, connection refused, etc.)
- **HTTP ошибки сервера** (статус коды 500-599)
- **Таймауты запросов**

**Не считается неудачей**:
- HTTP статус коды 400-499 (клиентские ошибки)
- Успешные ответы со статус кодами 200-299

## Настройки по умолчанию

```go
// SimpleCircuitBreaker использует эти настройки:
failureThreshold: 5        // 5 ошибок подряд
timeout: 10 * time.Second  // 10 секунд ожидания
maxRequests: 3             // 3 тестовых запроса
```

## Мониторинг состояния

```go
// Получение текущего состояния
state := circuitBreaker.State()

switch state {
case httpclient.CircuitBreakerClosed:
    fmt.Println("Circuit breaker закрыт - нормальная работа")
case httpclient.CircuitBreakerOpen:
    fmt.Println("Circuit breaker открыт - сервис недоступен")
case httpclient.CircuitBreakerHalfOpen:
    fmt.Println("Circuit breaker полуоткрыт - проверка восстановления")
}
```

## Ручное управление

```go
// Принудительный сброс circuit breaker
circuitBreaker.Reset()
```

## Интеграция с метриками

Circuit breaker автоматически отправляет метрики о своем состоянии:

```go
client, err := httpclient.NewClient(
    httpclient.WithCircuitBreaker(circuitBreaker),
    httpclient.WithMetrics(true), // Включить сбор метрик
)

// Получение метрик
metrics := client.GetMetrics()
fmt.Printf("Состояние circuit breaker отслеживается в метриках\n")
```

## Примеры использования

### Для внешних API

```go
// Консервативные настройки для внешних сервисов
circuitBreaker := httpclient.NewCircuitBreaker(
    3,                    // 3 ошибки для открытия
    30*time.Second,       // 30 секунд ожидания
    2,                    // 2 тестовых запроса
)
```

### Для внутренних микросервисов

```go
// Более чувствительные настройки для быстрого реагирования
circuitBreaker := httpclient.NewCircuitBreaker(
    10,                   // 10 ошибок для открытия
    5*time.Second,        // 5 секунд ожидания
    5,                    // 5 тестовых запросов
)
```

### Комбинация с retry механизмом

```go
client, err := httpclient.NewClient(
    // Сначала пытаемся повторить запрос
    httpclient.WithRetryMax(3),
    httpclient.WithRetryStrategy(httpclient.NewExponentialBackoffStrategy(3, 100*time.Millisecond, 2*time.Second)),
    
    // Если много неудач - circuit breaker блокирует запросы
    httpclient.WithCircuitBreaker(httpclient.NewCircuitBreaker(5, 10*time.Second, 3)),
)
```

## Преимущества

### Быстрое восстановление системы
- Блокирует запросы к недоступным сервисам
- Предотвращает накопление таймаутов и ошибок
- Освобождает ресурсы для здоровых сервисов

### Graceful degradation
- Позволяет системе продолжать работу с ограниченной функциональностью
- Возможность показать пользователю cached данные или fallback ответ

### Автоматическое восстановление
- Периодически проверяет доступность сервиса
- Автоматически возобновляет работу при восстановлении

## Лучшие практики

1. **Правильные threshold**: Не слишком низкие (ложные срабатывания) и не слишком высокие (медленная реакция)

2. **Мониторинг**: Всегда отслеживайте состояние circuit breaker в метриках

3. **Fallback**: Предусмотрите fallback логику для случаев когда circuit breaker открыт

```go
resp, err := client.Get("https://api.external.com/data")
if err != nil {
    // Проверяем, не заблокирован ли запрос circuit breaker
    if circuitBreaker.State() == httpclient.CircuitBreakerOpen {
        // Используем cached данные или показываем fallback
        return getCachedData(), nil
    }
    return nil, err
}
```

## См. также

- [Стратегии повтора](retry-strategies.md) - Комбинирование с retry механизмами
- [Метрики](metrics.md) - Мониторинг состояния circuit breaker  
- [Middleware](middleware.md) - Дополнительная обработка ошибок