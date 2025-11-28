# Микросервисная Архитектура

Проект состоит из двух сервисов:
- **API Gateway** - точка входа, проксирование
- **Converter Service** - конвертация SVG в react-planner JSON

## Запуск

### Make (рекомендуется)

```bash
# Terminal 1
make converter

# Terminal 2
make gateway

# Или собрать все
make build-all
```

### Вручную

```bash
# Terminal 1 - Converter Service
PORT=3001 go run ./cmd/converter/main.go

# Terminal 2 - API Gateway
PORT=3000 CONVERTER_URL=http://localhost:3001 go run ./cmd/gateway/main.go
```

## Конфигурация

### API Gateway
```bash
PORT=3000                              # default
CONVERTER_URL=http://localhost:3001   # default
```

### Converter Service
```bash
PORT=3001  # default
```

## Endpoints

### API Gateway (`:3000`)
- `GET /health/live` - liveness probe
- `GET /health/ready` - readiness probe
- `GET /health/startup` - startup probe
- `GET /api/v1/` - API info
- `POST /api/v1/convert` - SVG конвертер (proxy → Converter)

### Converter Service (`:3001`)
- `GET /health/live` - liveness probe
- `GET /health/ready` - readiness probe
- `POST /convert` - конвертация SVG

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
  └── converter/     - Converter логика
      ├── handlers/
      ├── models/
      ├── parser/
      ├── graph/
      └── mapper/
docs/                - документация
source/              - примеры SVG
```

Подробности в [docs/architecture.md](docs/architecture.md).
