package main

import "math"

// Braille dot bit for local subpixel coordinate (x in 0..1, y in 0..3)
// within a cell, per the Unicode braille pattern block layout.
var brailleBit = [2][4]rune{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

const brailleBase = 0x2800

// SubpixW and SubpixH are how many subpixel dots make up one terminal cell.
const (
	SubpixW = 2
	SubpixH = 4
)

// cell is one character cell of the rasterized canvas: which dots are lit
// and, for text edits, a literal rune overriding the braille glyph.
type cell struct {
	dots  rune
	color string
	glyph rune   // 0 unless a text edit drew a literal character here
	bg    string // "" unless the fill tool painted this cell's background
}

// empty reports whether a cell has no ink at all — the boundary condition
// the fill tool flood-fills up to.
func (c *cell) empty() bool {
	return c.dots == 0 && c.glyph == 0
}

// Raster is a rasterized view of a Document's edits over a viewport.
type Raster struct {
	Cols, Rows int
	cells      []cell
}

func (r *Raster) at(col, row int) *cell {
	return &r.cells[row*r.Cols+col]
}

// Rune returns the glyph and color to display at (col, row).
func (r *Raster) Rune(col, row int) (rune, string) {
	c := r.at(col, row)
	if c.glyph != 0 {
		return c.glyph, c.color
	}
	if c.dots == 0 {
		return ' ', ""
	}
	return brailleBase + c.dots, c.color
}

// Background returns the fill color at (col, row), or "" if unfilled.
func (r *Raster) Background(col, row int) string {
	return r.at(col, row).bg
}

// RasterizeDocument draws edits into a Cols x Rows grid. (ox, oy) is the
// world coordinate at the viewport's top-left and zoom is world-to-screen
// scale (subpixels of screen per subpixel of world).
//
// Fill edits are applied in a second pass, after every other edit, since a
// flood fill needs the full set of ink boundaries already in place to know
// where to stop.
func RasterizeDocument(edits []*Edit, cols, rows int, ox, oy, zoom float64, selectColor string) *Raster {
	r := &Raster{Cols: cols, Rows: rows, cells: make([]cell, cols*rows)}
	var fills []*Edit
	for _, e := range edits {
		if e.Kind == KindFill {
			fills = append(fills, e)
			continue
		}
		color := e.Color
		if e.Selected {
			color = selectColor
		}
		r.drawEdit(e, ox, oy, zoom, color)
	}
	for _, e := range fills {
		color := e.Color
		if e.Selected {
			color = selectColor
		}
		r.floodFill(e, ox, oy, zoom, color)
	}
	return r
}

type gridCoord struct{ col, row int }

// floodRegion walks every empty cell reachable from (col, row) without
// crossing ink, 4-directionally. touchesEdge reports whether the region
// reached the viewport boundary rather than being fully enclosed by ink —
// i.e. whether this is (as far as the current viewport can tell) open
// background rather than a closed shape.
func (r *Raster) floodRegion(col, row int) (cells []gridCoord, touchesEdge bool) {
	if col < 0 || col >= r.Cols || row < 0 || row >= r.Rows {
		return nil, true
	}
	if !r.at(col, row).empty() {
		return nil, false
	}

	queue := []gridCoord{{col, row}}
	visited := map[gridCoord]bool{{col, row}: true}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c.col == 0 || c.col == r.Cols-1 || c.row == 0 || c.row == r.Rows-1 {
			touchesEdge = true
		}
		cells = append(cells, c)
		for _, d := range [4]gridCoord{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := gridCoord{c.col + d.col, c.row + d.row}
			if n.col < 0 || n.col >= r.Cols || n.row < 0 || n.row >= r.Rows {
				touchesEdge = true
				continue
			}
			if visited[n] {
				continue
			}
			visited[n] = true
			if r.at(n.col, n.row).empty() {
				queue = append(queue, n)
			}
		}
	}
	return cells, touchesEdge
}

// floodFill paints the background of every empty cell reachable from e's
// seed point without crossing ink. It silently no-ops on an open region —
// callers should reject that at fill-creation time (see mouse.go) so the
// edit never gets added in the first place; this is just a safety net if
// the viewport boundaries have shifted since.
func (r *Raster) floodFill(e *Edit, ox, oy, zoom float64, color string) {
	if len(e.Points) == 0 {
		return
	}
	p := e.Points[0]
	col := int(((p.X - ox) * zoom) / SubpixW)
	row := int(((p.Y - oy) * zoom) / SubpixH)
	cells, touchesEdge := r.floodRegion(col, row)
	if touchesEdge {
		return
	}
	for _, c := range cells {
		r.at(c.col, c.row).bg = color
	}
}

func (r *Raster) drawEdit(e *Edit, ox, oy, zoom float64, color string) {
	pts := make([]Point, len(e.Points))
	for i, p := range e.Points {
		pts[i] = Point{X: (p.X - ox) * zoom, Y: (p.Y - oy) * zoom}
	}
	// Clamp screen-space thickness: past a certain point a fatter brush is
	// visually just a solid blob, but plotThick's cost is quadratic in
	// radius, so an unclamped size*zoom (e.g. a fat brush at 800% zoom)
	// makes every point of every stroke drawn this frame far more
	// expensive than the extra thickness is worth.
	const maxScreenSize = 64
	size := e.Size * zoom
	if size > maxScreenSize {
		size = maxScreenSize
	}

	switch e.Kind {
	case KindStroke, KindLine:
		r.drawPolyline(pts, size, color)
	case KindRect:
		r.drawRect(pts, size, color)
	case KindCircle:
		r.drawEllipse(pts, size, color)
	case KindText:
		r.drawText(pts, e.Text, color)
	}
}

func (r *Raster) drawPolyline(pts []Point, size float64, color string) {
	if len(pts) == 1 {
		r.plotThick(pts[0].X, pts[0].Y, size, color)
		return
	}
	for i := 0; i+1 < len(pts); i++ {
		r.drawSegment(pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y, size, color)
	}
}

func (r *Raster) drawRect(pts []Point, size float64, color string) {
	if len(pts) < 2 {
		return
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	corners := [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	for i := 0; i < 4; i++ {
		a, b := corners[i], corners[(i+1)%4]
		r.drawSegment(a[0], a[1], b[0], b[1], size, color)
	}
}

// drawEllipse draws the ellipse inscribed in the bounding box pts[0],pts[1],
// so dragging opposite corners freely stretches it into an oval.
func (r *Raster) drawEllipse(pts []Point, size float64, color string) {
	if len(pts) < 2 {
		return
	}
	cx, cy := (pts[0].X+pts[1].X)/2, (pts[0].Y+pts[1].Y)/2
	rx, ry := math.Abs(pts[1].X-pts[0].X)/2, math.Abs(pts[1].Y-pts[0].Y)/2
	steps := int(math.Max(16, (rx+ry)*2))
	prevX, prevY := cx+rx, cy
	for i := 1; i <= steps; i++ {
		t := 2 * math.Pi * float64(i) / float64(steps)
		x, y := cx+rx*math.Cos(t), cy+ry*math.Sin(t)
		r.drawSegment(prevX, prevY, x, y, size, color)
		prevX, prevY = x, y
	}
}

func (r *Raster) drawText(pts []Point, text string, color string) {
	if len(pts) == 0 {
		return
	}
	col := int(pts[0].X / SubpixW)
	row := int(pts[0].Y / SubpixH)
	for i, ch := range text {
		c, ro := col+i, row
		if c < 0 || c >= r.Cols || ro < 0 || ro >= r.Rows {
			continue
		}
		cell := r.at(c, ro)
		cell.glyph = ch
		cell.color = color
	}
}

// drawSegment rasterizes a line between two points in screen subpixel
// space, thickened to size subpixels, using a Bresenham-style walk plus a
// splat at each step to give it width.
func (r *Raster) drawSegment(x0, y0, x1, y1, size float64, color string) {
	steps := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0)))
	if steps == 0 {
		r.plotThick(x0, y0, size, color)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		r.plotThick(x0+(x1-x0)*t, y0+(y1-y0)*t, size, color)
	}
}

// plotThick lights subpixels within radius size/2 of (x, y).
func (r *Raster) plotThick(x, y, size float64, color string) {
	radius := size / 2
	if radius < 0.5 {
		radius = 0.5
	}
	ix, iy := int(math.Round(x)), int(math.Round(y))
	ri := int(math.Ceil(radius))
	for dy := -ri; dy <= ri; dy++ {
		for dx := -ri; dx <= ri; dx++ {
			if math.Hypot(float64(dx), float64(dy)) > radius {
				continue
			}
			r.plot(ix+dx, iy+dy, color)
		}
	}
}

// plot lights a single subpixel dot at absolute screen subpixel coordinate
// (x, y).
func (r *Raster) plot(x, y int, color string) {
	if x < 0 || y < 0 {
		return
	}
	col, row := x/SubpixW, y/SubpixH
	if col >= r.Cols || row >= r.Rows {
		return
	}
	lx, ly := x%SubpixW, y%SubpixH
	c := r.at(col, row)
	c.dots |= brailleBit[lx][ly]
	c.color = color
}
