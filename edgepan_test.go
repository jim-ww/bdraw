package main

import "testing"

// TestEdgePanDeltaTriggersNearEdges checks edgePanDelta only fires within
// edgePanMarginCells of a viewport edge, in the expected direction.
func TestEdgePanDeltaTriggersNearEdges(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 40, 20
	m.cursorVisible = true

	cols, rows := m.canvasSize()

	m.cursorCol, m.cursorRow = 0, rows/2
	if dx, _, ok := m.edgePanDelta(); !ok || dx >= 0 {
		t.Fatalf("expected a leftward pan at the left edge, got dx=%v ok=%v", dx, ok)
	}

	m.cursorCol, m.cursorRow = cols-1, rows/2
	if dx, _, ok := m.edgePanDelta(); !ok || dx <= 0 {
		t.Fatalf("expected a rightward pan at the right edge, got dx=%v ok=%v", dx, ok)
	}

	m.cursorCol, m.cursorRow = cols/2, rows/2
	if _, _, ok := m.edgePanDelta(); ok {
		t.Fatal("expected no pan in the middle of the viewport")
	}
}

// TestEdgePanExtendsDragAcrossPan is the core behavior check: panning
// alone is useless for an in-progress drag if the drag's endpoint never
// follows along, since the physical mouse hasn't actually moved — the
// whole point of edge-pan is to let a drag reach content currently
// off-screen.
func TestEdgePanExtendsDragAcrossPan(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 40, 20
	m.tool = ToolLine
	m.cursorVisible = true

	cols, _ := m.canvasSize()
	m.cursorCol, m.cursorRow = cols - 1, 5 // sit at the right edge

	pt := m.cellToPoint(m.cursorCol, m.cursorRow)
	newModel, _ := m.toolDown(pt)
	m = newModel.(Model)
	if !m.dragging {
		t.Fatal("expected a drag to have started")
	}
	before := m.dragEdit.Points[1]

	newModel, _ = m.handleEdgePan()
	m = newModel.(Model)

	after := m.dragEdit.Points[1]
	if after == before {
		t.Fatal("expected the in-progress drag's endpoint to move along with the edge-pan, not stay fixed under a stationary cursor")
	}
}

// TestEdgePanHoverAloneTriggers checks edge-pan fires from the cursor
// merely resting at the viewport edge, with no button held and no drag
// in progress — like an RTS camera, not gated on an active drag.
func TestEdgePanHoverAloneTriggers(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 40, 20
	m.cursorVisible = true
	if m.dragging {
		t.Fatal("test setup: expected no drag in progress")
	}

	cols, _ := m.canvasSize()
	m.cursorCol, m.cursorRow = cols - 1, 5
	before := m.viewOffset()

	newModel, _ := m.handleEdgePan()
	m = newModel.(Model)

	if m.viewOffset() == before {
		t.Fatal("expected hovering at the edge alone (no drag, no button held) to pan the view")
	}
}

// TestEdgePanDisabledByConfig checks the DisableEdgePan config flag is
// actually honored.
func TestEdgePanDisabledByConfig(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 40, 20
	m.cfg.DisableEdgePan = true
	m.tool = ToolLine
	m.cursorVisible = true

	cols, _ := m.canvasSize()
	m.cursorCol, m.cursorRow = cols - 1, 5

	pt := m.cellToPoint(m.cursorCol, m.cursorRow)
	newModel, _ := m.toolDown(pt)
	m = newModel.(Model)
	before := m.viewOffset()

	newModel, _ = m.handleEdgePan()
	m = newModel.(Model)

	if m.viewOffset() != before {
		t.Fatal("DisableEdgePan should prevent any panning")
	}
}
