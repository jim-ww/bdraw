package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func room() []*Edit {
	return []*Edit{
		{ID: 1, Kind: KindLine, Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}, Color: "#fff", Size: 1},
		{ID: 2, Kind: KindLine, Points: []Point{{X: 100, Y: 0}, {X: 100, Y: 100}}, Color: "#fff", Size: 1},
		{ID: 3, Kind: KindLine, Points: []Point{{X: 100, Y: 100}, {X: 0, Y: 100}}, Color: "#fff", Size: 1},
		{ID: 4, Kind: KindLine, Points: []Point{{X: 0, Y: 100}, {X: 0, Y: 0}}, Color: "#fff", Size: 1},
	}
}

// TestFillNearEdge checks that clicking just outside a wall, as when zoomed
// in close, is not treated as enclosed.
func TestFillNearEdge(t *testing.T) {
	edits := room()
	if fillAreaAt(edits, Point{X: 50, Y: 50}) == nil {
		t.Error("inside a genuinely enclosed room should be fillable")
	}
	if fillAreaAt(edits, Point{X: -5, Y: 50}) != nil {
		t.Error("clicking just outside a wall must not be treated as bounded")
	}
	if fillAreaAt(edits, Point{X: 105, Y: -5}) != nil {
		t.Error("clicking just outside a corner must not be treated as bounded")
	}
}

func TestFillSmallAndLargeShapes(t *testing.T) {
	small := &Edit{ID: 1, Kind: KindRect, Points: []Point{{X: 0, Y: 0}, {X: 20, Y: 20}}, Color: "#fff", Size: 1}
	if fillAreaAt([]*Edit{small}, Point{X: 10, Y: 10}) == nil {
		t.Error("small enclosed rectangle should be fillable")
	}
	large := &Edit{ID: 1, Kind: KindRect, Points: []Point{{X: 0, Y: 0}, {X: 1500, Y: 1000}}, Color: "#fff", Size: 1}
	a := fillAreaAt([]*Edit{large}, Point{X: 750, Y: 500})
	if a == nil || !a.contains(Point{X: 20, Y: 20}) || a.contains(Point{X: -20, Y: 20}) {
		t.Error("large rectangle not filled to its border")
	}
	ring := &Edit{ID: 1, Kind: KindCircle, Points: []Point{{X: 0, Y: 0}, {X: 1200, Y: 900}}, Color: "#fff", Size: 1}
	if a := fillAreaAt([]*Edit{ring}, Point{X: 600, Y: 450}); a == nil || !a.contains(Point{X: 1150, Y: 450}) {
		t.Error("large ellipse not filled to its border")
	}
}

func countFillCells(r *Raster, cols, rows int) int {
	n := 0
	for row := range rows {
		for col := range cols {
			if r.Background(col, row) != "" {
				n++
			}
		}
	}
	return n
}

func filledRoom(t *testing.T) (edits []*Edit, fill *Edit) {
	t.Helper()
	edits = room()
	fill = &Edit{ID: 5, Kind: KindFill, Points: []Point{{X: 50, Y: 50}}, Color: "#ff0000"}
	fill.Area = fillAreaAt(edits, fill.Points[0])
	if fill.Area == nil {
		t.Fatal("room not fillable")
	}
	return append(edits, fill), fill
}

// TestFillKeepsAreaWhenSplit: drawing a line across a filled shape must not
// unfill the part on the far side of it.
func TestFillKeepsAreaWhenSplit(t *testing.T) {
	edits, _ := filledRoom(t)
	before := countFillCells(RasterizeDocument(edits, 60, 30, 0, 0, 1, "", 0, ""), 60, 30)
	split := &Edit{ID: 6, Kind: KindLine, Points: []Point{{X: 50, Y: -10}, {X: 50, Y: 110}}, Color: "#fff", Size: 1}
	r := RasterizeDocument(append(edits, split), 60, 30, 0, 0, 1, "", 0, "")
	if r.Background(40, 12) == "" {
		t.Error("fill lost the part cut off by the new line")
	}
	if after := countFillCells(r, 60, 30); after != before {
		t.Errorf("fill cells changed from %d to %d", before, after)
	}
}

// TestFillStaysWhenBoundaryRemoved: erasing the border keeps the fill as it
// was, without spilling across the canvas.
func TestFillStaysWhenBoundaryRemoved(t *testing.T) {
	edits, fill := filledRoom(t)
	with := countFillCells(RasterizeDocument(edits, 100, 40, -10, -10, 1, "", 0, ""), 100, 40)
	without := countFillCells(RasterizeDocument([]*Edit{fill}, 100, 40, -10, -10, 1, "", 0, ""), 100, 40)
	if without == 0 || without != with {
		t.Fatalf("fill cells with border %d, without %d; want equal and nonzero", with, without)
	}
}

// TestFillSurvivesZoomIn: zooming in past a fill's border keeps it visible.
func TestFillSurvivesZoomIn(t *testing.T) {
	edits, _ := filledRoom(t)
	r := RasterizeDocument(edits, 100, 40, 45, 45, 8, "", 0, "")
	if countFillCells(r, 100, 40) != 100*40 {
		t.Fatal("fill not covering a viewport zoomed in inside it")
	}
}

func TestFillMovesWithArea(t *testing.T) {
	_, fill := filledRoom(t)
	fill.Translate(1000, 0)
	if !fill.Area.contains(Point{X: 1050, Y: 50}) || fill.Area.contains(Point{X: 50, Y: 50}) {
		t.Error("area did not move with the fill")
	}
}

func TestFillAreaJSON(t *testing.T) {
	_, fill := filledRoom(t)
	data, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	var got Edit
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Area.Origin != fill.Area.Origin || got.Area.Scale != fill.Area.Scale || !bytes.Equal(got.Area.Mask.Pix, fill.Area.Mask.Pix) {
		t.Error("area changed in JSON round trip")
	}
}

// TestLoadBakesOldFills: files saved before areas were stored get one.
func TestLoadBakesOldFills(t *testing.T) {
	edits := room()
	edits = append(edits, &Edit{ID: 5, Kind: KindFill, Points: []Point{{X: 50, Y: 50}}, Color: "#ff0000"})
	data, err := json.Marshal(fileFormat{Version: CurrentFileVersion, Edits: edits})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "old.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if a := d.Edits[4].Area; a == nil || !a.contains(Point{X: 10, Y: 10}) {
		t.Error("old fill has no area after load")
	}
}
