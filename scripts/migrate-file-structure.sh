#!/bin/bash

# Скрипт миграции файловой структуры
# Переносит файлы из старой структуры в новую

set -e

SOURCE_DIR="source"

echo "=== Миграция файловой структуры ==="
echo

# Функция для миграции файлов пользователя
migrate_user() {
    local user_dir="$1"
    local user_id=$(basename "$user_dir")

    echo "Обработка пользователя: $user_id"

    # Создаем новые директории если их нет
    mkdir -p "$user_dir/svg/edited"
    mkdir -p "$user_dir/png"
    mkdir -p "$user_dir/pdf"
    mkdir -p "$user_dir/json/edited"

    # Миграция SVG из assets/ в svg/
    if [ -d "$user_dir/assets" ]; then
        echo "  Найдена папка assets/"
        shopt -s nullglob
        for file in "$user_dir/assets"/*.svg; do
            filename=$(basename "$file")
            echo "    Перемещение: assets/$filename -> svg/$filename"
            mv "$file" "$user_dir/svg/$filename"
        done
        shopt -u nullglob
        # Удаляем пустую папку
        if [ -z "$(ls -A "$user_dir/assets" 2>/dev/null)" ]; then
            rmdir "$user_dir/assets"
            echo "    Удалена пустая папка assets/"
        fi
    fi

    # Миграция файлов из корня в соответствующие папки
    shopt -s nullglob
    for file in "$user_dir"/*.svg; do
        filename=$(basename "$file")
        echo "    Перемещение: $filename -> svg/$filename"
        mv "$file" "$user_dir/svg/$filename"
    done

    for file in "$user_dir"/*.png "$user_dir"/*.jpg "$user_dir"/*.jpeg; do
        filename=$(basename "$file")
        echo "    Перемещение: $filename -> png/$filename"
        mv "$file" "$user_dir/png/$filename"
    done

    for file in "$user_dir"/*.pdf; do
        filename=$(basename "$file")
        echo "    Перемещение: $filename -> pdf/$filename"
        mv "$file" "$user_dir/pdf/$filename"
    done

    for file in "$user_dir"/*.json; do
        filename=$(basename "$file")
        echo "    Перемещение: $filename -> json/$filename"
        mv "$file" "$user_dir/json/$filename"
    done
    shopt -u nullglob

    # Миграция из uploads/ в png/ и svg/
    if [ -d "$user_dir/uploads" ]; then
        echo "  Найдена папка uploads/"
        shopt -s nullglob
        for file in "$user_dir/uploads"/*.png "$user_dir/uploads"/*.jpg "$user_dir/uploads"/*.jpeg; do
            filename=$(basename "$file")
            echo "    Перемещение: uploads/$filename -> png/$filename"
            mv "$file" "$user_dir/png/$filename"
        done
        for file in "$user_dir/uploads"/*.svg; do
            filename=$(basename "$file")
            echo "    Перемещение: uploads/$filename -> svg/$filename"
            mv "$file" "$user_dir/svg/$filename"
        done
        shopt -u nullglob
        # Удаляем пустую папку
        if [ -z "$(ls -A "$user_dir/uploads" 2>/dev/null)" ]; then
            rmdir "$user_dir/uploads"
            echo "    Удалена пустая папка uploads/"
        fi
    fi

    # Миграция из edited/ в svg/edited/
    if [ -d "$user_dir/edited" ]; then
        echo "  Найдена папка edited/"
        shopt -s nullglob
        for file in "$user_dir/edited"/*.svg; do
            filename=$(basename "$file")
            echo "    Перемещение: edited/$filename -> svg/edited/$filename"
            mv "$file" "$user_dir/svg/edited/$filename"
        done
        shopt -u nullglob
        # Удаляем пустую папку
        if [ -z "$(ls -A "$user_dir/edited" 2>/dev/null)" ]; then
            rmdir "$user_dir/edited"
            echo "    Удалена пустая папка edited/"
        fi
    fi

    echo
}

# Проверяем наличие директории source
if [ ! -d "$SOURCE_DIR" ]; then
    echo "Ошибка: директория $SOURCE_DIR не найдена"
    exit 1
fi

# Обрабатываем каждого пользователя
for user_dir in "$SOURCE_DIR"/*; do
    if [ -d "$user_dir" ]; then
        # Пропускаем файлы в корне source/
        if [ "$(basename "$user_dir")" != "$(basename "$user_dir" | grep -E '^[0-9a-f-]+$|^demo-user$')" ]; then
            continue
        fi
        migrate_user "$user_dir"
    fi
done

echo "=== Миграция завершена ==="
