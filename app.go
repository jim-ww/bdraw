package main

import (
	"fmt"
	"math"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Tool is which drawing tool is currently active.
type Tool string

const (
	ToolBrush      Tool = "brush"
	ToolRect       Tool = "rect"
	ToolCircle     Tool = "circle"
	ToolLine       Tool = "line"
	ToolEraser     Tool = "eraser"
	ToolSelect     Tool = "select"
	ToolText       Tool = "text"
	ToolMove       Tool = "move"
	ToolFill       Tool = "fill"
	ToolArrow      Tool = "arrow"
	ToolEyedropper Tool = "eyedropper"
)

// toolCursor is the glyph drawn at the mouse position for each tool, so the
// cursor reflects what a click will do.
var toolCursor = map[Tool]rune{
	ToolBrush:      '●',
	ToolRect:       '▭',
	ToolCircle:     '◯',
	ToolLine:       '╱',
	ToolEraser:     '⌫',
	ToolSelect:     '↖',
	ToolText:       'T',
	ToolMove:       '✥',
	ToolFill:       '▓',
	ToolArrow:      '➔',
	ToolEyedropper: '⚲',
}

// drawTools are tools whose cursor should reflect the current draw color;
// everything else (select/move/eraser) doesn't paint in that color, so its
// cursor stays a neutral, always-visible tone instead.
var drawTools = map[Tool]bool{
	ToolBrush: true, ToolRect: true, ToolCircle: true, ToolLine: true,
	ToolText: true, ToolFill: true, ToolArrow: true,
}

const neutralCursorColor = "#00ffaa"
const hoverEditColor = "#00ffaa"

// cursorColor is what the tool cursor glyph is drawn in: the current draw
// color for tools that paint with it, or a neutral highlight for tools
// (select/move/eraser) that don't.
func (m Model) cursorColor() string {
	if drawTools[m.tool] {
		return m.color
	}
	return neutralCursorColor
}

// Palette is the fixed set of quick-pick colors offered in the color-picker
// modal. Overridable via Config.Palette (see applyPaletteOverride).
var Palette = []string{
	"#ffffff", "#ff0000", "#00ff00", "#0000ff",
	"#ffff00", "#ff00ff", "#00ffff", "#808080",
	"#000000",
}

// applyPaletteOverride replaces Palette with cfg's, if it's non-empty and
// every entry is a valid hex color — an invalid config value falls back to
// the built-in palette rather than leaving the app with a broken color
// picker.
func applyPaletteOverride(custom []string) {
	if len(custom) == 0 {
		return
	}
	normalized := make([]string, len(custom))
	for i, c := range custom {
		v, ok := normalizeHexColor(c)
		if !ok {
			return
		}
		normalized[i] = v
	}
	Palette = normalized
}

const (
	sizeMin  = 1.0
	sizeMax  = 200.0
	sizeStep = 1.4 // multiplicative, so size scales smoothly from hairline to very fat
)

const (
	zoomMin  = 0.25
	zoomMax  = 8
	zoomStep = 1.25
	panStep  = 8 // world subpixels per arrow-key press
)

// snapStep is the fixed world-unit grid snap-to-grid rounds points to. Kept
// separate from the visual dot grid's adaptive spacing (view.go) — that
// one's tuned to always look reasonable on screen at any zoom, while this
// one needs to stay put so the same click always snaps the same way.
const snapStep = 16.0

// snapPoint rounds p to the nearest snapStep grid intersection, when snap
// is enabled; otherwise it's the identity.
func (m Model) snapPoint(p Point) Point {
	if !m.snap {
		return p
	}
	return Point{
		X: math.Round(p.X/snapStep) * snapStep,
		Y: math.Round(p.Y/snapStep) * snapStep,
	}
}

// mode is which modal input state the UI is in. Everything drawing-related
// happens in modeNormal; the others take over input for one piece of text
// entry or a small overlay.
type mode int

const (
	modeNormal mode = iota
	modeTextEntry
	modePromptSaveAs
	modePromptOpen
	modeConfirmClose
	modeConfirmClear
	modeColorPicker
	modeNumberEntry
)

// Model is the whole application state for bubbletea.
type Model struct {
	km  KeyMap
	cfg Config

	tabs   []*Document
	active int

	tool  Tool
	color string
	size  float64

	width, height int

	mode            mode
	input           textinput.Model
	pendingCloseIdx int

	// drag state for the tool currently being dragged with the mouse
	dragging    bool
	dragEdit    *Edit
	erasedIDs   map[int]bool
	textPos     Point
	moveTargets []*Edit
	moveLast    Point
	selectStart Point
	selectLast  Point

	// recent is the most-recent-first list of opened/saved document paths,
	// persisted to ~/.config/bdraw/recent.json.
	recent []string

	// clipboard holds copied edits (deep clones, so later mutation of the
	// originals can't corrupt a pending paste). In-app only — not the OS
	// clipboard.
	clipboard []*Edit

	// middle-mouse-button pan drag
	panning    bool
	panLastCol int
	panLastRow int

	// last known mouse cell, for drawing a tool cursor
	cursorCol, cursorRow int
	cursorVisible        bool

	// hoverZone is the bubblezone ID currently under the mouse (any zone:
	// button, tab, swatch), so buttons can highlight on hover.
	hoverZone string

	// hoverEditID is the edit currently under the cursor for the move and
	// eraser tools (0 = none), highlighted so it's clear what a click
	// would act on.
	hoverEditID int

	showGrid bool
	filled   bool
	compact  bool
	snap     bool

	// numberEntry is the shared click-to-type-a-number state, used by both
	// the size and zoom controls (see numberentry.go).
	numEntryTarget string
	numEntryValue  string

	status string

	// cache is a pointer so it survives across the value-receiver
	// Update/View calls bubbletea makes on Model: the pointer itself gets
	// copied along with everything else, but it keeps pointing at the same
	// canvasCache, so writes through it persist. See viewCanvas.
	cache *canvasCache
}

// canvasCache holds the last rasterized canvas frame plus the exact view
// state it was built for. Rasterizing every edit is the most expensive
// thing this program does every frame (see BenchmarkRasterizeDocument); at
// high zoom a full re-rasterize on every single mouse-motion event — even
// pure cursor movement with nothing being drawn — was the "cursor drags
// like an old CRT" lag. Reusing the last frame whenever the document
// hasn't actually changed fixes that.
type canvasCache struct {
	raster      *Raster
	cols        int
	rows        int
	offset      Point
	zoom        float64
	doc         *Document
	docVer      int
	highlightID int
}

func NewModel() Model {
	cfg := LoadConfig()
	applyPaletteOverride(cfg.Palette)
	color := Palette[0]
	if c, ok := normalizeHexColor(cfg.DefaultColor); ok {
		color = c
	}

	status := "mouse to draw · b/r/c/l/a/e/s/t/m tools · scroll/[/] zoom · arrows/middle-drag pan · ctrl+q quit"
	if leftover := findAutosaveFiles(); len(leftover) > 0 {
		status = fmt.Sprintf("found %d autosave file(s) from a previous session in the autosave folder — open to recover", len(leftover))
	}

	ti := textinput.New()
	ti.Prompt = "> "
	return Model{
		km:       LoadKeyMap(),
		cfg:      cfg,
		tabs:     []*Document{NewDocument()},
		active:   0,
		tool:     ToolBrush,
		color:    color,
		size:     1,
		input:    ti,
		cache:    &canvasCache{},
		showGrid: true,
		recent:   loadRecentFiles(),
		status:   status,
	}
}

func (m Model) Init() tea.Cmd {
	return autosaveTick()
}

func (m *Model) doc() *Document {
	return m.tabs[m.active]
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case autosaveMsg:
		m.runAutosave()
		return m, autosaveTick()

	case tea.KeyMsg:
		if m.mode == modeColorPicker {
			return m.handleColorPickerKey(msg)
		}
		if m.mode == modeNumberEntry {
			return m.handleNumberEntryKey(msg)
		}
		if m.mode != modeNormal {
			return m.handlePromptKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}
