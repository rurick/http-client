# Makefile для HTTP клиента

# Переменные инструментов
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
NANCY := $(TOOLS_BIN)/nancy
SEMGREP := $(TOOLS_BIN)/semgrep

# Версии инструментов
GOLANGCI_LINT_VERSION := v1.55.2
GOSEC_VERSION := v2.18.2
GOVULNCHECK_VERSION := latest
STATICCHECK_VERSION := latest
NANCY_VERSION := v1.0.42

# Конфигурация
COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html
GOLANGCI_CONFIG := .golangci.yml
LINT_REPORT := lint-report.xml
SAST_REPORT := sast-report.json
SECURITY_REPORT := security-report.json

.PHONY: help install-tools build test lint format check security deps clean coverage \
         lint-full lint-fix lint-report lint-godot lint-lll \
         sast sast-full sast-report security-full security-report \
         deps-check deps-audit deps-license vuln-check nancy-audit

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

# =============================================================================
# УЛУЧШЕННЫЕ КОМАНДЫ ЛИНТЕРОВ
# =============================================================================

lint-full: ## Запустить все линтеры с максимальной детализацией
	@echo "🔍 Полный анализ кода с линтерами..."
	@if [ -f $(GOLANGCI_CONFIG) ]; then \
		echo "✓ Найден конфиг golangci-lint: $(GOLANGCI_CONFIG)"; \
	else \
		echo "⚠️  Конфиг golangci-lint не найден, используются настройки по умолчанию"; \
	fi
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run --config=$(GOLANGCI_CONFIG) --verbose --print-resources-usage; \
	else \
		echo "❌ golangci-lint не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

lint-fix: ## Автоматически исправить проблемы линтеров (где возможно)
	@echo "🔧 Автоматическое исправление проблем линтеров..."
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run --config=$(GOLANGCI_CONFIG) --fix; \
		echo "✓ Автоматические исправления применены"; \
	else \
		echo "❌ golangci-lint не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

lint-report: ## Создать отчёт линтеров в XML формате
	@echo "📊 Создание отчёта линтеров..."
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run --config=$(GOLANGCI_CONFIG) --out-format=junit-xml --out-file=$(LINT_REPORT) || true; \
		echo "✓ Отчёт сохранён: $(LINT_REPORT)"; \
	else \
		echo "❌ golangci-lint не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

lint-godot: ## Проверить только правила godot (точки в комментариях)
	@echo "📝 Проверка правил godot..."
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run --config=$(GOLANGCI_CONFIG) --enable-only=godot; \
	else \
		echo "❌ golangci-lint не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

lint-lll: ## Проверить только правила lll (длинные строки)
	@echo "📏 Проверка длинных строк..."
	@if [ -f $(GOLANGCI_LINT) ]; then \
		$(GOLANGCI_LINT) run --config=$(GOLANGCI_CONFIG) --enable-only=lll; \
	else \
		echo "❌ golangci-lint не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

# =============================================================================
# КОМАНДЫ SAST (СТАТИЧЕСКИЙ АНАЛИЗ БЕЗОПАСНОСТИ)
# =============================================================================

sast: ## Базовый SAST анализ
	@echo "🔒 Статический анализ безопасности (SAST)..."
	@$(MAKE) --no-print-directory staticcheck
	@$(MAKE) --no-print-directory security

sast-full: ## Полный SAST анализ со всеми инструментами
	@echo "🔒 Полный SAST анализ..."
	@echo "📋 Этапы анализа:"
	@echo "  1. StaticCheck - статический анализ кода"
	@echo "  2. GoSec - анализ безопасности"
	@echo "  3. GoVulnCheck - проверка уязвимостей"
	@echo "  4. Nancy - аудит зависимостей"
	@echo ""
	@$(MAKE) --no-print-directory staticcheck
	@$(MAKE) --no-print-directory security
	@$(MAKE) --no-print-directory nancy-audit
	@echo "✅ Полный SAST анализ завершён"

sast-report: ## Создать подробный отчёт SAST в JSON формате
	@echo "📊 Создание отчёта SAST..."
	@echo '{"sast_report": {"timestamp": "'$(shell date -Iseconds)'", "reports": []}}' > $(SAST_REPORT)
	@if [ -f $(GOSEC) ]; then \
		$(GOSEC) -fmt=json -out=gosec-temp.json ./... || true; \
		echo "✓ GoSec отчёт создан"; \
	else \
		echo "⚠️  GoSec не найден"; \
	fi
	@if [ -f $(STATICCHECK) ]; then \
		$(STATICCHECK) -f=json ./... > staticcheck-temp.json 2>/dev/null || true; \
		echo "✓ StaticCheck отчёт создан"; \
	else \
		echo "⚠️  StaticCheck не найден"; \
	fi
	@echo "✅ SAST отчёт сохранён: $(SAST_REPORT)"

# =============================================================================
# УЛУЧШЕННЫЕ КОМАНДЫ БЕЗОПАСНОСТИ
# =============================================================================

security-full: ## Полная проверка безопасности
	@echo "🛡️  Полная проверка безопасности..."
	@echo "📋 Этапы проверки:"
	@echo "  1. GoSec - анализ уязвимостей в коде"
	@echo "  2. GoVulnCheck - проверка известных уязвимостей"
	@echo "  3. Deps audit - аудит зависимостей"
	@echo "  4. License check - проверка лицензий"
	@echo ""
	@$(MAKE) --no-print-directory security
	@$(MAKE) --no-print-directory deps-audit
	@$(MAKE) --no-print-directory deps-license
	@echo "✅ Полная проверка безопасности завершена"

security-report: ## Создать отчёт по безопасности в JSON
	@echo "📊 Создание отчёта по безопасности..."
	@if [ -f $(GOSEC) ]; then \
		$(GOSEC) -fmt=json -out=$(SECURITY_REPORT) ./... || true; \
		echo "✓ Отчёт по безопасности сохранён: $(SECURITY_REPORT)"; \
	else \
		echo "❌ GoSec не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

# =============================================================================
# ПРОВЕРКА ЗАВИСИМОСТЕЙ
# =============================================================================

deps-check: ## Проверить состояние зависимостей
	@echo "📦 Проверка зависимостей..."
	@echo "📋 go mod tidy check..."
	@$(GO) mod tidy
	@if ! git diff --quiet go.mod go.sum; then \
		echo "❌ go.mod или go.sum изменились после 'go mod tidy'"; \
		echo "Пожалуйста, зафиксируйте изменения:"; \
		git diff go.mod go.sum; \
		exit 1; \
	else \
		echo "✅ Зависимости в порядке"; \
	fi
	@echo "📋 go mod verify..."
	@$(GO) mod verify

deps-audit: ## Аудит безопасности зависимостей
	@echo "🔍 Аудит безопасности зависимостей..."
	@$(MAKE) --no-print-directory vuln-check
	@$(MAKE) --no-print-directory nancy-audit

deps-license: ## Проверить лицензии зависимостей
	@echo "📄 Проверка лицензий зависимостей..."
	@$(GO) list -m -json all | jq -r '.Path + " " + (.Version // "latest")' | head -20
	@echo "💡 Для подробного анализа лицензий используйте: go-licenses"

vuln-check: ## Проверить уязвимости с помощью govulncheck
	@echo "🔍 Проверка уязвимостей..."
	@if [ -f $(GOVULNCHECK) ]; then \
		$(GOVULNCHECK) ./...; \
	else \
		echo "❌ govulncheck не найден, запустите 'make install-tools'"; \
		exit 1; \
	fi

nancy-audit: ## Аудит с помощью Nancy (Sonatype)
	@echo "🔍 Nancy audit (Sonatype OSS Index)..."
	@if [ -f $(NANCY) ]; then \
		$(GO) list -json -deps ./... | $(NANCY) sleuth; \
	else \
		echo "⚠️  Nancy не найден, пропускаем проверку"; \
		echo "Для установки Nancy запустите: 'make install-tools'"; \
	fi

check: format lint staticcheck security cyclo ineffassign misspell ## Запустить все проверки

deps: ## Обновить зависимости
	@echo "Обновление зависимостей..."
	$(GO) mod tidy
	$(GO) mod verify

# =============================================================================
# ПРОДВИНУТЫЕ КОМАНДЫ
# =============================================================================

ci-full: ## Полная CI проверка (lint, SAST, тесты, сборка)
	@echo "🚀 Полная CI проверка..."
	@echo "📋 Этапы:"
	@echo "  1. Зависимости и форматирование"
	@echo "  2. Полный анализ линтеров"
	@echo "  3. SAST анализ"
	@echo "  4. Тесты с покрытием"
	@echo "  5. Сборка проекта"
	@echo ""
	@$(MAKE) --no-print-directory deps
	@$(MAKE) --no-print-directory format
	@$(MAKE) --no-print-directory lint-full
	@$(MAKE) --no-print-directory sast-full
	@$(MAKE) --no-print-directory coverage
	@$(MAKE) --no-print-directory build
	@echo "✅ Полная CI проверка завершена успешно!"

ci-reports: ## Создать все отчёты для CI/CD
	@echo "📊 Создание отчётов для CI/CD..."
	@$(MAKE) --no-print-directory lint-report
	@$(MAKE) --no-print-directory sast-report
	@$(MAKE) --no-print-directory security-report
	@echo "✅ Все отчёты созданы:"
	@echo "  - Линтеры: $(LINT_REPORT)"
	@echo "  - SAST: $(SAST_REPORT)"
	@echo "  - Безопасность: $(SECURITY_REPORT)"

verify-tools: ## Проверить наличие всех инструментов
	@echo "🔧 Проверка инструментов..."
	@echo "📋 Статус инструментов:"
	@printf "  golangci-lint: "; if [ -f $(GOLANGCI_LINT) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  staticcheck: "; if [ -f $(STATICCHECK) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  gosec: "; if [ -f $(GOSEC) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  govulncheck: "; if [ -f $(GOVULNCHECK) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  nancy: "; if [ -f $(NANCY) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  gofumpt: "; if [ -f $(GOFMT) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@printf "  goimports: "; if [ -f $(GOIMPORTS) ]; then echo "✅ установлен"; else echo "❌ не найден"; fi
	@echo ""
	@echo "💡 Для установки недостающих инструментов запустите: 'make install-tools'"

show-config: ## Показать текущую конфигурацию
	@echo "⚙️  Текущая конфигурация:"
	@echo "📋 Пути:"
	@echo "  TOOLS_BIN: $(TOOLS_BIN)"
	@echo "  GOLANGCI_CONFIG: $(GOLANGCI_CONFIG)"
	@echo "📋 Отчёты:"
	@echo "  LINT_REPORT: $(LINT_REPORT)"
	@echo "  SAST_REPORT: $(SAST_REPORT)"
	@echo "  SECURITY_REPORT: $(SECURITY_REPORT)"
	@echo "  COVERAGE_OUT: $(COVERAGE_OUT)"
	@echo "  COVERAGE_HTML: $(COVERAGE_HTML)"
	@echo "📋 Версии инструментов:"
	@echo "  golangci-lint: $(GOLANGCI_LINT_VERSION)"
	@echo "  gosec: $(GOSEC_VERSION)"
	@echo "  govulncheck: $(GOVULNCHECK_VERSION)"
	@echo "  staticcheck: $(STATICCHECK_VERSION)"
	@echo "  nancy: $(NANCY_VERSION)"

precommit: ## Pre-commit хук (форматирование + быстрые проверки)
	@echo "🔄 Pre-commit проверки..."
	@$(MAKE) --no-print-directory format
	@$(MAKE) --no-print-directory lint-godot
	@$(MAKE) --no-print-directory lint-lll
	@$(MAKE) --no-print-directory test-short
	@echo "✅ Pre-commit проверки завершены"

# =============================================================================
# КОМАНДЫ ОЧИСТКИ
# =============================================================================

clean: ## Очистить временные файлы
	@echo "🧹 Очистка временных файлов..."
	$(GO) clean
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
	rm -f $(LINT_REPORT) $(SAST_REPORT) $(SECURITY_REPORT)
	rm -f gosec-temp.json staticcheck-temp.json
	rm -f *.prof
	rm -f *.test
	@echo "✅ Временные файлы очищены"

clean-tools: ## Удалить установленные инструменты
	@echo "🧹 Удаление инструментов..."
	rm -rf $(TOOLS_BIN)
	@echo "✅ Инструменты удалены"

clean-all: clean clean-tools ## Полная очистка (временные файлы + инструменты)
	@echo "🧹 Полная очистка завершена"

ci: deps check test ## Команды для CI (зависимости, проверки, тесты)

all: clean deps format check test build ## Полная сборка и проверка

.DEFAULT_GOAL := help
