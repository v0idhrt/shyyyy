# Микросервисная Архитектура

Проект состоит из двух сервисов:
- **API Gateway** - точка входа, проксирование
- **Converter Service** - конвертация SVG в react-planner JSON
- **Auth Service** - простая аутентификация и файлы пользователя

## Запуск

### Make (рекомендуется)

```bash
# Terminal 1
make converter

# Terminal 2
make gateway

# Terminal 3
make auth

# Или собрать все
make build-all
```

### Вручную

```bash
# Terminal 1 - Converter Service
PORT=3001 go run ./cmd/converter/main.go

# Terminal 2 - Auth Service
PORT=3002 AUTH_DB_PATH=data/db/auth.db go run ./cmd/auth/main.go

# Terminal 3 - API Gateway
PORT=3000 CONVERTER_URL=http://localhost:3001 go run ./cmd/gateway/main.go
```

## Конфигурация

### API Gateway
```bash
PORT=3000                              # default
CONVERTER_URL=http://localhost:3001   # default
AUTH_URL=http://localhost:3002        # default
```

### Converter Service
```bash
PORT=3001  # default
```

### Auth Service
```bash
PORT=3002                 # default
AUTH_DB_PATH=data/db/auth.db
```

## Endpoints

### API Gateway (`:3000`)
- `GET /health/live` - liveness probe
- `GET /health/ready` - readiness probe
- `GET /health/startup` - startup probe
- `GET /api/v1/` - API info
- `POST /api/v1/convert` - SVG конвертер (proxy → Converter)
- `POST /api/v1/render` - обратная конвертация JSON → SVG (proxy → Converter)
- `POST /api/v1/login` - логин (proxy → Auth)
- `GET /api/v1/users/:id` - профиль (proxy → Auth)
- `GET /api/v1/users/:id/svg` - SVG файл (proxy → Auth)
- `GET /api/v1/users/:id/pdf` - PDF файл (proxy → Auth)
- `POST /api/v1/users/:id/svg` - загрузка SVG (proxy → Auth)
- `POST /api/v1/users/:id/pdf` - загрузка PDF (proxy → Auth)

### Converter Service (`:3001`)
- `GET /health/live` - liveness probe
- `GET /health/ready` - readiness probe
- `POST /convert` - конвертация SVG
- `POST /render` - конвертация JSON → SVG

### Auth Service (`:3002`)
- `GET /health/live` - liveness probe
- `GET /health/ready` - readiness probe
- `POST /login` - логин
- `GET /users/:id` - профиль
- `GET /users/:id/svg` - SVG файл
- `GET /users/:id/pdf` - PDF файл
- `POST /users/:id/svg` - загрузка SVG
- `POST /users/:id/pdf` - загрузка PDF

## SVG Converter

**Пример запроса:**
```bash
curl -X POST http://localhost:3000/api/v1/convert \
  -F "file=@source/1.svg"
```

**SVG требования:**
- ID префиксы: `Wall_*`, `Door_*`, `Window_*`, `Room_*`, `Balcony_*`
- Поддержка: `<rect>`, `<path>`

Подробности в [docs/converter.md](docs/converter.md).

## Структура

```
cmd/
  ├── gateway/       - API Gateway сервис
  └── converter/     - Converter сервис
internal/
  ├── common/        - общие модули
  │   ├── config/
  │   └── middleware/
  ├── gateway/       - Gateway логика
  │   ├── handlers/
  │   └── proxy/
  ├── auth/          - Auth логика
  │   ├── handlers/
  │   ├── repository/
  │   └── service/
  └── converter/     - Converter логика
      ├── handlers/
      ├── models/
      ├── parser/
      ├── graph/
      └── mapper/
docs/                - документация
migrations/          - миграции (sqlite)
source/              - примеры SVG
```

Подробности в [docs/architecture.md](docs/architecture.md).
