package main

import "testing"

func BenchmarkThickBrushHighZoom(b *testing.B) {
	pts := make([]Point, 60)
	x, y := 0.0, 0.0
	for i := range pts {
		x += 2
		y += 1
		pts[i] = Point{X: x, Y: y}
	}
	e := &Edit{ID: 1, Kind: KindStroke, Points: pts, Color: "#ffffff", Size: 6}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RasterizeDocument([]*Edit{e}, 160, 45, 0, 0, 8, "#ffaa00", 0, "", nil, 0)
	}
}
