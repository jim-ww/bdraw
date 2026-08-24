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

func (m Model) toolbarLines() []string {
	colorSwatch := lipgloss.NewStyle().Foreground(lipgloss.Color(m.color)).Render("●")
	buttons := []string{
		button(zoneNew, "New", false),
		button(zoneOpen, "Open", false),
		button(zoneSave, "Save", false),
		button(zoneSaveAs, "Save As", false),
		button(zoneUndo, "Undo", false),
		button(zoneRedo, "Redo", false),
		button(zoneToolBrush, "Brush", m.tool == ToolBrush),
		button(zoneToolRect, "Rect", m.tool == ToolRect),
		button(zoneToolCircle, "Oval", m.tool == ToolCircle),
		button(zoneToolLine, "Line", m.tool == ToolLine),
		button(zoneToolEraser, "Eraser", m.tool == ToolEraser),
		button(zoneToolSelect, "Select", m.tool == ToolSelect),
		button(zoneToolMove, "Move", m.tool == ToolMove),
		button(zoneToolFill, "Fill", m.tool == ToolFill),
		button(zoneToolText, "Text", m.tool == ToolText),
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

func (m Model) viewCanvas() string {
	cols := m.width
	rows := m.height - m.headerHeight() - FooterRows
	if cols <= 0 || rows <= 0 {
		return ""
	}
	d := m.doc()
	zoom := d.Zoom
	if zoom == 0 {
		zoom = 1
	}
	r := RasterizeDocument(d.Edits, cols, rows, d.Offset.X, d.Offset.Y, zoom, selectColor)

	if m.cursorVisible && m.mode == modeNormal &&
		m.cursorCol >= 0 && m.cursorCol < cols && m.cursorRow >= 0 && m.cursorRow < rows {
		c := r.at(m.cursorCol, m.cursorRow)
		c.glyph = toolCursor[m.tool]
		c.color = cursorColor
	}

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
			key := styleKey{fg: color, bg: r.Background(col, row)}
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
	}
	return dimStyle.Render(m.status)
}
