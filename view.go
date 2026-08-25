package main

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const selectColor = "#ffaa00"

var (
	activeStyle   = lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1)
	inactiveStyle = lipgloss.NewStyle().Padding(0, 1)
	hoverStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#444444")).Padding(0, 1)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	modalStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

// gridDotColor is deliberately close to the terminal's black background —
// barely noticeable, there purely to help line things up.
const gridDotColor = "#3a3a3a"

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	var b strings.Builder
	for _, line := range m.tabLines() {
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, line := range m.toolbarLines() {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if m.mode == modeColorPicker {
		b.WriteString(m.viewColorPicker())
		b.WriteString("\n")
	}
	if m.mode == modeNumberEntry {
		b.WriteString(m.viewNumberSlider())
		b.WriteString("\n")
	}
	if m.mode == modePromptOpen && len(m.recent) > 0 {
		b.WriteString(m.viewRecentFiles())
		b.WriteString("\n")
	}
	b.WriteString(m.viewCanvas())
	b.WriteString("\n")
	b.WriteString(m.viewStatus())

	v := tea.NewView(m.zm.Scan(b.String()))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

// headerHeight is how many rows the tabs + toolbar (+ color picker modal,
// if open) occupy, wrapped to the current terminal width. The canvas
// viewport and mouse hit-testing both key off this.
func (m Model) headerHeight() int {
	h := len(m.tabLines()) + len(m.toolbarLines())
	if m.mode == modeColorPicker {
		h += strings.Count(m.viewColorPicker(), "\n") + 1
	}
	if m.mode == modeNumberEntry {
		h += strings.Count(m.viewNumberSlider(), "\n") + 1
	}
	if m.mode == modePromptOpen && len(m.recent) > 0 {
		h += strings.Count(m.viewRecentFiles(), "\n") + 1
	}
	return h
}

// viewRecentFiles renders the numbered recent-files list shown below the
// Open prompt: press a digit (with the path field empty) to open directly.
func (m Model) viewRecentFiles() string {
	var b strings.Builder
	b.WriteString("recent (press a number):")
	for i, p := range m.recent {
		fmt.Fprintf(&b, "\n  %d. %s", i+1, p)
	}
	return modalStyle.Render(b.String())
}

// wrapButtons packs pre-rendered, already-styled button strings onto as few
// lines as fit within width, wrapping to a new line rather than overflowing
// the terminal.
func wrapButtons(buttons []string, width int) []string {
	if width <= 0 {
		return []string{strings.Join(buttons, "")}
	}
	var lines []string
	var cur strings.Builder
	curWidth := 0
	for _, btn := range buttons {
		w := lipgloss.Width(btn)
		if curWidth > 0 && curWidth+w > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curWidth = 0
		}
		cur.WriteString(btn)
		curWidth += w
	}
	if curWidth > 0 || len(lines) == 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func (m Model) tabLines() []string {
	var parts []string
	for i, d := range m.tabs {
		style := m.styleFor(zoneTab(i), i == m.active)
		closeStyle := m.styleFor(zoneTabClose(i), false)
		label := style.Render(d.Title()) + " " + m.zm.Mark(zoneTabClose(i), closeStyle.Render("x"))
		parts = append(parts, m.zm.Mark(zoneTab(i), label))
	}
	parts = append(parts, m.zm.Mark(zoneNewTab, m.styleFor(zoneNewTab, false).Render("+")))
	return wrapButtons(parts, m.width)
}

// styleFor picks a button's style: active beats hover beats plain, so a
// button always shows some feedback for what the mouse and keyboard are
// doing, not just clicks.
func (m Model) styleFor(id string, active bool) lipgloss.Style {
	switch {
	case active:
		return activeStyle
	case m.hoverZone == id:
		return hoverStyle
	default:
		return inactiveStyle
	}
}

func (m Model) button(id, label string, active bool) string {
	return m.zm.Mark(id, m.styleFor(id, active).Render(label))
}

// toolButton renders a tool's toolbar button as its icon glyph when icons
// are enabled (Config.UseIcons), or its name otherwise — togglable so
// users who prefer clarity over density can switch back.
func (m Model) toolButton(id string, t Tool, active bool) string {
	label := strings.ToUpper(string(t[:1])) + string(t[1:])
	if m.cfg.UseIcons {
		label = string(toolCursor[t])
	}
	return m.button(id, label, active)
}

// colorButton renders the color-picker toggle. The swatch dot needs its own
// Foreground style; nesting that pre-rendered (already-ANSI-coded) string
// inside another Style.Render call as if it were plain text corrupted the
// output (raw escape codes showing up as literal text) whenever the outer
// style also applied a background — so the swatch and the surrounding
// button text are rendered as separate, sibling segments instead.
func (m Model) colorButton() string {
	active := m.mode == modeColorPicker
	text := m.styleFor(zoneColorButton, active)
	swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(m.color))
	if !active && m.hoverZone == zoneColorButton {
		swatch = swatch.Background(lipgloss.Color("#444444"))
	}
	return m.zm.Mark(zoneColorButton, text.Render(" ")+swatch.Render("●")+text.Render(" Color "))
}

// compactToolbarLine renders the minimal toolbar: current tool, filled
// status, size, and zoom, plus the toggle to get the full toolbar back.
func (m Model) compactToolbarLine() []string {
	toolLabel := strings.ToUpper(string(m.tool[:1])) + string(m.tool[1:])
	buttons := []string{
		m.button(zoneCompact, "☰", false),
		inactiveStyle.Render(toolLabel),
		m.button(zoneFilled, "Filled: "+onOff(m.filled), m.filled),
		m.button(zoneSizeValue, "size "+m.numberEntryLabel("size", "", m.size), m.mode == modeNumberEntry && m.numEntryTarget == "size"),
		m.button(zoneHardnessValue, "hard "+m.numberEntryLabel("hardness", "%", m.hardness), m.mode == modeNumberEntry && m.numEntryTarget == "hardness"),
		m.button(zoneZoomValue, "zoom "+m.numberEntryLabel("zoom", "%", m.viewZoom()*100), m.mode == modeNumberEntry && m.numEntryTarget == "zoom"),
	}
	return wrapButtons(buttons, m.width)
}

func (m Model) toolbarLines() []string {
	if m.compact {
		return m.compactToolbarLine()
	}
	buttons := []string{
		m.button(zoneCompact, "☰", false),
		m.button(zoneNew, "New", false),
		m.button(zoneOpen, "Open", false),
		m.button(zoneSave, "Save", false),
		m.button(zoneSaveAs, "Save As", false),
		m.button(zoneExport, "Export", false),
		m.button(zoneClear, "Clear", false),
		m.button(zoneUndo, "Undo", false),
		m.button(zoneRedo, "Redo", false),
		m.toolButton(zoneToolBrush, ToolBrush, m.tool == ToolBrush),
		m.toolButton(zoneToolRect, ToolRect, m.tool == ToolRect),
		m.toolButton(zoneToolCircle, ToolCircle, m.tool == ToolCircle),
		m.toolButton(zoneToolLine, ToolLine, m.tool == ToolLine),
		m.toolButton(zoneToolEraser, ToolEraser, m.tool == ToolEraser),
		m.toolButton(zoneToolSelect, ToolSelect, m.tool == ToolSelect),
		m.toolButton(zoneToolMove, ToolMove, m.tool == ToolMove),
		m.toolButton(zoneToolFill, ToolFill, m.tool == ToolFill),
		m.toolButton(zoneToolArrow, ToolArrow, m.tool == ToolArrow),
		m.toolButton(zoneToolEyedropper, ToolEyedropper, m.tool == ToolEyedropper),
		m.toolButton(zoneToolText, ToolText, m.tool == ToolText),
		m.button(zoneFilled, "Filled: "+onOff(m.filled), m.filled),
		m.colorButton(),
		m.button(zoneSizeDec, "-", false),
		m.button(zoneSizeValue, "size "+m.numberEntryLabel("size", "", m.size), m.mode == modeNumberEntry && m.numEntryTarget == "size"),
		m.button(zoneSizeInc, "+", false),
		m.button(zoneHardnessDec, "-", false),
		m.button(zoneHardnessValue, "hard "+m.numberEntryLabel("hardness", "%", m.hardness), m.mode == modeNumberEntry && m.numEntryTarget == "hardness"),
		m.button(zoneHardnessInc, "+", false),
		m.button(zoneZoomOut, "-", false),
		m.button(zoneZoomValue, "zoom "+m.numberEntryLabel("zoom", "%", m.viewZoom()*100), m.mode == modeNumberEntry && m.numEntryTarget == "zoom"),
		m.button(zoneZoomIn, "+", false),
		m.button(zoneGrid, "Grid: "+onOff(m.showGrid), m.showGrid),
		m.button(zoneSnap, "Snap: "+onOff(m.snap), m.snap),
		m.button(zoneMinimap, "Minimap: "+onOff(m.showMinimap), m.showMinimap),
	}
	return wrapButtons(buttons, m.width)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m Model) viewColorPicker() string {
	var swatches strings.Builder
	for i, c := range Palette {
		style := lipgloss.NewStyle().Background(lipgloss.Color(c)).Padding(0, 1)
		if m.hoverZone == zoneColor(i) || m.colorPickerFocus == i {
			style = style.Underline(true)
		}
		label := "  "
		if c == m.color {
			label = "[]"
		}
		swatches.WriteString(m.zm.Mark(zoneColor(i), style.Render(label)))
		swatches.WriteString(" ")
	}
	hint := dimStyle.Render(" (tab to switch swatches, enter to apply hex, esc to cancel)")
	body := swatches.String() + "\nhex: " + m.input.View() + hint
	return modalStyle.Render(body)
}

// canvasRaster returns the rasterized canvas for the current document/view
// state, reusing the previous frame's raster whenever nothing that would
// change it (edits, pan, zoom, viewport size, active tab) has changed.
func (m Model) canvasRaster(cols, rows int, d *Document, offset Point, zoom float64) *Raster {
	c := m.cache
	sameView := c.raster != nil && c.doc == d && c.highlightID == m.hoverEditID &&
		c.cols == cols && c.rows == rows && c.offset == offset && c.zoom == zoom

	if sameView && c.docVer == d.Version {
		return c.raster
	}

	// Incremental fast path: an active brush drag calls Touch() exactly
	// once per point appended, so if the current version is exactly the
	// cached version plus the number of new points, nothing else could
	// have touched the document in between — any other edit, toggle, or
	// undo/redo would throw that arithmetic off and fall through to a
	// full rebuild. Without this, every single motion event during a long
	// drag re-rasterized the whole stroke from scratch, and that cost
	// grew without bound the longer and faster you drew (see
	// BenchmarkRasterizeDocument).
	if sameView && m.dragging && m.tool == ToolBrush && m.dragEdit != nil &&
		c.dragEditID == m.dragEdit.ID {
		newPoints := len(m.dragEdit.Points) - c.dragPointsBaked
		if newPoints > 0 && d.Version == c.docVer+newPoints {
			c.raster.appendStroke(m.dragEdit.Points, c.dragPointsBaked, m.dragEdit.Size, m.dragEdit.hardness(), offset.X, offset.Y, zoom, m.dragEdit.Color)
			c.docVer = d.Version
			c.dragPointsBaked = len(m.dragEdit.Points)
			return c.raster
		}
	}

	// Overlay fast path for dragging out a rect/circle/line/arrow: that
	// edit is always just 2 points (its far corner following the cursor),
	// so unlike the brush there's nothing to append — but every *other*
	// edit is untouched by the drag, so they don't need to be redrawn
	// either. The base (everything else) is cached once per drag and
	// reused: dragEditID/tool/dragging all still matching what the base
	// was captured under, plus the active edit still being the document's
	// last one (i.e. it wasn't undone out from under us), are enough to
	// trust nothing else changed — there's no natural per-frame delta to
	// check arithmetically here the way there is for point-appending.
	if sameView && m.dragging && isShapeDragTool(m.tool) && m.dragEdit != nil &&
		c.baseRaster != nil && c.baseEditID == m.dragEdit.ID &&
		len(d.Edits) > 0 && d.Edits[len(d.Edits)-1] == m.dragEdit {
		r := c.baseRaster.clone()
		color := m.dragEdit.Color
		if m.dragEdit.Selected {
			color = selectColor
		}
		r.drawEdit(m.dragEdit, offset.X, offset.Y, zoom, color)
		c.raster = r
		c.docVer = d.Version
		return r
	}

	r := RasterizeDocument(d.Edits, cols, rows, offset.X, offset.Y, zoom, selectColor, m.hoverEditID, hoverEditColor)
	dragEditID, dragPointsBaked := 0, 0
	var baseRaster *Raster
	baseEditID, baseVer := 0, 0
	if m.dragging && m.tool == ToolBrush && m.dragEdit != nil {
		dragEditID, dragPointsBaked = m.dragEdit.ID, len(m.dragEdit.Points)
	}
	if m.dragging && isShapeDragTool(m.tool) && m.dragEdit != nil {
		baseRaster = RasterizeDocument(withoutEdit(d.Edits, m.dragEdit.ID), cols, rows, offset.X, offset.Y, zoom, selectColor, m.hoverEditID, hoverEditColor)
		baseEditID, baseVer = m.dragEdit.ID, d.Version
	}
	*c = canvasCache{
		raster: r, cols: cols, rows: rows, offset: offset, zoom: zoom, doc: d, docVer: d.Version,
		highlightID: m.hoverEditID, dragEditID: dragEditID, dragPointsBaked: dragPointsBaked,
		baseRaster: baseRaster, baseEditID: baseEditID, baseVer: baseVer,
	}
	return r
}

// isShapeDragTool reports whether tool creates a single 2-point edit that
// just follows the cursor while dragging (as opposed to brush, which grows
// a point list, or move/select, which touch other edits).
func isShapeDragTool(t Tool) bool {
	return t == ToolRect || t == ToolCircle || t == ToolLine || t == ToolArrow
}

// withoutEdit returns edits minus the one with the given ID.
func withoutEdit(edits []*Edit, id int) []*Edit {
	out := make([]*Edit, 0, len(edits))
	for _, e := range edits {
		if e.ID != id {
			out = append(out, e)
		}
	}
	return out
}

// marqueeBounds returns the select-tool drag rectangle in screen cells,
// while it's being dragged.
func (m Model) marqueeBounds(offset Point, zoom float64) (x0, y0, x1, y1 int, ok bool) {
	if !(m.dragging && m.tool == ToolSelect) {
		return 0, 0, 0, 0, false
	}
	c0 := int(((m.selectStart.X - offset.X) * zoom) / SubpixW)
	r0 := int(((m.selectStart.Y - offset.Y) * zoom) / SubpixH)
	c1 := int(((m.selectLast.X - offset.X) * zoom) / SubpixW)
	r1 := int(((m.selectLast.Y - offset.Y) * zoom) / SubpixH)
	if c0 > c1 {
		c0, c1 = c1, c0
	}
	if r0 > r1 {
		r0, r1 = r1, r0
	}
	return c0, r0, c1, r1, true
}

// gridTargetScreenSpacing is roughly how far apart, in screen subpixels,
// the grid dots should appear regardless of zoom.
const gridTargetScreenSpacing = 16

// adaptiveGridWorldStep picks the world-space dot spacing so its on-screen
// spacing stays near gridTargetScreenSpacing at any zoom — a fixed world
// step either vanished (too many world-units per screen dot at high zoom)
// or turned into visual noise (too few at low zoom). Snapping to a power
// of 2 keeps the step from jittering continuously as zoom changes.
func adaptiveGridWorldStep(zoom float64) float64 {
	if zoom <= 0 {
		zoom = 1
	}
	raw := gridTargetScreenSpacing / zoom
	step := math.Pow(2, math.Round(math.Log2(raw)))
	if step < 1 {
		step = 1
	}
	return step
}

// gridLineInCell reports whether a grid line falls inside the screen-space
// cell spanning [idx*cellSize-phase, (idx+1)*cellSize-phase). period is the
// grid's screen-space spacing; if it's smaller than a cell (zoomed out
// past one grid line per cell) every cell counts as on-grid, since drawing
// only some of them would look like aliasing noise rather than a grid.
func gridLineInCell(idx int, cellSize, period, phase float64) bool {
	if period <= 0 {
		return false
	}
	if period < cellSize {
		return true
	}
	a := float64(idx)*cellSize - phase
	b := a + cellSize
	return math.Floor(a/period) != math.Floor(b/period)
}

// isGridDot reports whether the dot-grid overlay should show at cell
// (col, row) — purely a drawing aid, computed live and never written into
// the Raster, so it can never leak into a saved document or an export.
func isGridDot(col, row int, offset Point, zoom float64) bool {
	period := adaptiveGridWorldStep(zoom) * zoom
	return gridLineInCell(col, SubpixW, period, offset.X*zoom) &&
		gridLineInCell(row, SubpixH, period, offset.Y*zoom)
}

func (m Model) viewCanvas() string {
	if m.showHelp {
		return m.viewHelp()
	}
	cols, rows := m.canvasSize()
	if cols <= 0 || rows <= 0 {
		return ""
	}
	d := m.doc()
	offset, zoom := m.viewOffset(), m.viewZoom()
	r := m.canvasRaster(cols, rows, d, offset, zoom)
	mx0, my0, mx1, my1, marquee := m.marqueeBounds(offset, zoom)

	// Other collab peers' cursors, converted from their broadcast world
	// coordinates into this viewer's own screen cells — necessary since
	// each connection independently pans/zooms.
	type peerDot struct {
		ru    rune
		color string
	}
	peerAt := map[[2]int]peerDot{}
	for _, pc := range m.peerCursors {
		if !pc.Visible {
			continue
		}
		col := int((pc.Pt.X - offset.X) * zoom / SubpixW)
		row := int((pc.Pt.Y - offset.Y) * zoom / SubpixH)
		if col < 0 || col >= cols || row < 0 || row >= rows {
			continue
		}
		peerAt[[2]int{col, row}] = peerDot{ru: '●', color: pc.Color}
		label := " " + pc.Name
		for i, r := range label {
			lc := col + 1 + i
			if lc >= cols {
				break
			}
			if _, taken := peerAt[[2]int{lc, row}]; taken {
				break
			}
			peerAt[[2]int{lc, row}] = peerDot{ru: r, color: pc.Color}
		}
	}

	minimapAt := m.minimapOverlay(cols, rows, d, offset, zoom)

	type styleKey struct{ fg, bg string }
	styles := map[styleKey]lipgloss.Style{}
	styleFor := func(k styleKey) lipgloss.Style {
		if s, ok := styles[k]; ok {
			return s
		}
		s := lipgloss.NewStyle()
		if k.fg != "" {
			s = s.Foreground(lipgloss.Color(k.fg))
		}
		if k.bg != "" {
			s = s.Background(lipgloss.Color(k.bg))
		}
		styles[k] = s
		return s
	}

	var b strings.Builder
	for row := 0; row < rows; row++ {
		runKey := styleKey{}
		var plain strings.Builder
		var run strings.Builder

		flush := func() {
			if run.Len() == 0 {
				return
			}
			if runKey == (styleKey{}) {
				plain.WriteString(run.String())
			} else {
				plain.WriteString(styleFor(runKey).Render(run.String()))
			}
			run.Reset()
		}

		for col := 0; col < cols; col++ {
			ru, color := r.Rune(col, row)
			bg := r.Background(col, row)
			pd, isPeer := peerAt[[2]int{col, row}]
			mc, isMinimap := minimapAt[[2]int{col, row}]

			switch {
			case isMinimap:
				ru, color, bg = mc.ru, mc.color, ""
			case m.cursorVisible && m.mode == modeNormal && col == m.cursorCol && row == m.cursorRow:
				ru, color = toolCursor[m.tool], m.cursorColor()
			case isPeer:
				ru, color = pd.ru, pd.color
			case marquee && (col == mx0 || col == mx1) && row >= my0 && row <= my1,
				marquee && (row == my0 || row == my1) && col >= mx0 && col <= mx1:
				if ru == ' ' {
					ru = '·'
				}
				color = selectColor
			case m.showGrid && ru == ' ' && bg == "" && isGridDot(col, row, offset, zoom):
				ru, color = '·', gridDotColor
			}

			key := styleKey{fg: color, bg: bg}
			if key != runKey {
				flush()
			}
			runKey = key
			run.WriteRune(ru)
		}
		flush()
		b.WriteString(plain.String())
		if row < rows-1 {
			b.WriteRune('\n')
		}
	}
	return b.String()
}

func (m Model) viewStatus() string {
	switch m.mode {
	case modePromptSaveAs:
		return "save as: " + m.input.View()
	case modePromptOpen:
		return "open: " + m.input.View()
	case modeTextEntry:
		return "text: " + m.input.View()
	case modeConfirmClose:
		return "unsaved changes — close anyway? (y/n)"
	case modeConfirmClear:
		return "clear the whole canvas? (y/n)"
	case modeNumberEntry:
		return ""
	}
	if tip := m.tooltip(m.hoverZone); tip != "" {
		return dimStyle.Render(tip)
	}
	return dimStyle.Render(m.status)
}
