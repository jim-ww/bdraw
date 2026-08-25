package main

import (
	"fmt"
	"math"
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
	// Round to 1 decimal: repeated multiplicative steps (sizeInc/sizeDec)
	// otherwise drift into float noise like 2.3424343243243.
	m.size = math.Round(s*10) / 10
}

// hardnessInc/hardnessDec step brush hardness by a fixed amount, additive
// rather than multiplicative like size/zoom — the whole range
// (hardnessMin..100) spans less than one decade, so a multiplicative
// step would feel uneven across it.
func (m *Model) hardnessInc() {
	m.setHardness(m.hardness + hardnessStep)
}

func (m *Model) hardnessDec() {
	m.setHardness(m.hardness - hardnessStep)
}

func (m *Model) setHardness(h float64) {
	if h < hardnessMin {
		h = hardnessMin
	}
	if h > hardnessMax {
		h = hardnessMax
	}
	m.hardness = math.Round(h)
}

func (m *Model) openColorPicker() {
	m.mode = modeColorPicker
	m.colorPickerFocus = -1
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
	worldBefore := m.cellToPoint(col, row)

	newZoom := m.viewZoom() * factor
	if newZoom < zoomMin {
		newZoom = zoomMin
	}
	if newZoom > zoomMax {
		newZoom = zoomMax
	}
	// Round the same way setSize does: repeated multiplicative steps
	// otherwise drift into float noise in the displayed percentage.
	newZoom = math.Round(newZoom*1000) / 1000
	m.setViewZoom(newZoom)

	sx := float64(col)*SubpixW + SubpixW/2
	sy := float64(row)*SubpixH + SubpixH/2
	m.setViewOffset(Point{
		X: worldBefore.X - sx/newZoom,
		Y: worldBefore.Y - sy/newZoom,
	})

	m.status = fmt.Sprintf("zoom %.0f%%", newZoom*100)
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
	offset := m.viewOffset()
	offset.X += dx
	offset.Y += dy
	m.setViewOffset(offset)
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
// doCloseTab closes tab i, prompting for confirmation first if it has
// unsaved changes. Reports whether the program should quit as a result
// (closing the last tab), so the caller can issue tea.Quit.
func (m *Model) doCloseTab(i int) bool {
	if i < 0 || i >= len(m.tabs) {
		return false
	}
	if m.tabs[i].Dirty {
		m.mode = modeConfirmClose
		m.pendingCloseIdx = i
		return false
	}
	return m.closeTab(i)
}

// closeTab removes tab i. Closing the last remaining tab quits the
// program, matching what almost every tabbed editor does, rather than
// silently replacing it with a blank Untitled document.
func (m *Model) closeTab(i int) bool {
	// Closing the last tab normally quits the program — fine solo, but in
	// a collab session that would wipe the shared session down to zero
	// tabs out from under every other connected peer. Refuse instead;
	// whoever wants to actually end the session can just disconnect (a
	// guest) or quit (the host), which doesn't touch shared state.
	if m.hub != nil && len(m.tabs) <= 1 {
		return false
	}
	clearAutosave(m.tabs[i])
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if len(m.tabs) == 0 {
		return true
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	return false
}

// pasteOffset nudges a paste from its source position (world subpixels),
// so pasting repeatedly builds a visible staircase instead of every copy
// landing exactly on top of the last.
const pasteOffset = 12

// doCopy snapshots every currently selected edit into the in-app
// clipboard. No-op if nothing is selected — a copy shouldn't silently
// clear a previous, still-useful clipboard.
func (m *Model) doCopy() {
	var copied []*Edit
	for _, e := range m.doc().Edits {
		if e.Selected {
			copied = append(copied, e.Clone())
		}
	}
	if len(copied) == 0 {
		return
	}
	m.clipboard = copied
	m.status = fmt.Sprintf("copied %d edit(s)", len(copied))
}

// doPaste inserts fresh copies of the clipboard, offset slightly, selected
// and ready to move — mirrors doCopy in ignoring an empty clipboard rather
// than starting an undo point for nothing.
func (m *Model) doPaste() {
	if len(m.clipboard) == 0 {
		return
	}
	d := m.doc()
	d.BeginChange()
	for _, e := range d.Edits {
		e.Selected = false
	}
	for _, src := range m.clipboard {
		e := src.Clone()
		e.ID = d.NextID()
		e.Selected = true
		for i := range e.Points {
			e.Points[i].X += pasteOffset
			e.Points[i].Y += pasteOffset
		}
		d.Edits = append(d.Edits, e)
	}
	m.tool = ToolMove
	m.status = fmt.Sprintf("pasted %d edit(s)", len(m.clipboard))
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
	switch {
	case IsPNGPath(path):
		if err := ExportPNG(d, path); err != nil {
			m.status = fmt.Sprintf("save failed: %v", err)
			return
		}
		m.status = "saved " + path

	case IsSVGPath(path):
		skipped, err := ExportSVG(d, path)
		if err != nil {
			m.status = fmt.Sprintf("save failed: %v", err)
			return
		}
		m.status = "saved " + path
		if skipped > 0 {
			m.status += fmt.Sprintf(" (%d fill(s) skipped — not representable in SVG)", skipped)
		}

	default:
		if err := d.Save(path); err != nil {
			m.status = fmt.Sprintf("save failed: %v", err)
			return
		}
		m.status = "saved " + path
		m.rememberFile(path)
	}
}

// openInitialFile loads path (given as a CLI argument) as the starting
// document, replacing the blank Untitled tab NewModel already created —
// unlike openFrom, which always adds a new tab. Failure just leaves the
// blank tab in place with an error status, rather than refusing to start.
func (m *Model) openInitialFile(path string) {
	d, err := LoadDocument(path)
	if err != nil {
		m.status = fmt.Sprintf("open failed: %v", err)
		return
	}
	m.tabs[0] = d
	m.active = 0
	m.status = "opened " + path
	m.rememberFile(path)
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
	m.rememberFile(path)
}

// rememberFile adds path to the recent-files list (JSON documents only —
// PNG/SVG exports aren't reopenable) and persists it.
func (m *Model) rememberFile(path string) {
	if IsPNGPath(path) || IsSVGPath(path) {
		return
	}
	m.recent = rememberRecentFile(m.recent, path)
	saveRecentFiles(m.recent)
}
