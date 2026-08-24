package main

import "fmt"

func (m *Model) setTool(t Tool) {
	m.tool = t
	m.status = "tool: " + string(t)
}

func (m *Model) setColor(c string) {
	m.color = c
}

func (m *Model) sizeInc() {
	m.setSizeIndex(m.sizeIndex() + 1)
}

func (m *Model) sizeDec() {
	m.setSizeIndex(m.sizeIndex() - 1)
}

func (m *Model) sizeIndex() int {
	for i, s := range Sizes {
		if s == m.size {
			return i
		}
	}
	return 0
}

func (m *Model) setSizeIndex(i int) {
	if i < 0 {
		i = 0
	}
	if i >= len(Sizes) {
		i = len(Sizes) - 1
	}
	m.size = Sizes[i]
}

func (m *Model) openColorPicker() {
	m.mode = modeColorPicker
	m.input.SetValue(m.color)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *Model) zoomBy(factor float64) {
	d := m.doc()
	if d.Zoom == 0 {
		d.Zoom = 1
	}
	d.Zoom *= factor
	if d.Zoom < zoomMin {
		d.Zoom = zoomMin
	}
	if d.Zoom > zoomMax {
		d.Zoom = zoomMax
	}
	m.status = fmt.Sprintf("zoom %.0f%%", d.Zoom*100)
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
