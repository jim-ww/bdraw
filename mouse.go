package main

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
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
	if msg.Action == tea.MouseActionMotion {
		m.hoverZone = m.zoneAt(msg)
	}

	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return m.handleWheel(msg)
	}

	if id := m.zoneAt(msg); id != "" {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m.handleZoneClick(id)
		}
		return m, nil
	}

	col, row, ok := m.canvasCell(msg)
	if !ok {
		return m, nil
	}

	if msg.Button == tea.MouseButtonMiddle || m.panning {
		return m.handlePan(msg, col, row)
	}

	m.cursorCol, m.cursorRow, m.cursorVisible = col, row, true

	if m.mode != modeNormal {
		return m, nil
	}

	pt := m.cellToPoint(col, row)
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			return m.toolDown(pt)
		}
	case tea.MouseActionMotion:
		if m.dragging {
			return m.toolDrag(pt)
		}
	case tea.MouseActionRelease:
		if m.dragging {
			return m.toolUp(pt)
		}
	}
	return m, nil
}

// handleWheel zooms in/out centered on the cursor's world position (if the
// wheel event landed on the canvas) so the point under the mouse stays put
// instead of the view re-centering somewhere else.
func (m *Model) handleWheel(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	factor := zoomStep
	if msg.Button == tea.MouseButtonWheelDown {
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
	switch msg.Action {
	case tea.MouseActionPress:
		m.panning = true
		m.panLastCol, m.panLastRow = col, row
	case tea.MouseActionMotion:
		if m.panning {
			d := m.doc()
			zoom := d.Zoom
			if zoom == 0 {
				zoom = 1
			}
			dCol, dRow := col-m.panLastCol, row-m.panLastRow
			d.Offset.X -= float64(dCol) * SubpixW / zoom
			d.Offset.Y -= float64(dRow) * SubpixH / zoom
			m.panLastCol, m.panLastRow = col, row
		}
	case tea.MouseActionRelease:
		m.panning = false
	}
	return *m, nil
}

// zoneAt returns the bubblezone ID under the mouse event, if any.
func (m Model) zoneAt(msg tea.MouseMsg) string {
	for _, id := range m.zoneIDs() {
		if z := zone.Get(id); z != nil && z.InBounds(msg) {
			return id
		}
	}
	return ""
}

func (m *Model) toolDown(pt Point) (tea.Model, tea.Cmd) {
	d := m.doc()
	switch m.tool {
	case ToolBrush:
		d.BeginChange()
		e := &Edit{ID: d.NextID(), Kind: KindStroke, Points: []Point{pt}, Color: m.color, Size: m.size}
		d.Edits = append(d.Edits, e)
		m.dragEdit, m.dragging = e, true

	case ToolLine, ToolRect, ToolCircle:
		d.BeginChange()
		kind := map[Tool]Kind{ToolLine: KindLine, ToolRect: KindRect, ToolCircle: KindCircle}[m.tool]
		e := &Edit{ID: d.NextID(), Kind: kind, Points: []Point{pt, pt}, Color: m.color, Size: m.size}
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
		cols, rows := m.canvasSize()
		zoom := d.Zoom
		if zoom == 0 {
			zoom = 1
		}
		check := RasterizeDocument(d.Edits, cols, rows, d.Offset.X, d.Offset.Y, zoom, selectColor)
		col := int(((pt.X - d.Offset.X) * zoom) / SubpixW)
		row := int(((pt.Y - d.Offset.Y) * zoom) / SubpixH)
		if _, touchesEdge := check.floodRegion(col, row); touchesEdge {
			m.status = "fill needs an enclosed area — background fill isn't supported"
			return *m, nil
		}
		d.BeginChange()
		e := &Edit{ID: d.NextID(), Kind: KindFill, Points: []Point{pt}, Color: m.color}
		d.Edits = append(d.Edits, e)

	case ToolText:
		m.textPos = pt
		m.mode = modeTextEntry
		m.input.SetValue("")
		m.input.Focus()
		return *m, nil
	}
	return *m, nil
}

// minDragPointDist is the minimum distance (world subpixels) between
// consecutive brush points. Terminals can report mouse motion faster than
// the cursor visibly moves; without decimation a fast, long drag piles up
// far more points than the line needs, and since every point is
// redrawn every frame (see BenchmarkRasterizeDocument), that cost compounds
// into visible lag the longer and faster you draw.
const minDragPointDist = 3

func (m *Model) toolDrag(pt Point) (tea.Model, tea.Cmd) {
	switch m.tool {
	case ToolBrush:
		last := m.dragEdit.Points[len(m.dragEdit.Points)-1]
		if distance(last, pt) >= minDragPointDist {
			m.dragEdit.Points = append(m.dragEdit.Points, pt)
			m.doc().Touch()
		}
	case ToolLine, ToolRect, ToolCircle:
		m.dragEdit.Points[1] = pt
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
	}
	return *m, nil
}

// selectDragThreshold is how far the mouse has to move between press and
// release, in world subpixels, before a select-tool drag is treated as a
// marquee rectangle rather than a click.
const selectDragThreshold = 10

func (m *Model) toolUp(pt Point) (tea.Model, tea.Cmd) {
	switch m.tool {
	case ToolLine, ToolRect, ToolCircle:
		m.dragEdit.Points[1] = pt
		m.doc().Touch()
	case ToolEraser:
		m.eraseAt(pt)
	case ToolSelect:
		m.selectLast = pt
		if distance(m.selectStart, m.selectLast) < selectDragThreshold {
			if e := m.nearestEdit(pt, selectTolerance); e != nil {
				e.Selected = !e.Selected
			}
		} else {
			m.selectRect(m.selectStart, m.selectLast)
		}
		m.doc().Touch()
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
