package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
	top := m.headerHeight()
	bottom := m.height - FooterRows
	if msg.Y < top || msg.Y >= bottom || msg.X < 0 || msg.X >= m.width {
		return 0, 0, false
	}
	return msg.X, msg.Y - top, true
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

	zoneSizeInc = "size-inc"
	zoneSizeDec = "size-dec"

	zoneColorButton = "color-button"
	zoneZoomIn      = "zoom-in"
	zoneZoomOut     = "zoom-out"

	zoneNewTab = "tab-new"
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
		zoneSizeInc, zoneSizeDec, zoneColorButton, zoneZoomIn, zoneZoomOut,
		zoneNewTab,
	}
	if m.mode == modeColorPicker {
		for i := range Palette {
			ids = append(ids, zoneColor(i))
		}
	}
	for i := range m.tabs {
		ids = append(ids, zoneTab(i), zoneTabClose(i))
	}
	return ids
}

func (m Model) handleZoneClick(id string) (tea.Model, tea.Cmd) {
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
	case zoneColorButton:
		m.openColorPicker()
	case zoneZoomIn:
		m.zoomBy(zoomStep)
	case zoneZoomOut:
		m.zoomBy(1 / zoomStep)
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
