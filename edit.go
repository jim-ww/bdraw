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
	KindArrow  Kind = "arrow"
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
	Filled bool    `json:"filled,omitempty"` // rect/circle only: solid interior instead of outline

	// Hardness is brush edge softness as a percentage (hardnessMin..100):
	// 100 is a crisp, fully solid edge (the historical, pre-hardness
	// look); lower values feather the outer part of the stroke through
	// progressively sparser block glyphs. 0 is the zero-value a file
	// saved before this field existed unmarshals to, and is treated as
	// "unset → 100" (see Edit.hardness) rather than a real 0% — a brush
	// that's invisible everywhere isn't a meaningful setting, so 0 never
	// needs to mean anything else, which avoids a file-format migration.
	Hardness float64 `json:"hardness,omitempty"`

	// Area is the region a fill edit paints, computed when the fill is
	// made. Fills from files saved before it existed get one on load.
	Area *FillArea `json:"area,omitempty"`

	// Selected is UI-only state (select tool) and is never persisted.
	Selected bool `json:"-"`
}

// hardness returns e.Hardness, treating the zero value (unset, or an
// explicit 0 — never meaningfully different) as fully hard.
func (e *Edit) hardness() float64 {
	if e.Hardness <= 0 {
		return 100
	}
	return e.Hardness
}

// Clone returns a deep copy of e.
func (e *Edit) Clone() *Edit {
	c := *e
	c.Points = append([]Point(nil), e.Points...)
	if e.Area != nil {
		a := *e.Area
		c.Area = &a
	}
	return &c
}

// Translate moves e by (dx, dy).
func (e *Edit) Translate(dx, dy float64) {
	for i := range e.Points {
		e.Points[i].X += dx
		e.Points[i].Y += dy
	}
	if e.Area != nil {
		e.Area.Origin.X += dx
		e.Area.Origin.Y += dy
	}
}
