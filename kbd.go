package main

import tea "charm.land/bubbletea/v2"

// Keyboard-only operation: Shift+Arrow moves a virtual "keyboard cursor"
// (m.kbdCol, m.kbdRow, in absolute terminal coordinates — the same space
// real mouse events arrive in), and space toggles a simulated left-button
// press/release at that position.
//
// Rather than reimplementing tool/button dispatch for keyboard input, both
// operations synthesize the exact same tea.MouseClickMsg / MouseMotionMsg /
// MouseReleaseMsg values a real mouse would produce and feed them through
// the existing handleMouse pipeline. That's what makes every tool, every
// toolbar button, hover highlighting, and tooltips work identically
// whether driven by mouse or keyboard, for free.

// kbdEnsureInit lazily centers the keyboard cursor on first use, so
// there's no separate "enable keyboard mode" step to discover.
func (m *Model) kbdEnsureInit() {
	if m.kbdInit || m.width == 0 {
		return
	}
	m.kbdInit = true
	m.kbdCol, m.kbdRow = m.width/2, m.height/2
}

// kbdMove shifts the keyboard cursor by one cell and reports it via a
// synthetic motion event — with the left button "held" if a keyboard-driven
// drag is in progress, so toolDrag keeps advancing, exactly like real
// mouse motion during a drag.
func (m *Model) kbdMove(dCol, dRow int) {
	m.kbdEnsureInit()
	if m.width == 0 {
		return
	}
	m.kbdCol += dCol
	m.kbdRow += dRow
	if m.kbdCol < 0 {
		m.kbdCol = 0
	}
	if m.kbdCol >= m.width {
		m.kbdCol = m.width - 1
	}
	if m.kbdRow < 0 {
		m.kbdRow = 0
	}
	if m.kbdRow >= m.height {
		m.kbdRow = m.height - 1
	}

	btn := tea.MouseNone
	if m.dragging {
		btn = tea.MouseLeft
	}
	m.dispatchSynthetic(tea.MouseMotionMsg{X: m.kbdCol, Y: m.kbdRow, Button: btn})
}

// kbdActivate simulates a left-button press, or — if a drag is already in
// progress from a previous Activate — its release. Click-only actions
// (buttons, fill, eyedropper, text placement) complete entirely on the
// press and never set m.dragging, so a second Activate there simply starts
// a fresh action, matching how a real mouse click works.
func (m *Model) kbdActivate() {
	m.kbdEnsureInit()
	if m.width == 0 {
		return
	}
	if m.dragging {
		m.dispatchSynthetic(tea.MouseReleaseMsg{X: m.kbdCol, Y: m.kbdRow, Button: tea.MouseLeft})
		return
	}
	m.dispatchSynthetic(tea.MouseClickMsg{X: m.kbdCol, Y: m.kbdRow, Button: tea.MouseLeft})
}

// dispatchSynthetic feeds a synthetic mouse message through the real mouse
// pipeline and applies the resulting state back onto m.
func (m *Model) dispatchSynthetic(msg tea.MouseMsg) {
	result, _ := m.handleMouse(msg)
	*m = result.(Model)
}
