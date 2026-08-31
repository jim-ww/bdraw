package main

import "testing"

// TestHardnessDefaultMatchesHistoricalBraille verifies the zero-value/
// unset Edit.Hardness (as every file saved before this feature existed
// unmarshals to) renders through the exact same crisp braille-dot path
// as before — no glyph override anywhere in the stroke.
func TestHardnessDefaultMatchesHistoricalBraille(t *testing.T) {
	e := &Edit{ID: 1, Kind: KindStroke, Points: []Point{{X: 30, Y: 30}}, Color: "#fff", Size: 20}
	r := RasterizeDocument([]*Edit{e}, 30, 15, 0, 0, 1, "", 0, "", nil, 0)
	for row := 0; row < 15; row++ {
		for col := 0; col < 30; col++ {
			if r.at(col, row).glyph != 0 {
				t.Fatalf("default hardness must never set a glyph override, found one at (%d,%d)", col, row)
			}
		}
	}
}

// TestHardnessSoftEdgeFeathers checks a soft brush has a solid core and
// a feathered outer band using the density glyphs, unlike the crisp
// hard-edge disc.
func TestHardnessSoftEdgeFeathers(t *testing.T) {
	e := &Edit{ID: 1, Kind: KindStroke, Points: []Point{{X: 30, Y: 30}}, Color: "#fff", Size: 20, Hardness: 30}
	r := RasterizeDocument([]*Edit{e}, 30, 15, 0, 0, 1, "", 0, "", nil, 0)

	center := r.at(15, 7)
	if center.glyph != '█' {
		t.Fatalf("brush center should be the solid glyph, got %q", center.glyph)
	}

	var sawFeather bool
	for row := 0; row < 15; row++ {
		for col := 0; col < 30; col++ {
			g := r.at(col, row).glyph
			for _, feather := range hardnessGlyphs[1:] {
				if g == feather {
					sawFeather = true
				}
			}
		}
	}
	if !sawFeather {
		t.Fatal("expected at least one feathered (non-solid) glyph in a soft brush's outer band")
	}
}

// TestHardnessDraggedStrokeHasNoCheckerboard is the regression test for a
// real rendering bug: a soft brush is drawn as many overlapping disc
// stamps along its drag path, and a cell near the middle of the stroke
// sits close to one stamp's center (solid) but only within a neighboring
// stamp's sparser outer band. Without keeping the densest classification
// any stamp assigned a cell, whichever stamp got processed last won
// regardless of which was actually closer — since stamps overlap
// heavily, nearly the whole interior of a dragged stroke alternated
// between densities in a visible checkerboard instead of reading as one
// continuous solid core with a feathered edge.
func TestHardnessDraggedStrokeHasNoCheckerboard(t *testing.T) {
	e := &Edit{ID: 1, Kind: KindStroke, Points: []Point{{X: 10, Y: 30}, {X: 150, Y: 30}}, Color: "#fff", Size: 20, Hardness: 40}
	r := RasterizeDocument([]*Edit{e}, 80, 15, 0, 0, 1, "", 0, "", nil, 0)

	// Well clear of both endpoints, the interior of the capsule's core
	// row should be solid the entire way across — not alternating with
	// sparser glyphs.
	for col := 10; col < 70; col++ {
		if g := r.at(col, 7).glyph; g != '█' {
			t.Fatalf("expected a solid core all the way along the stroke's interior, got %q at col %d", g, col)
		}
	}
}

// TestHardnessZeroTreatedAsFull covers the file-compatibility sentinel:
// Edit.Hardness's zero value (what an old save file unmarshals to, since
// the field didn't exist) must mean fully hard, not 0% (invisible).
func TestHardnessZeroTreatedAsFull(t *testing.T) {
	e := &Edit{Hardness: 0}
	if got := e.hardness(); got != 100 {
		t.Fatalf("zero-value Hardness should mean fully hard (100), got %v", got)
	}
}
