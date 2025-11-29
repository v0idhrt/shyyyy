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
	elements      []models.SVGElement
	builder       *graph.GraphBuilder
	bbox          *boundingBox
	transformFunc func(models.Point) models.Point
}

func New() *Converter {
	return &Converter{
		builder: graph.NewGraphBuilder(),
	}
}

// Convert SVG → react-planner JSON
func (c *Converter) Convert(r io.Reader) (*models.Scene, error) {
	const sceneWidth = 3000.0
	const sceneHeight = 2000.0

	// Парсинг SVG
	elements, err := parser.ParseSVG(r)
	if err != nil {
		return nil, fmt.Errorf("parse SVG: %w", err)
	}
	c.elements = elements

	// Bounding box для трансформаций (отзеркалить и нормализовать в (0,0))
	box, err := calculateBoundingBox(elements)
	if err != nil {
		return nil, fmt.Errorf("bbox: %w", err)
	}
	c.bbox = box
	c.transformFunc = c.mirrorTransform(sceneWidth, sceneHeight)
	c.builder.SetTransform(c.transformFunc)

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
	items := make(map[string]models.Item)
	for _, room := range rooms {
		c.createArea(room, "room", areas)
	}
	c.createBalconyItems(balconies, items)

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
		Items:    items,
		Selected: models.ElementsSet{Vertices: []string{}, Lines: []string{}, Holes: []string{}, Areas: []string{}, Items: []string{}},
	}

	scene := &models.Scene{
		Unit:          "cm",
		Layers:        map[string]models.Layer{"layer-1": layer},
		SelectedLayer: "layer-1",
		Grids:         defaultGrids(),
		Groups:        map[string]any{},
		Width:         sceneWidth,
		Height:        sceneHeight,
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

	lineThickness := c.getLineThickness(lineID)

	hole := &models.Hole{
		ID:         elem.ID,
		Name:       elem.ID,
		Type:       holeType,
		Prototype:  "holes",
		Line:       lineID,
		Offset:     offset,
		Properties: holeProperties(elem, holeType, lineThickness),
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

	areaTypeName := areaType
	if areaType == "balcony" {
		areaTypeName = "area" // используем базовый тип каталога
	}

	area := models.Area{
		ID:         elem.ID,
		Name:       elem.ID,
		Type:       areaTypeName,
		Prototype:  "areas",
		Vertices:   vertexIDs,
		Holes:      []string{},
		Properties: defaultAreaProperties(),
	}

	// замыкаем контур
	if len(area.Vertices) > 2 {
		first := area.Vertices[0]
		last := area.Vertices[len(area.Vertices)-1]
		if first != last {
			area.Vertices = append(area.Vertices, first)
		}
	}

	target[elem.ID] = area
}

// createBalconyItems группирует все balcony элементы в один item (как в demo).
func (c *Converter) createBalconyItems(elems []models.SVGElement, target map[string]models.Item) {
	if len(elems) == 0 {
		return
	}

	var allPoints []models.Point
	for _, elem := range elems {
		points, err := c.getElementPoints(elem)
		if err != nil || len(points) == 0 {
			continue
		}
		allPoints = append(allPoints, points...)
	}
	if len(allPoints) == 0 {
		return
	}

	minX, maxX := allPoints[0].X, allPoints[0].X
	minY, maxY := allPoints[0].Y, allPoints[0].Y
	for _, p := range allPoints {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	width := maxX - minX
	depth := maxY - minY
	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2

	itemID := elems[0].ID

	item := models.Item{
		ID:         itemID,
		Name:       "Balcony",
		Type:       "balcony",
		Prototype:  "items",
		X:          cx,
		Y:          cy,
		Rotation:   0,
		Selected:   false,
		Visible:    true,
		Properties: defaultBalconyProperties(width, depth),
	}

	target[itemID] = item
}

// ============================================================
// Geometry helpers
// ============================================================

func (c *Converter) getElementCenter(elem models.SVGElement) *models.Point {
	switch geom := elem.Geometry.(type) {
	case models.RectGeometry:
		p := models.Point{
			X: geom.X + geom.Width/2,
			Y: geom.Y + geom.Height/2,
		}
		t := c.transformFunc(p)
		return &t
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
		p := models.Point{
			X: sumX / float64(len(points)),
			Y: sumY / float64(len(points)),
		}
		t := c.transformFunc(p)
		return &t
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
		return c.applyTransform(points), nil
	case models.RectGeometry:
		return c.applyTransform([]models.Point{
			{X: geom.X, Y: geom.Y},
			{X: geom.X + geom.Width, Y: geom.Y},
			{X: geom.X + geom.Width, Y: geom.Y + geom.Height},
			{X: geom.X, Y: geom.Y + geom.Height},
		}), nil
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

func (c *Converter) getLineThickness(lineID string) float64 {
	line, ok := c.builder.GetLines()[lineID]
	if !ok {
		return 0
	}
	if line.Properties == nil {
		return 0
	}
	if t, ok := line.Properties["thickness"].(map[string]any); ok {
		if val, ok := t["length"].(float64); ok {
			return val
		}
	}
	return 0
}

// Вычисляет свойства проемов на основе геометрии (ширина/толщина), остальное — дефолты.
func holeProperties(elem models.SVGElement, holeType string, lineThickness float64) map[string]any {
	width := 80.0
	thickness := 30.0

	switch geom := elem.Geometry.(type) {
	case models.RectGeometry:
		w := math.Max(geom.Width, geom.Height)
		t := math.Min(geom.Width, geom.Height)
		if w > 0 {
			width = w
		}
		if t > 0 {
			thickness = t
		}
	case models.PathGeometry:
		points, err := parser.ParsePath(geom.D)
		if err == nil && len(points) > 0 {
			minX, maxX := points[0].X, points[0].X
			minY, maxY := points[0].Y, points[0].Y
			for _, p := range points {
				if p.X < minX {
					minX = p.X
				}
				if p.X > maxX {
					maxX = p.X
				}
				if p.Y < minY {
					minY = p.Y
				}
				if p.Y > maxY {
					maxY = p.Y
				}
			}
			w := math.Max(maxX-minX, maxY-minY)
			t := math.Min(maxX-minX, maxY-minY)
			if w > 0 {
				width = w
			}
			if t > 0 {
				thickness = t
			}
		}
	}

	if lineThickness > 0 && thickness > lineThickness {
		thickness = lineThickness
	}

	if holeType == "window" {
		return map[string]any{
			"width":     map[string]any{"length": width},
			"height":    map[string]any{"length": 100.0},
			"altitude":  map[string]any{"length": 90.0},
			"thickness": map[string]any{"length": thickness},
		}
	}

	// door
	return map[string]any{
		"width":           map[string]any{"length": width},
		"height":          map[string]any{"length": 215.0},
		"altitude":        map[string]any{"length": 0.0},
		"thickness":       map[string]any{"length": thickness},
		"flip_orizzontal": false,
	}
}

func defaultBalconyProperties(width, depth float64) map[string]any {
	return map[string]any{
		"width":        map[string]any{"length": width},
		"depth":        map[string]any{"length": depth},
		"height":       map[string]any{"length": 100.0},
		"altitude":     map[string]any{"length": 0.0},
		"patternColor": "#f5f4f4",
	}
}

// ============================================================
// Coordinate transforms
// ============================================================

type boundingBox struct {
	minX float64
	maxX float64
	minY float64
	maxY float64
}

func calculateBoundingBox(elems []models.SVGElement) (*boundingBox, error) {
	box := &boundingBox{
		minX: math.MaxFloat64,
		maxX: -math.MaxFloat64,
		minY: math.MaxFloat64,
		maxY: -math.MaxFloat64,
	}

	update := func(p models.Point) {
		if p.X < box.minX {
			box.minX = p.X
		}
		if p.X > box.maxX {
			box.maxX = p.X
		}
		if p.Y < box.minY {
			box.minY = p.Y
		}
		if p.Y > box.maxY {
			box.maxY = p.Y
		}
	}

	for _, elem := range elems {
		switch geom := elem.Geometry.(type) {
		case models.RectGeometry:
			update(models.Point{X: geom.X, Y: geom.Y})
			update(models.Point{X: geom.X + geom.Width, Y: geom.Y + geom.Height})
		case models.PathGeometry:
			points, err := parser.ParsePath(geom.D)
			if err != nil {
				return nil, err
			}
			for _, p := range points {
				update(p)
			}
		}
	}

	return box, nil
}

func (c *Converter) mirrorTransform(sceneWidth, sceneHeight float64) func(models.Point) models.Point {
	if c.bbox == nil {
		return func(p models.Point) models.Point { return p }
	}

	box := *c.bbox
	width := box.maxX - box.minX
	height := box.maxY - box.minY

	return func(p models.Point) models.Point {
		// Зеркалим по Y, затем центрируем на сцену
		x := p.X - box.minX
		y := height - (p.Y - box.minY)
		x += (sceneWidth - width) / 2
		y += (sceneHeight - height) / 2
		return models.Point{X: x, Y: y}
	}
}

func (c *Converter) applyTransform(points []models.Point) []models.Point {
	tf := c.transformFunc
	out := make([]models.Point, 0, len(points))
	for _, p := range points {
		out = append(out, tf(p))
	}
	return out
}
