package shell

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
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
	_, tt := openOutline(t, 900)
	idle, ok := tt.Find("H2 two")
	if !ok {
		t.Fatal("outline row missing")
	}
	active, ok := tt.Find("H1 one")
	if !ok {
		t.Fatal("active outline row missing")
	}
	// The row's Find box is its text, indented off the row's own edge;
	// a few DIPs left of it still sit on the row's background.
	x, y := idle.X-6, idle.Y+idle.H/2
	ax, ay := active.X-6, active.Y+active.H/2
	before := pixelAt(tt.Image(), x, y)
	activeIdle := pixelAt(tt.Image(), ax, ay)
	// The pane's translucent fill lands composited (A=255), so a token
	// match means the channels.
	if !channelClose(before, windowLight().Surface, 1) {
		t.Fatalf("idle row pixel %v, want the pane surface %v", before, windowLight().Surface)
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

	tt.Move(880, 580)
	tt.Frame()
	if got := pixelAt(tt.Image(), x, y); got != before {
		t.Fatalf("off hover pixel %v, want back to %v", got, before)
	}
}

// TestSidebarTabHoverFeedback checks the inactive Files/Outline header
// label lights on hover with the hover surface; the active tab keeps its
// accent wash whatever the pointer does. The tabs carry their own labels,
// so Find boxes the whole button; the sample keeps off the text.
func TestSidebarTabHoverFeedback(t *testing.T) {
	_, tt := openOutline(t, 900)
	files, ok := tt.Find("Files")
	if !ok {
		t.Fatal("Files tab missing")
	}
	x, y := files.X+3, files.Y+files.H/2
	before := pixelAt(tt.Image(), x, y)
	if !channelClose(before, windowLight().Surface, 1) {
		t.Fatalf("idle tab pixel %v, want the header surface %v", before, windowLight().Surface)
	}

	tt.Move(x, y)
	tt.Frame()
	if got := pixelAt(tt.Image(), x, y); !channelClose(got, windowLight().SurfaceHover, 1) {
		t.Fatalf("inactive tab hover pixel %v, want %v (before %v)", got, windowLight().SurfaceHover, before)
	}

	// The active Outline tab stays as it is under the pointer.
	outline, ok := tt.Find("Outline")
	if !ok {
		t.Fatal("Outline tab missing")
	}
	ox, oy := outline.X+3, outline.Y+outline.H/2
	idle := pixelAt(tt.Image(), ox, oy)
	tt.Move(ox, oy)
	tt.Frame()
	if got := pixelAt(tt.Image(), ox, oy); got != idle {
		t.Fatalf("active tab hover changed the pixels: %v -> %v, want the accent wash to hold", idle, got)
	}
}

// TestSidebarTabHoverActivatesAsFiles checks the files pane under the
// same hover branch: its rows come from the tree, the header wash and
// the frame must build without a panic.
func TestSidebarTabHoverActivatesAsFiles(t *testing.T) {
	s, dir := newShell(t)
	p := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(p, []byte("# one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	s.SetSidebarTab(0)
	tt := s.renderSize(t, 900, 600)
	tt.Frame()
	tt.Move(60, 300)
	tt.Frame()
}
