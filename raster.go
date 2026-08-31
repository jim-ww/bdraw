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

// clone returns an independent copy, so it can be safely drawn onto
// without disturbing the original (used to overlay a shape's current
// geometry on top of a cached base raster of everything else).
func (r *Raster) clone() *Raster {
	cells := make([]cell, len(r.cells))
	copy(cells, r.cells)
	return &Raster{Cols: r.Cols, Rows: r.Rows, cells: cells}
}

// fillBoundEntry is one cached isFillBounded verdict, tagged with the
// document version it was computed against.
type fillBoundEntry struct {
	ver     int
	bounded bool
}

// FillBoundCache memoizes isFillBounded per fill edit, keyed by document
// version. isFillBounded is independent of the viewport, but panning or
// zooming still forces a full RasterizeDocument every frame (the render
// cache keys off viewport state too — see canvasRaster) — without this,
// every fill on the canvas re-probed the entire document on every single
// pan/zoom tick, which is what made scrolling a canvas with more than a
// few fills visibly lag (see BenchmarkRasterizeDocumentWithFills).
type FillBoundCache map[int]fillBoundEntry

// RasterizeDocument draws edits into a Cols x Rows grid. (ox, oy) is the
// world coordinate at the viewport's top-left and zoom is world-to-screen
// scale (subpixels of screen per subpixel of world).
//
// highlightID, if nonzero, recolors that one edit with highlightColor —
// used for the move/eraser tools' hover highlight — the same way a
// Selected edit is recolored with selectColor.
//
// Fill edits are applied in a second pass, after every other edit, since a
// flood fill needs the full set of ink boundaries already in place to know
// where to stop.
//
// cache and ver together memoize each fill's boundedness check across
// calls that share the same document version; cache may be nil to skip
// memoization (e.g. a one-off export).
func RasterizeDocument(edits []*Edit, cols, rows int, ox, oy, zoom float64, selectColor string, highlightID int, highlightColor string, cache FillBoundCache, ver int) *Raster {
	r := &Raster{Cols: cols, Rows: rows, cells: make([]cell, cols*rows)}
	pick := func(e *Edit) string {
		switch {
		case highlightID != 0 && e.ID == highlightID:
			return highlightColor
		case e.Selected:
			return selectColor
		default:
			return e.Color
		}
	}
	var fills []*Edit
	var nonFills []*Edit
	for _, e := range edits {
		if e.Kind == KindFill {
			fills = append(fills, e)
			continue
		}
		nonFills = append(nonFills, e)
		r.drawEdit(e, ox, oy, zoom, pick(e))
	}
	for _, e := range fills {
		if len(e.Points) == 0 {
			continue
		}
		// Re-validate boundedness on every render, not just at creation:
		// the enclosing shape can be edited or deleted after the fill
		// exists. This has to be independent of the current viewport (see
		// isFillBounded) — checking against the visible raster itself is
		// exactly the bug that made a valid fill disappear on zoom.
		bounded, ok := false, false
		if cache != nil {
			if entry, hit := cache[e.ID]; hit && entry.ver == ver {
				bounded, ok = entry.bounded, true
			}
		}
		if !ok {
			bounded = isFillBounded(nonFills, e.Points[0])
			if cache != nil {
				cache[e.ID] = fillBoundEntry{ver: ver, bounded: bounded}
			}
		}
		if bounded {
			r.floodFill(e, ox, oy, zoom, pick(e))
		}
	}
	return r
}

// fillProbeCols/fillProbeRows is the fixed cell budget spent on every
// boundedness check. fillProbeLevels are the (radius, zoom) windows tried,
// finest first: a single fixed-radius coarse probe (the original design)
// resolved a distant boundary fine, but for a small shape — the common
// case, since most drawings are far smaller than the world — its cells
// were wider than the whole shape, so the seed and the enclosing ink could
// land in the very same cell and the flood couldn't even get started (see
// isFillBounded). Starting fine and only widening the window when the
// flood actually runs off the edge of it gives small shapes the
// resolution they need while still falling back to the old coarse, wide
// probe for shapes bigger than the fine window.
var fillProbeLevels = [...]struct {
	radius, zoom float64
}{
	{radius: 128, zoom: 1},
	{radius: 2048, zoom: 0.125},
}

// isFillBounded reports whether a flood fill seeded at p is enclosed by ink
// from edits, regardless of the current viewport.
func isFillBounded(edits []*Edit, p Point) bool {
	for _, lvl := range fillProbeLevels {
		ox, oy := p.X-lvl.radius, p.Y-lvl.radius
		probe := &Raster{Cols: fillProbeCols, Rows: fillProbeRows, cells: make([]cell, fillProbeCols*fillProbeRows)}
		for _, e := range edits {
			probe.drawEdit(e, ox, oy, lvl.zoom, e.Color)
		}
		col := int(((p.X - ox) * lvl.zoom) / SubpixW)
		row := int(((p.Y - oy) * lvl.zoom) / SubpixH)
		cells, touchesEdge := probe.floodRegion(col, row)
		// touchesEdge means the flood ran off this window without finding
		// enclosing ink — could be genuinely open, or could just mean the
		// shape is bigger than this window. Escalate to the next, wider
		// level rather than concluding either way.
		if touchesEdge {
			continue
		}
		// len(cells) == 0 means the seed's own probe cell already reads as
		// ink, so floodRegion never even started — that's not necessarily
		// because the seed is genuinely on a line: the probe cell can
		// cover a wide swath of world space, so a seed a few world units
		// outside a thin wall can land in the very same cell as that
		// wall's dot and read as solid. Reading that as "bounded" was the
		// actual bug: it let a click just outside an object's edge — the
		// extremely common case of zooming in close to something and
		// clicking just past its border — create a fill that, rendered at
		// real resolution where the seed truly is empty, had nothing
		// stopping it and flooded the whole open background. Treat "the
		// probe couldn't even start" as unknown and escalate, rather than
		// as safe.
		if len(cells) > 0 {
			return true
		}
	}
	return false
}

const fillProbeCols, fillProbeRows = 256, 128

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
	// touchesEdge is deliberately ignored here: it's only meaningful at
	// creation time (see mouse.go), where it's checked against the
	// viewport the user actually clicked in. At render time, "the flood
	// reaches the current viewport's edge" is completely expected and
	// correct once you've zoomed in past the shape's boundary — the
	// enclosing ink is simply off-screen, not gone. Aborting the paint
	// here made a valid fill vanish the moment you zoomed in far enough.
	cells, _ := r.floodRegion(col, row)
	for _, c := range cells {
		r.at(c.col, c.row).bg = color
	}
}

// maxScreenSize clamps screen-space thickness: past this point a fatter
// brush is visually just a solid blob, but plotThick's cost grows with
// radius, so an unclamped size*zoom (e.g. a fat brush at 800% zoom) makes
// every point of every stroke drawn this frame far more expensive than the
// extra thickness is worth.
const maxScreenSize = 64

func screenSize(size, zoom float64) float64 {
	s := size * zoom
	if s > maxScreenSize {
		s = maxScreenSize
	}
	return s
}

func (r *Raster) drawEdit(e *Edit, ox, oy, zoom float64, color string) {
	pts := make([]Point, len(e.Points))
	for i, p := range e.Points {
		pts[i] = Point{X: (p.X - ox) * zoom, Y: (p.Y - oy) * zoom}
	}
	size := screenSize(e.Size, zoom)
	hardness := e.hardness()

	switch e.Kind {
	case KindStroke, KindLine:
		r.drawPolyline(pts, size, hardness, color)
	case KindArrow:
		r.drawArrow(pts, size, hardness, color)
	case KindRect:
		if e.Filled {
			r.fillRect(pts, color)
		} else {
			r.drawRect(pts, size, hardness, color)
		}
	case KindCircle:
		if e.Filled {
			r.fillEllipse(pts, color)
		} else {
			r.drawEllipse(pts, size, hardness, color)
		}
	case KindText:
		r.drawText(pts, e.Text, color)
	}
}

// appendStroke draws only the newly-added segments of a growing stroke
// (from index max(0,from-1), so the segment connecting to the
// already-drawn portion is included) directly onto this raster. It mirrors
// drawEdit's transform and clamp exactly, so incrementally appending is
// pixel-identical to a full rebuild — see canvasRaster in view.go.
func (r *Raster) appendStroke(pts []Point, from int, size, hardness, ox, oy, zoom float64, color string) {
	s := screenSize(size, zoom)
	transform := func(p Point) (float64, float64) {
		return (p.X - ox) * zoom, (p.Y - oy) * zoom
	}
	if len(pts) == 1 {
		x, y := transform(pts[0])
		r.plotThick(x, y, s, hardness, color)
		return
	}
	start := from - 1
	if start < 0 {
		start = 0
	}
	for i := start; i+1 < len(pts); i++ {
		x0, y0 := transform(pts[i])
		x1, y1 := transform(pts[i+1])
		r.drawSegment(x0, y0, x1, y1, s, hardness, color)
	}
}

// fillRect paints the background of every cell inside the rect's bounding
// box. Drawn before the outline stroke so the border stays crisp on top.
func (r *Raster) fillRect(pts []Point, color string) {
	if len(pts) < 2 {
		return
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	r.fillCellRange(x0, y0, x1, y1, color, func(float64, float64) bool { return true })
}

// fillEllipse paints the background of every cell whose center falls
// inside the ellipse inscribed in pts' bounding box.
func (r *Raster) fillEllipse(pts []Point, color string) {
	if len(pts) < 2 {
		return
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	rx, ry := (x1-x0)/2, (y1-y0)/2
	inside := func(x, y float64) bool {
		if rx == 0 || ry == 0 {
			return false
		}
		nx, ny := (x-cx)/rx, (y-cy)/ry
		return nx*nx+ny*ny <= 1
	}
	r.fillCellRange(x0, y0, x1, y1, color, inside)
}

// fillCellRange sets bg on every cell within the screen-space box
// [x0,y0]-[x1,y1] whose center passes the inside test.
func (r *Raster) fillCellRange(x0, y0, x1, y1 float64, color string, inside func(x, y float64) bool) {
	colFrom, colTo := int(x0/SubpixW), int(x1/SubpixW)
	rowFrom, rowTo := int(y0/SubpixH), int(y1/SubpixH)
	if colFrom < 0 {
		colFrom = 0
	}
	if rowFrom < 0 {
		rowFrom = 0
	}
	if colTo >= r.Cols {
		colTo = r.Cols - 1
	}
	if rowTo >= r.Rows {
		rowTo = r.Rows - 1
	}
	for row := rowFrom; row <= rowTo; row++ {
		for col := colFrom; col <= colTo; col++ {
			cx, cy := float64(col)*SubpixW+SubpixW/2, float64(row)*SubpixH+SubpixH/2
			if inside(cx, cy) {
				r.at(col, row).bg = color
			}
		}
	}
}

func (r *Raster) drawPolyline(pts []Point, size, hardness float64, color string) {
	if len(pts) == 1 {
		r.plotThick(pts[0].X, pts[0].Y, size, hardness, color)
		return
	}
	for i := 0; i+1 < len(pts); i++ {
		r.drawSegment(pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y, size, hardness, color)
	}
}

// arrowHeadAngle and arrowHeadFrac control the arrowhead's shape: the angle
// each barb makes with the shaft, and the barb length as a fraction of the
// shaft's own length (capped so a short arrow doesn't grow a head bigger
// than itself).
const arrowHeadAngle = 0.5 // radians, ~29°
const arrowHeadFrac = 0.3

func (r *Raster) drawArrow(pts []Point, size, hardness float64, color string) {
	if len(pts) < 2 {
		return
	}
	from, to := pts[0], pts[1]
	r.drawSegment(from.X, from.Y, to.X, to.Y, size, hardness, color)

	dx, dy := to.X-from.X, to.Y-from.Y
	shaftLen := math.Hypot(dx, dy)
	if shaftLen == 0 {
		return
	}
	headLen := shaftLen * arrowHeadFrac
	const maxHead, minHead = 24.0, 4.0
	if headLen > maxHead {
		headLen = maxHead
	}
	if headLen < minHead {
		headLen = minHead
	}
	angle := math.Atan2(dy, dx)
	for _, sign := range [2]float64{1, -1} {
		a := angle + math.Pi - sign*arrowHeadAngle
		bx := to.X + headLen*math.Cos(a)
		by := to.Y + headLen*math.Sin(a)
		r.drawSegment(to.X, to.Y, bx, by, size, hardness, color)
	}
}

func (r *Raster) drawRect(pts []Point, size, hardness float64, color string) {
	if len(pts) < 2 {
		return
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	corners := [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	for i := 0; i < 4; i++ {
		a, b := corners[i], corners[(i+1)%4]
		r.drawSegment(a[0], a[1], b[0], b[1], size, hardness, color)
	}
}

// drawEllipse draws the ellipse inscribed in the bounding box pts[0],pts[1],
// so dragging opposite corners freely stretches it into an oval.
func (r *Raster) drawEllipse(pts []Point, size, hardness float64, color string) {
	if len(pts) < 2 {
		return
	}
	cx, cy := (pts[0].X+pts[1].X)/2, (pts[0].Y+pts[1].Y)/2
	rx, ry := math.Abs(pts[1].X-pts[0].X)/2, math.Abs(pts[1].Y-pts[0].Y)/2
	// Sample count scales with screen-space radius so curves stay smooth,
	// but is capped to the viewport's own size: past that, most of the
	// ellipse is off-screen anyway, and an uncapped radius (a big circle
	// at high zoom) turned this into tens of thousands of drawSegment
	// calls per frame — the "circles + zoom get super slow" report.
	steps := int(math.Max(16, math.Min((rx+ry)*2, r.maxSteps())))
	prevX, prevY := cx+rx, cy
	for i := 1; i <= steps; i++ {
		t := 2 * math.Pi * float64(i) / float64(steps)
		x, y := cx+rx*math.Cos(t), cy+ry*math.Sin(t)
		r.drawSegment(prevX, prevY, x, y, size, hardness, color)
		prevX, prevY = x, y
	}
}

// maxSteps bounds any per-edit sampling loop to the viewport's own
// resolution: no drawing operation can usefully touch more subpixels than
// that in one pass, however far its geometry actually extends off-screen.
func (r *Raster) maxSteps() float64 {
	return float64(r.Cols*SubpixW+r.Rows*SubpixH) * 2
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
// splat at each step to give it width. The segment is first clipped to the
// (padded) viewport: without that, a shape whose endpoints are far
// off-screen — the near-inevitable case once you're zoomed into a small
// part of a big shape — makes steps enormous even though almost none of
// it is ever visible, which was the real source of both the zoomed-in
// slowdown and edges rendering while a huge fill's outline never finished.
func (r *Raster) drawSegment(x0, y0, x1, y1, size, hardness float64, color string) {
	pad := size + 2
	maxX, maxY := float64(r.Cols*SubpixW), float64(r.Rows*SubpixH)
	cx0, cy0, cx1, cy1, visible := clipSegment(x0, y0, x1, y1, -pad, -pad, maxX+pad, maxY+pad)
	if !visible {
		return
	}
	length := math.Hypot(cx1-cx0, cy1-cy0)
	if length == 0 {
		r.plotThick(cx0, cy0, size, hardness, color)
		return
	}
	// Splat spacing scales with the brush's *solid core* radius, not its
	// full radius: discs spaced up to a radius apart fully tile a hard
	// disc's capsule with no gaps (stepping every single subpixel was
	// redrawing nearly the same disc dozens of times over for a thick
	// brush — that's what made "thick brush + high zoom" so slow, cost
	// O(length * radius^2) instead of the O(length * radius) this
	// achieves), but a soft brush's actually-solid core is only
	// radius*hardness/100 wide. Spacing stamps by the full radius still
	// left gaps between each stamp's small core, so a point between two
	// stamp centers could fall in *both* neighbors' sparse outer bands
	// at once — alternating which one "won" along the path produced a
	// visible ░▒░▒ checkerboard the whole length of any dragged soft
	// stroke, rather than one continuous feathered edge. At hardness 100
	// coreRadius == radius, so this is exactly the original formula.
	coreRadius := (size / 2) * hardness / 100
	spacing := math.Max(coreRadius, 1) // never coarser than 1 subpixel, so thin strokes are unaffected
	steps := int(length / spacing)
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		r.plotThick(cx0+(cx1-cx0)*t, cy0+(cy1-cy0)*t, size, hardness, color)
	}
}

// clipSegment clips segment (x0,y0)-(x1,y1) to the box [minX,minY]-[maxX,maxY]
// using the Liang-Barsky algorithm. visible is false if the segment misses
// the box entirely.
func clipSegment(x0, y0, x1, y1, minX, minY, maxX, maxY float64) (cx0, cy0, cx1, cy1 float64, visible bool) {
	dx, dy := x1-x0, y1-y0
	tMin, tMax := 0.0, 1.0
	edges := [4]struct{ p, q float64 }{
		{-dx, x0 - minX},
		{dx, maxX - x0},
		{-dy, y0 - minY},
		{dy, maxY - y0},
	}
	for _, e := range edges {
		if e.p == 0 {
			if e.q < 0 {
				return 0, 0, 0, 0, false
			}
			continue
		}
		t := e.q / e.p
		if e.p < 0 {
			if t > tMax {
				return 0, 0, 0, 0, false
			}
			if t > tMin {
				tMin = t
			}
		} else {
			if t < tMin {
				return 0, 0, 0, 0, false
			}
			if t < tMax {
				tMax = t
			}
		}
	}
	return x0 + tMin*dx, y0 + tMin*dy, x0 + tMax*dx, y0 + tMax*dy, true
}

// hardnessGlyphs shades a soft brush's outer band from just-past-the-core
// (index 0, densest) out to the brush's radius (last index, sparsest
// before nothing) — the closest a terminal cell can get to alpha falloff
// without real transparency.
var hardnessGlyphs = []rune{'█', '▓', '▒', '░'}

// plotThick lights subpixels within radius size/2 of (x, y). At full
// hardness (100) this is exactly the original crisp braille-dot disc,
// pixel-for-pixel — anything softer switches to plotThickSoft, which
// operates at whole-cell granularity instead, since a cell can only show
// one glyph and braille's per-subpixel dots have no notion of "half
// covered".
func (r *Raster) plotThick(x, y, size, hardness float64, color string) {
	radius := size / 2
	if radius < 0.5 {
		radius = 0.5
	}
	if hardness >= 100 {
		r.plotThickHard(x, y, radius, color)
		return
	}
	r.plotThickSoft(x, y, radius, hardness, color)
}

func (r *Raster) plotThickHard(x, y, radius float64, color string) {
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

// glyphDensity returns g's index into hardnessGlyphs (0 = solid, higher =
// sparser) and whether g is actually one of them.
func glyphDensity(g rune) (level int, ok bool) {
	for i, hg := range hardnessGlyphs {
		if g == hg {
			return i, true
		}
	}
	return 0, false
}

// plotThickSoft renders a disc of the given hardness (0..100, exclusive
// of 100 — see plotThick) by walking whole terminal cells rather than
// subpixels: every cell within coreRadius (radius scaled by hardness)
// gets a solid block, and every cell beyond that out to radius gets one
// of hardnessGlyphs based on how far past the core it falls, fading out
// entirely past radius.
func (r *Raster) plotThickSoft(x, y, radius, hardness float64, color string) {
	coreRadius := radius * hardness / 100
	band := radius - coreRadius

	ccol, crow := int(math.Round(x/SubpixW)), int(math.Round(y/SubpixH))
	rc := int(math.Ceil(radius/SubpixW)) + 1
	rr := int(math.Ceil(radius/SubpixH)) + 1
	for dr := -rr; dr <= rr; dr++ {
		for dc := -rc; dc <= rc; dc++ {
			col, row := ccol+dc, crow+dr
			if col < 0 || row < 0 || col >= r.Cols || row >= r.Rows {
				continue
			}
			cx := float64(col)*SubpixW + SubpixW/2
			cy := float64(row)*SubpixH + SubpixH/2
			d := math.Hypot(cx-x, cy-y)
			if d > radius {
				continue
			}
			idx := 0
			if d > coreRadius {
				t := 1.0
				if band > 0 {
					t = (d - coreRadius) / band
				}
				idx = int(t * float64(len(hardnessGlyphs)))
				if idx >= len(hardnessGlyphs) {
					idx = len(hardnessGlyphs) - 1
				}
			}

			cell := r.at(col, row)
			// A soft brush is drawn as many overlapping disc stamps
			// along its path (see drawSegment); a cell near the middle
			// of the stroke sits close to one stamp's center (call for
			// a solid glyph) but only within a neighboring stamp's
			// outer band (call for a sparser one). Without this check,
			// whichever stamp happened to be processed last would win
			// regardless of which one was actually closer, which — since
			// stamps overlap heavily — meant most of the stroke's
			// interior flickered between densities in a checkerboard
			// instead of reading as one continuous solid core. Same
			// color only: a different edit or color legitimately
			// overwrites, same as everywhere else in this renderer.
			if existing, ok := glyphDensity(cell.glyph); ok && cell.color == color && existing < idx {
				continue
			}
			// A soft splat supersedes any braille dots this cell had —
			// mixing subpixel dots and a cell-wide glyph in the same
			// cell isn't representable, so the glyph wins.
			cell.dots = 0
			cell.color = color
			cell.glyph = hardnessGlyphs[idx]
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
