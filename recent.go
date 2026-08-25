package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const maxRecentFiles = 9

func recentFilesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bdraw", "recent.json"), nil
}

// loadRecentFiles reads the most-recent-first list of previously
// opened/saved paths, or nil if there isn't one yet.
func loadRecentFiles() []string {
	path, err := recentFilesPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var recent []string
	if err := json.Unmarshal(data, &recent); err != nil {
		return nil
	}
	return recent
}

// saveRecentFiles persists the list; errors are silently ignored — recent
// files is a convenience, not something worth interrupting the user over.
func saveRecentFiles(recent []string) {
	path, err := recentFilesPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(recent, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// rememberRecentFile moves path to the front of the list, dedupes it, and
// caps the list at maxRecentFiles.
func rememberRecentFile(recent []string, path string) []string {
	out := []string{path}
	for _, p := range recent {
		if p != path {
			out = append(out, p)
		}
	}
	if len(out) > maxRecentFiles {
		out = out[:maxRecentFiles]
	}
	return out
}
