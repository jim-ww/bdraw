package main

import (
	"fmt"
	"math"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
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

	// kbdCol/kbdRow are the keyboard cursor's absolute terminal position
	// (see kbd.go) — lazily initialized to the viewport center on first
	// use so a mouse-free session doesn't need any separate "enable"
	// step.
	kbdCol, kbdRow int
	kbdInit        bool

	// hoverEditID is the edit currently under the cursor for the move and
	// eraser tools (0 = none), highlighted so it's clear what a click
	// would act on.
	hoverEditID int

	showGrid bool
	filled   bool
	compact  bool
	snap     bool
	showHelp bool

	// colorPickerFocus is which palette swatch Tab/Shift+Tab currently has
	// selected, for keyboard-only color switching; -1 = none focused yet.
	colorPickerFocus int

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

	// zm is this Model's own bubblezone manager, not the package-level
	// global one — every collab peer (host and every SSH guest) runs its
	// own separate tea.Program concurrently in the same process, and
	// they'd all be mutating the exact same global zone registry if they
	// shared it: one peer's render (different terminal size, different
	// tab layout) would clobber another's zone bounding boxes between
	// scans, so clicks — tabs most visibly, since their layout differs
	// most between peers, but really anything — could resolve against
	// stale or another peer's geometry. A private Manager per Model
	// (zone.New(), not zone.NewGlobal()) makes each peer's zone state
	// fully independent.
	zm *zone.Manager

	// hub is non-nil for every peer in an SSH collaboration session,
	// host included (see collab.go and main.go): m.tabs then mirrors the
	// Hub's shared tab list rather than a private one, m.peerID/peerName/
	// peerColor identify this connection to everyone else, and every
	// mouse/key event routes through collabWrap to serialize edits and
	// broadcast cursor/tab changes.
	hub       *Hub
	peerID    int
	peerName  string
	peerColor string

	// isHost is true only for the host's own local terminal session, not
	// any guest — the host is a Hub peer like everyone else (so its edits
	// and tab changes broadcast out too), but it still needs to be
	// distinguishable from guests for the handful of things that stay
	// host-only regardless of read-only status: local file I/O
	// (Save/SaveAs/Open/Export always touch the *host's* disk — see
	// isFileIOKey in keys.go) and never being subject to readOnly.
	isHost bool

	// readOnly disables every action that mutates the document — set for
	// guests on a collab server started with the read-only flag. Never
	// true for the host (see isHost).
	readOnly bool

	// peerCursors holds every other connected peer's last-broadcast
	// cursor position, keyed by peer ID, so it can be drawn as a small
	// dot with their name regardless of this viewer's own pan/zoom.
	peerCursors map[int]peerCursor

	// viewOffsets/viewZooms hold this peer's own private pan/zoom per
	// Document, used only in a collab session — see viewOffset/viewZoom.
	// Keyed by document pointer rather than tab index, since a collab
	// peer's own tab list is a synced copy of the Hub's (see collab.go),
	// not something safe to use as a stable index into private state.
	viewOffsets map[*Document]Point
	viewZooms   map[*Document]float64
}

// viewOffset and viewZoom return the effective pan/zoom for the active
// document from this peer's own point of view. Solo, that's just the
// document's own Offset/Zoom fields, same as always. In a collab session
// every peer's Model shares the same *Document by pointer — but pan and
// zoom are exploration, not document state, and panning shouldn't drag
// everyone else's screen along with it, so collab peers keep their pan
// and zoom in Model.viewOffsets/viewZooms instead of ever writing back to
// the shared Document.
func (m *Model) viewOffset() Point {
	d := m.doc()
	if m.hub == nil {
		return d.Offset
	}
	if o, ok := m.viewOffsets[d]; ok {
		return o
	}
	return d.Offset
}

func (m *Model) viewZoom() float64 {
	d := m.doc()
	z := d.Zoom
	if m.hub != nil {
		if v, ok := m.viewZooms[d]; ok {
			z = v
		}
	}
	if z == 0 {
		z = 1
	}
	return z
}

func (m *Model) setViewOffset(p Point) {
	if m.hub == nil {
		m.doc().Offset = p
		return
	}
	if m.viewOffsets == nil {
		m.viewOffsets = map[*Document]Point{}
	}
	m.viewOffsets[m.doc()] = p
}

func (m *Model) setViewZoom(z float64) {
	if m.hub == nil {
		m.doc().Zoom = z
		return
	}
	if m.viewZooms == nil {
		m.viewZooms = map[*Document]float64{}
	}
	m.viewZooms[m.doc()] = z
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

	// dragEditID/dragPointsBaked support the incremental fast path for an
	// actively-growing brush stroke (see canvasRaster in view.go): rather
	// than re-rasterizing every point of the stroke on every single motion
	// event — cost that grows without bound the longer and faster you
	// draw — only the segments added since the last frame get drawn onto
	// the already-cached raster.
	dragEditID      int
	dragPointsBaked int

	// baseRaster/baseEditID/baseVer support the analogous fast path for
	// dragging out a rect/circle/line/arrow: that edit always has exactly
	// 2 points (just its far corner moving), so there's nothing to append
	// incrementally — but every other edit in the document is completely
	// unaffected by the drag, so there's no need to redraw them on every
	// motion event either. baseRaster is everything except the actively
	// dragged edit, computed once and reused for the rest of the drag;
	// each frame just composites the shape's current geometry on top of a
	// copy of it.
	baseRaster *Raster
	baseEditID int
	baseVer    int
}

// NewModel builds the initial application state. configPath overrides
// where the config file is loaded from (see the -c flag); pass "" to use
// the default location.
func NewModel(configPath string) Model {
	cfg := LoadConfig(configPath)
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
		zm:       zone.New(),
		showGrid: true,
		recent:   loadRecentFiles(),
		status:   status,
	}
}

func (m Model) Init() tea.Cmd {
	return autosaveTick()
}

// doc returns the active document. Guarded against an empty m.tabs: after
// closing the last tab we return tea.Quit, but bubbletea still renders one
// more frame on the model that returned it before actually exiting, and
// that frame's View() calls doc() same as any other.
func (m *Model) doc() *Document {
	if len(m.tabs) == 0 {
		return NewDocument()
	}
	return m.tabs[m.active]
}

// setTabs replaces m.tabs with newTabs — the shared list pulled from the
// Hub in a collab session — while keeping m.active pointing at the same
// *Document by identity, not by index. Tab list mutations elsewhere
// (another peer creating or closing a tab) can shift indices around; an
// index-only resync would silently swap which document this peer is
// looking at out from under them. Falls back to clamping into range,
// same as solo doCloseTab, only if the previously-active document is no
// longer present at all (it was closed).
func (m *Model) setTabs(newTabs []*Document) {
	var was *Document
	if m.active >= 0 && m.active < len(m.tabs) {
		was = m.tabs[m.active]
	}
	m.tabs = newTabs
	if was != nil {
		for i, d := range newTabs {
			if d == was {
				m.active = i
				return
			}
		}
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	if m.active < 0 {
		m.active = 0
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.collabWrap(func(mm Model) (tea.Model, tea.Cmd) { return mm.handleMouse(msg) })

	case collabCursorMsg:
		if m.peerCursors == nil {
			m.peerCursors = map[int]peerCursor{}
		}
		m.peerCursors[msg.ID] = peerCursor{Name: msg.Name, Color: msg.Color, Pt: msg.Pt, Visible: msg.Visible}
		return m, nil

	case collabByeMsg:
		delete(m.peerCursors, msg.ID)
		return m, nil

	case collabRefreshMsg:
		if m.hub != nil {
			m.hub.mu.Lock()
			m.setTabs(m.hub.snapshot())
			m.hub.mu.Unlock()
		}
		return m, nil

	case autosaveMsg:
		m.runAutosave()
		return m, autosaveTick()

	case tea.KeyMsg:
		return m.collabWrap(func(mm Model) (tea.Model, tea.Cmd) { return mm.handleKeyMsg(msg) })
	}
	return m, nil
}

// handleKeyMsg is the body of the old tea.KeyMsg case, split out so
// collabWrap can serialize it (under the Hub's lock, for a collab
// session) the same way it serializes mouse events.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch msg.String() {
		case "?", "esc":
			m.showHelp = false
		}
		return m, nil
	}
	if key.Matches(msg, m.km.Help) {
		m.showHelp = true
		return m, nil
	}
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
