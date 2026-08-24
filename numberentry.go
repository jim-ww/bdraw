package main

import (
	"math"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Shared click-to-type-a-number component, used by both the size and zoom
// toolbar controls: click the value to start typing, digits and a single
// '.' are accepted (no minus sign — negative brush size or zoom is
// meaningless), enter confirms and clamps into range, esc cancels leaving
// the old value untouched.

func (m *Model) startNumberEntry(target string, current float64) {
	m.mode = modeNumberEntry
	m.numEntryTarget = target
	m.numEntryValue = strconv.FormatFloat(current, 'f', -1, 64)
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
				m.zoomAtCursor(v / 100 / m.doc().Zoom)
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

// viewNumberSlider renders the little modal shown below the size/zoom
// control while typing: a min-to-max slider tracking the in-progress
// value, on a log scale since both size and zoom are multiplicative
// ranges (1..200, 25%..800%) where a linear slider would waste most of its
// width on the low end.
func (m Model) viewNumberSlider() string {
	var lo, hi float64
	var label string
	switch m.numEntryTarget {
	case "size":
		lo, hi, label = sizeMin, sizeMax, "size"
	case "zoom":
		lo, hi, label = zoomMin*100, zoomMax*100, "zoom"
	default:
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

	body := label + " " + bar.String() + "\n" +
		dimStyle.Render("type digits, enter to confirm, esc to cancel")
	return modalStyle.Render(body)
}
