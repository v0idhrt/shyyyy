#!/bin/bash

# Скрипт проверки миграции (dry-run)
# Показывает, какие файлы будут перемещены

SOURCE_DIR="source"

echo "=== Проверка файлов для миграции ==="
echo

check_user() {
    local user_dir="$1"
    local user_id=$(basename "$user_dir")
    local found_files=0

    echo "Пользователь: $user_id"

    # Проверка SVG в assets/
    if [ -d "$user_dir/assets" ]; then
        shopt -s nullglob
        for file in "$user_dir/assets"/*.svg; do
            if [ -f "$file" ]; then
                echo "  [MIGRATE] assets/$(basename "$file") -> svg/$(basename "$file")"
                ((found_files++))
            fi
        done
        shopt -u nullglob
    fi

    # Проверка файлов в корне
    shopt -s nullglob
    for file in "$user_dir"/*.svg "$user_dir"/*.png "$user_dir"/*.jpg "$user_dir"/*.jpeg "$user_dir"/*.pdf "$user_dir"/*.json; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            ext="${filename##*.}"
            case "$ext" in
                svg)
                    echo "  [MIGRATE] $filename -> svg/$filename"
                    ;;
                png|jpg|jpeg)
                    echo "  [MIGRATE] $filename -> png/$filename"
                    ;;
                pdf)
                    echo "  [MIGRATE] $filename -> pdf/$filename"
                    ;;
                json)
                    echo "  [MIGRATE] $filename -> json/$filename"
                    ;;
            esac
            ((found_files++))
        fi
    done
    shopt -u nullglob

    # Проверка uploads/
    if [ -d "$user_dir/uploads" ]; then
        shopt -s nullglob
        for file in "$user_dir/uploads"/*.svg "$user_dir/uploads"/*.png "$user_dir/uploads"/*.jpg "$user_dir/uploads"/*.jpeg; do
            if [ -f "$file" ]; then
                filename=$(basename "$file")
                ext="${filename##*.}"
                case "$ext" in
                    svg)
                        echo "  [MIGRATE] uploads/$filename -> svg/$filename"
                        ;;
                    png|jpg|jpeg)
                        echo "  [MIGRATE] uploads/$filename -> png/$filename"
                        ;;
                esac
                ((found_files++))
            fi
        done
        shopt -u nullglob
    fi

    # Проверка edited/
    if [ -d "$user_dir/edited" ]; then
        shopt -s nullglob
        for file in "$user_dir/edited"/*.svg; do
            if [ -f "$file" ]; then
                echo "  [MIGRATE] edited/$(basename "$file") -> svg/edited/$(basename "$file")"
                ((found_files++))
            fi
        done
        shopt -u nullglob
    fi

    # Проверка файлов в новой структуре
    for dir in svg png pdf json; do
        if [ -d "$user_dir/$dir" ]; then
            count=$(find "$user_dir/$dir" -type f 2>/dev/null | wc -l)
            if [ $count -gt 0 ]; then
                echo "  [OK] $dir/: $count файл(ов) уже в новой структуре"
            fi
        fi
    done

    if [ $found_files -eq 0 ]; then
        echo "  [OK] Файлы для миграции не найдены"
    else
        echo "  [SUMMARY] Найдено $found_files файл(ов) для миграции"
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
        check_user "$user_dir"
    fi
done

echo "=== Проверка завершена ==="
echo "Для выполнения миграции запустите: ./scripts/migrate-file-structure.sh"
