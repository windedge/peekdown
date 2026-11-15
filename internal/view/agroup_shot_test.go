package view

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// writeShot encodes img to path for the human eye.
func writeShot(t *testing.T, img image.Image, path string) {
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

// TestAGroupShot writes the A-group items' screenshots for a human to
// look at: the nested list's per-level markers, the mermaid error
// panel's three facts and a wide diagram in its sideways scroller.
// Skipped unless PEEKDOWN_SHOT is set, the observation shots' gating.
func TestAGroupShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the group A screenshots")
	}

	nested := "- one\n  - two\n    - three\n      - four\n\n1. first\n   1. inner A\n   2. inner B\n2. second\n\n- [x] done\n- [ ] open\n"
	m := doc.New(nested)
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 640, 420)
	tt.Frame()
	writeShot(t, tt.Image(), "C:/tmp/shot-nested-list.png")

	fail := &mermaidState{err: errors.New("mermaid: parse: unexpected token at line 3")}
	fail.anchor(t, mermaidDoc, "C:/tmp/shot-mermaid-error.png")

	wide := &mermaidState{}
	wide.img = gradientShot(1200, 200)
	fail.anchor(t, mermaidDocWide, "C:/tmp/shot-mermaid-wide.png")
}

// gradientShot paints w x h of stripes, wide enough that the diagram's
// sideways scroller has room to scroll.
func gradientShot(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 256), uint8(y), 128, 255})
		}
	}
	return img
}

// anchor renders src through the state's loader and writes the frame to
// path, the production wiring of one mermaid row.
func (ms *mermaidState) anchor(t *testing.T, src, path string) {
	t.Helper()
	m := doc.New(src)
	ms.AnchorOf = anchorsOf(m)
	ms.LoadMermaid = ms.load
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &ms.List, m.Len(), func(r int) {
			Render(c, m, &ms.State, m.RowBlock(r).Index)
		})
	}, 640, 300)
	tt.Frame()
	writeShot(t, tt.Image(), path)
}
