package main

import (
	"fmt"
	"math/rand"
	"testing"

	zone "github.com/lrstanley/bubblezone/v2"
)

func TestMain(m *testing.M) {
	zone.NewGlobal()
	m.Run()
}

// makeStrokes builds n strokes of pointsPerStroke points each, scattered
// across a viewport-sized area, mimicking what fast freehand drawing
// produces.
func makeStrokes(n, pointsPerStroke int) []*Edit {
	rng := rand.New(rand.NewSource(1))
	edits := make([]*Edit, n)
	for i := range edits {
		pts := make([]Point, pointsPerStroke)
		x, y := rng.Float64()*400, rng.Float64()*200
		for j := range pts {
			x += rng.Float64()*6 - 3
			y += rng.Float64()*6 - 3
			pts[j] = Point{X: x, Y: y}
		}
		edits[i] = &Edit{ID: i, Kind: KindStroke, Points: pts, Color: "#ffffff", Size: 1}
	}
	return edits
}

// BenchmarkRasterizeDocument covers the full per-frame rasterization cost
// (Raster.drawEdit -> drawPolyline -> drawSegment -> plotThick) at a
// typical terminal viewport size, across a range of edit counts.
func BenchmarkRasterizeDocument(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		edits := makeStrokes(n, 40)
		b.Run(fmt.Sprintf("edits=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				RasterizeDocument(edits, 160, 45, 0, 0, 1, "#ffaa00", 0, "")
			}
		})
	}
}

// BenchmarkViewCanvas covers the full render-to-terminal-string path,
// including the per-run lipgloss styling.
func BenchmarkViewCanvas(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		m := NewModel()
		m.width, m.height = 160, 48
		m.doc().Edits = makeStrokes(n, 40)
		b.Run(fmt.Sprintf("edits=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = m.viewCanvas()
			}
		})
	}
}

// BenchmarkEraseAt covers eraser hit-testing cost: one erase pass against
// every edit on the canvas.
func BenchmarkEraseAt(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("edits=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				m := NewModel()
				m.doc().Edits = makeStrokes(n, 40)
				m.erasedIDs = map[int]bool{}
				b.StartTimer()
				m.eraseAt(Point{X: 200, Y: 100})
			}
		})
	}
}

// BenchmarkEditDistance covers the hit-test math itself (select/eraser),
// isolated from slice churn.
func BenchmarkEditDistance(b *testing.B) {
	edits := makeStrokes(1, 40)
	e := edits[0]
	p := Point{X: 200, Y: 100}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e.Distance(p)
	}
}

// BenchmarkDocumentBeginChange covers the undo-snapshot cost paid once per
// discrete drawing action (mouse down), which must stay cheap even with a
// large canvas, since it happens on every stroke/shape/erase/move start.
func BenchmarkDocumentBeginChange(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("edits=%d", n), func(b *testing.B) {
			d := NewDocument()
			d.Edits = makeStrokes(n, 40)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.BeginChange()
			}
		})
	}
}

// BenchmarkDrawSegment isolates the inner rasterization loop (Bresenham
// walk + thick splat) used by every stroke, line, rect, and ellipse edge.
func BenchmarkDrawSegment(b *testing.B) {
	r := &Raster{Cols: 160, Rows: 45, cells: make([]cell, 160*45)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.drawSegment(0, 0, 300, 150, 1, "#ffffff")
	}
}

// TestCanvasRasterCacheHit verifies pure cursor movement (no edits/pan/zoom
// change) reuses the same Raster instead of re-rasterizing, which is the
// actual fix for the high-zoom cursor lag.
func TestCanvasRasterCacheHit(t *testing.T) {
	m := NewModel()
	m.width, m.height = 160, 48
	m.doc().Edits = makeStrokes(200, 40)

	r1 := m.canvasRaster(160, 40, m.doc(), 1)
	m.cursorCol, m.cursorRow = 10, 10
	r2 := m.canvasRaster(160, 40, m.doc(), 1)
	if r1 != r2 {
		t.Fatal("expected cached raster to be reused when only the cursor moved")
	}

	m.doc().Edits[0].Points[0].X += 1
	m.doc().Touch()
	r3 := m.canvasRaster(160, 40, m.doc(), 1)
	if r3 == r1 {
		t.Fatal("expected a rebuilt raster after a real edit change")
	}
}

// BenchmarkCanvasRasterCached measures the cache-hit path cost (cursor
// motion only), which should be tiny regardless of edit count or zoom,
// unlike a full RasterizeDocument call.
func BenchmarkCanvasRasterCached(b *testing.B) {
	m := NewModel()
	m.width, m.height = 160, 48
	m.doc().Edits = makeStrokes(1000, 40)
	m.doc().Zoom = 8
	m.canvasRaster(160, 40, m.doc(), 8)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.canvasRaster(160, 40, m.doc(), 8)
	}
}
