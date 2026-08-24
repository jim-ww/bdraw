package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// FooterRows is the fixed-height status row at the bottom. The header
// (tabs + toolbar) is variable height: it wraps to as many lines as needed
// to fit m.width, computed by headerHeight.
const FooterRows = 1

// canvasSize is the canvas viewport's size in cells.
func (m Model) canvasSize() (cols, rows int) {
	return m.width, m.height - m.headerHeight() - FooterRows
}

func (m Model) canvasCell(msg tea.MouseMsg) (col, row int, ok bool) {
	mouse := msg.Mouse()
	top := m.headerHeight()
	bottom := m.height - FooterRows
	if mouse.Y < top || mouse.Y >= bottom || mouse.X < 0 || mouse.X >= m.width {
		return 0, 0, false
	}
	return mouse.X, mouse.Y - top, true
}

// cellToPoint converts a screen cell in the canvas viewport to a world
// coordinate, accounting for pan (Offset) and zoom.
func (m Model) cellToPoint(col, row int) Point {
	d := m.doc()
	zoom := d.Zoom
	if zoom == 0 {
		zoom = 1
	}
	sx := float64(col)*SubpixW + SubpixW/2
	sy := float64(row)*SubpixH + SubpixH/2
	return Point{
		X: d.Offset.X + sx/zoom,
		Y: d.Offset.Y + sy/zoom,
	}
}

const (
	zoneNew    = "new"
	zoneOpen   = "open"
	zoneSave   = "save"
	zoneSaveAs = "saveas"
	zoneExport = "export"
	zoneClear  = "clear"
	zoneUndo   = "undo"
	zoneRedo   = "redo"

	zoneToolBrush  = "tool-brush"
	zoneToolRect   = "tool-rect"
	zoneToolCircle = "tool-circle"
	zoneToolLine   = "tool-line"
	zoneToolEraser = "tool-eraser"
	zoneToolSelect = "tool-select"
	zoneToolText   = "tool-text"
	zoneToolMove   = "tool-move"
	zoneToolFill   = "tool-fill"

	zoneSizeInc   = "size-inc"
	zoneSizeDec   = "size-dec"
	zoneSizeValue = "size-value"
	zoneZoomValue = "zoom-value"
	zoneGrid      = "grid"
	zoneFilled    = "filled"

	zoneColorButton = "color-button"
	zoneZoomIn      = "zoom-in"
	zoneZoomOut     = "zoom-out"

	zoneNewTab = "tab-new"
	zoneSlider = "number-slider"

	zoneCompact = "compact"
)

func zoneColor(i int) string    { return fmt.Sprintf("color-%d", i) }
func zoneTab(i int) string      { return fmt.Sprintf("tab-%d", i) }
func zoneTabClose(i int) string { return fmt.Sprintf("tabclose-%d", i) }

// zoneIDs enumerates every zone ID that might be on screen right now, for
// hit-testing mouse events against them.
func (m Model) zoneIDs() []string {
	ids := []string{
		zoneNew, zoneOpen, zoneSave, zoneSaveAs, zoneExport, zoneClear, zoneUndo, zoneRedo,
		zoneToolBrush, zoneToolRect, zoneToolCircle, zoneToolLine,
		zoneToolEraser, zoneToolSelect, zoneToolText, zoneToolMove, zoneToolFill,
		zoneSizeInc, zoneSizeDec, zoneSizeValue, zoneColorButton, zoneZoomIn, zoneZoomOut, zoneZoomValue,
		zoneGrid, zoneFilled, zoneNewTab, zoneCompact,
	}
	if m.mode == modeColorPicker {
		for i := range Palette {
			ids = append(ids, zoneColor(i))
		}
	}
	if m.mode == modeNumberEntry {
		ids = append(ids, zoneSlider)
	}
	for i := range m.tabs {
		// The close 'x' zone is nested inside the tab's own zone, so it
		// must be checked first — zoneAt returns the first match, and the
		// tab zone's bounding box also covers the 'x', so checking it
		// first would swallow every click meant for close.
		ids = append(ids, zoneTabClose(i), zoneTab(i))
	}
	return ids
}

func (m Model) handleZoneClick(id string) (tea.Model, tea.Cmd) {
	if m.mode == modeNumberEntry && id != zoneSizeValue && id != zoneZoomValue {
		m.mode = modeNormal
	}

	if m.mode == modeColorPicker {
		for i := range Palette {
			if id == zoneColor(i) {
				m.setColor(Palette[i])
				m.mode = modeNormal
				return m, nil
			}
		}
		return m, nil
	}

	switch id {
	case zoneNew:
		m.doNew()
	case zoneOpen:
		m.doOpen()
	case zoneSave:
		m.doSave()
	case zoneSaveAs:
		m.doSaveAs()
	case zoneExport:
		m.doExport()
	case zoneClear:
		m.doClearCanvas()
	case zoneUndo:
		m.doUndo()
	case zoneRedo:
		m.doRedo()

	case zoneToolBrush:
		m.setTool(ToolBrush)
	case zoneToolRect:
		m.setTool(ToolRect)
	case zoneToolCircle:
		m.setTool(ToolCircle)
	case zoneToolLine:
		m.setTool(ToolLine)
	case zoneToolEraser:
		m.setTool(ToolEraser)
	case zoneToolSelect:
		m.setTool(ToolSelect)
	case zoneToolText:
		m.setTool(ToolText)
	case zoneToolMove:
		m.setTool(ToolMove)
	case zoneToolFill:
		m.setTool(ToolFill)

	case zoneSizeInc:
		m.sizeInc()
	case zoneSizeDec:
		m.sizeDec()
	case zoneSizeValue:
		m.startNumberEntry("size", m.size)
	case zoneColorButton:
		m.openColorPicker()
	case zoneZoomIn:
		m.zoomAtCursor(zoomStep)
	case zoneZoomOut:
		m.zoomAtCursor(1 / zoomStep)
	case zoneZoomValue:
		m.startNumberEntry("zoom", m.doc().Zoom*100)
	case zoneGrid:
		m.showGrid = !m.showGrid
	case zoneFilled:
		m.filled = !m.filled
	case zoneCompact:
		m.compact = !m.compact
	case zoneNewTab:
		m.doNewTab()

	default:
		for i := range m.tabs {
			if id == zoneTab(i) {
				m.doSelectTab(i)
				return m, nil
			}
			if id == zoneTabClose(i) {
				m.doCloseTab(i)
				return m, nil
			}
		}
	}
	return m, nil
}
