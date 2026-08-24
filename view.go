package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const selectColor = "#ffaa00"

var (
	activeStyle   = lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1)
	inactiveStyle = lipgloss.NewStyle().Padding(0, 1)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	modalStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m Model) View() string {
	if m.width == 0 {
		return ""
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
	b.WriteString(m.viewCanvas())
	b.WriteString("\n")
	b.WriteString(m.viewStatus())
	return zone.Scan(b.String())
}

// headerHeight is how many rows the tabs + toolbar (+ color picker modal,
// if open) occupy, wrapped to the current terminal width. The canvas
// viewport and mouse hit-testing both key off this.
func (m Model) headerHeight() int {
	h := len(m.tabLines()) + len(m.toolbarLines())
	if m.mode == modeColorPicker {
		h += strings.Count(m.viewColorPicker(), "\n") + 1
	}
	return h
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
		style := inactiveStyle
		if i == m.active {
			style = activeStyle
		}
		label := d.Title() + " " + zone.Mark(zoneTabClose(i), "x")
		parts = append(parts, zone.Mark(zoneTab(i), style.Render(label)))
	}
	parts = append(parts, zone.Mark(zoneNewTab, inactiveStyle.Render("+")))
	return wrapButtons(parts, m.width)
}

func button(id, label string, active bool) string {
	style := inactiveStyle
	if active {
		style = activeStyle
	}
	return zone.Mark(id, style.Render(label))
}

// toolButton renders a tool's toolbar button as its icon glyph when icons
// are enabled (Config.UseIcons), or its name otherwise — togglable so
// users who prefer clarity over density can switch back.
func (m Model) toolButton(id string, t Tool, active bool) string {
	label := string(t)
	if m.cfg.UseIcons {
		label = string(toolCursor[t])
	}
	return button(id, label, active)
}

func (m Model) toolbarLines() []string {
	colorSwatch := lipgloss.NewStyle().Foreground(lipgloss.Color(m.color)).Render("●")
	buttons := []string{
		button(zoneNew, "New", false),
		button(zoneOpen, "Open", false),
		button(zoneSave, "Save", false),
		button(zoneSaveAs, "Save As", false),
		button(zoneExport, "Export", false),
		button(zoneClear, "Clear", false),
		button(zoneUndo, "Undo", false),
		button(zoneRedo, "Redo", false),
		m.toolButton(zoneToolBrush, ToolBrush, m.tool == ToolBrush),
		m.toolButton(zoneToolRect, ToolRect, m.tool == ToolRect),
		m.toolButton(zoneToolCircle, ToolCircle, m.tool == ToolCircle),
		m.toolButton(zoneToolLine, ToolLine, m.tool == ToolLine),
		m.toolButton(zoneToolEraser, ToolEraser, m.tool == ToolEraser),
		m.toolButton(zoneToolSelect, ToolSelect, m.tool == ToolSelect),
		m.toolButton(zoneToolMove, ToolMove, m.tool == ToolMove),
		m.toolButton(zoneToolFill, ToolFill, m.tool == ToolFill),
		m.toolButton(zoneToolText, ToolText, m.tool == ToolText),
		button(zoneColorButton, colorSwatch+" Color", m.mode == modeColorPicker),
		button(zoneSizeDec, "-", false),
		inactiveStyle.Render(fmt.Sprintf("size %.0f", m.size)),
		button(zoneSizeInc, "+", false),
		button(zoneZoomOut, "-", false),
		inactiveStyle.Render(fmt.Sprintf("zoom %.0f%%", m.doc().Zoom*100)),
		button(zoneZoomIn, "+", false),
	}
	return wrapButtons(buttons, m.width)
}

func (m Model) viewColorPicker() string {
	var swatches strings.Builder
	for i, c := range Palette {
		style := lipgloss.NewStyle().Background(lipgloss.Color(c)).Padding(0, 1)
		label := "  "
		if c == m.color {
			label = "[]"
		}
		swatches.WriteString(zone.Mark(zoneColor(i), style.Render(label)))
		swatches.WriteString(" ")
	}
	body := swatches.String() + "\nhex: " + m.input.View() + dimStyle.Render(" (enter to apply, esc to cancel)")
	return modalStyle.Render(body)
}

// canvasRaster returns the rasterized canvas for the current document/view
// state, reusing the previous frame's raster whenever nothing that would
// change it (edits, pan, zoom, viewport size, active tab) has changed.
func (m Model) canvasRaster(cols, rows int, d *Document, zoom float64) *Raster {
	c := m.cache
	if c.raster != nil && c.doc == d && c.docVer == d.Version &&
		c.cols == cols && c.rows == rows && c.offset == d.Offset && c.zoom == zoom {
		return c.raster
	}
	r := RasterizeDocument(d.Edits, cols, rows, d.Offset.X, d.Offset.Y, zoom, selectColor)
	*c = canvasCache{raster: r, cols: cols, rows: rows, offset: d.Offset, zoom: zoom, doc: d, docVer: d.Version}
	return r
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

func (m Model) viewCanvas() string {
	cols, rows := m.canvasSize()
	if cols <= 0 || rows <= 0 {
		return ""
	}
	d := m.doc()
	zoom := d.Zoom
	if zoom == 0 {
		zoom = 1
	}
	r := m.canvasRaster(cols, rows, d, zoom)
	mx0, my0, mx1, my1, marquee := m.marqueeBounds(d.Offset, zoom)

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

			switch {
			case m.cursorVisible && m.mode == modeNormal && col == m.cursorCol && row == m.cursorRow:
				ru, color = toolCursor[m.tool], cursorColor
			case marquee && (col == mx0 || col == mx1) && row >= my0 && row <= my1,
				marquee && (row == my0 || row == my1) && col >= mx0 && col <= mx1:
				if ru == ' ' {
					ru = '·'
				}
				color = selectColor
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
	}
	return dimStyle.Render(m.status)
}
