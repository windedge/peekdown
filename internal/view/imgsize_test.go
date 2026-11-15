package view

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// pngSource builds a solid-color picture at w x h, the image the tests
// load through the State's LoadImage.
func pngSource(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// frameWithImage renders src with LoadImage handing out a real bitmap of
// bw x bh, the picture pngSource made, so the Image element draws pixels a
// geometric test can measure. A bitmap whose aspect matches the box the
// test asks for draws at the box's full size under Fit(Contain).
func frameWithImage(t *testing.T, src string, bw, bh, w, h int) (*doc.Model, *ui.Tester, *State, image.Image) {
	t.Helper()
	m := doc.New(src)
	st := &State{AnchorOf: anchorsOf(m)}
	st.LoadImage = func(url string) (ui.ImageSource, error) {
		return ui.DecodeImage(pngSource(bw, bh, color.RGBA{255, 0, 0, 255}))
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, w, h)
	tt.Frame()
	return m, tt, st, tt.Image()
}

// TestHTMLImgAttrsFixRenderSize draws an HTML block's <img width height>
// and asserts the picture's box: 200x100, the attributes' size, not the
// 600x400 the bitmap itself is.
func TestHTMLImgAttrsFixRenderSize(t *testing.T) {
	_, _, _, img := frameWithImage(t,
		"<div>\n<img src=\"https://e.x/p.png\" alt=\"pic\" width=\"200\" height=\"100\">\n</div>\n", 200, 100, 640, 400)
	red := ui.Color{R: 255, G: 0, B: 0, A: 255}
	x0, y0, x1, y1 := redBounds(img, red)
	if x0 < 0 {
		t.Fatal("no picture drew")
	}
	if w, h := x1-x0+1, y1-y0+1; w < 190 || w > 210 || h < 90 || h > 110 {
		t.Fatalf("picture box %dx%d, want about 200x100", w, h)
	}
}

// TestMarkdownImageStaysAtNaturalSize checks the markdown image without
// attributes keeps its own size, the 600x400 the bitmap is, no box the
// HTML path would shrink it into.
func TestMarkdownImageStaysAtNaturalSize(t *testing.T) {
	_, _, _, img := frameWithImage(t, "![pic](https://e.x/p.png)\n", 600, 400, 800, 600)
	red := ui.Color{R: 255, G: 0, B: 0, A: 255}
	x0, y0, x1, y1 := redBounds(img, red)
	if x0 < 0 {
		t.Fatal("no picture drew")
	}
	if w, h := x1-x0+1, y1-y0+1; w < 580 || h < 380 {
		t.Fatalf("picture box %dx%d, want the bitmap's 600x400", w, h)
	}
}

// redBounds returns the bounding box of the pixels exactly c, -1 when
// none drew.
func redBounds(img image.Image, c ui.Color) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = -1, -1, -1, -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bl>>8) == c.B && uint8(a>>8) == c.A {
				if x0 < 0 || x < x0 {
					x0 = x
				}
				if y0 < 0 || y < y0 {
					y0 = y
				}
				if x > x1 {
					x1 = x
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return
}
