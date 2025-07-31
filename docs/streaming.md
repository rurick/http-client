# Потоковые запросы

Поддержка потоковой передачи больших запросов и ответов без загрузки всех данных в память.

## Основы потоковой передачи

### Интерфейс StreamResponse

```go
type StreamResponse interface {
    Body() io.ReadCloser  // Поток данных ответа
    Header() http.Header  // HTTP заголовки
    StatusCode() int      // Статус код
    Close() error         // Закрытие потока
}
```

## Потоковое чтение ответов

### Базовый пример

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "log"
    "net/http"
    
    httpclient "gitlab.citydrive.tech/back-end/go/pkg/http-client"
)

func main() {
    client, err := httpclient.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    
    // Создание запроса
    req, err := http.NewRequest("GET", "https://httpbin.org/stream/100", nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Потоковый запрос
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()
    
    // Чтение данных построчно
    scanner := bufio.NewScanner(stream.Body())
    lineCount := 0
    
    for scanner.Scan() {
        line := scanner.Text()
        lineCount++
        
        fmt.Printf("Строка %d: %s\n", lineCount, line)
        
        // Ограничиваем количество строк для примера
        if lineCount >= 10 {
            break
        }
    }
    
    if err := scanner.Err(); err != nil {
        log.Printf("Ошибка чтения потока: %v", err)
    }
    
    fmt.Printf("Прочитано %d строк, статус: %d\n", lineCount, stream.StatusCode())
}
```

### Обработка больших файлов

```go
func downloadLargeFile(client httpclient.ExtendedHTTPClient, url, filename string) error {
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    
    // Потоковый запрос
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    if stream.StatusCode() != 200 {
        return fmt.Errorf("неудачный статус: %d", stream.StatusCode())
    }
    
    // Создание файла
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Копирование с прогресс баром
    reader := stream.Body()
    
    // Получаем размер файла из заголовков (если доступен)
    contentLength := stream.Header().Get("Content-Length")
    var totalSize int64
    if contentLength != "" {
        totalSize, _ = strconv.ParseInt(contentLength, 10, 64)
    }
    
    // Буферизованная запись с прогрессом
    buffer := make([]byte, 32*1024) // 32KB buffer
    var written int64
    
    for {
        n, err := reader.Read(buffer)
        if n > 0 {
            _, writeErr := file.Write(buffer[:n])
            if writeErr != nil {
                return writeErr
            }
            
            written += int64(n)
            
            // Показываем прогресс
            if totalSize > 0 {
                progress := float64(written) / float64(totalSize) * 100
                fmt.Printf("\rЗагружено: %.1f%% (%d/%d bytes)", progress, written, totalSize)
            } else {
                fmt.Printf("\rЗагружено: %d bytes", written)
            }
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    
    fmt.Printf("\nЗагрузка завершена: %s (%d bytes)\n", filename, written)
    return nil
}
```

## Потоковая отправка данных

### Отправка больших данных

```go
func uploadLargeData(client httpclient.ExtendedHTTPClient, url string, data io.Reader) error {
    req, err := http.NewRequest("POST", url, data)
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "application/octet-stream")
    
    // Потоковая отправка
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    if stream.StatusCode() < 200 || stream.StatusCode() >= 300 {
        // Читаем ошибку из ответа
        body, _ := io.ReadAll(stream.Body())
        return fmt.Errorf("ошибка загрузки (статус %d): %s", 
            stream.StatusCode(), string(body))
    }
    
    fmt.Printf("Данные успешно загружены, статус: %d\n", stream.StatusCode())
    return nil
}

// Пример использования
func main() {
    client, _ := httpclient.NewClient()
    
    // Открываем большой файл
    file, err := os.Open("large_file.dat")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    
    // Загружаем файл потоково
    err = uploadLargeData(client, "https://httpbin.org/post", file)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Отправка данных по частям (chunked)

```go
func sendChunkedData(client httpclient.ExtendedHTTPClient, url string) error {
    // Создаем pipe для потоковой отправки
    reader, writer := io.Pipe()
    
    // Горутина для записи данных
    go func() {
        defer writer.Close()
        
        for i := 0; i < 1000; i++ {
            data := fmt.Sprintf("Chunk %d: %s\n", i, strings.Repeat("data", 100))
            _, err := writer.Write([]byte(data))
            if err != nil {
                log.Printf("Ошибка записи chunk %d: %v", i, err)
                return
            }
            
            // Небольшая задержка для эмуляции генерации данных
            time.Sleep(10 * time.Millisecond)
        }
    }()
    
    // Отправляем данные потоком
    req, err := http.NewRequest("POST", url, reader)
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "text/plain")
    req.Header.Set("Transfer-Encoding", "chunked")
    
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    fmt.Printf("Chunked данные отправлены, статус: %d\n", stream.StatusCode())
    return nil
}
```

## Server-Sent Events (SSE)

### Получение SSE потока

```go
func handleSSE(client httpclient.ExtendedHTTPClient, url string) error {
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    
    // Заголовки для SSE
    req.Header.Set("Accept", "text/event-stream")
    req.Header.Set("Cache-Control", "no-cache")
    
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    if stream.StatusCode() != 200 {
        return fmt.Errorf("неудачный статус SSE: %d", stream.StatusCode())
    }
    
    scanner := bufio.NewScanner(stream.Body())
    
    for scanner.Scan() {
        line := scanner.Text()
        
        // Парсинг SSE событий
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            fmt.Printf("📨 SSE событие: %s\n", data)
        } else if strings.HasPrefix(line, "event: ") {
            eventType := strings.TrimPrefix(line, "event: ")
            fmt.Printf("🏷️  Тип события: %s\n", eventType)
        } else if line == "" {
            // Пустая строка означает конец события
            fmt.Println("---")
        }
    }
    
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("ошибка чтения SSE: %v", err)
    }
    
    return nil
}
```

## JSON потоки

### Обработка JSON Lines (JSONL)

```go
func processJSONLines(client httpclient.ExtendedHTTPClient, url string) error {
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    scanner := bufio.NewScanner(stream.Body())
    lineNum := 0
    
    for scanner.Scan() {
        lineNum++
        line := scanner.Text()
        
        // Парсинг JSON строки
        var data map[string]interface{}
        if err := json.Unmarshal([]byte(line), &data); err != nil {
            log.Printf("Ошибка парсинга JSON в строке %d: %v", lineNum, err)
            continue
        }
        
        // Обработка данных
        fmt.Printf("Запись %d: %+v\n", lineNum, data)
        
        // Можно добавить обработку конкретных полей
        if id, ok := data["id"]; ok {
            fmt.Printf("  ID: %v\n", id)
        }
    }
    
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("ошибка чтения JSONL: %v", err)
    }
    
    fmt.Printf("Обработано %d JSON записей\n", lineNum)
    return nil
}
```

### Потоковый парсинг JSON массива

```go
func streamJSONArray(client httpclient.ExtendedHTTPClient, url string) error {
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    
    stream, err := client.Stream(context.Background(), req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    decoder := json.NewDecoder(stream.Body())
    
    // Читаем открывающую скобку массива
    token, err := decoder.Token()
    if err != nil {
        return fmt.Errorf("ошибка чтения начала массива: %v", err)
    }
    
    if delim, ok := token.(json.Delim); !ok || delim != '[' {
        return fmt.Errorf("ожидался массив JSON, получено: %v", token)
    }
    
    itemCount := 0
    
    // Читаем элементы массива по одному
    for decoder.More() {
        var item map[string]interface{}
        
        if err := decoder.Decode(&item); err != nil {
            log.Printf("Ошибка декодирования элемента %d: %v", itemCount, err)
            continue
        }
        
        itemCount++
        fmt.Printf("Элемент %d: %+v\n", itemCount, item)
        
        // Обработка элемента...
    }
    
    // Читаем закрывающую скобку массива
    token, err = decoder.Token()
    if err != nil {
        return fmt.Errorf("ошибка чтения конца массива: %v", err)
    }
    
    fmt.Printf("Обработано %d элементов массива\n", itemCount)
    return nil
}
```

## Контроль потоков

### Ограничение скорости чтения

```go
type ThrottledReader struct {
    reader    io.Reader
    bytesPerSec int
    lastRead    time.Time
}

func NewThrottledReader(reader io.Reader, bytesPerSec int) *ThrottledReader {
    return &ThrottledReader{
        reader:      reader,
        bytesPerSec: bytesPerSec,
        lastRead:    time.Now(),
    }
}

func (tr *ThrottledReader) Read(p []byte) (n int, err error) {
    n, err = tr.reader.Read(p)
    
    if n > 0 && tr.bytesPerSec > 0 {
        // Вычисляем необходимую задержку
        expectedDuration := time.Duration(float64(n)/float64(tr.bytesPerSec)) * time.Second
        actualDuration := time.Since(tr.lastRead)
        
        if sleepTime := expectedDuration - actualDuration; sleepTime > 0 {
            time.Sleep(sleepTime)
        }
        
        tr.lastRead = time.Now()
    }
    
    return n, err
}

// Использование
func downloadWithThrottling(client httpclient.ExtendedHTTPClient, url string) error {
    req, _ := http.NewRequest("GET", url, nil)
    stream, _ := client.Stream(context.Background(), req)
    defer stream.Close()
    
    // Ограничиваем скорость до 1MB/s
    throttledReader := NewThrottledReader(stream.Body(), 1024*1024)
    
    buffer := make([]byte, 8192)
    for {
        n, err := throttledReader.Read(buffer)
        if n > 0 {
            // Обработка данных...
            fmt.Printf("Прочитано %d байт\n", n)
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### Таймауты для потоков

```go
func streamWithTimeout(client httpclient.ExtendedHTTPClient, url string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    stream, err := client.Stream(ctx, req)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    // Чтение с контролем контекста
    done := make(chan error, 1)
    
    go func() {
        scanner := bufio.NewScanner(stream.Body())
        for scanner.Scan() {
            select {
            case <-ctx.Done():
                done <- ctx.Err()
                return
            default:
                line := scanner.Text()
                fmt.Printf("Данные: %s\n", line)
            }
        }
        done <- scanner.Err()
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return fmt.Errorf("поток прерван по таймауту: %v", ctx.Err())
    }
}
```

## Лучшие практики

### 1. Всегда закрывайте потоки
```go
defer stream.Close() // Обязательно закрывать
```

### 2. Обрабатывайте ошибки чтения
```go
for scanner.Scan() {
    // обработка...
}
if err := scanner.Err(); err != nil {
    // обработка ошибки
}
```

### 3. Используйте буферизацию
```go
reader := bufio.NewReaderSize(stream.Body(), 64*1024) // 64KB buffer
```

### 4. Контролируйте память
```go
// Не загружайте весь поток в память сразу
// Обрабатывайте данные по частям
```

### 5. Мониторьте производительность
```go
start := time.Now()
defer func() {
    fmt.Printf("Поток обработан за %v\n", time.Since(start))
}()
```

## См. также

- [API Reference](api-reference.md) - Полное описание Stream методов
- [Примеры](examples.md) - Дополнительные примеры потоковой обработки
- [Конфигурация](configuration.md) - Настройка таймаутов для потоков