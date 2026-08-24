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
}

// NewDocument returns an empty, untitled document.
func NewDocument() *Document {
	return &Document{Zoom: 1}
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
}

// Undo reverts to the previous committed state, if any.
func (d *Document) Undo() {
	if prev, ok := d.History.Undo(d.snapshot()); ok {
		d.Edits = prev
		d.Dirty = true
	}
}

// Redo reapplies a state undone by Undo, if any.
func (d *Document) Redo() {
	if next, ok := d.History.Redo(d.snapshot()); ok {
		d.Edits = next
		d.Dirty = true
	}
}
