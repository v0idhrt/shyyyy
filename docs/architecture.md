# Микросервисная Архитектура

## Сервисы

### API Gateway (порт 3000)
Точка входа для всех запросов. Проксирует запросы к микросервисам.

**Endpoints:**
- `GET /health/*` - health checks
- `GET /api/v1/` - info
- `POST /api/v1/convert` - proxy → Converter Service
- `POST /api/v1/render` - proxy → Converter Service
- `POST /api/v1/login` - proxy → Auth Service
- `GET /api/v1/users/:id` - proxy → Auth Service
- `GET /api/v1/users/:id/svg` - proxy → Auth Service
- `GET /api/v1/users/:id/pdf` - proxy → Auth Service
- `POST /api/v1/users/:id/svg` - proxy → Auth Service
- `POST /api/v1/users/:id/pdf` - proxy → Auth Service
- `POST /api/v1/pdf/generate` - proxy → PDF Service

**Компоненты:**
- `cmd/gateway` - точка входа
- `internal/gateway/handlers` - health handlers
- `internal/gateway/proxy` - reverse proxy, поддерживает raw и multipart

### Converter Service (порт 3001)
Конвертация SVG планировок в react-planner JSON.

**Endpoints:**
- `GET /health/*` - health checks
- `POST /convert` - конвертация SVG
- `POST /render` - конвертация JSON → SVG

**Компоненты:**
- `cmd/converter` - точка входа
- `internal/converter/handlers` - convert handler
- `internal/converter/parser` - SVG парсинг
- `internal/converter/graph` - граф стен
- `internal/converter/mapper` - конвертация
- `internal/converter/models` - типы данных

### Auth Service (порт 3002)
Простая аутентификация + выдача пользовательских данных и файлов.

**Endpoints:**
- `GET /health/*` - health checks
- `POST /login` - выдает токен для пользователя
- `GET /users/:id` - профиль пользователя
- `GET /users/:id/svg` - отдать SVG файл пользователя
- `GET /users/:id/pdf` - отдать PDF файл пользователя
- `POST /users/:id/svg` - загрузить SVG
- `POST /users/:id/pdf` - загрузить PDF

**Компоненты:**
- `cmd/auth` - точка входа
- `internal/auth/repository` - sqlite repository
- `internal/auth/handlers` - http handlers
- `internal/auth/service` - sessions + storage

### PDF Service (порт 3004)
Генерация PDF отчётов о изменениях планировок (Python/Flask).

**Endpoints:**
- `GET /health` - health check
- `POST /generate` - генерация PDF отчёта

**Компоненты:**
- `cmd/pdf_service/main.py` - Flask сервер
- `cmd/pdf_service/svg_comparator.py` - сравнение SVG
- `cmd/pdf_service/templates/report.html` - HTML шаблон для PDF

## Общие модули

### internal/common
Переиспользуемые компоненты для всех сервисов:
- **config** - конфигурация через env
- **middleware** - logger

## Конфигурация

### API Gateway
```bash
PORT=3000
ENV=development
CONVERTER_URL=http://localhost:3001
```

### Converter Service
```bash
PORT=3001
ENV=development
```

## Запуск

**Локально:**
```bash
# Terminal 1 - Converter
PORT=3001 go run ./cmd/converter/main.go

# Terminal 2 - Gateway
PORT=3000 CONVERTER_URL=http://localhost:3001 go run ./cmd/gateway/main.go
```

**Запрос:**
```bash
curl -X POST http://localhost:3000/api/v1/convert \
  -F "file=@source/1.svg"
```

## Принципы

- **Разделение ответственности**: каждый сервис — одна задача
- **Независимое развертывание**: сервисы запускаются отдельно
- **Проксирование**: Gateway не содержит бизнес-логики
- **Health checks**: все сервисы имеют /health endpoints
- **Env конфигурация**: URLs сервисов через переменные окружения
