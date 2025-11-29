package parser

import (
	"encoding/xml"
	"io"
	"strings"

	"api-gateway/internal/converter/models"
)

// ============================================================
// XML Structures
// ============================================================

type SVG struct {
	XMLName xml.Name `xml:"svg"`
	Rects   []Rect   `xml:"rect"`
	Paths   []Path   `xml:"path"`
}

type Rect struct {
	ID     string  `xml:"id,attr"`
	X      float64 `xml:"x,attr"`
	Y      float64 `xml:"y,attr"`
	Width  float64 `xml:"width,attr"`
	Height float64 `xml:"height,attr"`
}

type Path struct {
	ID string `xml:"id,attr"`
	D  string `xml:"d,attr"`
}

// ============================================================
// Parser
// ============================================================

func ParseSVG(r io.Reader) ([]models.SVGElement, error) {
	var svg SVG
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&svg); err != nil {
		return nil, err
	}

	var elements []models.SVGElement

	// Parse rects
	for _, rect := range svg.Rects {
		elemType := classifyElementByID(rect.ID)
		if elemType == "" {
			continue
		}

		elements = append(elements, models.SVGElement{
			ID:   rect.ID,
			Type: elemType,
			Geometry: models.RectGeometry{
				X:      rect.X,
				Y:      rect.Y,
				Width:  rect.Width,
				Height: rect.Height,
			},
		})
	}

	// Parse paths
	for _, path := range svg.Paths {
		elemType := classifyElementByID(path.ID)
		if elemType == "" {
			continue
		}

		elements = append(elements, models.SVGElement{
			ID:   path.ID,
			Type: elemType,
			Geometry: models.PathGeometry{
				D: path.D,
			},
		})
	}

	return elements, nil
}

func classifyElementByID(id string) string {
	if strings.HasPrefix(id, "Wall_") {
		return "wall"
	}
	if strings.HasPrefix(id, "Hui_Wall_") { // новые префиксы стен
		return "wall"
	}
	if strings.HasPrefix(id, "Door_") {
		return "door"
	}
	if strings.HasPrefix(id, "Window_") {
		return "window"
	}
	if strings.HasPrefix(id, "Room_") ||
		strings.HasSuffix(id, "_room") || // Hall_room, Toilet_room
		strings.HasSuffix(id, "_Room") {
		return "room"
	}
	if strings.HasPrefix(id, "Balcony_") || strings.HasPrefix(id, "Balcony") {
		return "balcony"
	}
	return ""
}
