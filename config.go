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

// LoadConfig starts from DefaultConfig and applies overrides from
// ~/.config/bdraw/config.json, if it exists.
func LoadConfig() Config {
	cfg := DefaultConfig()
	dir, err := os.UserConfigDir()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(filepath.Join(dir, "bdraw", "config.json"))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}
