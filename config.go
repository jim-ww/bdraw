package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds general (non-keybind) user preferences, loaded from
// ~/.config/bdraw/config.json.
type Config struct {
	// UseIcons shows tool buttons as icon glyphs instead of names. Off by
	// default — the glyphs are compact but not self-explanatory; set
	// "icons": true in the config file to enable them.
	UseIcons bool `json:"icons"`

	// Palette overrides the default color-picker swatches, as a list of
	// "#rrggbb" strings. Leave unset to keep the built-in palette.
	Palette []string `json:"palette,omitempty"`

	// DefaultColor overrides the color a new document starts with (also
	// "#rrggbb"). Leave unset to keep the built-in default (white).
	DefaultColor string `json:"default_color,omitempty"`
}

func DefaultConfig() Config {
	return Config{UseIcons: false}
}

// defaultConfigPath returns ~/.config/bdraw/config.json (or the platform
// equivalent).
func defaultConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig starts from DefaultConfig and applies overrides from a config
// file. If path is empty, it uses the default location
// (~/.config/bdraw/config.json); otherwise path is used as-is, e.g. from
// the -c flag.
func LoadConfig(path string) Config {
	cfg := DefaultConfig()
	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return cfg
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}
