package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (m *Model) setTool(t Tool) {
	m.tool = t
	m.status = "tool: " + string(t)
}

func (m *Model) setColor(c string) {
	m.color = c
}

// sizeInc/sizeDec step brush size multiplicatively rather than through a
// short fixed list, so there's no arbitrary ceiling on how fat a brush can
// get: repeated presses keep scaling it up to sizeMax.
func (m *Model) sizeInc() {
	m.setSize(m.size * sizeStep)
}

func (m *Model) sizeDec() {
	m.setSize(m.size / sizeStep)
}

func (m *Model) setSize(s float64) {
	if s < sizeMin {
		s = sizeMin
	}
	if s > sizeMax {
		s = sizeMax
	}
	m.size = s
}

func (m *Model) openColorPicker() {
	m.mode = modeColorPicker
	m.input.SetValue(m.color)
	m.input.CursorEnd()
	m.input.Focus()
}

// zoomBy changes zoom around the viewport center.
func (m *Model) zoomBy(factor float64) {
	cols, rows := m.canvasSize()
	m.zoomAt(factor, cols/2, rows/2)
}

// zoomAt changes zoom while keeping the world point currently under screen
// cell (col, row) fixed on screen, so zooming in/out tracks the cursor
// (or, for keyboard zoom, wherever it last was) instead of the view
// jumping to re-center itself.
func (m *Model) zoomAt(factor float64, col, row int) {
	d := m.doc()
	if d.Zoom == 0 {
		d.Zoom = 1
	}
	worldBefore := m.cellToPoint(col, row)

	newZoom := d.Zoom * factor
	if newZoom < zoomMin {
		newZoom = zoomMin
	}
	if newZoom > zoomMax {
		newZoom = zoomMax
	}
	d.Zoom = newZoom

	sx := float64(col)*SubpixW + SubpixW/2
	sy := float64(row)*SubpixH + SubpixH/2
	d.Offset.X = worldBefore.X - sx/newZoom
	d.Offset.Y = worldBefore.Y - sy/newZoom

	m.status = fmt.Sprintf("zoom %.0f%%", d.Zoom*100)
}

// zoomAtCursor zooms centered on the last known mouse position, if any,
// falling back to the viewport center — used by the keyboard zoom keys and
// the toolbar +/- buttons, which have no mouse position of their own.
func (m *Model) zoomAtCursor(factor float64) {
	if m.cursorVisible {
		m.zoomAt(factor, m.cursorCol, m.cursorRow)
		return
	}
	m.zoomBy(factor)
}

func (m *Model) panBy(dx, dy float64) {
	d := m.doc()
	d.Offset.X += dx
	d.Offset.Y += dy
}

func (m *Model) doUndo() {
	m.doc().Undo()
}

func (m *Model) doRedo() {
	m.doc().Redo()
}

func (m *Model) doNewTab() {
	m.tabs = append(m.tabs, NewDocument())
	m.active = len(m.tabs) - 1
}

func (m *Model) doSelectTab(i int) {
	if i >= 0 && i < len(m.tabs) {
		m.active = i
	}
}

// doCloseTab closes tab i, prompting for confirmation first if it has
// unsaved changes.
func (m *Model) doCloseTab(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	if m.tabs[i].Dirty {
		m.mode = modeConfirmClose
		m.pendingCloseIdx = i
		return
	}
	m.closeTab(i)
}

func (m *Model) closeTab(i int) {
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if len(m.tabs) == 0 {
		m.tabs = []*Document{NewDocument()}
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
}

// doDeleteSelected removes every edit currently marked Selected (select
// tool). No-op, and no undo point, if nothing is selected.
func (m *Model) doDeleteSelected() {
	d := m.doc()
	var removedAny bool
	for _, e := range d.Edits {
		if e.Selected {
			removedAny = true
			break
		}
	}
	if !removedAny {
		return
	}
	d.BeginChange()
	kept := d.Edits[:0]
	for _, e := range d.Edits {
		if !e.Selected {
			kept = append(kept, e)
		}
	}
	d.Edits = kept
}

func (m *Model) doNew() {
	m.doNewTab()
}

func (m *Model) doSave() {
	d := m.doc()
	if d.Path == "" {
		m.startPrompt(modePromptSaveAs, "")
		return
	}
	m.saveTo(d.Path)
}

func (m *Model) doSaveAs() {
	m.startPrompt(modePromptSaveAs, m.doc().Path)
}

func (m *Model) doOpen() {
	m.startPrompt(modePromptOpen, "")
}

// doExport opens the save-as prompt prefilled with a .png path, since a
// plain "Save" button doesn't make the PNG export option discoverable.
func (m *Model) doExport() {
	base := m.doc().Path
	if base == "" {
		base = "untitled"
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	m.startPrompt(modePromptSaveAs, base+".png")
}

// doClearCanvas empties the current document, prompting for confirmation
// first unless it's already empty.
func (m *Model) doClearCanvas() {
	if len(m.doc().Edits) == 0 {
		return
	}
	m.mode = modeConfirmClear
}

func (m *Model) clearCanvas() {
	d := m.doc()
	d.BeginChange()
	d.Edits = nil
}

// clearSelection deselects every edit (esc, select tool).
func (m *Model) clearSelection() {
	d := m.doc()
	for _, e := range d.Edits {
		e.Selected = false
	}
	d.Touch()
}

func (m *Model) saveTo(path string) {
	d := m.doc()
	var err error
	if IsPNGPath(path) {
		err = ExportPNG(d, path)
	} else {
		err = d.Save(path)
	}
	if err != nil {
		m.status = fmt.Sprintf("save failed: %v", err)
		return
	}
	m.status = "saved " + path
}

func (m *Model) openFrom(path string) {
	d, err := LoadDocument(path)
	if err != nil {
		m.status = fmt.Sprintf("open failed: %v", err)
		return
	}
	m.tabs = append(m.tabs, d)
	m.active = len(m.tabs) - 1
	m.status = "opened " + path
}
