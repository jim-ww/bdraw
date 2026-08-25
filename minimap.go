package main

import "math"

// minimapBoxW/H are the minimap's total footprint in terminal cells,
// border included. Off by default (Model.showMinimap), toggled from the
// toolbar or ToggleMinimap.
const (
	minimapBoxW = 20
	minimapBoxH = 10
)

const (
	minimapBorderColor   = "#555555"
	minimapContentColor  = "#888888"
	minimapViewportColor = selectColor
)

type minimapCell struct {
	ru    rune
	color string
}

// minimapOverlay returns cell overrides (keyed by canvas cell position)
// for the minimap box, positioned in the top-right corner of the cols x
// rows canvas viewport. Returns an empty map if the viewport is too
// narrow to fit it, or minimap isn't enabled — callers don't need to
// check either condition themselves.
func (m Model) minimapOverlay(cols, rows int, d *Document, offset Point, zoom float64) map[[2]int]minimapCell {
	out := map[[2]int]minimapCell{}
	if !m.showMinimap || cols < minimapBoxW+2 || rows < minimapBoxH+2 {
		return out
	}

	originCol := cols - minimapBoxW - 1
	originRow := 1

	// The world-space window the minimap covers: every edit's bounding
	// box, unioned with the current viewport — so panning away from the
	// drawing doesn't push the viewport marker outside the minimap, and
	// an empty canvas still shows something sane (just the viewport
	// itself) rather than a degenerate zero-size box.
	vx0, vy0 := offset.X, offset.Y
	vx1 := offset.X + float64(cols)*SubpixW/zoom
	vy1 := offset.Y + float64(rows)*SubpixH/zoom
	minX, minY, maxX, maxY := vx0, vy0, vx1, vy1
	for _, e := range d.Edits {
		for _, p := range e.Points {
			minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
			minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
		}
	}
	// A little breathing room so content/viewport markers never sit
	// exactly on the box's border.
	padX, padY := (maxX-minX)*0.08+1, (maxY-minY)*0.08+1
	minX, maxX = minX-padX, maxX+padX
	minY, maxY = minY-padY, maxY+padY

	contentW, contentH := minimapBoxW-2, minimapBoxH-2
	toCell := func(x, y float64) (col, row int) {
		fx, fy := 0.5, 0.5
		if maxX > minX {
			fx = (x - minX) / (maxX - minX)
		}
		if maxY > minY {
			fy = (y - minY) / (maxY - minY)
		}
		col = originCol + 1 + int(fx*float64(contentW-1))
		row = originRow + 1 + int(fy*float64(contentH-1))
		if col < originCol+1 {
			col = originCol + 1
		}
		if col > originCol+contentW {
			col = originCol + contentW
		}
		if row < originRow+1 {
			row = originRow + 1
		}
		if row > originRow+contentH {
			row = originRow + contentH
		}
		return col, row
	}

	// Border.
	for c := 0; c < minimapBoxW; c++ {
		out[[2]int{originCol + c, originRow}] = minimapCell{'─', minimapBorderColor}
		out[[2]int{originCol + c, originRow + minimapBoxH - 1}] = minimapCell{'─', minimapBorderColor}
	}
	for r := 0; r < minimapBoxH; r++ {
		out[[2]int{originCol, originRow + r}] = minimapCell{'│', minimapBorderColor}
		out[[2]int{originCol + minimapBoxW - 1, originRow + r}] = minimapCell{'│', minimapBorderColor}
	}
	out[[2]int{originCol, originRow}] = minimapCell{'┌', minimapBorderColor}
	out[[2]int{originCol + minimapBoxW - 1, originRow}] = minimapCell{'┐', minimapBorderColor}
	out[[2]int{originCol, originRow + minimapBoxH - 1}] = minimapCell{'└', minimapBorderColor}
	out[[2]int{originCol + minimapBoxW - 1, originRow + minimapBoxH - 1}] = minimapCell{'┘', minimapBorderColor}

	// Content: a rough dot per edit endpoint — this is an overview, not
	// a faithful mini-render, so plotting every point (not rasterizing
	// segments) is plenty to see roughly where the drawing is.
	for _, e := range d.Edits {
		for _, p := range e.Points {
			col, row := toCell(p.X, p.Y)
			out[[2]int{col, row}] = minimapCell{'·', minimapContentColor}
		}
	}

	// Viewport rectangle, drawn last so it's never hidden under content
	// dots.
	c0, r0 := toCell(vx0, vy0)
	c1, r1 := toCell(vx1, vy1)
	for c := c0; c <= c1; c++ {
		out[[2]int{c, r0}] = minimapCell{'─', minimapViewportColor}
		out[[2]int{c, r1}] = minimapCell{'─', minimapViewportColor}
	}
	for r := r0; r <= r1; r++ {
		out[[2]int{c0, r}] = minimapCell{'│', minimapViewportColor}
		out[[2]int{c1, r}] = minimapCell{'│', minimapViewportColor}
	}
	out[[2]int{c0, r0}] = minimapCell{'┌', minimapViewportColor}
	out[[2]int{c1, r0}] = minimapCell{'┐', minimapViewportColor}
	out[[2]int{c0, r1}] = minimapCell{'└', minimapViewportColor}
	out[[2]int{c1, r1}] = minimapCell{'┘', minimapViewportColor}

	return out
}
