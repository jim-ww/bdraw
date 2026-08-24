package main

import "charm.land/bubbles/v2/key"

// bindingTooltips maps a zone ID to the KeyMap binding that does the same
// thing, so hovering a button can show its keyboard shortcut.
func (m Model) bindingTooltips() map[string]key.Binding {
	km := m.km
	return map[string]key.Binding{
		zoneNew:    km.New,
		zoneOpen:   km.Open,
		zoneSave:   km.Save,
		zoneSaveAs: km.SaveAs,
		zoneExport: km.Export,
		zoneClear:  km.Clear,
		zoneUndo:   km.Undo,
		zoneRedo:   km.Redo,

		zoneToolBrush:  km.ToolBrush,
		zoneToolRect:   km.ToolRect,
		zoneToolCircle: km.ToolCircle,
		zoneToolLine:   km.ToolLine,
		zoneToolEraser: km.ToolEraser,
		zoneToolSelect: km.ToolSelect,
		zoneToolMove:   km.ToolMove,
		zoneToolFill:   km.ToolFill,
		zoneToolText:   km.ToolText,

		zoneColorButton: km.ColorPicker,
		zoneSizeInc:     km.SizeInc,
		zoneSizeDec:     km.SizeDec,
		zoneZoomIn:      km.ZoomIn,
		zoneZoomOut:     km.ZoomOut,
		zoneGrid:        km.ToggleGrid,
		zoneFilled:      km.ToggleFill,
		zoneNewTab:      km.NewTab,
	}
}

// literalTooltips covers zones with no single KeyMap binding behind them.
var literalTooltips = map[string]string{
	zoneSizeValue: "click to type a size",
	zoneZoomValue: "click to type a zoom level",
}

// tooltip returns the hover hint for a zone ID, or "" if it has none (e.g.
// a tab, which has no fixed keybinding).
func (m Model) tooltip(id string) string {
	if id == "" {
		return ""
	}
	if s, ok := literalTooltips[id]; ok {
		return s
	}
	if b, ok := m.bindingTooltips()[id]; ok {
		h := b.Help()
		if h.Key == "" {
			return h.Desc
		}
		return h.Desc + " (" + h.Key + ")"
	}
	return ""
}
