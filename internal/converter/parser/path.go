package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"api-gateway/internal/converter/models"
)

// ============================================================
// Path Parser
// ============================================================

// ParsePath парсит SVG path в список точек
func ParsePath(d string) ([]models.Point, error) {
	d = strings.TrimSpace(d)
	if d == "" {
		return nil, fmt.Errorf("empty path")
	}

	var points []models.Point
	var currentX, currentY float64

	// Простой парсер команд M, m, L, l, H, h, V, v, Z
	re := regexp.MustCompile(`([MmLlHhVvZz])([^MmLlHhVvZz]*)`)
	matches := re.FindAllStringSubmatch(d, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		cmd := match[1]
		args := strings.TrimSpace(match[2])

		switch cmd {
		case "M": // MoveTo absolute
			coords := parseCoords(args)
			if len(coords) >= 2 {
				currentX, currentY = coords[0], coords[1]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "m": // MoveTo relative
			coords := parseCoords(args)
			if len(coords) >= 2 {
				currentX += coords[0]
				currentY += coords[1]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "L": // LineTo absolute
			coords := parseCoords(args)
			if len(coords) >= 2 {
				currentX, currentY = coords[0], coords[1]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "l": // LineTo relative
			coords := parseCoords(args)
			if len(coords) >= 2 {
				currentX += coords[0]
				currentY += coords[1]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "H": // Horizontal line absolute
			coords := parseCoords(args)
			if len(coords) >= 1 {
				currentX = coords[0]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "h": // Horizontal line relative
			coords := parseCoords(args)
			if len(coords) >= 1 {
				currentX += coords[0]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "V": // Vertical line absolute
			coords := parseCoords(args)
			if len(coords) >= 1 {
				currentY = coords[0]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "v": // Vertical line relative
			coords := parseCoords(args)
			if len(coords) >= 1 {
				currentY += coords[0]
				points = append(points, models.Point{X: currentX, Y: currentY})
			}

		case "Z", "z": // Close path
			// Замыкаем путь, возвращаясь к первой точке
			if len(points) > 0 {
				points = append(points, points[0])
			}
		}
	}

	return points, nil
}

func parseCoords(s string) []float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Разделитель: запятая или пробел
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)

	var coords []float64
	for _, part := range parts {
		val, err := strconv.ParseFloat(part, 64)
		if err == nil {
			coords = append(coords, val)
		}
	}

	return coords
}
