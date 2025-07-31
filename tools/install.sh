#!/bin/bash

# Скрипт установки инструментов разработки в tools/bin

set -e

TOOLS_DIR="$(dirname "$0")/bin"
FULL_TOOLS_DIR="$(pwd)/$TOOLS_DIR"

echo "Установка инструментов разработки в $FULL_TOOLS_DIR..."

# Создаем папку если не существует
mkdir -p "$FULL_TOOLS_DIR"

# Временная папка для сборки
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

cd "$TMP_DIR"

# Функция для установки Go инструмента
install_tool() {
    local package=$1
    local binary=$2
    
    echo "Устанавливаем $binary..."
    
    # Создаем временный go.mod
    cat > go.mod << EOF
module temp
go 1.24
EOF
    
    # Устанавливаем пакет
    GOBIN="$FULL_TOOLS_DIR" go install "$package@latest"
    
    # Очищаем
    rm -f go.mod go.sum
}

# Установка основных инструментов
install_tool "github.com/golangci/golangci-lint/cmd/golangci-lint" "golangci-lint"
install_tool "mvdan.cc/gofumpt" "gofumpt"
install_tool "golang.org/x/tools/cmd/goimports" "goimports"
install_tool "honnef.co/go/tools/cmd/staticcheck" "staticcheck"
install_tool "github.com/securecodewarrior/gosec/cmd/gosec" "gosec"
install_tool "golang.org/x/vuln/cmd/govulncheck" "govulncheck"
install_tool "github.com/fzipp/gocyclo/cmd/gocyclo" "gocyclo"
install_tool "github.com/gordonklaus/ineffassign" "ineffassign"
install_tool "github.com/client9/misspell/cmd/misspell" "misspell"

cd - > /dev/null

echo "Все инструменты установлены в $FULL_TOOLS_DIR"
echo "Для использования добавьте $FULL_TOOLS_DIR в PATH или используйте абсолютные пути"

# Выводим список установленных инструментов
echo ""
echo "Установленные инструменты:"
ls -la "$FULL_TOOLS_DIR"