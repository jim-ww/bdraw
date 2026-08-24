package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Tool is which drawing tool is currently active.
type Tool string

const (
	ToolBrush  Tool = "brush"
	ToolRect   Tool = "rect"
	ToolCircle Tool = "circle"
	ToolLine   Tool = "line"
	ToolEraser Tool = "eraser"
	ToolSelect Tool = "select"
	ToolText   Tool = "text"
	ToolMove   Tool = "move"
	ToolFill   Tool = "fill"
)

// toolCursor is the glyph drawn at the mouse position for each tool, so the
// cursor reflects what a click will do.
var toolCursor = map[Tool]rune{
	ToolBrush:  '●',
	ToolRect:   '▭',
	ToolCircle: '◯',
	ToolLine:   '╱',
	ToolEraser: '▢',
	ToolSelect: '↖',
	ToolText:   'T',
	ToolMove:   '✥',
	ToolFill:   '▓',
}

const cursorColor = "#00ffaa"

// Palette is the fixed set of quick-pick colors offered in the color-picker
// modal.
var Palette = []string{
	"#ffffff", "#ff0000", "#00ff00", "#0000ff",
	"#ffff00", "#ff00ff", "#00ffff", "#808080",
	"#000000",
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

	// middle-mouse-button pan drag
	panning    bool
	panLastCol int
	panLastRow int

	// last known mouse cell, for drawing a tool cursor
	cursorCol, cursorRow int
	cursorVisible        bool

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
	raster *Raster
	cols   int
	rows   int
	offset Point
	zoom   float64
	doc    *Document
	docVer int
}

func NewModel() Model {
	ti := textinput.New()
	ti.Prompt = "> "
	return Model{
		km:     LoadKeyMap(),
		cfg:    LoadConfig(),
		tabs:   []*Document{NewDocument()},
		active: 0,
		tool:   ToolBrush,
		color:  Palette[0],
		size:   1,
		input:  ti,
		cache:  &canvasCache{},
		status: "mouse to draw · b/r/c/l/e/s/t/m tools · scroll/ctrl+=/- zoom · arrows/middle-drag pan · ctrl+q quit",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
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

	case tea.KeyMsg:
		if m.mode == modeColorPicker {
			return m.handleColorPickerKey(msg)
		}
		if m.mode != modeNormal {
			return m.handlePromptKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}
