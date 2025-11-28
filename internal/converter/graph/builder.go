package graph

import (
	"fmt"
	"math"

	"api-gateway/internal/converter/models"
	"api-gateway/internal/converter/parser"
)

// ============================================================
// Graph Builder
// ============================================================

const tolerance = 2.0 // Tolerance для объединения близких точек

type GraphBuilder struct {
	vertices map[string]models.Vertex
	lines    map[string]models.Line
	vertexID int
}

func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		vertices: make(map[string]models.Vertex),
		lines:    make(map[string]models.Line),
		vertexID: 0,
	}
}

// BuildFromWalls создает граф из стен
func (g *GraphBuilder) BuildFromWalls(walls []models.SVGElement) error {
	for _, wall := range walls {
		if err := g.addWall(wall); err != nil {
			return err
		}
	}
	return nil
}

func (g *GraphBuilder) addWall(wall models.SVGElement) error {
	switch geom := wall.Geometry.(type) {
	case models.RectGeometry:
		return g.addRectWall(wall.ID, geom)
	case models.PathGeometry:
		return g.addPathWall(wall.ID, geom)
	}
	return nil
}

func (g *GraphBuilder) addRectWall(id string, rect models.RectGeometry) error {
	// Rect преобразуем в линию (используем длинную сторону)
	var p1, p2 models.Point

	if rect.Width > rect.Height {
		// Горизонтальная линия
		p1 = models.Point{X: rect.X, Y: rect.Y + rect.Height/2}
		p2 = models.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height/2}
	} else {
		// Вертикальная линия
		p1 = models.Point{X: rect.X + rect.Width/2, Y: rect.Y}
		p2 = models.Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height}
	}

	v1ID := g.findOrCreateVertex(p1)
	v2ID := g.findOrCreateVertex(p2)

	g.lines[id] = models.Line{
		ID:         id,
		Name:       id,
		Type:       "wall",
		Prototype:  "lines",
		Vertices:   []string{v1ID, v2ID},
		Holes:      []string{},
		Properties: defaultWallProperties(),
	}

	g.attachLineToVertex(v1ID, id)
	g.attachLineToVertex(v2ID, id)

	return nil
}

func (g *GraphBuilder) addPathWall(id string, path models.PathGeometry) error {
	points, err := parser.ParsePath(path.D)
	if err != nil {
		return err
	}

	if len(points) < 2 {
		return nil
	}

	// Для path берем центр длинной стороны bounding box
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

	width := maxX - minX
	height := maxY - minY

	var p1, p2 models.Point
	switch {
	case width == 0 && height == 0:
		p1, p2 = points[0], points[len(points)-1]
	case width >= height:
		// горизонтальная: середина по Y, края по X
		midY := minY + height/2
		p1 = models.Point{X: minX, Y: midY}
		p2 = models.Point{X: maxX, Y: midY}
	default:
		// вертикальная: середина по X, края по Y
		midX := minX + width/2
		p1 = models.Point{X: midX, Y: minY}
		p2 = models.Point{X: midX, Y: maxY}
	}

	v1ID := g.findOrCreateVertex(p1)
	v2ID := g.findOrCreateVertex(p2)

	g.lines[id] = models.Line{
		ID:         id,
		Name:       id,
		Type:       "wall",
		Prototype:  "lines",
		Vertices:   []string{v1ID, v2ID},
		Holes:      []string{},
		Properties: defaultWallProperties(),
	}

	g.attachLineToVertex(v1ID, id)
	g.attachLineToVertex(v2ID, id)

	return nil
}

func (g *GraphBuilder) findOrCreateVertex(p models.Point) string {
	// Ищем существующую близкую точку
	for _, v := range g.vertices {
		if distance(p, models.Point{X: v.X, Y: v.Y}) < tolerance {
			return v.ID
		}
	}

	// Создаем новую вершину
	g.vertexID++
	id := fmt.Sprintf("v%d", g.vertexID)
	g.vertices[id] = models.Vertex{
		ID:        id,
		Name:      "Vertex",
		Type:      "vertex",
		Prototype: "vertices",
		X:         p.X,
		Y:         p.Y,
		Lines:     []string{},
		Areas:     []string{},
		Selected:  false,
	}

	return id
}

func distance(p1, p2 models.Point) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (g *GraphBuilder) GetVertices() map[string]models.Vertex {
	return g.vertices
}

func (g *GraphBuilder) GetLines() map[string]models.Line {
	return g.lines
}

// ============================================================
// Helpers
// ============================================================

func (g *GraphBuilder) attachLineToVertex(vertexID, lineID string) {
	vertex := g.vertices[vertexID]
	if !contains(vertex.Lines, lineID) {
		vertex.Lines = append(vertex.Lines, lineID)
	}
	g.vertices[vertexID] = vertex
}

func (g *GraphBuilder) AddAreaVertices(points []models.Point, areaID string) []string {
	var ids []string
	for _, p := range points {
		id := g.findOrCreateVertex(p)
		vertex := g.vertices[id]
		if !contains(vertex.Areas, areaID) {
			vertex.Areas = append(vertex.Areas, areaID)
		}
		g.vertices[id] = vertex
		ids = append(ids, id)
	}
	return ids
}

func (g *GraphBuilder) AttachHoleToLine(lineID, holeID string) {
	line, ok := g.lines[lineID]
	if !ok {
		return
	}
	if !contains(line.Holes, holeID) {
		line.Holes = append(line.Holes, holeID)
	}
	g.lines[lineID] = line
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func defaultWallProperties() map[string]any {
	return map[string]any{
		"height":    map[string]any{"length": 300.0},
		"thickness": map[string]any{"length": 20.0},
		"textureA":  "bricks",
		"textureB":  "bricks",
	}
}
