package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestScrollbarHugsWindowEdge walks the scrollbar geometry: the shell
// pads the content's right edge 0, so the document's list reaches the
// window's right edge and its overlay scrollbar hugs it; each row and
// the frontmatter card pad their own right 16 DIPs, so the text clears
// the 6 DIP bar without the bar covering any glyphs.
func TestScrollbarHugsWindowEdge(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "scroll.md")
	var b strings.Builder
	for range 50 {
		b.WriteString("Scrollbar geometry probe paragraph with enough words to wrap in a narrow window.\n\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}

	tt := s.renderSize(t, 1400, 500)
	tt.Frame()

	// The mygo scrollbar is the list's own overlay: scrollBars paints
	// the thumb with its right edge 3 DIPs inside the list's box right
	// edge (paint.go, g.v = box.X+box.W-right-width-3), so with the
	// shell's right padding 0 the bar hugs the window's edge. The list
	// box's own rect depends on the layout (a centered 900 DIP column
	// floats inside the window), so the geometry lands in the full_width
	// row assertion below instead.

	// The body rows clear the scrollbar: a line's text box ends at least
	// 12 DIPs before the window's right edge (the bar paints within 3),
	// and the frontmatter card pads its own right the same way.
	r, ok := paragraphRect(tt, "Scrollbar geometry probe")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	if got := 1400 - (r.X + r.W); got < 12 {
		t.Fatalf("paragraph text ends %v DIPs from the window's edge, want >= 12", got)
	}

	// The shell's left padding stays: the text keeps its 24 DIP left
	// margin against the window (the sidebar is hidden in these tests).
	if r.X < 24 {
		t.Fatalf("paragraph text starts at x=%v, want the 24 DIP left padding", r.X)
	}
}

// TestScrollbarListReachesWindowEdge pins the full_width geometry end to
// end: the list's box right edge sits within the bar's own 3 DIP inset
// of the window's right edge, so the overlay bar hugs the edge.
func TestScrollbarListReachesWindowEdge(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "scroll.md")
	if err := os.WriteFile(path, []byte("# T\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved := s.Saved
	saved.Layout = "full_width"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}

	// The hook grabs the frame's Context while the view builds; Bounds
	// answers against the committed frame afterwards, the way the view's
	// own handler reads row rectangles between frames.
	var viewCtx *ui.Context
	s.ContentHook = func(c *ui.Context) { viewCtx = c }
	tt := s.renderSize(t, 1400, 500)
	tt.Frame()

	list := s.ActiveTab().State.List.Handle.Bounds(viewCtx)
	if list.W <= 0 {
		t.Fatal("the list has no committed bounds")
	}
	if got := 1400 - (list.X + list.W); got > 4 {
		t.Fatalf("list right edge sits %v DIPs from the window's edge, want <= 4", got)
	}
}
