# Тестирование

Руководство по тестированию HTTP-клиента и написанию собственных тестов.

## Запуск тестов

### Основные команды

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с покрытием
go test ./... -cover -coverprofile=coverage.out

# Просмотр покрытия
go tool cover -func=coverage.out

# HTML отчёт по покрытию  
go tool cover -html=coverage.out -o coverage.html
