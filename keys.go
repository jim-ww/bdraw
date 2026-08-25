package main

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Switching tools mid-drag left m.tool and m.dragEdit inconsistent —
	// e.g. a 1-point brush stroke, still being dragged, reinterpreted on
	// the next motion event as a 2-point shape because m.tool had
	// changed — and crashed indexing Points[1] on an edit that only had
	// one point. A drag has to finish (or be abandoned by releasing the
	// button) before the tool can change, same as every mouse-based paint
	// program.
	if m.dragging && isToolSwitchKey(m.km, msg) {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.km.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.km.ToolBrush):
		m.setTool(ToolBrush)
	case key.Matches(msg, m.km.ToolRect):
		m.setTool(ToolRect)
	case key.Matches(msg, m.km.ToolCircle):
		m.setTool(ToolCircle)
	case key.Matches(msg, m.km.ToolLine):
		m.setTool(ToolLine)
	case key.Matches(msg, m.km.ToolEraser):
		m.setTool(ToolEraser)
	case key.Matches(msg, m.km.ToolSelect):
		m.setTool(ToolSelect)
	case key.Matches(msg, m.km.ToolText):
		m.setTool(ToolText)
	case key.Matches(msg, m.km.ToolMove):
		m.setTool(ToolMove)
	case key.Matches(msg, m.km.ToolFill):
		m.setTool(ToolFill)
	case key.Matches(msg, m.km.ToolArrow):
		m.setTool(ToolArrow)
	case key.Matches(msg, m.km.ToolEyedropper):
		m.setTool(ToolEyedropper)

	case key.Matches(msg, m.km.Undo):
		m.doUndo()
	case key.Matches(msg, m.km.Redo):
		m.doRedo()
	case key.Matches(msg, m.km.Delete):
		m.doDeleteSelected()
	case key.Matches(msg, m.km.ClearSelection):
		m.clearSelection()
	case key.Matches(msg, m.km.Copy):
		m.doCopy()
	case key.Matches(msg, m.km.Paste):
		m.doPaste()

	case key.Matches(msg, m.km.New):
		m.doNew()
	case key.Matches(msg, m.km.Open):
		m.doOpen()
	case key.Matches(msg, m.km.Save):
		m.doSave()
	case key.Matches(msg, m.km.SaveAs):
		m.doSaveAs()
	case key.Matches(msg, m.km.Export):
		m.doExport()
	case key.Matches(msg, m.km.Clear):
		m.doClearCanvas()

	case key.Matches(msg, m.km.ColorPicker):
		m.openColorPicker()
	case key.Matches(msg, m.km.ToggleGrid):
		m.showGrid = !m.showGrid
	case key.Matches(msg, m.km.ToggleSnap):
		m.snap = !m.snap
	case key.Matches(msg, m.km.ToggleFill):
		m.filled = !m.filled
	case key.Matches(msg, m.km.ToggleCompact):
		m.compact = !m.compact

	case key.Matches(msg, m.km.NewTab):
		m.doNewTab()
	case key.Matches(msg, m.km.CloseTab):
		if m.doCloseTab(m.active) {
			return m, tea.Quit
		}
	case key.Matches(msg, m.km.NextTab):
		m.doSelectTab((m.active + 1) % len(m.tabs))
	case key.Matches(msg, m.km.PrevTab):
		m.doSelectTab((m.active - 1 + len(m.tabs)) % len(m.tabs))

	case key.Matches(msg, m.km.SizeInc):
		m.sizeInc()
	case key.Matches(msg, m.km.SizeDec):
		m.sizeDec()

	case key.Matches(msg, m.km.PanUp):
		m.panBy(0, -panStep)
	case key.Matches(msg, m.km.PanDown):
		m.panBy(0, panStep)
	case key.Matches(msg, m.km.PanLeft):
		m.panBy(-panStep, 0)
	case key.Matches(msg, m.km.PanRight):
		m.panBy(panStep, 0)
	case key.Matches(msg, m.km.ZoomIn):
		m.zoomAtCursor(zoomStep)
	case key.Matches(msg, m.km.ZoomOut):
		m.zoomAtCursor(1 / zoomStep)

	case key.Matches(msg, m.km.KbdCursorUp):
		m.kbdMove(0, -1)
	case key.Matches(msg, m.km.KbdCursorDown):
		m.kbdMove(0, 1)
	case key.Matches(msg, m.km.KbdCursorLeft):
		m.kbdMove(-1, 0)
	case key.Matches(msg, m.km.KbdCursorRight):
		m.kbdMove(1, 0)
	case key.Matches(msg, m.km.KbdActivate):
		m.kbdActivate()
	}
	return m, nil
}

// isToolSwitchKey reports whether msg matches any tool-selection binding.
func isToolSwitchKey(km KeyMap, msg tea.KeyMsg) bool {
	return key.Matches(msg,
		km.ToolBrush, km.ToolRect, km.ToolCircle, km.ToolLine, km.ToolEraser,
		km.ToolSelect, km.ToolText, km.ToolMove, km.ToolFill, km.ToolArrow, km.ToolEyedropper,
	)
}
