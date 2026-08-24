package main

// History is a snapshot-based undo/redo stack. It trades memory for
// simplicity and correctness: each undo point is a full copy of the edit
// list, so undo/redo can never diverge from what was actually drawn.
type History struct {
	undo [][]*Edit
	redo [][]*Edit
}

// Push records state as a new undo point and clears the redo stack.
func (h *History) Push(state []*Edit) {
	h.undo = append(h.undo, state)
	h.redo = nil
}

// Undo pops the last undo point, pushes current onto redo, and returns the
// state to restore.
func (h *History) Undo(current []*Edit) ([]*Edit, bool) {
	if len(h.undo) == 0 {
		return nil, false
	}
	n := len(h.undo) - 1
	prev := h.undo[n]
	h.undo = h.undo[:n]
	h.redo = append(h.redo, current)
	return prev, true
}

// Redo pops the last redone point, pushes current onto undo, and returns
// the state to restore.
func (h *History) Redo(current []*Edit) ([]*Edit, bool) {
	if len(h.redo) == 0 {
		return nil, false
	}
	n := len(h.redo) - 1
	next := h.redo[n]
	h.redo = h.redo[:n]
	h.undo = append(h.undo, current)
	return next, true
}
