package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
)

// FillArea is the area a fill edit paints: a mask placed in world space.
// It is computed once, when the fill is made, so ink drawn across it later
// doesn't change it, and erasing its border doesn't remove it. Masks are
// never mutated, so clones share them.
type FillArea struct {
	Origin Point        // world position of the mask's top-left corner
	Scale  float64      // mask pixels per world unit
	Mask   *image.Alpha // bounds start at (0, 0)
}

// contains reports whether world point p lies inside the area.
func (a *FillArea) contains(p Point) bool {
	x := int(math.Floor((p.X - a.Origin.X) * a.Scale))
	y := int(math.Floor((p.Y - a.Origin.Y) * a.Scale))
	q := image.Pt(x, y)
	return q.In(a.Mask.Rect) && a.Mask.Pix[a.Mask.PixOffset(x, y)] != 0
}

// bounds returns the world rectangle the mask covers.
func (a *FillArea) bounds() (lo, hi Point) {
	r := a.Mask.Rect
	return a.Origin, Point{X: a.Origin.X + float64(r.Dx())/a.Scale, Y: a.Origin.Y + float64(r.Dy())/a.Scale}
}

type fillAreaJSON struct {
	Origin Point   `json:"origin"`
	Scale  float64 `json:"scale"`
	Mask   []byte  `json:"mask"` // grayscale PNG
}

func (a *FillArea) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	gray := &image.Gray{Pix: a.Mask.Pix, Stride: a.Mask.Stride, Rect: a.Mask.Rect}
	if err := png.Encode(&buf, gray); err != nil {
		return nil, err
	}
	return json.Marshal(fillAreaJSON{a.Origin, a.Scale, buf.Bytes()})
}

func (a *FillArea) UnmarshalJSON(data []byte) error {
	var j fillAreaJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Scale <= 0 {
		return fmt.Errorf("fill mask: invalid scale %v", j.Scale)
	}
	img, err := png.Decode(bytes.NewReader(j.Mask))
	if err != nil {
		return fmt.Errorf("fill mask: %w", err)
	}
	b := img.Bounds()
	m := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	if g, ok := img.(*image.Gray); ok {
		for y := range b.Dy() {
			copy(m.Pix[y*m.Stride:(y+1)*m.Stride], g.Pix[y*g.Stride:])
		}
	} else {
		draw.Draw(m, m.Rect, img, b.Min, draw.Src)
	}
	*a = FillArea{Origin: j.Origin, Scale: j.Scale, Mask: m}
	return nil
}

// fillLevels are the probe zooms a fill area is computed at, finest first.
// Each probe covers the same number of dots around the seed, so coarser
// levels reach larger enclosed areas.
var fillLevels = [...]float64{4, 1, 0.125}

const fillProbeCols, fillProbeRows = 256, 128

// fillAreaAt floods the area around p enclosed by the ink of edits, at
// braille-dot resolution. It returns nil if p is on ink or the area isn't
// enclosed.
func fillAreaAt(edits []*Edit, p Point) *FillArea {
	w, h := fillProbeCols*SubpixW, fillProbeRows*SubpixH
	for _, z := range fillLevels {
		ox, oy := p.X-float64(w)/2/z, p.Y-float64(h)/2/z
		probe := &Raster{Cols: fillProbeCols, Rows: fillProbeRows, cells: make([]cell, fillProbeCols*fillProbeRows)}
		for _, e := range edits {
			if e.Kind != KindFill {
				probe.drawEdit(e, ox, oy, z, e.Color)
			}
		}
		region, box, enclosed := probe.floodDots(image.Pt(w/2, h/2))
		if region == nil {
			return nil
		}
		if !enclosed {
			continue
		}
		m := image.NewAlpha(image.Rect(0, 0, box.Dx(), box.Dy()))
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				if region[y*w+x] {
					m.Pix[m.PixOffset(x-box.Min.X, y-box.Min.Y)] = 255
				}
			}
		}
		origin := Point{X: ox + float64(box.Min.X)/z, Y: oy + float64(box.Min.Y)/z}
		return &FillArea{Origin: origin, Scale: z, Mask: m}
	}
	return nil
}

// inkDot reports whether braille dot (x, y) of r is covered by ink.
func (r *Raster) inkDot(x, y int) bool {
	c := r.at(x/SubpixW, y/SubpixH)
	return c.glyph != 0 || c.dots&brailleBit[x%SubpixW][y%SubpixH] != 0
}

// floodDots returns the 4-connected inkless dots reachable from seed, their
// bounding box, and whether they stay clear of the raster's border. region
// is nil if seed itself is ink.
func (r *Raster) floodDots(seed image.Point) (region []bool, box image.Rectangle, enclosed bool) {
	w, h := r.Cols*SubpixW, r.Rows*SubpixH
	if r.inkDot(seed.X, seed.Y) {
		return nil, box, false
	}
	region = make([]bool, w*h)
	region[seed.Y*w+seed.X] = true
	box = image.Rectangle{Min: seed, Max: seed.Add(image.Pt(1, 1))}
	stack := []image.Point{seed}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p.X == 0 || p.Y == 0 || p.X == w-1 || p.Y == h-1 {
			return region, box, false
		}
		box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		for _, q := range [4]image.Point{{p.X + 1, p.Y}, {p.X - 1, p.Y}, {p.X, p.Y + 1}, {p.X, p.Y - 1}} {
			if i := q.Y*w + q.X; !region[i] && !r.inkDot(q.X, q.Y) {
				region[i] = true
				stack = append(stack, q)
			}
		}
	}
	return region, box, true
}

// paintArea sets the background of every cell whose center lies in a.
func (r *Raster) paintArea(a *FillArea, ox, oy, zoom float64, color string) {
	lo, hi := a.bounds()
	cw, ch := SubpixW/zoom, SubpixH/zoom // world size of one cell
	col0 := max(0, int(math.Floor((lo.X-ox)/cw)))
	row0 := max(0, int(math.Floor((lo.Y-oy)/ch)))
	col1 := min(r.Cols, int(math.Ceil((hi.X-ox)/cw)))
	row1 := min(r.Rows, int(math.Ceil((hi.Y-oy)/ch)))
	for row := row0; row < row1; row++ {
		for col := col0; col < col1; col++ {
			if a.contains(Point{X: ox + (float64(col)+0.5)*cw, Y: oy + (float64(row)+0.5)*ch}) {
				r.at(col, row).bg = color
			}
		}
	}
}

// bakeFills computes the areas of fills saved before areas were stored.
func bakeFills(edits []*Edit) {
	for _, e := range edits {
		if e.Kind == KindFill && e.Area == nil && len(e.Points) > 0 {
			e.Area = fillAreaAt(edits, e.Points[0])
		}
	}
}
