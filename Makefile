# Makefile для HTTP клиента

TOOLS_BIN := tools/bin
GO := go
GOFMT := $(TOOLS_BIN)/gofumpt
GOIMPORTS := $(TOOLS_BIN)/goimports
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
STATICCHECK := $(TOOLS_BIN)/staticcheck
GOSEC := $(TOOLS_BIN)/gosec
GOVULNCHECK := $(TOOLS_BIN)/govulncheck
GOCYCLO := $(TOOLS_BIN)/gocyclo
INEFFASSIGN := $(TOOLS_BIN)/ineffassign
MISSPELL := $(TOOLS_BIN)/misspell

.PHONY: help install-tools build test lint format check security deps clean coverage

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

install-tools: ## Установить инструменты разработки в tools/bin
	@echo "Установка инструментов разработки..."
	@./tools/install.sh

build: ## Собрать проект
	@echo "Сборка проекта..."
	$(GO) build ./...

test: ## Запустить тесты
	@echo "Запуск тестов..."
	$(GO) test -v -race ./...
	$(GO) test -v -race -tags=integration ./... -timeout 120s

test-short: ## Запустить быстрые тесты
	@echo "Запуск быстрых тестов..."
	$(GO) test -short -v ./...

coverage: ## Запустить тесты с покрытием
	@echo "Запуск тестов с покрытием..."
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Покрытие сохранено в coverage.html"

benchmark: ## Запустить бенчмарки
	@echo "Запуск бенчмарков..."
	$(GO) test -bench=. -benchmem ./...

format: ## Форматировать код
	@echo "Форматирование кода..."
	@if [ -f $(GOFMT) ]; then \
		$(GOFMT) -l -w .; \
	else \
		echo "gofumpt не найден, используем gofmt"; \
		$(GO) fmt ./...; \
	fi
	@if [ -f $(GOIMPORTS) ]; then \
		$(GOIMPORTS) -w .; \
	fi

lint: ## Запустить линтеры
	@echo "Запуск линтеров..."
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run; \
	else \
		echo "golangci-lint не найден, запустите 'make install-tools'"; \
		$(GO) vet ./...; \
	fi

staticcheck: ## Запустить статический анализ
	@echo "Статический анализ..."
	@if [ -f $(STATICCHECK) ]; then \
		$(STATICCHECK) ./...; \
	else \
		echo "staticcheck не найден, запустите 'make install-tools'"; \
	fi

security: ## Проверка безопасности
	@echo "Проверка безопасности..."
	@if [ -f $(GOSEC) ]; then \
		$(GOSEC) ./...; \
	else \
		echo "gosec не найден, запустите 'make install-tools'"; \
	fi
	@if [ -f $(GOVULNCHECK) ]; then \
		$(GOVULNCHECK) ./...; \
	else \
		echo "govulncheck не найден, запустите 'make install-tools'"; \
	fi

split:
	./project2file.sh

zip:
	rm project.zip
	zip -r project.zip ./ -x ".*" "*/.*"

cyclo: ## Проверка цикломатической сложности
	@echo "Проверка цикломатической сложности..."
	@if [ -f $(GOCYCLO) ]; then \
		$(GOCYCLO) -over 15 .; \
	else \
		echo "gocyclo не найден, запустите 'make install-tools'"; \
	fi

ineffassign: ## Поиск неиспользуемых присваиваний
	@echo "Поиск неиспользуемых присваиваний..."
	@if [ -f $(INEFFASSIGN) ]; then \
		$(INEFFASSIGN) ./...; \
	else \
		echo "ineffassign не найден, запустите 'make install-tools'"; \
	fi

misspell: ## Проверка орфографии
	@echo "Проверка орфографии..."
	@if [ -f $(MISSPELL) ]; then \
		$(MISSPELL) -error .; \
	else \
		echo "misspell не найден, запустите 'make install-tools'"; \
	fi

check: format lint staticcheck security cyclo ineffassign misspell ## Запустить все проверки

deps: ## Обновить зависимости
	@echo "Обновление зависимостей..."
	$(GO) mod tidy
	$(GO) mod verify

clean: ## Очистить временные файлы
	@echo "Очистка временных файлов..."
	$(GO) clean
	rm -f coverage.out coverage.html
	rm -f *.prof
	rm -f *.test

clean-tools: ## Удалить установленные инструменты
	@echo "Удаление инструментов..."
	rm -rf $(TOOLS_BIN)

ci: deps check test ## Команды для CI (зависимости, проверки, тесты)

all: clean deps format check test build ## Полная сборка и проверка

.DEFAULT_GOAL := help
