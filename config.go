package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds general (non-keybind) user preferences, loaded from
// ~/.config/bdraw/config.json.
type Config struct {
	// UseIcons shows tool buttons as icons instead of names. Defaults to
	// true; set "icons": false in the config file to switch to text
	// labels.
	UseIcons bool `json:"icons"`
}

func DefaultConfig() Config {
	return Config{UseIcons: true}
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
