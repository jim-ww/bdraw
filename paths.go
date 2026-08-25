package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// configDir is where user-editable configuration lives: config.json,
// keymap.json. os.UserConfigDir() already respects $XDG_CONFIG_HOME on
// Linux/BSD (falling back to ~/.config), and uses the platform convention
// on macOS (~/Library/Application Support) and Windows (%AppData%).
func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bdraw"), nil
}

// dataDir is where bdraw-managed (not user-edited) data lives: the
// recent-files list and autosave recovery files. On Linux/BSD this is
// $XDG_DATA_HOME (falling back to ~/.local/share), per the XDG basedir
// spec's distinction between config and data — losing config.json is
// losing your settings, losing recent.json or an autosave file is just
// losing convenience/recovery data, and they don't belong in the same
// place a user might reasonably back up or version-control. macOS and
// Windows don't draw that line the same way in practice, so both just
// share the config directory there.
func dataDir() (string, error) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return configDir()
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "bdraw"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "bdraw"), nil
}
