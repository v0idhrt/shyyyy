package mapper

import (
	"fmt"
	"io"
	"math"

	"api-gateway/internal/converter/graph"
	"api-gateway/internal/converter/models"
	"api-gateway/internal/converter/parser"
)

// ============================================================
// Converter
// ============================================================

type Converter struct {
	elements []models.SVGElement
	builder  *graph.GraphBuilder
}

func New() *Converter {
	return &Converter{
		builder: graph.NewGraphBuilder(),
	}
}

// Convert SVG → react-planner JSON
func (c *Converter) Convert(r io.Reader) (*models.Scene, error) {
	// Парсинг SVG
	elements, err := parser.ParseSVG(r)
	if err != nil {
		return nil, fmt.Errorf("parse SVG: %w", err)
	}
	c.elements = elements

	// Разделяем элементы по типам
	var walls, doors, windows, rooms, balconies []models.SVGElement
	for _, elem := range elements {
		switch elem.Type {
		case "wall":
			walls = append(walls, elem)
		case "door":
			doors = append(doors, elem)
		case "window":
			windows = append(windows, elem)
		case "room":
			rooms = append(rooms, elem)
		case "balcony":
			balconies = append(balconies, elem)
		}
	}

	// Строим граф стен
	if err := c.builder.BuildFromWalls(walls); err != nil {
		return nil, fmt.Errorf("build walls graph: %w", err)
	}

	// Создаем holes (двери + окна)
	holes := make(map[string]models.Hole)
	for _, door := range doors {
		if hole := c.createHole(door, "door"); hole != nil {
			holes[door.ID] = *hole
		}
	}
	for _, window := range windows {
		if hole := c.createHole(window, "window"); hole != nil {
			holes[window.ID] = *hole
		}
	}

	// Создаем areas (комнаты + балконы)
	areas := make(map[string]models.Area)
	for _, room := range rooms {
		c.createArea(room, "room", areas)
	}
	for _, balcony := range balconies {
		c.createArea(balcony, "balcony", areas)
	}

	// Собираем scene
	layer := models.Layer{
		ID:       "layer-1",
		Altitude: 0,
		Order:    0,
		Opacity:  1,
		Name:     "default",
		Visible:  true,
		Vertices: c.builder.GetVertices(),
		Lines:    c.builder.GetLines(),
		Holes:    holes,
		Areas:    areas,
		Items:    map[string]any{},
		Selected: models.ElementsSet{Vertices: []string{}, Lines: []string{}, Holes: []string{}, Areas: []string{}, Items: []string{}},
	}

	scene := &models.Scene{
		Unit:          "cm",
		Layers:        map[string]models.Layer{"layer-1": layer},
		SelectedLayer: "layer-1",
		Grids:         defaultGrids(),
		Groups:        map[string]any{},
		Width:         3000,
		Height:        2000,
		Meta:          map[string]any{},
		Guides:        defaultGuides(),
	}

	return scene, nil
}

// createHole создает hole из элемента (дверь/окно)
func (c *Converter) createHole(elem models.SVGElement, holeType string) *models.Hole {
	// Получаем центр проема
	center := c.getElementCenter(elem)
	if center == nil {
		return nil
	}

	// Ищем ближайшую стену
	lineID, offset := c.findNearestLine(*center)
	if lineID == "" {
		return nil
	}

	hole := &models.Hole{
		ID:         elem.ID,
		Name:       elem.ID,
		Type:       holeType,
		Prototype:  "holes",
		Line:       lineID,
		Offset:     offset,
		Properties: defaultHoleProperties(holeType),
	}

	// Привязываем hole к линии
	c.builder.AttachHoleToLine(lineID, elem.ID)

	return hole
}

// createArea создает area из элемента (комната/балкон)
func (c *Converter) createArea(elem models.SVGElement, areaType string, target map[string]models.Area) {
	points, err := c.getElementPoints(elem)
	if err != nil || len(points) == 0 {
		return
	}

	vertexIDs := c.builder.AddAreaVertices(points, elem.ID)

	area := models.Area{
		ID:         elem.ID,
		Name:       elem.ID,
		Type:       areaType,
		Prototype:  "areas",
		Vertices:   vertexIDs,
		Holes:      []string{},
		Properties: defaultAreaProperties(),
	}

	target[elem.ID] = area
}

// ============================================================
// Geometry helpers
// ============================================================

func (c *Converter) getElementCenter(elem models.SVGElement) *models.Point {
	switch geom := elem.Geometry.(type) {
	case models.RectGeometry:
		return &models.Point{
			X: geom.X + geom.Width/2,
			Y: geom.Y + geom.Height/2,
		}
	case models.PathGeometry:
		points, err := parser.ParsePath(geom.D)
		if err != nil || len(points) == 0 {
			return nil
		}
		// Центр = средняя точка
		var sumX, sumY float64
		for _, p := range points {
			sumX += p.X
			sumY += p.Y
		}
		return &models.Point{
			X: sumX / float64(len(points)),
			Y: sumY / float64(len(points)),
		}
	}
	return nil
}

func (c *Converter) getElementPoints(elem models.SVGElement) ([]models.Point, error) {
	switch geom := elem.Geometry.(type) {
	case models.PathGeometry:
		points, err := parser.ParsePath(geom.D)
		if err != nil {
			return nil, err
		}
		// убираем дубль замыкания
		if len(points) > 1 {
			first := points[0]
			last := points[len(points)-1]
			if first.X == last.X && first.Y == last.Y {
				points = points[:len(points)-1]
			}
		}
		return points, nil
	case models.RectGeometry:
		return []models.Point{
			{X: geom.X, Y: geom.Y},
			{X: geom.X + geom.Width, Y: geom.Y},
			{X: geom.X + geom.Width, Y: geom.Y + geom.Height},
			{X: geom.X, Y: geom.Y + geom.Height},
		}, nil
	}
	return nil, fmt.Errorf("unknown geometry type")
}

func (c *Converter) findNearestLine(p models.Point) (string, float64) {
	var nearestLineID string
	var nearestOffset float64
	minDist := math.MaxFloat64

	vertices := c.builder.GetVertices()
	lines := c.builder.GetLines()

	for lineID, line := range lines {
		if len(line.Vertices) < 2 {
			continue
		}

		v1, ok1 := vertices[line.Vertices[0]]
		v2, ok2 := vertices[line.Vertices[1]]
		if !ok1 || !ok2 {
			continue
		}

		// Расстояние от точки до линии
		dist, offset := pointToLineDistance(p, v1, v2)
		if dist < minDist {
			minDist = dist
			nearestLineID = lineID
			nearestOffset = offset
		}
	}

	return nearestLineID, nearestOffset
}

func pointToLineDistance(p models.Point, v1, v2 models.Vertex) (float64, float64) {
	// Вектор линии
	dx := v2.X - v1.X
	dy := v2.Y - v1.Y
	lineLen := math.Sqrt(dx*dx + dy*dy)

	if lineLen == 0 {
		return math.Sqrt((p.X-v1.X)*(p.X-v1.X) + (p.Y-v1.Y)*(p.Y-v1.Y)), 0
	}

	// Проекция точки на линию
	t := ((p.X-v1.X)*dx + (p.Y-v1.Y)*dy) / (lineLen * lineLen)
	t = math.Max(0, math.Min(1, t))

	// Ближайшая точка на линии
	projX := v1.X + t*dx
	projY := v1.Y + t*dy

	// Расстояние
	dist := math.Sqrt((p.X-projX)*(p.X-projX) + (p.Y-projY)*(p.Y-projY))
	offset := t

	return dist, offset
}

// ============================================================
// Defaults
// ============================================================

func defaultHoleProperties(holeType string) map[string]any {
	switch holeType {
	case "door":
		return map[string]any{
			"width":           map[string]any{"length": 80.0},
			"height":          map[string]any{"length": 215.0},
			"altitude":        map[string]any{"length": 0.0},
			"thickness":       map[string]any{"length": 30.0},
			"flip_orizzontal": false,
		}
	case "window":
		return map[string]any{
			"width":     map[string]any{"length": 90.0},
			"height":    map[string]any{"length": 100.0},
			"altitude":  map[string]any{"length": 90.0},
			"thickness": map[string]any{"length": 10.0},
		}
	default:
		return map[string]any{}
	}
}

func defaultAreaProperties() map[string]any {
	return map[string]any{
		"patternColor": "#F5F5F5",
		"thickness":    map[string]any{"length": 0.0},
	}
}

func defaultGrids() map[string]models.Grid {
	return map[string]models.Grid{
		"h1": {
			ID:   "h1",
			Type: "horizontal-streak",
			Properties: map[string]any{
				"step":   20,
				"colors": []string{"#808080", "#ddd", "#ddd", "#ddd", "#ddd"},
			},
		},
		"v1": {
			ID:   "v1",
			Type: "vertical-streak",
			Properties: map[string]any{
				"step":   20,
				"colors": []string{"#808080", "#ddd", "#ddd", "#ddd", "#ddd"},
			},
		},
	}
}

func defaultGuides() models.Guides {
	return models.Guides{
		Horizontal: map[string]any{},
		Vertical:   map[string]any{},
		Circular:   map[string]any{},
	}
}
