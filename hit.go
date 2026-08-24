package main

import "math"

// Distance returns the shortest distance from p to e's geometry, in
// subpixel units. It is the basis for both the eraser (erase whichever
// whole edit the cursor touches) and the select tool (click nearest edit).
func (e *Edit) Distance(p Point) float64 {
	switch e.Kind {
	case KindStroke, KindLine:
		return polylineDistance(e.Points, p)
	case KindRect:
		if e.Filled && pointInRect(e.Points, p) {
			return 0
		}
		return rectOutlineDistance(e.Points, p)
	case KindCircle:
		if e.Filled && pointInEllipse(e.Points, p) {
			return 0
		}
		return circleOutlineDistance(e.Points, p)
	case KindText, KindFill:
		if len(e.Points) == 0 {
			return math.Inf(1)
		}
		return distance(e.Points[0], p)
	default:
		return math.Inf(1)
	}
}

func distance(a, b Point) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Hypot(dx, dy)
}

// segmentDistance returns the distance from p to segment ab.
func segmentDistance(a, b, p Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return distance(a, p)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	t = math.Max(0, math.Min(1, t))
	proj := Point{X: a.X + t*dx, Y: a.Y + t*dy}
	return distance(proj, p)
}

func polylineDistance(pts []Point, p Point) float64 {
	if len(pts) == 0 {
		return math.Inf(1)
	}
	if len(pts) == 1 {
		return distance(pts[0], p)
	}
	min := math.Inf(1)
	for i := 0; i+1 < len(pts); i++ {
		if d := segmentDistance(pts[i], pts[i+1], p); d < min {
			min = d
		}
	}
	return min
}

// rectOutlineDistance treats pts[0],pts[1] as opposite corners and measures
// distance to the four edges (not the filled interior), so clicking inside
// an unfilled rect doesn't count as a hit.
func rectOutlineDistance(pts []Point, p Point) float64 {
	if len(pts) < 2 {
		return math.Inf(1)
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	corners := []Point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	min := math.Inf(1)
	for i := 0; i < 4; i++ {
		if d := segmentDistance(corners[i], corners[(i+1)%4], p); d < min {
			min = d
		}
	}
	return min
}

func pointInRect(pts []Point, p Point) bool {
	if len(pts) < 2 {
		return false
	}
	x0, x1 := math.Min(pts[0].X, pts[1].X), math.Max(pts[0].X, pts[1].X)
	y0, y1 := math.Min(pts[0].Y, pts[1].Y), math.Max(pts[0].Y, pts[1].Y)
	return p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1
}

func pointInEllipse(pts []Point, p Point) bool {
	if len(pts) < 2 {
		return false
	}
	cx, cy := (pts[0].X+pts[1].X)/2, (pts[0].Y+pts[1].Y)/2
	rx, ry := math.Abs(pts[1].X-pts[0].X)/2, math.Abs(pts[1].Y-pts[0].Y)/2
	if rx == 0 || ry == 0 {
		return false
	}
	nx, ny := (p.X-cx)/rx, (p.Y-cy)/ry
	return nx*nx+ny*ny <= 1
}

// circleOutlineDistance approximates distance to an ellipse's outline
// (pts[0],pts[1] are its bounding-box corners) by normalizing p into the
// ellipse's unit-circle space, then scaling the unit-circle distance back
// by the smaller radius. Exact for circles, a reasonable approximation for
// stretched ovals.
func circleOutlineDistance(pts []Point, p Point) float64 {
	if len(pts) < 2 {
		return math.Inf(1)
	}
	cx, cy := (pts[0].X+pts[1].X)/2, (pts[0].Y+pts[1].Y)/2
	rx, ry := math.Abs(pts[1].X-pts[0].X)/2, math.Abs(pts[1].Y-pts[0].Y)/2
	if rx == 0 || ry == 0 {
		return distance(Point{cx, cy}, p)
	}
	nx, ny := (p.X-cx)/rx, (p.Y-cy)/ry
	unitDist := math.Abs(math.Hypot(nx, ny) - 1)
	return unitDist * math.Min(rx, ry)
}
