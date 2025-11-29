# Auth Service

## Назначение

Простая аутентификация и выдача пользовательских данных/файлов (демо).

## Хранилище

- БД: SQLite `data/db/auth.db` (миграция `migrations/001_init_auth.sql`)
- Файлы: `source/<userUUID>/plan.svg`, `source/<userUUID>/document.pdf`
- Сид: `admin` / `admin` (id: `11111111-1111-1111-1111-111111111111`)

## API

- `POST /login` — body JSON `{ "login", "password" }` → `{ token, user }`
- `GET /users/:id` — `Authorization: Bearer <token>` → профиль
- `GET /users/:id/svg` — `Authorization` → SVG-файл
- `GET /users/:id/pdf` — `Authorization` → PDF-файл
- `GET /users/:id/png` — `Authorization` → PNG-файл
- `POST /users/:id/svg` — `Authorization`, `multipart/form-data` файл `file` → сохранение
- `POST /users/:id/pdf` — `Authorization`, `multipart/form-data` файл `file` → сохранение
- `POST /users/:id/png` — `Authorization`, `multipart/form-data` файл `file` → сохранение
- `POST /users/:id/png-to-svg` — `Authorization`, `multipart/form-data` PNG `file`, сохраняет в `uploads/` и отдает одноименный SVG из `uploads/` (если есть)
- `POST /users/:id/png-to-json` — `Authorization`, `multipart/form-data` PNG `file`, сохраняет в `uploads/`, ищет одноименный SVG в `uploads/`, отправляет его в Converter `/convert`, возвращает JSON
- `POST /users/:id/svg-edited` — `Authorization`, `multipart/form-data` SVG `file`, сохраняет в `edited/` под тем же именем
- `GET /users/:id/svg-edited?name=<filename>` — `Authorization`, отдаёт сохранённый в `edited/` SVG
- `GET /users/:id/svg-json?name=<filename>` — `Authorization`, конвертирует оригинальный SVG (по имени) через Converter и отдаёт JSON
- `GET /users/:id/svg-edited-json?name=<filename>` — `Authorization`, конвертирует edited SVG (по имени) через Converter и отдаёт JSON
- `GET /users/:id/files` — `Authorization`, список наличия базовых файлов и содержимого `uploads/`
- `POST /users/:id/json` — `Authorization`, `multipart/form-data` JSON `file`, сохраняет в `json/`
- `GET /users/:id/json?name=<filename>` — `Authorization`, отдаёт JSON файл из `json/`
- `POST /users/:id/json-edited` — `Authorization`, `multipart/form-data` JSON `file`, сохраняет в `json/edited/`
- `GET /users/:id/json-edited?name=<filename>` — `Authorization`, отдаёт JSON из `json/edited/`

## Запуск

```bash
PORT=3002 AUTH_DB_PATH=data/db/auth.db go run ./cmd/auth/main.go
```
