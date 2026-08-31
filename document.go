package main

import "path/filepath"

// Document is one canvas: its edits plus bookkeeping for saving and undo.
type Document struct {
	Path    string
	Dirty   bool
	Edits   []*Edit
	Offset  Point // world coordinate shown at the viewport's top-left
	Zoom    float64
	nextID  int
	History History

	// Version increments on every content change (a new/removed edit, a
	// point moved, a color or selection flipped — anything that could
	// change what gets rendered). The canvas render cache in Model keys
	// off it to know when a full re-rasterize is actually necessary,
	// versus when only the mouse cursor moved.
	Version int

	// autosaveID names this document's recovery file for the life of the
	// session (see autosave.go) — stable even before it has a real Path.
	autosaveID string

	// fillCache memoizes each fill edit's boundedness check across renders
	// that share the same Version — see FillBoundCache.
	fillCache FillBoundCache
}

// NewDocument returns an empty, untitled document.
func NewDocument() *Document {
	return &Document{Zoom: 1, autosaveID: newAutosaveID()}
}

// Title is what a tab shows: the base filename, or "Untitled" plus a dirty
// marker.
func (d *Document) Title() string {
	name := "Untitled"
	if d.Path != "" {
		name = filepath.Base(d.Path)
	}
	if d.Dirty {
		name += "*"
	}
	return name
}

// NextID returns a fresh, document-unique edit ID.
func (d *Document) NextID() int {
	d.nextID++
	return d.nextID
}

// Touch marks the document as changed for this frame's render cache. Call
// it after any mutation to Edits or to an edit's fields.
func (d *Document) Touch() {
	d.Version++
}

// snapshot deep-copies the current edit list, for pushing onto history.
func (d *Document) snapshot() []*Edit {
	s := make([]*Edit, len(d.Edits))
	for i, e := range d.Edits {
		s[i] = e.Clone()
	}
	return s
}

// BeginChange records the current state as an undo point. Call it once,
// before mutating d.Edits for a new discrete action (a stroke, a shape
// drag, an eraser drag, a text placement).
func (d *Document) BeginChange() {
	d.History.Push(d.snapshot())
	d.Dirty = true
	d.Touch()
}

// Undo reverts to the previous committed state, if any.
func (d *Document) Undo() {
	if prev, ok := d.History.Undo(d.snapshot()); ok {
		d.Edits = prev
		d.Dirty = true
		d.Touch()
	}
}

// Redo reapplies a state undone by Undo, if any.
func (d *Document) Redo() {
	if next, ok := d.History.Redo(d.snapshot()); ok {
		d.Edits = next
		d.Dirty = true
		d.Touch()
	}
}
