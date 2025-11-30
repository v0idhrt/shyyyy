package graph

import (
	"fmt"
	"math"
	"sort"

	"api-gateway/internal/converter/models"
	"api-gateway/internal/converter/parser"
)

// ============================================================
// Graph Builder
// ============================================================

const tolerance = 2.0         // Tolerance для объединения близких точек
const connectTolerance = 15   // Допуск для поиска пересечения и снаппинга
const mergeTolerance = 8.0    // Радиус склейки близких вершин после разрезания сегментов
const axisSnapTolerance = 4.0 // Насколько расходиться от оси, чтобы зафиксировать координату

type GraphBuilder struct {
	vertices  map[string]models.Vertex
	lines     map[string]models.Line
	segments  []wallSegment
	vertexID  int
	transform func(models.Point) models.Point
}

func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		vertices:  make(map[string]models.Vertex),
		lines:     make(map[string]models.Line),
		segments:  []wallSegment{},
		vertexID:  0,
		transform: func(p models.Point) models.Point { return p },
	}
}

// BuildFromWalls создает граф из стен
func (g *GraphBuilder) BuildFromWalls(walls []models.SVGElement) error {
	g.reset()

	for _, wall := range walls {
		if err := g.addWall(wall); err != nil {
			return err
		}
	}

	g.buildConnectedGraph()
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

	thickness := math.Min(rect.Width, rect.Height)

	if rect.Width > rect.Height {
		// Горизонтальная линия
		p1 = models.Point{X: rect.X, Y: rect.Y + rect.Height/2}
		p2 = models.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height/2}
	} else {
		// Вертикальная линия
		p1 = models.Point{X: rect.X + rect.Width/2, Y: rect.Y}
		p2 = models.Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height}
	}

	p1 = g.transform(p1)
	p2 = g.transform(p2)

	g.segments = append(g.segments, wallSegment{
		id:         id,
		name:       id,
		p1:         p1,
		p2:         p2,
		properties: defaultWallProperties(thickness),
	})
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

	thickness := math.Min(width, height)
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

	p1 = g.transform(p1)
	p2 = g.transform(p2)

	g.segments = append(g.segments, wallSegment{
		id:         id,
		name:       id,
		p1:         p1,
		p2:         p2,
		properties: defaultWallProperties(thickness),
	})
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
// Wall segments connection
// ============================================================

type wallSegment struct {
	id         string
	name       string
	p1         models.Point
	p2         models.Point
	properties map[string]any
}

type segmentInfo struct {
	segment     wallSegment
	horizontal  bool
	start       float64
	end         float64
	constant    float64
	splitPoints []float64
}

func (g *GraphBuilder) reset() {
	g.vertices = make(map[string]models.Vertex)
	g.lines = make(map[string]models.Line)
	g.segments = g.segments[:0]
	g.vertexID = 0
}

func (g *GraphBuilder) buildConnectedGraph() {
	segments := g.splitSegments(g.segments)

	for _, seg := range segments {
		v1ID := g.findOrCreateVertex(seg.p1)
		v2ID := g.findOrCreateVertex(seg.p2)

		line := models.Line{
			ID:         seg.id,
			Name:       seg.name,
			Type:       "wall",
			Prototype:  "lines",
			Vertices:   []string{v1ID, v2ID},
			Holes:      []string{},
			Properties: seg.properties,
		}

		g.lines[line.ID] = line
		g.attachLineToVertex(v1ID, line.ID)
		g.attachLineToVertex(v2ID, line.ID)
	}

	g.mergeCloseVertices()
	g.snapAxisAligned()
}

func (g *GraphBuilder) splitSegments(segments []wallSegment) []wallSegment {
	if len(segments) == 0 {
		return nil
	}

	infos := make([]*segmentInfo, 0, len(segments))
	for _, seg := range segments {
		horizontal := math.Abs(seg.p1.Y-seg.p2.Y) <= math.Abs(seg.p1.X-seg.p2.X)
		start, end := seg.p1.X, seg.p2.X
		constant := seg.p1.Y

		if !horizontal {
			start, end = seg.p1.Y, seg.p2.Y
			constant = seg.p1.X
		}
		if start > end {
			start, end = end, start
		}

		infos = append(infos, &segmentInfo{
			segment:     seg,
			horizontal:  horizontal,
			start:       start,
			end:         end,
			constant:    constant,
			splitPoints: []float64{start, end},
		})
	}

	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			a, b := infos[i], infos[j]
			if a.horizontal == b.horizontal {
				continue
			}
			var h, v *segmentInfo
			if a.horizontal {
				h, v = a, b
			} else {
				h, v = b, a
			}
			g.tryAddIntersection(h, v)
		}
	}

	counter := make(map[string]int)
	var result []wallSegment

	for _, info := range infos {
		points := append([]float64{}, info.splitPoints...)
		points = append(points, info.start, info.end)

		sort.Float64s(points)
		points = uniquePoints(points)

		if len(points) < 2 {
			continue
		}

		parts := len(points) - 1
		for idx := 0; idx < parts; idx++ {
			start := points[idx]
			end := points[idx+1]
			if almostEqual(start, end) {
				continue
			}

			var p1, p2 models.Point
			if info.horizontal {
				p1 = models.Point{X: start, Y: info.constant}
				p2 = models.Point{X: end, Y: info.constant}
			} else {
				p1 = models.Point{X: info.constant, Y: start}
				p2 = models.Point{X: info.constant, Y: end}
			}

			counter[info.segment.id]++
			lineID := info.segment.id
			if parts > 1 {
				lineID = fmt.Sprintf("%s_%d", info.segment.id, counter[info.segment.id])
			}

			result = append(result, wallSegment{
				id:         lineID,
				name:       info.segment.name,
				p1:         p1,
				p2:         p2,
				properties: info.segment.properties,
			})
		}
	}

	return result
}

func (g *GraphBuilder) tryAddIntersection(h, v *segmentInfo) {
	vx := v.constant
	hy := h.constant

	if vx < h.start-connectTolerance || vx > h.end+connectTolerance {
		return
	}
	if hy < v.start-connectTolerance || hy > v.end+connectTolerance {
		return
	}

	ix := clamp(vx, h.start, h.end)
	iy := clamp(hy, v.start, v.end)

	h.splitPoints = append(h.splitPoints, ix)
	v.splitPoints = append(v.splitPoints, iy)
}

func uniquePoints(points []float64) []float64 {
	if len(points) == 0 {
		return points
	}
	out := points[:1]
	for i := 1; i < len(points); i++ {
		if !almostEqual(points[i], points[i-1]) {
			out = append(out, points[i])
		}
	}
	return out
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// mergeCloseVertices объединяет вершины, которые находятся совсем рядом после разрезания сегментов.
func (g *GraphBuilder) mergeCloseVertices() {
	if len(g.vertices) == 0 {
		return
	}

	ids := make([]string, 0, len(g.vertices))
	for id := range g.vertices {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rep := make(map[string]string, len(ids))

	for i, id := range ids {
		if _, ok := rep[id]; ok {
			continue
		}
		base := g.vertices[id]
		rep[id] = id

		for j := i + 1; j < len(ids); j++ {
			otherID := ids[j]
			if _, ok := rep[otherID]; ok {
				continue
			}
			other := g.vertices[otherID]
			if distance(models.Point{X: base.X, Y: base.Y}, models.Point{X: other.X, Y: other.Y}) <= mergeTolerance {
				rep[otherID] = id
				base.Lines = appendUnique(base.Lines, other.Lines...)
				base.Areas = appendUnique(base.Areas, other.Areas...)
			}
		}
		g.vertices[id] = base
	}

	repOf := func(id string) string {
		if v, ok := rep[id]; ok {
			return v
		}
		return id
	}

	newLines := make(map[string]models.Line, len(g.lines))
	for id, line := range g.lines {
		if len(line.Vertices) < 2 {
			continue
		}

		v1 := repOf(line.Vertices[0])
		v2 := repOf(line.Vertices[1])
		if v1 == v2 {
			continue
		}
		line.Vertices = []string{v1, v2}
		newLines[id] = line
	}

	newVertices := make(map[string]models.Vertex)
	for oldID, v := range g.vertices {
		if repOf(oldID) != oldID {
			continue
		}
		newVertices[oldID] = models.Vertex{
			ID:        v.ID,
			Name:      v.Name,
			Type:      v.Type,
			Prototype: v.Prototype,
			X:         v.X,
			Y:         v.Y,
			Lines:     []string{},
			Areas:     append([]string{}, v.Areas...),
			Selected:  v.Selected,
			Properties: func() map[string]any {
				if v.Properties == nil {
					return nil
				}
				cp := make(map[string]any, len(v.Properties))
				for k, val := range v.Properties {
					cp[k] = val
				}
				return cp
			}(),
			Misc: func() map[string]any {
				if v.Misc == nil {
					return nil
				}
				cp := make(map[string]any, len(v.Misc))
				for k, val := range v.Misc {
					cp[k] = val
				}
				return cp
			}(),
		}
	}

	// Пересобираем Line ссылки
	for lineID, line := range newLines {
		for _, vid := range line.Vertices {
			v := newVertices[vid]
			v.Lines = appendUnique(v.Lines, []string{lineID}...)
			newVertices[vid] = v
		}
	}

	g.vertices = newVertices
	g.lines = newLines
}

// snapAxisAligned фиксирует координаты вершин по осям для почти горизонтальных/вертикальных линий.
func (g *GraphBuilder) snapAxisAligned() {
	if len(g.lines) == 0 {
		return
	}

	type agg struct {
		sumX float64
		cntX int
		sumY float64
		cntY int
	}

	aggMap := make(map[string]*agg)

	for _, line := range g.lines {
		if len(line.Vertices) < 2 {
			continue
		}
		v1 := g.vertices[line.Vertices[0]]
		v2 := g.vertices[line.Vertices[1]]

		dx := v1.X - v2.X
		dy := v1.Y - v2.Y

		if math.Abs(dy) <= axisSnapTolerance {
			targetY := (v1.Y + v2.Y) / 2
			for _, vid := range line.Vertices {
				a := aggMap[vid]
				if a == nil {
					a = &agg{}
					aggMap[vid] = a
				}
				a.sumY += targetY
				a.cntY++
			}
		} else if math.Abs(dx) <= axisSnapTolerance {
			targetX := (v1.X + v2.X) / 2
			for _, vid := range line.Vertices {
				a := aggMap[vid]
				if a == nil {
					a = &agg{}
					aggMap[vid] = a
				}
				a.sumX += targetX
				a.cntX++
			}
		}
	}

	for vid, a := range aggMap {
		v := g.vertices[vid]
		if a.cntX > 0 {
			v.X = a.sumX / float64(a.cntX)
		}
		if a.cntY > 0 {
			v.Y = a.sumY / float64(a.cntY)
		}
		g.vertices[vid] = v
	}
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

func appendUnique(dst []string, src ...string) []string {
	for _, s := range src {
		if !contains(dst, s) {
			dst = append(dst, s)
		}
	}
	return dst
}

func defaultWallProperties(thickness float64) map[string]any {
	return map[string]any{
		"height":    map[string]any{"length": 300.0},
		"thickness": map[string]any{"length": thickness},
		"textureA":  "bricks",
		"textureB":  "bricks",
	}
}

// SetTransform задает функцию трансформации координат (например, зеркалирование).
func (g *GraphBuilder) SetTransform(f func(models.Point) models.Point) {
	if f == nil {
		g.transform = func(p models.Point) models.Point { return p }
		return
	}
	g.transform = f
}
