package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// ExportPNG rasterizes every edit at full subpixel resolution (one image
// pixel per braille dot) and writes it as a PNG to path.
func ExportPNG(d *Document, path string) error {
	minX, minY, maxX, maxY := boundingBox(d.Edits)
	if minX > maxX {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}
	const pad = 4
	w := int(maxX-minX) + pad*2 + 1
	h := int(maxY-minY) + pad*2 + 1
	cols := (w + SubpixW - 1) / SubpixW
	rows := (h + SubpixH - 1) / SubpixH
	ox, oy := minX-pad, minY-pad

	r := RasterizeDocument(d.Edits, cols, rows, ox, oy, 1, "", 0, "")

	img := image.NewRGBA(image.Rect(0, 0, cols*SubpixW, rows*SubpixH))
	blank := color.RGBA{0, 0, 0, 255}
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			c := r.at(col, row)
			cellBg := blank
			if c.bg != "" {
				cellBg = hexColor(c.bg).(color.RGBA)
			}
			for ly := 0; ly < SubpixH; ly++ {
				for lx := 0; lx < SubpixW; lx++ {
					px, py := col*SubpixW+lx, row*SubpixH+ly
					if c.glyph != 0 || c.dots&brailleBit[lx][ly] != 0 {
						img.Set(px, py, hexColor(c.color))
					} else {
						img.Set(px, py, cellBg)
					}
				}
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}

func boundingBox(edits []*Edit) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, e := range edits {
		for _, p := range e.Points {
			minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
			maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
		}
	}
	return
}

func hexColor(s string) color.Color {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}
