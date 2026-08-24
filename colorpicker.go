package main

import (
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

var hexColorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// handleColorPickerKey handles keystrokes while the color picker modal is
// open: palette swatches are clicked (see zones.go), and a hex code can be
// typed and confirmed with Enter.
func (m Model) handleColorPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		if v := m.input.Value(); hexColorRE.MatchString(v) {
			m.setColor(v)
		} else {
			m.status = "invalid color, expected #rrggbb"
		}
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
