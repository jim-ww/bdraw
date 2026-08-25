package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// edgePanMsg fires on a recurring timer, same self-sustaining pattern as
// autosaveMsg: handling it reschedules the next one.
type edgePanMsg struct{}

func edgePanTick() tea.Cmd {
	return tea.Tick(edgePanInterval, func(time.Time) tea.Msg { return edgePanMsg{} })
}

const (
	// edgePanInterval is how often the canvas nudges while the cursor
	// sits at the viewport edge mid-drag — frequent enough to feel like
	// continuous scrolling, not so frequent it re-rasterizes needlessly
	// while idle (the common case: this ticks for the whole app
	// lifetime, but almost every tick is a no-op check).
	edgePanInterval = 60 * time.Millisecond

	// edgePanMarginCells is how close to the viewport's edge (in whole
	// terminal cells) the cursor has to be to trigger panning.
	edgePanMarginCells = 2

	// edgePanCellsPerTick is how many screen cells' worth of world
	// distance to pan per tick, converted through the current zoom the
	// same way a manual middle-drag pan is (see handlePan) — so the
	// visual scroll speed feels the same regardless of zoom level,
	// rather than a fixed world-subpixel step going nearly unnoticeable
	// at high zoom or wildly fast at low zoom.
	edgePanCellsPerTick = 1.0
)

// edgePanDelta returns the world-subpixel offset change to apply this
// tick, and whether the cursor is currently close enough to a viewport
// edge to warrant any panning at all.
func (m Model) edgePanDelta() (dx, dy float64, ok bool) {
	cols, rows := m.canvasSize()
	if cols <= 0 || rows <= 0 || !m.cursorVisible {
		return 0, 0, false
	}
	zoom := m.viewZoom()
	step := edgePanCellsPerTick * SubpixW / zoom
	stepY := edgePanCellsPerTick * SubpixH / zoom

	switch {
	case m.cursorCol < edgePanMarginCells:
		dx = -step
	case m.cursorCol >= cols-edgePanMarginCells:
		dx = step
	}
	switch {
	case m.cursorRow < edgePanMarginCells:
		dy = -stepY
	case m.cursorRow >= rows-edgePanMarginCells:
		dy = stepY
	}
	return dx, dy, dx != 0 || dy != 0
}

// handleEdgePan is called from Update on every edgePanMsg tick. Whenever
// the cursor is sitting at the viewport's edge — hovering is enough, no
// button needs to be held — it pans the view, the same way an RTS
// camera scrolls when the mouse rests at the screen edge. If a drag also
// happens to be in progress, it additionally re-drives that drag with
// the world point now under the (screen-stationary) cursor — exactly
// what a real mouse-motion event would do, except the physical mouse
// hasn't actually moved, so no such event exists on its own. Without
// that second part, panning while dragging would just slide the canvas
// out from under a stroke/selection that never actually extends past
// whatever was visible when the cursor first reached the edge.
func (m Model) handleEdgePan() (tea.Model, tea.Cmd) {
	if m.cfg.DisableEdgePan {
		return m, edgePanTick()
	}
	dx, dy, ok := m.edgePanDelta()
	if !ok {
		return m, edgePanTick()
	}
	offset := m.viewOffset()
	offset.X += dx
	offset.Y += dy
	m.setViewOffset(offset)

	if !m.dragging {
		return m, edgePanTick()
	}

	newModel, cmd := m.collabWrap(func(mm Model) (tea.Model, tea.Cmd) {
		return mm.toolDrag(mm.cellToPoint(mm.cursorCol, mm.cursorRow), mm.dragConstrain)
	})
	nm := newModel.(Model)
	return nm, tea.Batch(cmd, edgePanTick())
}
