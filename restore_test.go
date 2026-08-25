package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRestoreFromAutosaveFlow covers the -restore flag end to end:
// latestAutosaveFile finds the right file, and restoreFromAutosave loads
// it as an unsaved (no Path, Dirty) document that keeps writing to the
// same recovery file rather than starting a fresh one.
func TestRestoreFromAutosaveFlow(t *testing.T) {
	// Sandbox into a throwaway directory rather than the real XDG data
	// dir: a build sandbox (e.g. Nix's checkPhase) typically runs with
	// $HOME unset/unwritable, and even outside one, a test has no
	// business touching the user's actual autosave folder.
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	dir, err := autosaveDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "testrestore123"
	path := filepath.Join(dir, id+".json")
	defer os.Remove(path)

	d := NewDocument()
	d.autosaveID = id
	d.Edits = []*Edit{{ID: 1, Kind: KindStroke, Points: []Point{{X: 1, Y: 1}}, Color: "#fff", Size: 1}}
	data, err := marshalFileFormat(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got := latestAutosaveFile()
	if got != path {
		t.Fatalf("latestAutosaveFile() = %q, want %q", got, path)
	}

	m := NewModel("")
	if err := m.restoreFromAutosave(got); err != nil {
		t.Fatal(err)
	}
	if m.doc().Path != "" {
		t.Fatalf("restored document should have no Path (still Untitled), got %q", m.doc().Path)
	}
	if !m.doc().Dirty {
		t.Fatal("restored document should be marked Dirty")
	}
	if len(m.doc().Edits) != 1 {
		t.Fatalf("expected 1 restored edit, got %d", len(m.doc().Edits))
	}
	if m.doc().autosaveID != id {
		t.Fatalf("restored document should keep writing to the same autosave file, got autosaveID %q want %q", m.doc().autosaveID, id)
	}
}

// TestRestoreFromAutosaveNoFiles checks latestAutosaveFile degrades to ""
// rather than erroring when there's nothing to restore.
func TestRestoreFromAutosaveNoFiles(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if got := latestAutosaveFile(); got != "" {
		t.Fatalf("expected no autosave file, got %q", got)
	}
}
