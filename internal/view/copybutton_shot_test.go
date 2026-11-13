package view

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestCopyButtonShot writes the code block's copy button screenshots for
// a human to look at; skipped unless PEEKDOWN_SHOT is set, so the regular
// test runs stay headless-clean. The hover pass rests the pointer on the
// button past mygo's tooltip delay, so the icon-only button and its
// "Copy code" tooltip land in the same frame.
func TestCopyButtonShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the copy button screenshots")
	}
	src := "Intro paragraph.\n\n```go\nfunc main() {\n\tfmt.Println(\"hello peekdown\")\n}\n```\n"
	_, tt, st := frame(t, src, 640, 400)
	st.CopyText = func(string) {}
	tt.Frame()
	r, ok := tt.Find("Copy")
	if !ok {
		t.Fatalf("texts %v: no copy button", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(650 * time.Millisecond)
	tt.Frame()
	writeShotPng(t, tt, "C:/tmp/copyblock-full.png")

	// Crop the block's top-right corner around the button and blow it up
	// 3x nearest-neighbor, so the icon's shape is easy to judge.
	src3x := scale3x(shotCrop(tt.Image(), r))
	writeImagePng(t, src3x, "C:/tmp/copy-icon-only.png")
}

// shotCrop cuts a square around the button's top-right corner: the icon
// button, the block's edge and the tooltip's anchor all in one frame.
func shotCrop(img *image.RGBA, r ui.Rect) *image.RGBA {
	cx := int(r.X + r.W/2)
	cy := int(r.Y + r.H/2)
	half := 70
	x0, y0 := max(cx-half, 0), max(cy-half, 0)
	x1, y1 := min(cx+half, img.Bounds().Max.X), min(cy+half, img.Bounds().Max.Y)
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(x0, y0), draw.Src)
	return dst
}

// scale3x enlarges by 3 with nearest-neighbor sampling: no smoothing, so
// the shot shows the icon's actual pixels.
func scale3x(img *image.RGBA) *image.RGBA {
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

func writeShotPng(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	if img := tt.Image(); img == nil {
		t.Fatalf("no image for %s", path)
	} else {
		writeImagePng(t, img, path)
	}
}

func writeImagePng(t *testing.T, img image.Image, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}
