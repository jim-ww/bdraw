package main

import (
	"math"

	tea "charm.land/bubbletea/v2"
)

// eraserRadius returns how close (in world subpixels) the eraser must pass
// to an edit to remove it, scaled with the current brush size.
func eraserRadius(size float64) float64 {
	r := size * 4
	if r < 8 {
		r = 8
	}
	return r
}

// selectTolerance is the click tolerance, in world subpixels, for the
// select and move tools.
const selectTolerance = 6

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.MouseMotionMsg); ok {
		m.hoverZone = m.zoneAt(msg)
	}

	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		return m.handleWheel(wheel)
	}

	// An active pan-drag takes priority over zone hit-testing: once
	// started, motion/release must keep driving it even if the drag path
	// crosses a toolbar zone — otherwise a pan that passes near the top of
	// the canvas silently stalls, and a release that lands on a zone would
	// leave m.panning stuck true, breaking every mouse action after it.
	if m.panning {
		if col, row, ok := m.canvasCell(msg); ok {
			return m.handlePan(msg, col, row)
		}
		if _, ok := msg.(tea.MouseReleaseMsg); ok {
			m.panning = false
		}
		return m, nil
	}

	// A release landing on a toolbar zone (or anywhere outside the canvas)
	// mid-drag must still finish the drag: MouseReleaseMsg isn't a
	// MouseClickMsg, so it never matched the zone-click branch below, and
	// — since only a MouseClickMsg or an actual canvas release ever
	// called toolUp — that left m.dragging stuck true forever, extending
	// whatever was being drawn on every later motion event regardless of
	// tool. A release that *is* over the canvas still takes the normal
	// path below, using its own exact position.
	if _, ok := msg.(tea.MouseReleaseMsg); ok && m.dragging {
		if _, _, overCanvas := m.canvasCell(msg); !overCanvas {
			return m.toolUp(m.cellToPoint(m.cursorCol, m.cursorRow), false)
		}
	}

	if id := m.zoneAt(msg); id != "" {
		if id == zoneSlider && m.mode == modeNumberEntry {
			switch msg.(type) {
			case tea.MouseClickMsg:
				return m.sliderSeek(msg)
			case tea.MouseMotionMsg:
				if msg.Mouse().Button == tea.MouseLeft {
					return m.sliderSeek(msg)
				}
			}
			return m, nil
		}
		if click, ok := msg.(tea.MouseClickMsg); ok && click.Mouse().Button == tea.MouseLeft {
			return m.handleZoneClick(id)
		}
		return m, nil
	}

	col, row, ok := m.canvasCell(msg)
	if !ok {
		return m, nil
	}

	if msg.Mouse().Button == tea.MouseMiddle {
		return m.handlePan(msg, col, row)
	}

	m.cursorCol, m.cursorRow, m.cursorVisible = col, row, true

	if m.mode != modeNormal {
		return m, nil
	}

	pt := m.cellToPoint(col, row)

	m.hoverEditID = 0
	if !m.dragging && (m.tool == ToolMove || m.tool == ToolEraser) {
		tolerance := float64(selectTolerance)
		if m.tool == ToolEraser {
			tolerance = eraserRadius(m.size)
		}
		if e := m.nearestEdit(pt, tolerance); e != nil {
			m.hoverEditID = e.ID
		}
	}

	// Ctrl, not Shift: nearly every terminal (foot included) reserves
	// Shift+mouse to bypass app mouse-reporting entirely and do native
	// text selection instead, so a Shift-held click/drag frequently never
	// reaches bdraw as a mouse event at all — it can even surface as a
	// spurious release the instant Shift changes state mid-drag, since
	// the terminal itself is switching modes out from under the app.
	// Ctrl isn't conventionally claimed by terminals for anything at the
	// mouse-protocol level, so it doesn't have this problem.
	constrain := msg.Mouse().Mod&tea.ModCtrl != 0

	switch e := msg.(type) {
	case tea.MouseClickMsg:
		if e.Mouse().Button == tea.MouseLeft {
			return m.toolDown(pt)
		}
	case tea.MouseMotionMsg:
		if m.dragging {
			return m.toolDrag(pt, constrain)
		}
	case tea.MouseReleaseMsg:
		if m.dragging {
			return m.toolUp(pt, constrain)
		}
	}
	return m, nil
}

// handleWheel scrolls the size or zoom value up/down when the wheel event
// lands on that toolbar control, or otherwise zooms the canvas in/out
// centered on the cursor's world position so the point under the mouse
// stays put instead of the view re-centering somewhere else.
func (m *Model) handleWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	up := msg.Mouse().Button == tea.MouseWheelUp

	switch m.zoneAt(msg) {
	case zoneSizeValue:
		if up {
			m.sizeInc()
		} else {
			m.sizeDec()
		}
		return *m, nil
	case zoneZoomValue:
		if up {
			m.zoomAtCursor(zoomStep)
		} else {
			m.zoomAtCursor(1 / zoomStep)
		}
		return *m, nil
	case zoneHardnessValue:
		if up {
			m.hardnessInc()
		} else {
			m.hardnessDec()
		}
		return *m, nil
	}

	factor := zoomStep
	if !up {
		factor = 1 / zoomStep
	}
	if col, row, ok := m.canvasCell(msg); ok {
		m.zoomAt(factor, col, row)
	} else {
		m.zoomBy(factor)
	}
	return *m, nil
}

// handlePan drives middle-mouse-button drag panning: press starts it,
// motion translates the view by the screen-cell delta, release ends it.
func (m *Model) handlePan(msg tea.MouseMsg, col, row int) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.MouseClickMsg:
		m.panning = true
		m.panLastCol, m.panLastRow = col, row
	case tea.MouseMotionMsg:
		if m.panning {
			zoom := m.viewZoom()
			offset := m.viewOffset()
			dCol, dRow := col-m.panLastCol, row-m.panLastRow
			offset.X -= float64(dCol) * SubpixW / zoom
			offset.Y -= float64(dRow) * SubpixH / zoom
			m.setViewOffset(offset)
			m.panLastCol, m.panLastRow = col, row
		}
	case tea.MouseReleaseMsg:
		m.panning = false
	}
	return *m, nil
}

// sliderSeek jumps the size/zoom slider to wherever the mouse landed on
// its track, so — unlike a real GUI slider's grab handle — you can click
// or drag anywhere along the bar, not just exactly on the dot.
func (m *Model) sliderSeek(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	zi := m.zm.Get(zoneSlider)
	if zi == nil {
		return *m, nil
	}
	x, _ := zi.Pos(msg)
	if x < 0 {
		return *m, nil
	}
	lo, hi, _, ok := m.numberEntryRange()
	if !ok {
		return *m, nil
	}
	m.applyNumberEntryValue(sliderValueAt(x-1, lo, hi))
	return *m, nil
}

// zoneAt returns the bubblezone ID under the mouse event, if any.
func (m Model) zoneAt(msg tea.MouseMsg) string {
	for _, id := range m.zoneIDs() {
		if z := m.zm.Get(id); z != nil && z.InBounds(msg) {
			return id
		}
	}
	return ""
}

func (m *Model) toolDown(pt Point) (tea.Model, tea.Cmd) {
	// A read-only collab guest can look around (pan/zoom/select) but never
	// starts a drag that would mutate the shared document.
	if m.readOnly && m.tool != ToolSelect {
		return m, nil
	}
	d := m.doc()
	switch m.tool {
	case ToolBrush:
		d.BeginChange()
		e := &Edit{ID: d.NextID(), Kind: KindStroke, Points: []Point{pt}, Color: m.color, Size: m.size, Hardness: m.hardness}
		d.Edits = append(d.Edits, e)
		m.dragEdit, m.dragging = e, true

	case ToolLine, ToolRect, ToolCircle, ToolArrow:
		pt = m.snapPoint(pt)
		d.BeginChange()
		kind := map[Tool]Kind{ToolLine: KindLine, ToolRect: KindRect, ToolCircle: KindCircle, ToolArrow: KindArrow}[m.tool]
		e := &Edit{ID: d.NextID(), Kind: kind, Points: []Point{pt, pt}, Color: m.color, Size: m.size, Filled: m.filled, Hardness: m.hardness}
		d.Edits = append(d.Edits, e)
		m.dragEdit, m.dragging = e, true

	case ToolEraser:
		d.BeginChange()
		m.erasedIDs = map[int]bool{}
		m.dragging = true
		m.eraseAt(pt)

	case ToolSelect:
		m.selectStart, m.selectLast = pt, pt
		m.dragging = true

	case ToolMove:
		m.moveTargets = m.selectedOrNearest(pt)
		if len(m.moveTargets) > 0 {
			d.BeginChange()
			m.moveLast = pt
			m.dragging = true
		}

	case ToolFill:
		if !isFillBounded(d.Edits, pt) {
			m.status = "fill needs an enclosed area"
			return *m, nil
		}
		d.BeginChange()
		e := &Edit{ID: d.NextID(), Kind: KindFill, Points: []Point{pt}, Color: m.color}
		d.Edits = append(d.Edits, e)

	case ToolText:
		m.textPos = m.snapPoint(pt)
		m.mode = modeTextEntry
		m.input.SetValue("")
		m.input.Focus()
		return *m, nil

	case ToolEyedropper:
		if e := m.nearestEdit(pt, selectTolerance); e != nil {
			m.setColor(e.Color)
			m.status = "picked " + e.Color
		}
	}
	return *m, nil
}

// minDragScreenDist is the minimum distance, in screen subpixels, between
// consecutive brush points. Terminals can report mouse motion faster than
// the cursor visibly moves; without decimation a fast, long drag piles up
// far more points than the line needs, and since every point is redrawn
// every frame (see BenchmarkRasterizeDocument), that cost compounds into
// visible lag the longer and faster you draw.
//
// This has to be a screen-space distance converted to world units per
// current zoom, not a flat world-space one: mouse motion only ever arrives
// in whole terminal cells, so at high zoom a single cell of movement is a
// tiny world distance. A flat world threshold silently dropped nearly
// every point at high zoom, making strokes advance in visible cell-sized
// jumps instead of smoothly.
const minDragScreenDist = 3

// constrainPoint applies the ctrl-drag modifier: lines and arrows snap to
// the nearest 45° angle from start, rectangles and ovals snap to an equal
// width/height (square/circle) — the conventional meaning of shift-drag in
// most GUI paint tools, but bound to Ctrl here instead since Shift+mouse
// is conventionally claimed by the terminal itself (see handleMouse).
func constrainPoint(kind Kind, start, pt Point) Point {
	dx, dy := pt.X-start.X, pt.Y-start.Y
	switch kind {
	case KindLine, KindArrow:
		length := math.Hypot(dx, dy)
		if length == 0 {
			return pt
		}
		angle := math.Round(math.Atan2(dy, dx)/(math.Pi/4)) * (math.Pi / 4)
		return Point{X: start.X + length*math.Cos(angle), Y: start.Y + length*math.Sin(angle)}
	case KindRect, KindCircle:
		side := math.Max(math.Abs(dx), math.Abs(dy))
		return Point{X: start.X + math.Copysign(side, dx), Y: start.Y + math.Copysign(side, dy)}
	default:
		return pt
	}
}

func (m *Model) toolDrag(pt Point, constrain bool) (tea.Model, tea.Cmd) {
	switch m.tool {
	case ToolBrush:
		zoom := m.viewZoom()
		last := m.dragEdit.Points[len(m.dragEdit.Points)-1]
		if distance(last, pt)*zoom >= minDragScreenDist {
			m.dragEdit.Points = append(m.dragEdit.Points, pt)
			m.doc().Touch()
		}
	case ToolLine, ToolRect, ToolCircle, ToolArrow:
		if constrain {
			pt = constrainPoint(m.dragEdit.Kind, m.dragEdit.Points[0], pt)
		}
		m.dragEdit.Points[1] = m.snapPoint(pt)
		m.doc().Touch()
	case ToolEraser:
		m.eraseAt(pt)
	case ToolMove:
		dx, dy := pt.X-m.moveLast.X, pt.Y-m.moveLast.Y
		for _, e := range m.moveTargets {
			for i := range e.Points {
				e.Points[i].X += dx
				e.Points[i].Y += dy
			}
		}
		m.moveLast = pt
		m.doc().Touch()
	case ToolSelect:
		m.selectLast = pt
		// Highlight whatever the marquee currently covers as you drag,
		// not just once you release — but only once it's actually a
		// marquee (past the click threshold), so a small jitter before a
		// deliberate click doesn't flash the selection.
		if distance(m.selectStart, m.selectLast) >= selectDragThreshold {
			m.selectRect(m.selectStart, m.selectLast)
			m.doc().Touch()
		}
	}
	return *m, nil
}

// selectDragThreshold is how far the mouse has to move between press and
// release, in world subpixels, before a select-tool drag is treated as a
// marquee rectangle rather than a click.
const selectDragThreshold = 10

func (m *Model) toolUp(pt Point, constrain bool) (tea.Model, tea.Cmd) {
	switch m.tool {
	case ToolLine, ToolRect, ToolCircle, ToolArrow:
		if constrain {
			pt = constrainPoint(m.dragEdit.Kind, m.dragEdit.Points[0], pt)
		}
		m.dragEdit.Points[1] = m.snapPoint(pt)
		m.doc().Touch()
	case ToolEraser:
		m.eraseAt(pt)
	case ToolSelect:
		m.selectLast = pt
		if distance(m.selectStart, m.selectLast) < selectDragThreshold {
			if e := m.nearestEdit(pt, selectTolerance); e != nil {
				e.Selected = !e.Selected
			}
			m.doc().Touch()
		}
		// else: selectRect was already applied live during the drag.
	}
	m.dragging = false
	m.dragEdit = nil
	m.erasedIDs = nil
	m.moveTargets = nil
	return *m, nil
}

// selectRect replaces the current selection with every edit that has at
// least one point inside the rectangle spanned by a and b.
func (m *Model) selectRect(a, b Point) {
	x0, x1 := minF(a.X, b.X), maxF(a.X, b.X)
	y0, y1 := minF(a.Y, b.Y), maxF(a.Y, b.Y)
	for _, e := range m.doc().Edits {
		e.Selected = false
		for _, p := range e.Points {
			if p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
				e.Selected = true
				break
			}
		}
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// eraseAt removes every edit (not already removed this drag) whose
// geometry passes within eraserRadius of pt. Erasing always removes the
// whole edit it touches, never a partial stroke.
func (m *Model) eraseAt(pt Point) {
	d := m.doc()
	radius := eraserRadius(m.size)
	kept := d.Edits[:0]
	for _, e := range d.Edits {
		if !m.erasedIDs[e.ID] && e.Distance(pt) <= radius {
			m.erasedIDs[e.ID] = true
			continue
		}
		kept = append(kept, e)
	}
	d.Edits = kept
	d.Touch()
}

// nearestEdit returns the closest edit to pt within tolerance, or nil.
func (m *Model) nearestEdit(pt Point, tolerance float64) *Edit {
	var best *Edit
	bestDist := tolerance
	for _, e := range m.doc().Edits {
		if d := e.Distance(pt); d <= bestDist {
			best, bestDist = e, d
		}
	}
	return best
}

// selectedOrNearest returns every selected edit, if any; otherwise just
// the single edit nearest pt, so the move tool works both on a
// pre-selection and on an ad-hoc click.
func (m *Model) selectedOrNearest(pt Point) []*Edit {
	var selected []*Edit
	for _, e := range m.doc().Edits {
		if e.Selected {
			selected = append(selected, e)
		}
	}
	if len(selected) > 0 {
		return selected
	}
	if e := m.nearestEdit(pt, selectTolerance); e != nil {
		return []*Edit{e}
	}
	return nil
}
