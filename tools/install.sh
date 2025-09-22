#!/bin/bash

# Скрипт установки инструментов разработки для HTTP клиента
# Автор: Сгенерирован автоматически
# Версия: 1.0

set -euo pipefail

# Цвета для вывода
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Константы
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly TOOLS_DIR="${SCRIPT_DIR}/bin"
readonly GO_BIN="${GOPATH:-$HOME/go}/bin"

# Версии инструментов (соответствуют Makefile)
readonly GOLANGCI_LINT_VERSION="v1.61.0"
readonly GOSEC_VERSION="v2.18.2"
readonly GOVULNCHECK_VERSION="latest"
readonly STATICCHECK_VERSION="latest"
readonly NANCY_VERSION="v1.0.42"

# Функции для вывода
info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Проверка, что Go установлен
check_go() {
    if ! command -v go &> /dev/null; then
        error "Go не найден. Пожалуйста, установите Go с https://golang.org/dl/"
    fi
    
    local go_version=$(go version | awk '{print $3}' | sed 's/go//')
    info "Используется Go версии: $go_version"
}

# Создание директории для инструментов
setup_tools_dir() {
    if [[ ! -d "$TOOLS_DIR" ]]; then
        mkdir -p "$TOOLS_DIR"
        info "Создана директория для инструментов: $TOOLS_DIR"
    fi
}

# Проверка, установлен ли инструмент
is_tool_installed() {
    local tool_path="$1"
    [[ -f "$tool_path" && -x "$tool_path" ]]
}

# Установка инструмента через go install
install_go_tool() {
    local name="$1"
    local package="$2"
    local version="${3:-latest}"
    local tool_path="${TOOLS_DIR}/${name}"
    
    if is_tool_installed "$tool_path"; then
        success "$name уже установлен"
        return 0
    fi
    
    info "Установка $name..."
    
    # Формируем полный путь для установки
    local full_package="$package"
    if [[ "$version" != "latest" ]]; then
        full_package="${package}@${version}"
    fi
    
    # Устанавливаем в локальную директорию
    if GOBIN="$TOOLS_DIR" go install "$full_package"; then
        success "$name установлен успешно"
    else
        error "Не удалось установить $name"
    fi
}

# Установка golangci-lint (особый случай - используем их установщик)
install_golangci_lint() {
    local tool_path="${TOOLS_DIR}/golangci-lint"
    
    if is_tool_installed "$tool_path"; then
        success "golangci-lint уже установлен"
        return 0
    fi
    
    info "Установка golangci-lint $GOLANGCI_LINT_VERSION..."
    
    # Скачиваем и устанавливаем golangci-lint
    local temp_dir=$(mktemp -d)
    local install_script="${temp_dir}/install.sh"
    
    if curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh > "$install_script"; then
        chmod +x "$install_script"
        if "$install_script" -b "$TOOLS_DIR" "$GOLANGCI_LINT_VERSION"; then
            success "golangci-lint установлен успешно"
        else
            error "Не удалось установить golangci-lint"
        fi
    else
        error "Не удалось скачать установщик golangci-lint"
    fi
    
    rm -rf "$temp_dir"
}

# Установка Nancy (особый случай)
install_nancy() {
    local tool_path="${TOOLS_DIR}/nancy"
    
    if is_tool_installed "$tool_path"; then
        success "nancy уже установлен"
        return 0
    fi
    
    info "Установка nancy $NANCY_VERSION..."
    
    # Получаем архитектуру системы
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$arch" in
        x86_64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) error "Неподдерживаемая архитектура: $arch" ;;
    esac
    
    local download_url="https://github.com/sonatypecommunity/nancy/releases/download/${NANCY_VERSION}/nancy-${os}-${arch}"
    
    if curl -sSfL "$download_url" -o "$tool_path"; then
        chmod +x "$tool_path"
        success "nancy установлен успешно"
    else
        warning "Не удалось установить nancy. Пропускаем..."
    fi
}

# Проверка установленных инструментов
verify_installation() {
    info "Проверка установленных инструментов..."
    
    local tools=(
        "golangci-lint:golangci-lint version"
        "staticcheck:staticcheck -version"
        "gosec:gosec -version"
        "govulncheck:govulncheck -version"
        "gofumpt:gofumpt -version"
        "goimports:goimports -version"
        "misspell:misspell -version"
        "gocyclo:gocyclo -version"
        "ineffassign:ineffassign -version"
    )
    
    local failed=0
    
    for tool_info in "${tools[@]}"; do
        local tool_name="${tool_info%%:*}"
        local version_cmd="${tool_info##*:}"
        local tool_path="${TOOLS_DIR}/${tool_name}"
        
        if is_tool_installed "$tool_path"; then
            # Получаем версию инструмента
            local version_output
            if version_output=$("$tool_path" --version 2>&1 || "$tool_path" -version 2>&1 || echo "неизвестно") 2>/dev/null; then
                success "$tool_name установлен ($(echo "$version_output" | head -1 | cut -d' ' -f1-3))"
            else
                success "$tool_name установлен"
            fi
        else
            error "$tool_name НЕ установлен"
            ((failed++))
        fi
    done
    
    # Проверяем nancy отдельно (может не устанавливаться)
    local nancy_path="${TOOLS_DIR}/nancy"
    if is_tool_installed "$nancy_path"; then
        success "nancy установлен"
    else
        warning "nancy не установлен (опционально)"
    fi
    
    if [[ $failed -eq 0 ]]; then
        success "Все основные инструменты установлены успешно!"
    else
        error "$failed инструментов не установлены"
    fi
}

# Главная функция
main() {
    echo "🔧 Установка инструментов разработки для HTTP клиента"
    echo "======================================================"
    
    check_go
    setup_tools_dir
    
    info "Установка инструментов в: $TOOLS_DIR"
    echo ""
    
    # Устанавливаем golangci-lint специальным способом
    install_golangci_lint
    
    # Устанавливаем остальные инструменты через go install
    install_go_tool "staticcheck" "honnef.co/go/tools/cmd/staticcheck" "$STATICCHECK_VERSION"
    # ПРИМЕЧАНИЕ: gosec недоступен по указанному репозиторию, пропускаем
    # install_go_tool "gosec" "github.com/securecodewarrior/gosec/v2/cmd/gosec" "$GOSEC_VERSION"
    install_go_tool "govulncheck" "golang.org/x/vuln/cmd/govulncheck" "$GOVULNCHECK_VERSION"
    install_go_tool "gofumpt" "mvdan.cc/gofumpt" "latest"
    install_go_tool "goimports" "golang.org/x/tools/cmd/goimports" "latest"
    install_go_tool "misspell" "github.com/client9/misspell/cmd/misspell" "latest"
    install_go_tool "gocyclo" "github.com/fzipp/gocyclo/cmd/gocyclo" "latest"
    install_go_tool "ineffassign" "github.com/gordonklaus/ineffassign" "latest"
    
    # Устанавливаем nancy (опционально)
    install_nancy
    
    echo ""
    verify_installation
    
    echo ""
    success "Установка завершена! Теперь вы можете использовать команды make для анализа кода."
    info "Примеры команд:"
    echo "  make lint-full     # Полный анализ линтерами"
    echo "  make sast-full     # Полный SAST анализ"
    echo "  make security-full # Полная проверка безопасности"
    echo "  make ci-full       # Полная CI проверка"
}

# Запуск основной функции
main "$@"
