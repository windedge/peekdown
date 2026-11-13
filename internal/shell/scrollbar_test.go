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
	// edge (paint.go, g.v = box.X+box.W-right-width-3), and the list
	// spans the window in both layouts, so the bar hugs the window's
	// edge; TestCenteredScrollbarHugsWindowEdge pins that for the capped
	// column too.

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

// TestScrollbarListReachesWindowEdge pins the list's geometry end to
// end: its box right edge sits within the bar's own 3 DIP inset of the
// window's right edge, so the overlay bar hugs the edge.
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

// TestCenteredScrollbarHugsWindowEdge pins the centered layout's split:
// the text column alone caps at 900 DIPs and centers, while the list, and
// so its overlay scrollbar, keep spanning the window. Before the split the
// cap landed on the list itself, which floated the bar in from the edge to
// the column's right side.
func TestCenteredScrollbarHugsWindowEdge(t *testing.T) {
	for _, tc := range []struct {
		layout string
		capped bool
	}{
		{"centered", true},
		{"full_width", false},
	} {
		s, dir := newShell(t)
		path := filepath.Join(dir, "long.md")
		long := strings.Repeat("word ", 200) + "\n"
		if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.OpenFile(path); err != nil {
			t.Fatal(err)
		}
		saved := s.Saved
		saved.Layout = tc.layout
		if err := s.Apply(saved); err != nil {
			t.Fatal(err)
		}
		var viewCtx *ui.Context
		s.ContentHook = func(c *ui.Context) { viewCtx = c }
		tt := s.renderSize(t, 1600, 900)
		tt.Frame()

		list := s.ActiveTab().State.List.Handle.Bounds(viewCtx)
		if list.W <= 0 {
			t.Fatalf("%s: the list has no committed bounds", tc.layout)
		}
		if got := 1600 - (list.X + list.W); got > 4 {
			t.Fatalf("%s: the list's right edge sits %v DIPs from the window's edge, want <= 4 (the bar hugs it)",
				tc.layout, got)
		}

		r, ok := paragraphRect(tt, "word")
		if !ok {
			t.Fatalf("%s: texts %v: want the paragraph", tc.layout, tt.Texts())
		}
		// The bar paints within the list's right 9 DIPs; the text ends well
		// left of that in both layouts.
		if got := 1600 - (r.X + r.W); got < barClearance {
			t.Fatalf("%s: the text ends %v DIPs from the window's edge, want >= %d", tc.layout, got, barClearance)
		}
		if !tc.capped {
			if r.W < 1500 {
				t.Fatalf("full_width: the text is %v wide, want the full column", r.W)
			}
			continue
		}
		// Centered: the text column is the 900 DIP cap, centered in the
		// window rather than pulled to its left edge.
		if r.W > 900 {
			t.Fatalf("centered: the text is %v wide, want at most the 900 DIP cap", r.W)
		}
		if center := r.X + r.W/2; center < 800-20 || center > 800+20 {
			t.Fatalf("centered: the text centers at %v, want the window's 800", center)
		}
		if r.X < 200 {
			t.Fatalf("centered: the text starts at x=%v, want the centered column, not the window's left edge", r.X)
		}
	}
}

// TestCenteredColumnFitsNarrowWindow keeps the cap off a window narrower
// than it: the column follows the room and the text stays clear of the bar
// and the window's edges.
func TestCenteredColumnFitsNarrowWindow(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "long.md")
	long := strings.Repeat("word ", 200) + "\n"
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	saved := s.Saved
	saved.Layout = "centered"
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 700, 500)
	tt.Frame()
	r, ok := paragraphRect(tt, "word")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	if r.X < 24 {
		t.Fatalf("the text starts at x=%v, want the 24 DIP left padding", r.X)
	}
	if got := 700 - (r.X + r.W); got < barClearance {
		t.Fatalf("the text ends %v DIPs from the window's edge, want >= %d (the bar's strip)", got, barClearance)
	}
	if r.W > 700-24-barClearance {
		t.Fatalf("the text is %v wide, wider than a 700 DIP window leaves", r.W)
	}
}

// TestCenteredFrontmatterCapped centers the frontmatter card with the
// body: the card sits above the list, outside its rows, so it takes the
// same 900 DIP cap instead of spanning the window over a 900 DIP column.
func TestCenteredFrontmatterCapped(t *testing.T) {
	for _, tc := range []struct {
		layout string
		capped bool
	}{
		{"centered", true},
		{"full_width", false},
	} {
		s, dir := newShell(t)
		path := filepath.Join(dir, "front.md")
		if err := os.WriteFile(path, []byte("---\ntitle: Note\ntags:\n  - a\n---\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.OpenFile(path); err != nil {
			t.Fatal(err)
		}
		saved := s.Saved
		saved.Layout = tc.layout
		if err := s.Apply(saved); err != nil {
			t.Fatal(err)
		}
		tt := s.renderSize(t, 1600, 900)
		tt.Frame()
		tt.Frame()
		tag, ok := tt.Find("yaml")
		if !ok {
			t.Fatalf("%s: texts %v: want the format tag", tc.layout, tt.Texts())
		}
		button, ok := tt.Find("Copy YAML")
		if !ok {
			t.Fatalf("%s: texts %v: want the copy button", tc.layout, tt.Texts())
		}
		// The button sits at the card's right edge: it lands at the column
		// with the card, not out at the window's edge.
		cardRight := button.X + button.W
		if tc.capped {
			if cardRight > 900+362 {
				t.Fatalf("centered: the card ends at %v, past the 900 DIP column (which ends at %v)", cardRight, 900+362)
			}
			if tag.X < 200 {
				t.Fatalf("centered: the card's tag starts at x=%v, want the centered column", tag.X)
			}
			continue
		}
		if cardRight < 1400 {
			t.Fatalf("full_width: the card ends at %v, want it out by the window's edge", cardRight)
		}
	}
}
