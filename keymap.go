package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/key"
)

// KeyMap holds every keybinding, each independently overridable by the
// user's config file.
type KeyMap struct {
	ToolBrush      key.Binding
	ToolRect       key.Binding
	ToolCircle     key.Binding
	ToolLine       key.Binding
	ToolEraser     key.Binding
	ToolSelect     key.Binding
	ToolText       key.Binding
	ToolMove       key.Binding
	ToolFill       key.Binding
	ToolArrow      key.Binding
	ToolEyedropper key.Binding

	Undo           key.Binding
	Redo           key.Binding
	Delete         key.Binding
	ClearSelection key.Binding
	Copy           key.Binding
	Paste          key.Binding

	New    key.Binding
	Open   key.Binding
	Save   key.Binding
	SaveAs key.Binding
	Export key.Binding
	Clear  key.Binding

	ColorPicker   key.Binding
	ToggleGrid    key.Binding
	ToggleSnap    key.Binding
	ToggleFill    key.Binding
	ToggleCompact key.Binding

	NextTab  key.Binding
	PrevTab  key.Binding
	CloseTab key.Binding
	NewTab   key.Binding

	SizeInc key.Binding
	SizeDec key.Binding

	PanUp    key.Binding
	PanDown  key.Binding
	PanLeft  key.Binding
	PanRight key.Binding
	ZoomIn   key.Binding
	ZoomOut  key.Binding

	Quit key.Binding
}

// DefaultKeyMap is used for any binding not overridden by the user's
// config file.
func DefaultKeyMap() KeyMap {
	bind := func(keys []string, help string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], help))
	}
	return KeyMap{
		ToolBrush:      bind([]string{"b"}, "brush"),
		ToolRect:       bind([]string{"r"}, "rectangle"),
		ToolCircle:     bind([]string{"o", "c"}, "oval"),
		ToolLine:       bind([]string{"l"}, "line"),
		ToolEraser:     bind([]string{"e"}, "eraser"),
		ToolSelect:     bind([]string{"s"}, "select"),
		ToolText:       bind([]string{"t"}, "text"),
		ToolMove:       bind([]string{"m"}, "move"),
		ToolFill:       bind([]string{"f"}, "fill"),
		ToolArrow:      bind([]string{"a"}, "arrow"),
		ToolEyedropper: bind([]string{"q"}, "eyedropper"),

		Undo:           bind([]string{"ctrl+z"}, "undo"),
		Redo:           bind([]string{"ctrl+shift+z", "ctrl+y"}, "redo"),
		Delete:         bind([]string{"delete", "backspace"}, "delete selection"),
		ClearSelection: bind([]string{"esc"}, "deselect"),
		Copy:           bind([]string{"ctrl+c"}, "copy selection"),
		Paste:          bind([]string{"ctrl+v"}, "paste"),

		New:    bind([]string{"ctrl+n"}, "new"),
		Open:   bind([]string{"ctrl+o"}, "open"),
		Save:   bind([]string{"ctrl+s"}, "save"),
		SaveAs: bind([]string{"ctrl+shift+s"}, "save as"),
		Export: bind([]string{"ctrl+e"}, "export"),
		Clear:  bind([]string{"ctrl+x"}, "clear canvas"),

		ColorPicker: bind([]string{"p"}, "color picker"),
		ToggleGrid:  bind([]string{"g"}, "toggle grid"),
		ToggleSnap:  bind([]string{"n"}, "toggle snap to grid"),
		ToggleFill:  bind([]string{"ctrl+f"}, "toggle filled shapes"),
		// ctrl+m is included per request, but heads up: on most terminals
		// without kitty-protocol keyboard disambiguation, ctrl+m is
		// byte-for-byte indistinguishable from Enter. Since Enter already
		// means something in several modes (confirming prompts, size/zoom
		// entry), that alias may fire unexpectedly there. 'h' is kept as
		// the reliable default; override via config if you don't want
		// ctrl+m at all.
		ToggleCompact: bind([]string{"h", "ctrl+m"}, "toggle compact toolbar"),

		NextTab:  bind([]string{"tab"}, "next tab"),
		PrevTab:  bind([]string{"shift+tab"}, "prev tab"),
		CloseTab: bind([]string{"ctrl+w"}, "close tab"),
		NewTab:   bind([]string{"ctrl+t"}, "new tab"),

		SizeInc: bind([]string{"+", "="}, "size up"),
		SizeDec: bind([]string{"-"}, "size down"),

		PanUp:    bind([]string{"up"}, "pan up"),
		PanDown:  bind([]string{"down"}, "pan down"),
		PanLeft:  bind([]string{"left"}, "pan left"),
		PanRight: bind([]string{"right"}, "pan right"),
		// Not ctrl+=/ctrl+-: most terminal emulators intercept those
		// themselves for their own font-size zoom, so the app would never
		// even see the keypress. Brackets are rarely claimed by anything.
		ZoomIn:  bind([]string{"]"}, "zoom in"),
		ZoomOut: bind([]string{"["}, "zoom out"),

		// ctrl+q, not ctrl+c: ctrl+c is Copy now, and is also the universal
		// "just kill it" muscle-memory key, so overloading it for quit
		// risked an accidental copy silently eating a real interrupt too.
		Quit: bind([]string{"ctrl+q"}, "quit"),
	}
}

// keyOverrides is the JSON shape of the user's config file: field name to
// list of key strings, matching key.Binding's WithKeys.
type keyOverrides map[string][]string

// LoadKeyMap starts from DefaultKeyMap and applies overrides from
// ~/.config/bdraw/keymap.json, if it exists.
func LoadKeyMap() KeyMap {
	km := DefaultKeyMap()
	path, err := keymapConfigPath()
	if err != nil {
		return km
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return km
	}
	var overrides keyOverrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return km
	}
	applyOverrides(&km, overrides)
	return km
}

func keymapConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bdraw", "keymap.json"), nil
}

func applyOverrides(km *KeyMap, overrides keyOverrides) {
	fields := map[string]*key.Binding{
		"tool_brush":      &km.ToolBrush,
		"tool_rect":       &km.ToolRect,
		"tool_circle":     &km.ToolCircle,
		"tool_line":       &km.ToolLine,
		"tool_eraser":     &km.ToolEraser,
		"tool_select":     &km.ToolSelect,
		"tool_text":       &km.ToolText,
		"tool_move":       &km.ToolMove,
		"tool_fill":       &km.ToolFill,
		"tool_arrow":      &km.ToolArrow,
		"tool_eyedropper": &km.ToolEyedropper,
		"undo":            &km.Undo,
		"redo":            &km.Redo,
		"delete":          &km.Delete,
		"clear_selection": &km.ClearSelection,
		"copy":            &km.Copy,
		"paste":           &km.Paste,
		"new":             &km.New,
		"open":            &km.Open,
		"save":            &km.Save,
		"save_as":         &km.SaveAs,
		"export":          &km.Export,
		"clear":           &km.Clear,
		"color_picker":    &km.ColorPicker,
		"toggle_grid":     &km.ToggleGrid,
		"toggle_snap":     &km.ToggleSnap,
		"toggle_fill":     &km.ToggleFill,
		"toggle_compact":  &km.ToggleCompact,
		"next_tab":        &km.NextTab,
		"prev_tab":        &km.PrevTab,
		"close_tab":       &km.CloseTab,
		"new_tab":         &km.NewTab,
		"size_inc":        &km.SizeInc,
		"size_dec":        &km.SizeDec,
		"pan_up":          &km.PanUp,
		"pan_down":        &km.PanDown,
		"pan_left":        &km.PanLeft,
		"pan_right":       &km.PanRight,
		"zoom_in":         &km.ZoomIn,
		"zoom_out":        &km.ZoomOut,
		"quit":            &km.Quit,
	}
	for name, keys := range overrides {
		b, ok := fields[name]
		if !ok || len(keys) == 0 {
			continue
		}
		b.SetKeys(keys...)
	}
}
