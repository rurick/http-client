#!/bin/bash

# Переменные
PROJECT_DIR="./"   # корневая директория проекта
OUTPUT_FILE="./project.txt"   # выходной файл
EXCLUDE_MASKS=(".git" ".DS_Store" ".idea")  # маски исключаемых файлов и каталогов

# Функция рекурсивного обхода дерева каталога
process_directory() {
    local dir=$1
    for file in "$dir"/*; do
        if [[ -f "$file" ]]; then
            excluded=false
            for mask in "${EXCLUDE_MASKS[@]}"; do
                if [[ "$file" =~ $mask ]]; then
                    echo "Исключён файл: $file"
                    excluded=true
                    break
                fi
            done
            if ! $excluded; then
                echo "---- Начало файла: $file ----" >> "$OUTPUT_FILE"
                cat "$file" >> "$OUTPUT_FILE"
                echo "---- Конец файла: $file ----" >> "$OUTPUT_FILE"
            fi
        elif [[ -d "$file" && ! -L "$file" ]]; then
            process_directory "$file"
        fi
    done
}

# Очистка существующего выходного файла перед началом
if [[ -f "$OUTPUT_FILE" ]]; then
    rm "$OUTPUT_FILE"
fi

# Запускаем обработку
echo "Сборка всех файлов проекта..."
process_directory "$PROJECT_DIR"

echo "Файлы успешно собраны в $OUTPUT_FILE."