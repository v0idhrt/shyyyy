package models

// ============================================================
// SVG Elements
// ============================================================

type SVGElement struct {
	ID       string
	Type     string // wall, door, window, room, balcony
	Geometry interface{}
}

type RectGeometry struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type PathGeometry struct {
	D string
}

// ============================================================
// Geometry primitives
// ============================================================

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ============================================================
// React Planner core structures
// ============================================================

type LengthValue struct {
	Length float64 `json:"length"`
}

type ElementsSet struct {
	Vertices []string `json:"vertices"`
	Lines    []string `json:"lines"`
	Holes    []string `json:"holes"`
	Areas    []string `json:"areas"`
	Items    []string `json:"items"`
}

type Vertex struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Prototype  string         `json:"prototype"`
	X          float64        `json:"x"`
	Y          float64        `json:"y"`
	Lines      []string       `json:"lines"`
	Areas      []string       `json:"areas"`
	Selected   bool           `json:"selected"`
	Properties map[string]any `json:"properties,omitempty"`
	Misc       map[string]any `json:"misc,omitempty"`
}

type Line struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Prototype  string         `json:"prototype"`
	Vertices   []string       `json:"vertices"`
	Holes      []string       `json:"holes"`
	Properties map[string]any `json:"properties"`
	Misc       map[string]any `json:"misc,omitempty"`
}

type Hole struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Prototype  string         `json:"prototype"`
	Offset     float64        `json:"offset"`
	Line       string         `json:"line"`
	Properties map[string]any `json:"properties"`
	Misc       map[string]any `json:"misc,omitempty"`
}

type Area struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Prototype  string         `json:"prototype"`
	Vertices   []string       `json:"vertices"`
	Holes      []string       `json:"holes"`
	Properties map[string]any `json:"properties"`
	Misc       map[string]any `json:"misc,omitempty"`
}

type Grid struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

type Guides struct {
	Horizontal map[string]any `json:"horizontal"`
	Vertical   map[string]any `json:"vertical"`
	Circular   map[string]any `json:"circular"`
}

type Layer struct {
	ID       string            `json:"id"`
	Altitude float64           `json:"altitude"`
	Order    int               `json:"order"`
	Opacity  float64           `json:"opacity"`
	Name     string            `json:"name"`
	Visible  bool              `json:"visible"`
	Vertices map[string]Vertex `json:"vertices"`
	Lines    map[string]Line   `json:"lines"`
	Holes    map[string]Hole   `json:"holes"`
	Areas    map[string]Area   `json:"areas"`
	Items    map[string]any    `json:"items"`
	Selected ElementsSet       `json:"selected"`
}

type Scene struct {
	Unit          string           `json:"unit"`
	Layers        map[string]Layer `json:"layers"`
	SelectedLayer string           `json:"selectedLayer"`
	Grids         map[string]Grid  `json:"grids"`
	Groups        map[string]any   `json:"groups"`
	Width         float64          `json:"width"`
	Height        float64          `json:"height"`
	Meta          map[string]any   `json:"meta"`
	Guides        Guides           `json:"guides"`
}
