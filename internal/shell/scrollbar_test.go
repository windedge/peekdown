package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestScrollbarHugsCardEdge walks the scrollbar geometry: the shell
// pads the document card's right edge 0, so the list reaches that edge
// and its overlay scrollbar hugs the card, 8 DIPs inside the window;
// each row and the frontmatter card pad their own right 16 DIPs, so the
// text clears the bar without it covering any glyphs.
func TestScrollbarHugsCardEdge(t *testing.T) {
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
	// the thumb within the list's right 9 DIPs, and the list spans the
	// document card, so the bar now hugs the card's inner right edge;
	// TestCenteredScrollbarHugsCardEdge pins that for the capped column.

	// The body rows clear the scrollbar: a line's text box ends at least
	// 12 DIPs before the card's right edge, and the frontmatter card
	// pads its own right the same way.
	r, ok := paragraphRect(tt, "Scrollbar geometry probe")
	if !ok {
		t.Fatalf("texts %v: want the paragraph", tt.Texts())
	}
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	if got := card.X + card.W - (r.X + r.W); got < 12 {
		t.Fatalf("paragraph text ends %v DIPs from the card's edge, want >= 12", got)
	}

}

// TestScrollbarListReachesCardEdge pins the list's geometry end to end:
// its box right edge meets the document card's inner right edge, so the
// overlay bar hugs the card whether or not the side panels are open.
func TestScrollbarListReachesCardEdge(t *testing.T) {
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
	// The outline open by default narrows the card; the list must still
	// reach the card's inner right edge, not the pane's outer one.
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	if got := card.X + card.W - 1 - (list.X + list.W); got > 4 || got < -4 {
		t.Fatalf("list right edge sits %v DIPs from the card's inner edge, want <= 4", got)
	}

	// With every panel closed the card sits 8 DIPs inside the window.
	s.Outline.Visible = false
	tt = s.renderSize(t, 1400, 500)
	tt.Frame()
	list = s.ActiveTab().State.List.Handle.Bounds(viewCtx)
	card, ok = tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	if got := card.X + card.W - 1 - (list.X + list.W); got > 4 || got < -4 {
		t.Fatalf("list right edge sits %v DIPs from the card's inner edge, want <= 4", got)
	}
}

// TestCenteredScrollbarHugsCardEdge pins the centered layout's split:
// the text column alone caps at 900 DIPs and centers, while the list, and
// so its overlay scrollbar, keep spanning the card. Before the split the
// cap landed on the list itself, which floated the bar in from the card's
// edge to the column's right side.
func TestCenteredScrollbarHugsCardEdge(t *testing.T) {
	for _, tc := range []struct {
		layout string
		capped bool
	}{
		{"centered", true},
		{"full_width", false},
	} {
		s, dir := newShell(t)
		s.Outline.Visible = false
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
		card, ok := tt.Find("Document card")
		if !ok {
			t.Fatalf("%s: no document card: %v", tc.layout, tt.Texts())
		}
		if got := card.X + card.W - 1 - (list.X + list.W); got > 4 || got < -4 {
			t.Fatalf("%s: the list's right edge sits %v DIPs from the card's inner edge, want <= 4 (the bar hugs it)",
				tc.layout, got)
		}

		r, ok := paragraphRect(tt, "word")
		if !ok {
			t.Fatalf("%s: texts %v: want the paragraph", tc.layout, tt.Texts())
		}
		// The bar paints within the list's right 9 DIPs; the text ends well
		// left of that in both layouts.
		if got := card.X + card.W - (r.X + r.W); got < barClearance {
			t.Fatalf("%s: the text ends %v DIPs from the card's edge, want >= %d", tc.layout, got, barClearance)
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
// and the card's edges.
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
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card: %v", tt.Texts())
	}
	if got := r.X - card.X; got < 16 || got > 40 {
		t.Fatalf("the text starts %v DIPs inside the card, want its reading inset", got)
	}
	if got := card.X + card.W - (r.X + r.W); got < barClearance {
		t.Fatalf("the text ends %v DIPs from the card's edge, want >= %d (the bar's strip)", got, barClearance)
	}
	if r.W > card.W-32 {
		t.Fatalf("the text is %v wide, wider than the card leaves", r.W)
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
		s.Outline.Visible = false
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
