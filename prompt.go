package main

import tea "charm.land/bubbletea/v2"

// digitKey reports whether s is a single ASCII digit, and its value.
func digitKey(s string) (int, bool) {
	if len(s) != 1 || s[0] < '0' || s[0] > '9' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

func (m *Model) startPrompt(mo mode, prefill string) {
	m.mode = mo
	m.input.SetValue(prefill)
	m.input.CursorEnd()
	m.input.Focus()
}

// handlePromptKey handles keystrokes while a modal text prompt (save as,
// open, text tool entry, or close confirmation) has focus.
func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmClose {
		switch msg.String() {
		case "y", "enter":
			m.closeTab(m.pendingCloseIdx)
		}
		m.mode = modeNormal
		return m, nil
	}

	if m.mode == modeConfirmClear {
		switch msg.String() {
		case "y", "enter":
			m.clearCanvas()
		}
		m.mode = modeNormal
		return m, nil
	}

	// While the open prompt's path field is empty, digits pick a recent
	// file directly instead of being typed — a blank field has nothing
	// else a digit could mean.
	if m.mode == modePromptOpen && m.input.Value() == "" {
		if n, ok := digitKey(msg.String()); ok && n >= 1 && n <= len(m.recent) {
			m.openFrom(m.recent[n-1])
			m.mode = modeNormal
			m.input.Blur()
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil

	case "enter":
		value := m.input.Value()
		switch m.mode {
		case modePromptSaveAs:
			if value != "" {
				m.saveTo(value)
			}
		case modePromptOpen:
			if value != "" {
				m.openFrom(value)
			}
		case modeTextEntry:
			if value != "" {
				d := m.doc()
				d.BeginChange()
				d.Edits = append(d.Edits, &Edit{
					ID: d.NextID(), Kind: KindText,
					Points: []Point{m.textPos}, Color: m.color, Text: value,
				})
			}
		}
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
