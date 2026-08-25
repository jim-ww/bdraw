package main

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

type helpEntry struct {
	key   string
	label string
}

type helpSection struct {
	title   string
	entries []helpEntry
}

func he(b key.Binding, label string) helpEntry {
	return helpEntry{key: b.Help().Key, label: label}
}

// helpSections builds the full keybind reference from the live KeyMap, so
// it can never drift out of sync with the actual bindings (including user
// overrides from keymap.json).
func (m Model) helpSections() []helpSection {
	km := m.km
	return []helpSection{
		{"Tools", []helpEntry{
			he(km.ToolBrush, "brush"),
			he(km.ToolRect, "rectangle"),
			he(km.ToolCircle, "oval"),
			he(km.ToolLine, "line"),
			he(km.ToolArrow, "arrow"),
			he(km.ToolEraser, "eraser"),
			he(km.ToolSelect, "select"),
			he(km.ToolMove, "move"),
			he(km.ToolFill, "fill"),
			he(km.ToolText, "text"),
			he(km.ToolEyedropper, "eyedropper"),
		}},
		{"Edit", []helpEntry{
			he(km.Undo, "undo"),
			he(km.Redo, "redo"),
			he(km.Delete, "delete selection"),
			he(km.Copy, "copy selection"),
			he(km.Paste, "paste"),
			he(km.ClearSelection, "deselect"),
		}},
		{"File", []helpEntry{
			he(km.New, "new"),
			he(km.Open, "open"),
			he(km.Save, "save"),
			he(km.SaveAs, "save as"),
			he(km.Export, "export"),
			he(km.Clear, "clear canvas"),
		}},
		{"View", []helpEntry{
			he(km.ColorPicker, "color picker"),
			he(km.ToggleGrid, "toggle grid"),
			he(km.ToggleSnap, "toggle snap"),
			he(km.ToggleFill, "toggle filled shapes"),
			he(km.ToggleCompact, "toggle compact toolbar"),
			he(km.Help, "toggle this help"),
		}},
		{"Navigate", []helpEntry{
			he(km.PanUp, "pan up"),
			he(km.PanDown, "pan down"),
			he(km.PanLeft, "pan left"),
			he(km.PanRight, "pan right"),
			he(km.ZoomIn, "zoom in"),
			he(km.ZoomOut, "zoom out"),
			he(km.SizeInc, "size up"),
			he(km.SizeDec, "size down"),
		}},
		{"Tabs", []helpEntry{
			he(km.NewTab, "new tab"),
			he(km.CloseTab, "close tab"),
			he(km.NextTab, "next tab"),
			he(km.PrevTab, "prev tab"),
		}},
		{"Keyboard cursor (no mouse)", []helpEntry{
			he(km.KbdCursorUp, "move cursor up"),
			he(km.KbdCursorDown, "move cursor down"),
			he(km.KbdCursorLeft, "move cursor left"),
			he(km.KbdCursorRight, "move cursor right"),
			he(km.KbdActivate, "click / release"),
		}},
	}
}

var mouseHelp = []string{
	"left click/drag — draw or act with the current tool",
	"middle-drag — pan the canvas",
	"scroll — zoom (over canvas), or adjust size/zoom (over their value)",
	"shift + drag — constrain: 45° angle for line/arrow, square/circle for rect/oval",
	"hover — shows tooltips, highlights buttons and the move/eraser target",
	"click a tab / its × — switch or close that tab",
}

const helpColWidth = 32

var (
	helpTitleStyle = lipgloss.NewStyle().Bold(true)
	helpKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
)

// viewHelp renders the full keybind + mouse-bind reference, replacing the
// canvas while open. Columns are built and joined as plain padded-line
// slices, zipped row by row — lipgloss.JoinHorizontal doesn't repeat a
// short block's gap across every row of a taller neighboring block, which
// made column separators vanish after the first line.
func (m Model) viewHelp() string {
	sections := m.helpSections()
	const numCols = 3
	cols := make([][]string, numCols)
	for i, s := range sections {
		lines := sectionLines(s)
		col := i % numCols
		if len(cols[col]) > 0 {
			cols[col] = append(cols[col], "")
		}
		cols[col] = append(cols[col], lines...)
	}

	maxRows := 0
	for _, col := range cols {
		if len(col) > maxRows {
			maxRows = len(col)
		}
	}

	var b strings.Builder
	for row := 0; row < maxRows; row++ {
		for _, col := range cols {
			line := ""
			if row < len(col) {
				line = col[row]
			}
			b.WriteString(padDisplay(line, helpColWidth))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + helpTitleStyle.Render("Mouse") + "\n")
	for _, l := range mouseHelp {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n" + dimStyle.Render("? or esc to close"))

	return modalStyle.Render(strings.TrimRight(b.String(), "\n"))
}

func sectionLines(s helpSection) []string {
	lines := []string{helpTitleStyle.Render(s.title)}
	for _, e := range s.entries {
		lines = append(lines, "  "+helpKeyStyle.Render(padDisplay(e.key, 14))+e.label)
	}
	return lines
}

// padDisplay right-pads s to width visible columns, measuring width with
// lipgloss so ANSI styling in s doesn't count toward the padding.
func padDisplay(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
