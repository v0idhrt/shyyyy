package mapper

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"api-gateway/internal/converter/models"
)

// ============================================================
// Renderer
// ============================================================

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render собирает SVG из react-planner scene JSON.
func (r *Renderer) Render(scene *models.Scene) (string, error) {
	if scene == nil {
		return "", fmt.Errorf("scene is nil")
	}

	layer, err := r.pickLayer(scene)
	if err != nil {
		return "", err
	}

	width, height := r.sceneSize(scene, layer)

	var elements []string
	elements = append(elements, r.renderWalls(layer)...)
	elements = append(elements, r.renderAreas(layer)...)
	elements = append(elements, r.renderHoles(layer)...)
	elements = append(elements, r.renderBalconies(layer)...)

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		formatFloat(width), formatFloat(height), formatFloat(width), formatFloat(height)))
	builder.WriteString("\n")

	for _, elem := range elements {
		builder.WriteString("  ")
		builder.WriteString(elem)
		builder.WriteString("\n")
	}

	builder.WriteString(`</svg>`)
	return builder.String(), nil
}

// ============================================================
// Layer selection & sizing
// ============================================================

func (r *Renderer) pickLayer(scene *models.Scene) (models.Layer, error) {
	if len(scene.Layers) == 0 {
		return models.Layer{}, fmt.Errorf("scene has no layers")
	}

	if scene.SelectedLayer != "" {
		if layer, ok := scene.Layers[scene.SelectedLayer]; ok {
			return layer, nil
		}
	}

	var ids []string
	for id := range scene.Layers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return scene.Layers[ids[0]], nil
}

func (r *Renderer) sceneSize(scene *models.Scene, layer models.Layer) (float64, float64) {
	if scene.Width > 0 && scene.Height > 0 {
		return scene.Width, scene.Height
	}

	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	for _, v := range layer.Vertices {
		if v.X < minX {
			minX = v.X
		}
		if v.X > maxX {
			maxX = v.X
		}
		if v.Y < minY {
			minY = v.Y
		}
		if v.Y > maxY {
			maxY = v.Y
		}
	}

	if minX == math.MaxFloat64 || minY == math.MaxFloat64 {
		return 1000, 1000
	}

	width := maxX - minX
	height := maxY - minY
	if width <= 0 {
		width = 1000
	}
	if height <= 0 {
		height = 1000
	}

	return width, height
}

// ============================================================
// Element renderers
// ============================================================

func (r *Renderer) renderWalls(layer models.Layer) []string {
	var out []string

	for _, line := range layer.Lines {
		if len(line.Vertices) < 2 {
			continue
		}

		v1, ok1 := layer.Vertices[line.Vertices[0]]
		v2, ok2 := layer.Vertices[line.Vertices[1]]
		if !ok1 || !ok2 {
			continue
		}

		thickness := lengthFromProperties(line.Properties, "thickness", 10)
		if thickness == 0 {
			thickness = 10
		}

		dx := v2.X - v1.X
		dy := v2.Y - v1.Y
		horizontal := math.Abs(dx) >= math.Abs(dy)

		if horizontal {
			width := math.Abs(dx)
			if width == 0 {
				width = thickness
			}
			x := math.Min(v1.X, v2.X)
			y := ((v1.Y + v2.Y) / 2) - thickness/2
			out = append(out, fmt.Sprintf(`<rect id="%s" x="%s" y="%s" width="%s" height="%s" fill="none" stroke="#000" />`,
				line.ID, formatFloat(x), formatFloat(y), formatFloat(width), formatFloat(thickness)))
			continue
		}

		height := math.Abs(dy)
		if height == 0 {
			height = thickness
		}
		x := ((v1.X + v2.X) / 2) - thickness/2
		y := math.Min(v1.Y, v2.Y)

		out = append(out, fmt.Sprintf(`<rect id="%s" x="%s" y="%s" width="%s" height="%s" fill="none" stroke="#000" />`,
			line.ID, formatFloat(x), formatFloat(y), formatFloat(thickness), formatFloat(height)))
	}

	return out
}

func (r *Renderer) renderHoles(layer models.Layer) []string {
	var out []string

	for _, hole := range layer.Holes {
		line, ok := layer.Lines[hole.Line]
		if !ok || len(line.Vertices) < 2 {
			continue
		}

		v1, ok1 := layer.Vertices[line.Vertices[0]]
		v2, ok2 := layer.Vertices[line.Vertices[1]]
		if !ok1 || !ok2 {
			continue
		}

		dx := v2.X - v1.X
		dy := v2.Y - v1.Y
		offset := clamp(hole.Offset, 0, 1)

		cx := v1.X + dx*offset
		cy := v1.Y + dy*offset

		width := lengthFromProperties(hole.Properties, "width", 80)
		thickness := lengthFromProperties(hole.Properties, "thickness", lengthFromProperties(line.Properties, "thickness", 10))

		horizontal := math.Abs(dx) >= math.Abs(dy)

		var w, h, x, y float64
		if horizontal {
			w = width
			h = thickness
			x = cx - w/2
			y = cy - h/2
		} else {
			w = thickness
			h = width
			x = cx - w/2
			y = cy - h/2
		}

		stroke := "#1f77b4"
		if hole.Type == "door" {
			stroke = "#d62728"
		}

		out = append(out, fmt.Sprintf(`<rect id="%s" x="%s" y="%s" width="%s" height="%s" fill="none" stroke="%s" />`,
			hole.ID, formatFloat(x), formatFloat(y), formatFloat(w), formatFloat(h), stroke))
	}

	return out
}

func (r *Renderer) renderAreas(layer models.Layer) []string {
	var out []string

	for _, area := range layer.Areas {
		points := r.collectAreaPoints(area, layer.Vertices)
		if len(points) < 3 {
			continue
		}

		var path strings.Builder
		path.WriteString(`<path id="`)
		path.WriteString(area.ID)
		path.WriteString(`" d="M `)
		path.WriteString(formatPoint(points[0]))
		for _, p := range points[1:] {
			path.WriteString(" L ")
			path.WriteString(formatPoint(p))
		}
		path.WriteString(` Z" fill="none" stroke="#888" />`)

		out = append(out, path.String())
	}

	return out
}

func (r *Renderer) renderBalconies(layer models.Layer) []string {
	var out []string

	for _, item := range layer.Items {
		if item.Type != "balcony" {
			continue
		}

		width := lengthFromProperties(item.Properties, "width", 100)
		depth := lengthFromProperties(item.Properties, "depth", 100)
		points := rectanglePoints(item.X, item.Y, width, depth, item.Rotation)

		var path strings.Builder
		path.WriteString(`<path id="`)
		path.WriteString(item.ID)
		path.WriteString(`" d="M `)
		path.WriteString(formatPoint(points[0]))
		for _, p := range points[1:] {
			path.WriteString(" L ")
			path.WriteString(formatPoint(p))
		}
		path.WriteString(` Z" fill="none" stroke="#2ca02c" />`)

		out = append(out, path.String())
	}

	return out
}

// ============================================================
// Geometry helpers
// ============================================================

func (r *Renderer) collectAreaPoints(area models.Area, vertices map[string]models.Vertex) []models.Point {
	var points []models.Point

	for _, id := range area.Vertices {
		if v, ok := vertices[id]; ok {
			points = append(points, models.Point{X: v.X, Y: v.Y})
		}
	}

	if len(points) > 1 {
		first := points[0]
		last := points[len(points)-1]
		if first.X == last.X && first.Y == last.Y {
			points = points[:len(points)-1]
		}
	}

	return points
}

func rectanglePoints(cx, cy, width, height, rotationDeg float64) []models.Point {
	halfW := width / 2
	halfH := height / 2

	points := []models.Point{
		{X: cx - halfW, Y: cy - halfH},
		{X: cx + halfW, Y: cy - halfH},
		{X: cx + halfW, Y: cy + halfH},
		{X: cx - halfW, Y: cy + halfH},
	}

	if rotationDeg == 0 {
		return points
	}

	rad := rotationDeg * math.Pi / 180
	sin := math.Sin(rad)
	cos := math.Cos(rad)

	for i, p := range points {
		dx := p.X - cx
		dy := p.Y - cy
		points[i] = models.Point{
			X: cx + dx*cos - dy*sin,
			Y: cy + dx*sin + dy*cos,
		}
	}

	return points
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// ============================================================
// Formatting helpers
// ============================================================

func lengthFromProperties(props map[string]any, key string, def float64) float64 {
	if props == nil {
		return def
	}

	if raw, ok := props[key]; ok {
		switch v := raw.(type) {
		case float64:
			return v
		case map[string]any:
			if val, ok := v["length"]; ok {
				if f, ok := val.(float64); ok {
					return f
				}
			}
		}
	}
	return def
}

func formatFloat(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func formatPoint(p models.Point) string {
	return formatFloat(p.X) + " " + formatFloat(p.Y)
}
