package main

import (
	"regexp"

	tea "charm.land/bubbletea/v2"
)

var (
	hexColor6RE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	hexColor3RE = regexp.MustCompile(`^#([0-9a-fA-F])([0-9a-fA-F])([0-9a-fA-F])$`)
)

// normalizeHexColor accepts both #rrggbb and shorthand #rgb (each digit
// doubled, as in CSS), returning the full 6-digit form and whether input
// was valid at all.
func normalizeHexColor(v string) (string, bool) {
	if hexColor6RE.MatchString(v) {
		return v, true
	}
	if m := hexColor3RE.FindStringSubmatch(v); m != nil {
		return "#" + m[1] + m[1] + m[2] + m[2] + m[3] + m[3], true
	}
	return "", false
}

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
		if v, ok := normalizeHexColor(m.input.Value()); ok {
			m.setColor(v)
		} else {
			m.status = "invalid color, expected #rgb or #rrggbb"
		}
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "tab":
		m.colorPickerFocus = (m.colorPickerFocus + 1) % len(Palette)
		m.setColor(Palette[m.colorPickerFocus])
		m.input.SetValue(Palette[m.colorPickerFocus])
		m.input.CursorEnd()
		return m, nil
	case "shift+tab":
		m.colorPickerFocus = (m.colorPickerFocus - 1 + len(Palette)) % len(Palette)
		m.setColor(Palette[m.colorPickerFocus])
		m.input.SetValue(Palette[m.colorPickerFocus])
		m.input.CursorEnd()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
