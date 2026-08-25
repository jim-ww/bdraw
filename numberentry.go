package main

import (
	"math"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Shared click-to-type-a-number component, used by both the size and zoom
// toolbar controls: click the value to start typing, digits and a single
// '.' are accepted (no minus sign — negative brush size or zoom is
// meaningless), enter confirms and clamps into range, esc cancels leaving
// the old value untouched. Left/right arrows nudge it, and the slider shown
// below can be clicked anywhere along its length to jump straight there.

func (m *Model) startNumberEntry(target string, current float64) {
	m.mode = modeNumberEntry
	m.numEntryTarget = target
	m.numEntryValue = strconv.FormatFloat(current, 'f', 1, 64)
}

// numberEntryRange returns the valid range and display label for the
// number currently being entered.
func (m Model) numberEntryRange() (lo, hi float64, label string, ok bool) {
	switch m.numEntryTarget {
	case "size":
		return sizeMin, sizeMax, "size", true
	case "zoom":
		return zoomMin * 100, zoomMax * 100, "zoom", true
	default:
		return 0, 0, "", false
	}
}

// applyNumberEntryValue commits a value for the current target immediately
// (used by the slider and arrow keys, which are meant to preview live,
// unlike typed digits which wait for Enter).
func (m *Model) applyNumberEntryValue(v float64) {
	var applied float64
	switch m.numEntryTarget {
	case "size":
		m.setSize(v)
		applied = m.size
	case "zoom":
		m.zoomAtCursor(v / 100 / m.viewZoom())
		applied = m.viewZoom() * 100
	}
	// Reflect what actually got applied (setSize/zoomAtCursor round and
	// clamp), not the raw slider/arrow-key math — and format to a fixed 1
	// decimal rather than shortest-exact ('f', -1, ...): re-deriving the
	// zoom percentage as Zoom*100 reintroduces binary floating-point noise
	// even when Zoom itself was cleanly rounded, so shortest-exact would
	// still print things like 300.09999999999997.
	m.numEntryValue = strconv.FormatFloat(applied, 'f', 1, 64)
}

func (m Model) handleNumberEntryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil

	case "enter":
		if v, err := strconv.ParseFloat(m.numEntryValue, 64); err == nil && v >= 0 {
			switch m.numEntryTarget {
			case "size":
				m.setSize(v)
			case "zoom":
				m.zoomAtCursor(v / 100 / m.viewZoom())
			}
		} else {
			m.status = "invalid number"
		}
		m.mode = modeNormal
		return m, nil

	case "backspace":
		if n := len(m.numEntryValue); n > 0 {
			m.numEntryValue = m.numEntryValue[:n-1]
		}
		return m, nil

	case "left", "right":
		lo, hi, _, ok := m.numberEntryRange()
		if !ok {
			return m, nil
		}
		v, err := strconv.ParseFloat(m.numEntryValue, 64)
		if err != nil {
			v = lo
		}
		step := math.Pow(hi/lo, 1.0/float64(sliderWidth))
		if msg.String() == "left" {
			v /= step
		} else {
			v *= step
		}
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		m.applyNumberEntryValue(v)
		return m, nil
	}

	s := msg.String()
	if len(s) == 1 && strings.ContainsRune("0123456789", rune(s[0])) {
		m.numEntryValue += s
	} else if s == "." && !strings.Contains(m.numEntryValue, ".") {
		m.numEntryValue += s
	}
	return m, nil
}

// numberEntryLabel renders either the live typed value (while entering) or
// the current value, for the toolbar.
func (m Model) numberEntryLabel(target, suffix string, current float64) string {
	if m.mode == modeNumberEntry && m.numEntryTarget == target {
		return m.numEntryValue + "_"
	}
	return strconv.FormatFloat(current, 'f', 0, 64) + suffix
}

const sliderWidth = 30

// sliderValueAt maps a click's x offset within the slider bar (0-based,
// inside the brackets) to a value on the log-scale range.
func sliderValueAt(x int, lo, hi float64) float64 {
	if x < 0 {
		x = 0
	}
	if x > sliderWidth-1 {
		x = sliderWidth - 1
	}
	frac := float64(x) / float64(sliderWidth-1)
	return math.Exp(math.Log(lo) + frac*(math.Log(hi)-math.Log(lo)))
}

// viewNumberSlider renders the little modal shown below the size/zoom
// control while typing: a min-to-max slider tracking the in-progress
// value, on a log scale since both size and zoom are multiplicative
// ranges (1..200, 25%..800%) where a linear slider would waste most of its
// width on the low end. The bar is zone-marked as a whole so clicking
// anywhere along it — not just exactly on the dot — jumps the value there.
func (m Model) viewNumberSlider() string {
	lo, hi, label, ok := m.numberEntryRange()
	if !ok {
		return ""
	}

	value, err := strconv.ParseFloat(m.numEntryValue, 64)
	if err != nil {
		value = lo
	}
	if value < lo {
		value = lo
	}
	if value > hi {
		value = hi
	}

	frac := (math.Log(value) - math.Log(lo)) / (math.Log(hi) - math.Log(lo))
	pos := int(frac * float64(sliderWidth-1))

	var bar strings.Builder
	bar.WriteByte('[')
	for i := 0; i < sliderWidth; i++ {
		if i == pos {
			bar.WriteRune('●')
		} else {
			bar.WriteByte('-')
		}
	}
	bar.WriteByte(']')

	body := label + " " + zone.Mark(zoneSlider, bar.String()) + "\n" +
		dimStyle.Render("click or ←/→ to adjust, type digits, enter to confirm, esc to cancel")
	return modalStyle.Render(body)
}
