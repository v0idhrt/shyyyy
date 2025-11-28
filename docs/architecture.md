# Микросервисная Архитектура

## Сервисы

### API Gateway (порт 3000)
Точка входа для всех запросов. Проксирует запросы к микросервисам.

**Endpoints:**
- `GET /health/*` - health checks
- `GET /api/v1/` - info
- `POST /api/v1/convert` - proxy → Converter Service

**Компоненты:**
- `cmd/gateway` - точка входа
- `internal/gateway/handlers` - health handlers
- `internal/gateway/proxy` - reverse proxy на `httputil`, отдает запросы сервисам без модификации multipart тел

### Converter Service (порт 3001)
Конвертация SVG планировок в react-planner JSON.

**Endpoints:**
- `GET /health/*` - health checks
- `POST /convert` - конвертация SVG

**Компоненты:**
- `cmd/converter` - точка входа
- `internal/converter/handlers` - convert handler
- `internal/converter/parser` - SVG парсинг
- `internal/converter/graph` - граф стен
- `internal/converter/mapper` - конвертация
- `internal/converter/models` - типы данных

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
