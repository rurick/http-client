# Настройки клиента по умолчанию

HTTP клиент поставляется с тщательно подобранными настройками по умолчанию, которые подходят для большинства случаев использования.

## Обзор настроек по умолчанию

### Базовые параметры
```go
// Эти настройки применяются при создании клиента без опций
client, err := httpclient.NewClient()
```

| Параметр         | Значение по умолчанию | Описание                              |
|------------------|----------------------|---------------------------------------|
| Timeout          | 30 секунд | Общий таймаут HTTP запроса            |
| MaxIdleConns     | 100 | Максимум неактивных соединений в пуле |
| MaxConnsPerHost  | 10 | Максимум соединений на один хост      |
| RetryMax         | 0 | Повторы отключены по умолчанию        |
| RetryWaitMin     | 1 секунда | Минимальная задержка между повторами  |
| RetryWaitMax     | 10 секунд | Максимальная задержка между повторами |
| MetricsEnabled   | true | Встроенные метрики включены           |
| MetricsMeterName | httpclient | Метка метрик http-клиента             |
| TracingEnabled   | true | OpenTelemetry трейсинг включен        |
| Logger           | zap.NewNop() | Пустой логгер (без вывода)            |

## Подробное описание настроек

### Таймауты

#### Timeout (30 секунд)
Общий таймаут для выполнения HTTP запроса от начала до конца.

```go
// Значение по умолчанию
opts.Timeout = 30 * time.Second

// Как изменить
client, err := httpclient.NewClient(
    httpclient.WithTimeout(60 * time.Second), // Увеличить до минуты
)
```

**Обоснование**: 30 секунд - разумный баланс между отзывчивостью и надежностью для большинства API.

### Пул соединений

#### MaxIdleConns (100)
Общее количество неактивных HTTP соединений, которые клиент будет хранить для переиспользования.

```go
// Значение по умолчанию
opts.MaxIdleConns = 100

// Как изменить
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(200), // Больший пул для высокой нагрузки
)
```

**Обоснование**: 100 соединений достаточно для большинства приложений, обеспечивая хорошую производительность без чрезмерного потребления памяти.

#### MaxConnsPerHost (10)
Максимальное количество соединений (активных + неактивных) к одному хосту.

```go
// Значение по умолчанию  
opts.MaxConnsPerHost = 10

// Как изменить
client, err := httpclient.NewClient(
    httpclient.WithMaxConnsPerHost(25), // Больше параллелизма
)
```

**Обоснование**: 10 соединений позволяют достаточный параллелизм, не перегружая целевые серверы.

### Механизм повторов

#### RetryMax (0 - отключено)
По умолчанию повторы отключены для предсказуемого поведения.

```go
// Значение по умолчанию - повторы отключены
opts.RetryMax = 0

// Как включить
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3), // 3 попытки повтора
)
```

**Обоснование**: Повторы могут маскировать проблемы и создавать непредсказуемое поведение, поэтому отключены по умолчанию.

#### RetryWaitMin/Max (1-10 секунд)
Параметры для экспоненциальной задержки между повторами (когда повторы включены).

```go
// Значения по умолчанию
opts.RetryWaitMin = 1 * time.Second
opts.RetryWaitMax = 10 * time.Second

// Применяются только при включенных повторах
client, err := httpclient.NewClient(
    httpclient.WithRetryMax(3),
    httpclient.WithRetryWait(500*time.Millisecond, 5*time.Second),
)
```

### Наблюдаемость

#### MetricsEnabled (true)
Встроенный сбор метрик включен по умолчанию.

```go
// Значение по умолчанию
opts.MetricsEnabled = true

// Как отключить
client, err := httpclient.NewClient(
    httpclient.WithMetrics(false),
)
```

**Обоснование**: Метрики важны для мониторинга производительности и отладки.

#### TracingEnabled (true)
OpenTelemetry трейсинг включен по умолчанию.

```go
// Значение по умолчанию
opts.TracingEnabled = true

// Как отключить
client, err := httpclient.NewClient(
    httpclient.WithTracing(false),
)
```

**Обоснование**: Трейсинг помогает в отладке распределенных систем.

#### Logger (zap.NewNop())
По умолчанию используется пустой логгер, который не выводит сообщения.

```go
// Значение по умолчанию - нет вывода
opts.Logger = zap.NewNop()

// Как включить логирование
logger, _ := zap.NewProduction()
client, err := httpclient.NewClient(
    httpclient.WithLogger(logger),
)
```

**Обоснование**: Логирование отключено по умолчанию, чтобы не засорять логи приложения.

## Примеры конфигурации для разных сценариев

### Настройки по умолчанию (без изменений)
```go
client, err := httpclient.NewClient()
// Подходит для: обычные API вызовы, небольшая нагрузка
```

### Разработка и отладка
```go
logger, _ := zap.NewDevelopment()

client, err := httpclient.NewClient(
    httpclient.WithLogger(logger),          // Включить логирование
    httpclient.WithTimeout(10*time.Second), // Короткие таймауты
    httpclient.WithRetryMax(1),             // Минимальные повторы
)
```

### Продакшн с умеренной нагрузкой
```go
logger, _ := zap.NewProduction()

client, err := httpclient.NewClient(
    httpclient.WithTimeout(30*time.Second),    // Стандартный таймаут
    httpclient.WithMaxIdleConns(100),          // Стандартный пул
    httpclient.WithMaxConnsPerHost(15),        // Немного больше соединений
    httpclient.WithRetryMax(3),                // Включить повторы
    httpclient.WithLogger(logger),             // Продакшн логирование
)
```

### Высоконагруженные системы
```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(60*time.Second),    // Больший таймаут
    httpclient.WithMaxIdleConns(300),          // Большой пул
    httpclient.WithMaxConnsPerHost(50),        // Много соединений на хост
    httpclient.WithRetryMax(5),                // Больше повторов
    httpclient.WithMetrics(true),              // Обязательные метрики
)
```

### Микросервисная архитектура
```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(15*time.Second),    // Быстрые внутренние вызовы
    httpclient.WithMaxIdleConns(200),          // Пул для многих сервисов
    httpclient.WithMaxConnsPerHost(25),        // Соединения к каждому сервису
    httpclient.WithRetryMax(2),                // Быстрые повторы
    httpclient.WithTracing(true),              // Важно для трейсинга
)
```

### Внешние API
```go
client, err := httpclient.NewClient(
    httpclient.WithTimeout(45*time.Second),    // Больше времени для внешних API
    httpclient.WithMaxIdleConns(50),           // Меньший пул
    httpclient.WithMaxConnsPerHost(10),        // Ограничения внешних API
    httpclient.WithRetryMax(3),                // Повторы для сетевых проблем
    httpclient.WithRetryStrategy(              // Экспоненциальная задержка
        httpclient.NewExponentialBackoffStrategy(3, 1*time.Second, 30*time.Second),
    ),
)
```

## Когда менять настройки по умолчанию

### Увеличить таймауты, если:
- Работаете с медленными внешними API
- Загружаете большие файлы
- Выполняете сложные операции

### Увеличить пул соединений, если:
- Высокая нагрузка на приложение
- Много параллельных запросов
- Работаете с множеством разных хостов

### Включить повторы, если:
- Работаете в нестабильной сетевой среде
- Критически важные запросы
- Внешние API с временными сбоями

### Настроить логирование, если:
- Разработка и отладка
- Мониторинг продакшн системы
- Анализ производительности

## Как определить оптимальные настройки

### 1. Начните с настроек по умолчанию
```go
client, err := httpclient.NewClient()
```

### 2. Добавьте мониторинг
```go
client, err := httpclient.NewClient(
    httpclient.WithMetrics(true), // Уже включено по умолчанию
)

// Проверяйте метрики
metrics := client.GetMetrics()
```

### 3. Проведите нагрузочное тестирование
```go
// Тестируйте с реальной нагрузкой
for i := 0; i < 1000; i++ {
    go func() {
        resp, err := client.GetCtx(ctx, testURL)
        // обработка ответа
    }()
}
```

### 4. Постепенно увеличивайте параллелизм
```go
// Начните с малого
httpclient.WithMaxConnsPerHost(5)

// Увеличивайте до стабилизации производительности
httpclient.WithMaxConnsPerHost(10)
httpclient.WithMaxConnsPerHost(20)
```

## Валидация настроек

Клиент автоматически проверяет корректность настроек:

```go
// Некорректные значения автоматически исправляются
client, err := httpclient.NewClient(
    httpclient.WithMaxIdleConns(-10),    // Исправится на 0
    httpclient.WithTimeout(-5),          // Исправится на 0 (без таймаута)
)
```

Настройки по умолчанию тщательно протестированы и подходят для 80% случаев использования. Изменяйте их только при наличии конкретных требований или после анализа производительности.