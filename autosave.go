package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
)

const autosaveInterval = 30 * time.Second

// autosaveMsg fires on a recurring timer; handling it re-schedules the
// next one, giving a simple self-sustaining polling loop.
type autosaveMsg struct{}

func autosaveTick() tea.Cmd {
	return tea.Tick(autosaveInterval, func(time.Time) tea.Msg { return autosaveMsg{} })
}

func autosaveDir() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autosave"), nil
}

// newAutosaveID returns a short random identifier used to name a
// document's autosave file consistently for the life of the session, even
// before it has ever been saved to a real path.
func newAutosaveID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "untitled"
	}
	return hex.EncodeToString(b[:])
}

func autosavePath(d *Document) (string, error) {
	dir, err := autosaveDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, d.autosaveID+".json"), nil
}

// runAutosave writes every dirty, non-empty document to its recovery file.
// Failures are silent — autosave is a safety net, not something that
// should interrupt drawing with error dialogs.
func (m *Model) runAutosave() {
	dir, err := autosaveDir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	for _, d := range m.tabs {
		if !d.Dirty || len(d.Edits) == 0 {
			continue
		}
		path, err := autosavePath(d)
		if err != nil {
			continue
		}
		data, err := marshalFileFormat(d)
		if err != nil {
			continue
		}
		_ = os.WriteFile(path, data, 0o644)
	}
}

// clearAutosave removes a document's recovery file once it's been safely
// saved for real — the leftover would otherwise dangle and could confuse a
// later recovery check.
func clearAutosave(d *Document) {
	path, err := autosavePath(d)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// findAutosaveFiles lists any leftover recovery files from a previous
// session that didn't get cleared — i.e. crashed or was killed with
// unsaved work pending.
func findAutosaveFiles() []string {
	dir, err := autosaveDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// latestAutosaveFile returns the most recently modified autosave
// recovery file, or "" if there are none — used by the -restore flag to
// pick up the most recent crashed/killed session automatically, rather
// than making the user go hunt through the autosave directory by hand.
func latestAutosaveFile() string {
	var newest string
	var newestMod time.Time
	for _, f := range findAutosaveFiles() {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestMod) {
			newest, newestMod = f, info.ModTime()
		}
	}
	return newest
}
