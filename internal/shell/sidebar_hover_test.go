package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// pixelAt reads the rendered frame at DIPs coordinates, headless scale 1.
// Hover feedback is a background wash, so the change shows in pixels
// where no element's text or geometry does.
func pixelAt(img *image.RGBA, x, y float32) ui.Color {
	i := img.PixOffset(int(x), int(y))
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// TestOutlineRowHoverFeedback moves the pointer onto an outline row and
// checks its background turns the hover surface, and off again: the rows
// were flat before, hover lit nothing. The sample sits inside the row's
// start padding, clear of the text; H1 holds the active section's
// highlight, so it is the one to prove hover does not stack on it.
func TestOutlineRowHoverFeedback(t *testing.T) {
	// 1100 is the window width under which the outline hides itself, so
	// the pane has to be sampled wider than that.
	_, tt := openOutline(t, 1200)
	idle, ok := tt.Find("H2 two")
	if !ok {
		t.Fatalf("outline row missing: %v", tt.Texts())
	}
	active, ok := tt.Find("H1 one")
	if !ok {
		t.Fatal("active outline row missing")
	}
	// The row's Find box is its text; a few DIPs right of its end still
	// sit on the row's own padding, clear of the glyphs and of the pane's
	// hairline at the card edge.
	x, y := idle.X+idle.W+6, idle.Y+idle.H/2
	ax, ay := active.X+active.W+6, active.Y+active.H/2
	before := pixelAt(tt.Image(), x, y)
	activeIdle := pixelAt(tt.Image(), ax, ay)
	// The pane is a translucent card over the gradient now: its idle
	// row reads the composited Card token, not an opaque Surface.
	k := accentThemeByID(settings.AccentThemeDefault).Light
	wantIdle := over(k.Card, k.BgTop)
	if !channelClose(before, wantIdle, 1) {
		t.Fatalf("idle row pixel %v, want Card over the gradient %v", before, wantIdle)
	}
	if !channelClose(activeIdle, windowLight().SurfaceHover, 1) {
		t.Fatalf("active row pixel %v, want the highlight %v", activeIdle, windowLight().SurfaceHover)
	}

	tt.Move(x, y)
	tt.Frame()
	if got := pixelAt(tt.Image(), x, y); !channelClose(got, windowLight().SurfaceHover, 1) {
		t.Fatalf("hover pixel %v, want %v (before %v)", got, windowLight().SurfaceHover, before)
	}
	if got := pixelAt(tt.Image(), ax, ay); got != activeIdle {
		t.Fatalf("active row under the pointer changed: %v -> %v, the highlight must not stack", activeIdle, got)
	}

	tt.Move(600, 580)
	tt.Frame()
	if got := pixelAt(tt.Image(), x, y); got != before {
		t.Fatalf("off hover pixel %v, want back to %v", got, before)
	}
}
