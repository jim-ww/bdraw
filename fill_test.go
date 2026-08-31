package main

import "testing"

// TestFillZoomedNearEdgeRejected reproduces the reported bug: zooming in
// close to an object and clicking fill just outside its border wrongly
// treated it as enclosed, since the creation-time boundedness probe is
// deliberately coarse (for performance) and a seed a few world units
// outside a thin wall can land in the very same probe cell as that wall's
// dot, reading the whole cell as solid ink. That made floodRegion return
// zero cells with touchesEdge=false ("nothing to flood" — vacuously
// looked bounded), which isFillBounded read as "safe" instead of
// "unknown, and unknown must not mean yes."
func TestFillZoomedNearEdgeRejected(t *testing.T) {
	edits := []*Edit{
		{ID: 1, Kind: KindLine, Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}, Color: "#fff", Size: 1},
		{ID: 2, Kind: KindLine, Points: []Point{{X: 100, Y: 0}, {X: 100, Y: 100}}, Color: "#fff", Size: 1},
		{ID: 3, Kind: KindLine, Points: []Point{{X: 100, Y: 100}, {X: 0, Y: 100}}, Color: "#fff", Size: 1},
		{ID: 4, Kind: KindLine, Points: []Point{{X: 0, Y: 100}, {X: 0, Y: 0}}, Color: "#fff", Size: 1},
	}
	if !isFillBounded(edits, Point{X: 50, Y: 50}) {
		t.Error("inside a genuinely enclosed room should still be fillable")
	}
	if isFillBounded(edits, Point{X: -5, Y: 50}) {
		t.Error("clicking just outside a wall (as when zoomed in close) must not be treated as bounded")
	}
	if isFillBounded(edits, Point{X: 105, Y: -5}) {
		t.Error("clicking just outside a corner must not be treated as bounded")
	}
}

// TestFillBoundedSmallShape reproduces the reported "can't fill" bug: the
// boundedness probe used a single fixed-radius, coarse-resolution window
// sized for large shapes, so a small (and very ordinary) enclosed shape's
// walls were thinner than one probe cell and the flood couldn't even get
// started, reading a genuinely enclosed shape as unbounded.
func TestFillBoundedSmallShape(t *testing.T) {
	rect := &Edit{ID: 1, Kind: KindRect, Points: []Point{{X: 0, Y: 0}, {X: 20, Y: 20}}, Color: "#fff", Size: 1}
	if !isFillBounded([]*Edit{rect}, Point{X: 10, Y: 10}) {
		t.Error("small enclosed rectangle should still be fillable")
	}
}

// TestFillBoundCacheReusedAcrossSameVersion checks that isFillBounded's
// (relatively expensive) probe is skipped on a second render at the same
// document version — e.g. while just panning or zooming, not editing —
// and is correctly redone once the version changes.
func TestFillBoundCacheReusedAcrossSameVersion(t *testing.T) {
	rect := &Edit{ID: 1, Kind: KindRect, Points: []Point{{X: 0, Y: 0}, {X: 200, Y: 200}}, Color: "#ffffff", Size: 1}
	fill := &Edit{ID: 2, Kind: KindFill, Points: []Point{{X: 100, Y: 100}}, Color: "#ff0000"}
	edits := []*Edit{rect, fill}
	cache := FillBoundCache{}

	RasterizeDocument(edits, 100, 40, 0, 0, 1, "", 0, "", cache, 1)
	entry, ok := cache[fill.ID]
	if !ok || entry.ver != 1 || !entry.bounded {
		t.Fatalf("expected a cached bounded=true entry at ver 1, got %+v (ok=%v)", entry, ok)
	}

	// Same version, different viewport (pan/zoom): the cached verdict
	// should carry over unchanged rather than being recomputed.
	RasterizeDocument(edits, 100, 40, 50, 50, 2, "", 0, "", cache, 1)
	if cache[fill.ID] != entry {
		t.Fatal("cache entry should not change across renders at the same document version")
	}

	// New version: must be recomputed, not served stale.
	edits = []*Edit{fill} // boundary removed
	RasterizeDocument(edits, 100, 40, 0, 0, 1, "", 0, "", cache, 2)
	if got := cache[fill.ID]; got.ver != 2 || got.bounded {
		t.Fatalf("expected recomputed bounded=false entry at ver 2, got %+v", got)
	}
}

func countFillCells(r *Raster, cols, rows int) int {
	n := 0
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			if r.Background(col, row) != "" {
				n++
			}
		}
	}
	return n
}

// TestFillSurvivesZoomIn checks the earlier fix still holds: zooming in
// past a fill's enclosing boundary (so the boundary itself is off-screen)
// must not make the fill disappear.
func TestFillSurvivesZoomIn(t *testing.T) {
	rect := &Edit{ID: 1, Kind: KindRect, Points: []Point{{X: 0, Y: 0}, {X: 200, Y: 200}}, Color: "#ffffff", Size: 1}
	fill := &Edit{ID: 2, Kind: KindFill, Points: []Point{{X: 100, Y: 100}}, Color: "#ff0000"}
	r := RasterizeDocument([]*Edit{rect, fill}, 100, 40, 90, 90, 8, "", 0, "", nil, 0)
	if countFillCells(r, 100, 40) == 0 {
		t.Fatal("fill disappeared after zooming past the boundary")
	}
}

// TestFillDoesNotSpillWhenBoundaryRemoved checks the other earlier fix:
// if a fill's enclosing shape is deleted after the fill exists, the fill
// must not spill across the whole (now-open) canvas.
func TestFillDoesNotSpillWhenBoundaryRemoved(t *testing.T) {
	fill := &Edit{ID: 2, Kind: KindFill, Points: []Point{{X: 100, Y: 100}}, Color: "#ff0000"}
	r := RasterizeDocument([]*Edit{fill}, 100, 40, 0, 0, 1, "", 0, "", nil, 0)
	if n := countFillCells(r, 100, 40); n != 0 {
		t.Fatalf("fill spilled across the whole viewport after boundary removed: %d cells", n)
	}
}
