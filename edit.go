package main

// Kind identifies the shape of an Edit.
type Kind string

const (
	KindStroke Kind = "stroke"
	KindRect   Kind = "rect"
	KindCircle Kind = "circle"
	KindLine   Kind = "line"
	KindText   Kind = "text"
	KindFill   Kind = "fill"
)

// Point is a location in canvas subpixel space (2 subpixels per cell
// horizontally, 4 vertically, matching a braille glyph's dot grid).
type Point struct {
	X, Y float64
}

// Edit is a single undoable unit of drawing: one brush stroke, one shape,
// or one text placement.
type Edit struct {
	ID     int     `json:"id"`
	Kind   Kind    `json:"kind"`
	Points []Point `json:"points"`
	Color  string  `json:"color"`
	Size   float64 `json:"size"`
	Text   string  `json:"text,omitempty"`

	// Selected is UI-only state (select tool) and is never persisted.
	Selected bool `json:"-"`
}

// Clone returns a deep copy of e.
func (e *Edit) Clone() *Edit {
	c := *e
	c.Points = append([]Point(nil), e.Points...)
	return &c
}
