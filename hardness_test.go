package main

import "testing"

// TestHardnessDefaultMatchesHistoricalBraille verifies the zero-value/
// unset Edit.Hardness (as every file saved before this feature existed
// unmarshals to) renders through the exact same crisp braille-dot path
// as before — no glyph override anywhere in the stroke.
func TestHardnessDefaultMatchesHistoricalBraille(t *testing.T) {
	e := &Edit{ID: 1, Kind: KindStroke, Points: []Point{{X: 30, Y: 30}}, Color: "#fff", Size: 20}
	r := RasterizeDocument([]*Edit{e}, 30, 15, 0, 0, 1, "", 0, "")
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
	r := RasterizeDocument([]*Edit{e}, 30, 15, 0, 0, 1, "", 0, "")

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

// TestHardnessZeroTreatedAsFull covers the file-compatibility sentinel:
// Edit.Hardness's zero value (what an old save file unmarshals to, since
// the field didn't exist) must mean fully hard, not 0% (invisible).
func TestHardnessZeroTreatedAsFull(t *testing.T) {
	e := &Edit{Hardness: 0}
	if got := e.hardness(); got != 100 {
		t.Fatalf("zero-value Hardness should mean fully hard (100), got %v", got)
	}
}
