// Package testshot writes and shapes UI screenshots for a human to look
// at. It is test-only: every importer is a _test.go file, so it never
// reaches the production binary.
package testshot

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

// WritePng encodes img to path for the human eye.
func WritePng(t *testing.T, img image.Image, path string) {
	t.Helper()
	if img == nil {
		t.Fatalf("no image for %s", path)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// Crop cuts img at x, y, w x h, clamped to the image's bounds: a corner
// crop whose rect runs past an edge yields the intersection only.
func Crop(img image.Image, x, y, w, h int) image.Image {
	b := img.Bounds()
	x0, y0 := max(x, b.Min.X), max(y, b.Min.Y)
	x1, y1 := min(x+w, b.Max.X), min(y+h, b.Max.Y)
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(x0, y0), draw.Src)
	return dst
}

// Scale3x enlarges by 3 with nearest-neighbor sampling: no smoothing, so
// the shot shows the icon's actual pixels.
func Scale3x(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*3, b.Dy()*3))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			for dy := 0; dy < 3; dy++ {
				for dx := 0; dx < 3; dx++ {
					dst.SetRGBA(3*x+dx, 3*y+dy, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)})
				}
			}
		}
	}
	return dst
}
