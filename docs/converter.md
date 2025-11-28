# SVG → React-Planner Converter

## Назначение

Конвертация SVG планировок в JSON формат для react-planner.

## Архитектура

### Модули

- **parser** - парсинг SVG (XML + path команды)
- **graph** - построение графа стен (vertices + lines)
- **mapper** - основная логика конвертации
- **models** - типы данных

### Процесс конвертации

1. Парсинг SVG элементов (rect, path)
2. Классификация по ID префиксу (Wall_*, Door_*, Window_*, Room_*, Balcony_*)
3. Построение графа стен (vertices + lines)
4. Привязка проемов (holes) к стенам
5. Создание комнат и балконов (areas)
6. Сборка react-planner Scene JSON

## API

### POST /api/v1/convert

Конвертация SVG файла.

**Request:**
```
Content-Type: multipart/form-data

file: <SVG file>
```

**Response:**
```json
{
  "unit": "cm",
  "layers": {
    "layer-1": {
      "id": "layer-1",
      "name": "default",
      "altitude": 0,
      "order": 0,
      "opacity": 1,
      "visible": true,
      "vertices": {
        "v1": {"id": "v1", "name": "Vertex", "type": "vertex", "prototype": "vertices", "x": 489, "y": 1462, "lines": ["Wall_03"], "areas": [], "selected": false},
        "v2": {"id": "v2", "name": "Vertex", "type": "vertex", "prototype": "vertices", "x": 682, "y": 1462, "lines": ["Wall_03"], "areas": [], "selected": false}
      },
      "lines": {
        "Wall_03": {"id": "Wall_03", "name": "Wall_03", "type": "wall", "prototype": "lines", "vertices": ["v1", "v2"], "holes": ["Door_01"], "properties": {"height": {"length": 300}, "thickness": {"length": 20}, "textureA": "bricks", "textureB": "bricks"}}
      },
      "holes": {
        "Door_01": {"id": "Door_01", "name": "Door_01", "type": "door", "prototype": "holes", "line": "Wall_03", "offset": 0.32, "properties": {"width": {"length": 80}, "height": {"length": 215}, "altitude": {"length": 0}, "thickness": {"length": 30}, "flip_orizzontal": false}}
      },
      "areas": {
        "Room_Kitchen": {"id": "Room_Kitchen", "name": "Room_Kitchen", "type": "room", "prototype": "areas", "vertices": ["v10", "v11", "v12", "v13"], "holes": [], "properties": {"patternColor": "#F5F5F5", "thickness": {"length": 0}}}
      },
      "items": {},
      "selected": {"vertices": [], "lines": [], "holes": [], "areas": [], "items": []}
    }
  },
  "selectedLayer": "layer-1",
  "grids": {
    "h1": {"id": "h1", "type": "horizontal-streak", "properties": {"step": 20, "colors": ["#808080", "#ddd", "#ddd", "#ddd", "#ddd"]}},
    "v1": {"id": "v1", "type": "vertical-streak", "properties": {"step": 20, "colors": ["#808080", "#ddd", "#ddd", "#ddd", "#ddd"]}}
  },
  "groups": {},
  "width": 3000,
  "height": 2000,
  "meta": {},
  "guides": {"horizontal": {}, "vertical": {}, "circular": {}}
}
```

## SVG Требования

### ID префиксы

- `Wall_*` - стены (rect/path)
- `Door_*` - двери (rect/path)
- `Window_*` - окна (path)
- `Room_*` - комнаты (path/polygon)
- `Balcony_*` - балкон (path)

### Поддерживаемые элементы

- **rect** - атрибуты: x, y, width, height
- **path** - атрибут d с командами: M, m, L, l, H, h, V, v, Z

## Алгоритмы

### Граф стен

- Tolerance для объединения vertices: 2px
- Rect → линия по длинной стороне
- Path → линия от первой до последней точки

### Формат offsets

- `holes.offset` нормализован: `0..1` от начала линии до конца.

### Привязка проемов

- Поиск ближайшей стены к центру проема
- Вычисление offset через проекцию на линию

### Комнаты

- Парсинг контура из path/polygon
- Создание отдельных vertices для area
