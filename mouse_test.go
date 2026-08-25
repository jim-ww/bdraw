package main

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// TestToolSwitchMidDragDoesNotCrash reproduces the exact reported panic:
// start a brush stroke (1-point edit), switch tools mid-drag via keyboard,
// then keep moving — this used to index Points[1] on a 1-point edit.
func TestToolSwitchMidDragDoesNotCrash(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 100, 40
	m.tool = ToolBrush

	m.dispatchSynthetic(tea.MouseClickMsg{X: 50, Y: 20, Button: tea.MouseLeft})
	if !m.dragging || m.dragEdit == nil || len(m.dragEdit.Points) != 1 {
		t.Fatalf("expected a 1-point brush stroke in progress, got dragging=%v dragEdit=%v", m.dragging, m.dragEdit)
	}

	rMsg := tea.KeyPressMsg{Text: "r", Code: 'r'}
	newM, _ := m.handleKey(rMsg)
	m = newM.(Model)

	if m.tool != ToolBrush {
		t.Fatalf("expected tool switch to be ignored mid-drag, got %v", m.tool)
	}
	if !key.Matches(rMsg, m.km.ToolRect) {
		t.Fatal("test setup broken: 'r' doesn't match ToolRect binding")
	}

	m.dispatchSynthetic(tea.MouseMotionMsg{X: 55, Y: 25, Button: tea.MouseLeft})
	m.dispatchSynthetic(tea.MouseReleaseMsg{X: 55, Y: 25, Button: tea.MouseLeft})

	if m.doc().Edits[0].Kind != KindStroke {
		t.Fatalf("expected the finished edit to still be a stroke, got %v", m.doc().Edits[0].Kind)
	}
}

// TestReleaseOverToolbarMidDragFinishes reproduces the "stuck drag" bug:
// releasing the mouse over a toolbar zone mid-drag never called toolUp,
// since MouseReleaseMsg never matched the zone-click branch.
func TestReleaseOverToolbarMidDragFinishes(t *testing.T) {
	m := NewModel("")
	m.width, m.height = 100, 40
	m.tool = ToolBrush
	_ = m.View() // populate zones

	m.dispatchSynthetic(tea.MouseClickMsg{X: 50, Y: 20, Button: tea.MouseLeft})
	if !m.dragging {
		t.Fatal("expected drag to start")
	}

	// Release up in the toolbar area (row 1), well outside the canvas.
	m.dispatchSynthetic(tea.MouseReleaseMsg{X: 5, Y: 1, Button: tea.MouseLeft})

	if m.dragging {
		t.Fatal("drag stuck true after releasing over a toolbar zone")
	}
}
