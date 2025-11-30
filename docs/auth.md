# Auth Service

## Назначение

Простая аутентификация и выдача пользовательских данных/файлов (демо).

## Хранилище

**База данных:**
- SQLite `data/db/auth.db` (миграция `migrations/001_init_auth.sql`)
- Сид: `admin` / `admin` (id: `11111111-1111-1111-1111-111111111111`)

**Файловая структура:**
```
source/{userID}/
├── svg/              # оригинальные SVG
│   └── edited/       # измененные SVG
├── png/              # оригинальные PNG
├── pdf/              # PDF файлы
└── json/             # JSON файлы
    └── edited/       # измененные JSON
```

## API

**Аутентификация:**
- `POST /login` — body JSON `{ "login", "password" }` → `{ token, user }`
- `GET /users/:id` — `Authorization: Bearer <token>` → профиль

**Файлы (GET):**
- `GET /users/:id/svg?name=<filename>` — отдать SVG из `svg/` (auto-detect если один файл)
- `GET /users/:id/png?name=<filename>` — отдать PNG из `png/` (auto-detect если один файл)
- `GET /users/:id/pdf?name=<filename>` — отдать PDF из `pdf/` (auto-detect если один файл)
- `GET /users/:id/pdf-files?name=<filename>` — alias для `/pdf` (обратная совместимость)
- `GET /users/:id/json?name=<filename>` — отдать JSON из `json/`
- `GET /users/:id/svg-edited?name=<filename>` — отдать измененный SVG из `svg/edited/`
- `GET /users/:id/json-edited?name=<filename>` — отдать JSON из `json/edited/`
- `GET /users/:id/files` — список всех файлов пользователя

**Файлы (POST):**
- `POST /users/:id/svg` — сохранить SVG в `svg/`
- `POST /users/:id/png` — сохранить PNG в `png/`
- `POST /users/:id/pdf` — сохранить PDF в `pdf/`
- `POST /users/:id/json` — сохранить JSON в `json/`
- `POST /users/:id/svg-edited` — сохранить измененный SVG в `svg/edited/`
- `POST /users/:id/json-edited` — сохранить JSON в `json/edited/`

**Конвертация:**
- `POST /users/:id/png-to-svg` — загрузить PNG → сохранить в `png/` → вернуть одноименный SVG из `svg/`
- `POST /users/:id/png-to-json` — загрузить PNG → сохранить в `png/` → конвертировать одноименный SVG из `svg/` → вернуть JSON
- `GET /users/:id/svg-json?name=<filename>` — конвертировать SVG из `svg/` через Converter → вернуть JSON
- `GET /users/:id/svg-edited-json?name=<filename>` — конвертировать SVG из `svg/edited/` через Converter → вернуть JSON
- `POST /users/:id/json-to-svg?name=<filename>` — scene JSON → Converter `/render` → сохранить SVG в `svg/edited/<filename>.svg`

## Запуск

```bash
PORT=3002 AUTH_DB_PATH=data/db/auth.db CONVERTER_URL=http://localhost:3001 go run ./cmd/auth/main.go
```
