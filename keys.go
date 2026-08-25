package main

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.doCloseTab(m.active)
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
	}
	return m, nil
}
