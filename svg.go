package main

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// ExportSVG writes the document as an SVG, mapping each edit kind to its
// natural SVG primitive directly from world coordinates — unlike PNG, this
// isn't a rasterize-then-reencode path, so it stays crisp at any scale.
//
// Fill edits have no direct SVG equivalent (flood fill bounded by whatever
// else is on the canvas isn't an SVG primitive) and are skipped; the
// caller is told how many were skipped so it can warn the user.
func ExportSVG(d *Document, path string) (skippedFills int, err error) {
	minX, minY, maxX, maxY := boundingBox(d.Edits)
	if minX > maxX {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}
	const pad = 4
	minX, minY, maxX, maxY = minX-pad, minY-pad, maxX+pad, maxY+pad
	w, h := maxX-minX, maxY-minY

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%g %g %g %g">`+"\n", minX, minY, w, h)
	for _, e := range d.Edits {
		switch e.Kind {
		case KindStroke:
			writeSVGPolyline(&b, e)
		case KindLine:
			writeSVGLine(&b, e)
		case KindArrow:
			writeSVGArrow(&b, e)
		case KindRect:
			writeSVGRect(&b, e)
		case KindCircle:
			writeSVGEllipse(&b, e)
		case KindText:
			writeSVGText(&b, e)
		case KindFill:
			skippedFills++
		}
	}
	b.WriteString("</svg>\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return skippedFills, fmt.Errorf("write %s: %w", path, err)
	}
	return skippedFills, nil
}

func svgFill(filled bool, color string) string {
	if filled {
		return color
	}
	return "none"
}

func writeSVGPolyline(b *strings.Builder, e *Edit) {
	if len(e.Points) == 0 {
		return
	}
	fmt.Fprintf(b, `  <polyline points="`)
	for i, p := range e.Points {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(b, "%g,%g", p.X, p.Y)
	}
	fmt.Fprintf(b, `" fill="none" stroke="%s" stroke-width="%g" stroke-linecap="round" stroke-linejoin="round"/>`+"\n",
		e.Color, e.Size)
}

func writeSVGLine(b *strings.Builder, e *Edit) {
	if len(e.Points) < 2 {
		return
	}
	fmt.Fprintf(b, `  <line x1="%g" y1="%g" x2="%g" y2="%g" stroke="%s" stroke-width="%g" stroke-linecap="round"/>`+"\n",
		e.Points[0].X, e.Points[0].Y, e.Points[1].X, e.Points[1].Y, e.Color, e.Size)
}

// writeSVGArrow draws the shaft plus two head barbs as plain lines,
// matching raster.go's drawArrow, rather than an SVG <marker> — markers are
// defined once and can't easily vary per-edit color/size without a
// separate <marker> per combination.
func writeSVGArrow(b *strings.Builder, e *Edit) {
	if len(e.Points) < 2 {
		return
	}
	from, to := e.Points[0], e.Points[1]
	writeSVGLine(b, e)

	dx, dy := to.X-from.X, to.Y-from.Y
	shaftLen := math.Hypot(dx, dy)
	if shaftLen == 0 {
		return
	}
	headLen := shaftLen * arrowHeadFrac
	if headLen > 24 {
		headLen = 24
	}
	if headLen < 4 {
		headLen = 4
	}
	angle := math.Atan2(dy, dx)
	for _, sign := range [2]float64{1, -1} {
		a := angle + math.Pi - sign*arrowHeadAngle
		bx := to.X + headLen*math.Cos(a)
		by := to.Y + headLen*math.Sin(a)
		fmt.Fprintf(b, `  <line x1="%g" y1="%g" x2="%g" y2="%g" stroke="%s" stroke-width="%g" stroke-linecap="round"/>`+"\n",
			to.X, to.Y, bx, by, e.Color, e.Size)
	}
}

func writeSVGRect(b *strings.Builder, e *Edit) {
	if len(e.Points) < 2 {
		return
	}
	x0, x1 := minF(e.Points[0].X, e.Points[1].X), maxF(e.Points[0].X, e.Points[1].X)
	y0, y1 := minF(e.Points[0].Y, e.Points[1].Y), maxF(e.Points[0].Y, e.Points[1].Y)
	fmt.Fprintf(b, `  <rect x="%g" y="%g" width="%g" height="%g" fill="%s" stroke="%s" stroke-width="%g"/>`+"\n",
		x0, y0, x1-x0, y1-y0, svgFill(e.Filled, e.Color), e.Color, e.Size)
}

func writeSVGEllipse(b *strings.Builder, e *Edit) {
	if len(e.Points) < 2 {
		return
	}
	cx, cy := (e.Points[0].X+e.Points[1].X)/2, (e.Points[0].Y+e.Points[1].Y)/2
	rx := absF(e.Points[1].X-e.Points[0].X) / 2
	ry := absF(e.Points[1].Y-e.Points[0].Y) / 2
	fmt.Fprintf(b, `  <ellipse cx="%g" cy="%g" rx="%g" ry="%g" fill="%s" stroke="%s" stroke-width="%g"/>`+"\n",
		cx, cy, rx, ry, svgFill(e.Filled, e.Color), e.Color, e.Size)
}

const svgTextFontSize = 16

func writeSVGText(b *strings.Builder, e *Edit) {
	if len(e.Points) == 0 {
		return
	}
	fmt.Fprintf(b, `  <text x="%g" y="%g" fill="%s" font-size="%d" font-family="monospace">%s</text>`+"\n",
		e.Points[0].X, e.Points[0].Y, e.Color, svgTextFontSize, svgEscape(e.Text))
}

func svgEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func absF(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
