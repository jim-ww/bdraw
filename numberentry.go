package main

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Shared click-to-type-a-number component, used by both the size and zoom
// toolbar controls: click the value to start typing, digits and a single
// '.' are accepted (no minus sign — negative brush size or zoom is
// meaningless), enter confirms and clamps into range, esc cancels leaving
// the old value untouched.

func (m *Model) startNumberEntry(target string, current float64) {
	m.mode = modeNumberEntry
	m.numEntryTarget = target
	m.numEntryValue = strconv.FormatFloat(current, 'f', -1, 64)
}

func (m Model) handleNumberEntryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil

	case "enter":
		if v, err := strconv.ParseFloat(m.numEntryValue, 64); err == nil && v >= 0 {
			switch m.numEntryTarget {
			case "size":
				m.setSize(v)
			case "zoom":
				m.zoomAtCursor(v / 100 / m.doc().Zoom)
			}
		} else {
			m.status = "invalid number"
		}
		m.mode = modeNormal
		return m, nil

	case "backspace":
		if n := len(m.numEntryValue); n > 0 {
			m.numEntryValue = m.numEntryValue[:n-1]
		}
		return m, nil
	}

	s := msg.String()
	if len(s) == 1 && strings.ContainsRune("0123456789", rune(s[0])) {
		m.numEntryValue += s
	} else if s == "." && !strings.Contains(m.numEntryValue, ".") {
		m.numEntryValue += s
	}
	return m, nil
}

// numberEntryLabel renders either the live typed value (while entering) or
// the current value, for the toolbar.
func (m Model) numberEntryLabel(target, suffix string, current float64) string {
	if m.mode == modeNumberEntry && m.numEntryTarget == target {
		return m.numEntryValue + "_"
	}
	return strconv.FormatFloat(current, 'f', 0, 64) + suffix
}
