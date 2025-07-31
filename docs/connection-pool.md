# Пул соединений

HTTP клиент использует встроенный пул соединений для оптимизации производительности и управления ресурсами. Пул автоматически переиспользует TCP соединения, что значительно ускоряет HTTP запросы.

## Как работает пул соединений

### Принцип работы
1. **Создание соединения**: При первом запросе к хосту создается новое TCP соединение
2. **Переиспользование**: Завершенные запросы оставляют соединение в пуле для повторного использования
3. **Keep-Alive**: Соединения остаются активными между запросами
4. **Управление ресурсами**: Клиент автоматически закрывает неиспользуемые соединения

### Преимущества
- **Скорость**: Устранение времени установки TCP handshake (обычно 20-100мс)
- **Пропускная способность**: Одновременные запросы к одному хосту
- **Эффективность**: Меньшее потребление системных ресурсов
- **Надежность**: Автоматическое восстановление разорванных соединений

## Настройки пула соединений

### Основные параметры

#### MaxIdleConns
Максимальное количество неактивных соединений во всем пуле.

```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(200), // Пул из 200 соединений
)
```

**Значение по умолчанию**: 100
**Рекомендации**:
- Малая нагрузка: 20-50
- Средняя нагрузка: 50-100
- Высокая нагрузка: 100-500

#### MaxConnsPerHost
Максимальное количество соединений к одному хосту (включая активные и неактивные).

```go
client, err := httpclient.NewClient(
    httpclient.WithMaxConnsPerHost(30), // До 30 соединений на хост
)
```

**Значение по умолчанию**: 10
**Рекомендации**:
- API с низкой латентностью: 5-10
- Обычные API: 10-20
- Высоконагруженные API: 20-100

### Примеры конфигурации

#### Микросервисная архитектура
```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(150),      // Большой общий пул
    httpclient.WithMaxConnsPerHost(25),    // Много соединений к каждому сервису
    httpclient.WithTimeout(10*time.Second),
)
```

#### Внешние API
```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(50),       // Умеренный пул
    httpclient.WithMaxConnsPerHost(15),    // Ограничение на внешние API
    httpclient.WithTimeout(30*time.Second),
)
```

#### Ресурсо-ограниченные системы
```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(20),       // Минимальный пул
    httpclient.WithMaxConnsPerHost(5),     // Строгие ограничения
)
```

## Расширенная настройка транспорта

### Использование собственного http.Client

Для полного контроля над пулом соединений можно использовать собственный `http.Client`:

```go
customTransport := &http.Transport{
    // Настройки пула соединений
    MaxIdleConns:          200,                // Общий пул
    MaxIdleConnsPerHost:   50,                 // На хост
    MaxConnsPerHost:       100,                // Максимум на хост
    
    // Таймауты соединений
    IdleConnTimeout:       90 * time.Second,   // Время жизни неактивного соединения
    TLSHandshakeTimeout:   10 * time.Second,   // Таймаут TLS handshake
    ExpectContinueTimeout: 1 * time.Second,    // Expect: 100-continue
    
    // Настройки TCP
    DialTimeout:           5 * time.Second,    // Таймаут установки соединения
    KeepAlive:            30 * time.Second,    // TCP Keep-Alive
    
    // Дополнительные опции
    DisableKeepAlives:     false,              // Включить Keep-Alive
    DisableCompression:    false,              // Включить сжатие
    ForceAttemptHTTP2:     true,               // Предпочитать HTTP/2
}

customClient := &http.Client{
    Transport: customTransport,
    Timeout:   30 * time.Second,
}

client, err := httpclient.NewClient(
    httpclient.WithHTTPClient(customClient),
)
```

### HTTP/2 поддержка
```go
// HTTP/2 автоматически использует мультиплексирование соединений
transport := &http.Transport{
    ForceAttemptHTTP2:     true,
    MaxIdleConnsPerHost:   10,  // Для HTTP/2 достаточно меньше соединений
}
```

## Мониторинг пула соединений

### Встроенные метрики
```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true),
)

// Получение метрик
metrics := client.GetMetrics()
fmt.Printf("Active connections: %d\n", metrics.ActiveConnections)
fmt.Printf("Idle connections: %d\n", metrics.IdleConnections)
```

### Логирование активности
```go
logger, _ := zap.NewDevelopment()

client, err := httpclient.NewClient(
    httpclient.WithLogger(logger),
    httpclient.WithMiddleware(httpclient.NewLoggingMiddleware(logger)),
)
```

## Производительность и оптимизация

### Бенчмарки пула соединений

| Сценарий | Без пула | С пулом | Улучшение |
|----------|----------|---------|-----------|
| 1000 запросов к одному хосту | 15s | 3s | 5x быстрее |
| Параллельные запросы | Ограничено | Масштабируется | 10x+ |
| Латентность одного запроса | 50-150ms | 1-5ms | 30x+ |

### Рекомендации по оптимизации

1. **Размер пула**: Начните с значений по умолчанию, увеличивайте при необходимости
2. **Мониторинг**: Следите за метриками использования соединений
3. **Тестирование**: Проводите нагрузочное тестирование для определения оптимальных значений
4. **Балансировка**: Учитывайте ограничения целевых серверов

### Типичные проблемы и решения

#### Исчерпание соединений
```go
// Проблема: слишком мало соединений
httpclient.WithMaxConnsPerHost(5)  // Узкое горлышко

// Решение: увеличить лимит
httpclient.WithMaxConnsPerHost(25) // Больше параллелизма
```

#### Утечка соединений
```go
// Всегда закрывайте response.Body
resp, err := client.GetCtx(ctx, url)
if err != nil {
    return err
}
defer resp.Body.Close() // ВАЖНО!
```

#### Медленные соединения
```go
// Настройка таймаутов для медленных соединений
transport := &http.Transport{
    IdleConnTimeout:     30 * time.Second,  // Быстрое закрытие
    TLSHandshakeTimeout: 5 * time.Second,   // Быстрый TLS
}
```

## Контекстные методы и пул соединений

Контекстные методы (GetCtx, PostCtx) полностью совместимы с пулом соединений:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Соединение автоматически возвращается в пул после завершения
resp, err := client.GetCtx(ctx, "https://api.example.com/data")
```

### Отмена запросов
```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(2 * time.Second)
    cancel() // Отменяем запрос, соединение возвращается в пул
}()

resp, err := client.GetCtx(ctx, url)
```

## Интеграция с middleware

Middleware не влияет на работу пула соединений:

```go
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(100),
    httpclient.WithMiddleware(httpclient.NewBearerTokenMiddleware("token")),
    httpclient.WithMiddleware(httpclient.NewRateLimitMiddleware(10, 20)),
)
```

Пул соединений работает на транспортном уровне, middleware на уровне HTTP запросов.

## Лучшие практики

1. **Настройте размер пула под вашу нагрузку**
2. **Используйте контекстные методы для управления таймаутами**  
3. **Всегда закрывайте response.Body**
4. **Мониторьте использование соединений**
5. **Тестируйте под реальной нагрузкой**
6. **Учитывайте ограничения целевых серверов**

Правильно настроенный пул соединений может улучшить производительность приложения в 5-10 раз!