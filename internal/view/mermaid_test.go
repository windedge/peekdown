package view

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/mermaid"
)

// mermaidDoc is a document with one mermaid block, above the fold.
const mermaidDoc = "# Diagrams\n\n```mermaid\ngraph TD; A-->B\n```\n\nafter\n"

// solidImage returns a 20x12 picture of one color, distinct from any
// blank, for the tests to tell the loaded from the missing.
func solidImage(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 20, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// mermaidState is a State whose loader the test drives, counting the
// calls and keeping the last inputs.
type mermaidState struct {
	State
	calls int
	src   string
	theme mermaid.Theme
	scale float64
	img   image.Image
	err   error
}

func (ms *mermaidState) load(src string, theme mermaid.Theme, scale float64) (image.Image, error) {
	ms.calls++
	ms.src, ms.theme, ms.scale = src, theme, scale
	if ms.err != nil {
		return nil, ms.err
	}
	if ms.img == nil {
		ms.img = solidImage(color.RGBA{R: 255, A: 255})
	}
	return ms.img, nil
}

func renderMermaid(t *testing.T, ms *mermaidState, src string) (*ui.Tester, int) {
	t.Helper()
	m := doc.New(src)
	ms.AnchorOf = anchorsOf(m)
	ms.LoadMermaid = ms.load
	st := &ms.State
	frames := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		frames++
		ui.List(c, &st.List, m.Len(), func(r int) {
			Render(c, m, st, m.RowBlock(r).Index)
		})
	}, 640, 400)
	return tt, frames
}

func TestMermaidBlockRendersImage(t *testing.T) {
	ms := &mermaidState{}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	if ms.calls == 0 {
		t.Fatal("the render never asked the loader")
	}
	if ms.src != "graph TD; A-->B" {
		t.Fatalf("src = %q", ms.src)
	}
	if ms.theme != mermaid.ThemeDefault {
		t.Fatalf("theme = %q, want default in the light tester", ms.theme)
	}
	// The loader's picture is in: the source text is gone from the row.
	if has(tt, "graph TD; A-->B") {
		t.Fatalf("texts %v: the source still shows", texts(tt))
	}
}

func TestMermaidLoaderNilKeepsSource(t *testing.T) {
	m := doc.New(mermaidDoc)
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &st.List, m.Len(), func(r int) {
			Render(c, m, st, m.RowBlock(r).Index)
		})
	}, 640, 400)
	if !has(tt, "graph TD; A-->B") {
		t.Fatalf("texts %v: want the plain source block", texts(tt))
	}
	if has(tt, "mermaid:") {
		t.Fatalf("texts %v: an error line over a fine block", texts(tt))
	}
}

func TestMermaidFailureFallsBackToSource(t *testing.T) {
	ms := &mermaidState{err: errors.New("mermaid: parse: bad diagram")}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	if !has(tt, "graph TD; A-->B") {
		t.Fatalf("texts %v: want the source block back", texts(tt))
	}
	if !has(tt, "mermaid: parse: bad diagram") {
		t.Fatalf("texts %v: want the error line", texts(tt))
	}
}

func TestMermaidErrorLineTruncated(t *testing.T) {
	long := errors.New("mermaid: " + strings.Repeat("x", 200))
	ms := &mermaidState{err: long}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	var shown string
	for _, s := range texts(tt) {
		if strings.HasPrefix(s, "mermaid: ") {
			shown = s
		}
	}
	if shown == "" {
		t.Fatalf("texts %v: want one truncated error line", texts(tt))
	}
	// The line stops at the limit: the runes past it are cut, one ellipsis
	// added, and nothing else of the 200 came through.
	if got := len([]rune(shown)); got != mermaidErrLimit+1 {
		t.Fatalf("error line %d runes: %q", got, shown)
	}
	if !strings.HasSuffix(shown, "\u2026") {
		t.Fatalf("error line %q lacks the ellipsis", shown)
	}
}

func TestMermaidThemeFollowsDark(t *testing.T) {
	ms := &mermaidState{}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	ms.theme = ""
	tt.SetDark(true)
	tt.Frame()
	if ms.theme != mermaid.ThemeDark {
		t.Fatalf("theme after dark = %q, want dark", ms.theme)
	}
}

func TestMermaidScaleRidesZoom(t *testing.T) {
	ms := &mermaidState{}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	tt.SetZoom(1.5)
	tt.Frame()
	if ms.scale != 2 {
		t.Fatalf("scale at zoom 1.5 = %v, want 2", ms.scale)
	}
	tt.SetZoom(3)
	tt.Frame()
	if ms.scale != mermaid.MaxScale {
		t.Fatalf("scale at zoom 3 = %v, want the %v cap", ms.scale, mermaid.MaxScale)
	}
}

func TestMermaidFailureCountedOncePerFrame(t *testing.T) {
	// A failing render must not ask twice within one frame's row build.
	ms := &mermaidState{err: errors.New("mermaid: nope")}
	renderMermaid(t, ms, mermaidDoc)
	if ms.calls != 1 {
		t.Fatalf("calls = %d in one frame, want 1", ms.calls)
	}
}

// TestMermaidBitmapRebuiltOnce proves the bitmap is made once: the
// second frame reuses it instead of converting the pixels again.
func TestMermaidBitmapRebuiltOnce(t *testing.T) {
	ms := &mermaidState{}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	if len(ms.State.bitmapCache) != 1 {
		t.Fatalf("bitmapCache = %d entries after frame 1, want 1", len(ms.State.bitmapCache))
	}
	var first *ui.Bitmap
	for _, b := range ms.State.bitmapCache {
		first = b
	}
	tt.Frame()
	if len(ms.State.bitmapCache) != 1 {
		t.Fatalf("bitmapCache = %d entries after frame 2, want 1", len(ms.State.bitmapCache))
	}
	for _, b := range ms.State.bitmapCache {
		if b != first {
			t.Fatal("the second frame built a new bitmap")
		}
	}
	if ms.calls != 1 {
		t.Fatalf("loader calls = %d across two frames, want 1", ms.calls)
	}
}

// TestMermaidErrorSinglePrefix keeps the shown line to one "mermaid:",
// though the test loader's error carries one already.
func TestMermaidErrorSinglePrefix(t *testing.T) {
	ms := &mermaidState{err: errors.New("mermaid: parse: bad diagram")}
	tt, _ := renderMermaid(t, ms, mermaidDoc)
	for _, s := range texts(tt) {
		if strings.Count(s, "mermaid:") > 1 {
			t.Fatalf("texts %v: a doubled prefix slipped in", texts(tt))
		}
	}
}
