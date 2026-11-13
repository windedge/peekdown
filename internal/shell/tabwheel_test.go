package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTabStripTakesAStraightWheel pins the wheel over the tab strip: a
// notch of a straight wheel (dy) scrolls the sideways strip, which mygo
// alone leaves dead on a horizontal-only container, and Shift+wheel
// keeps its native sideways route.
func TestTabStripTakesAStraightWheel(t *testing.T) {
	s, dir := newShell(t)
	var paths []string
	for i := range 14 {
		p := filepath.Join(dir, fmt.Sprintf("tab-%02d-with-a-longer-name.md", i))
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	s.OpenPaths(paths)
	tt := s.renderSize(t, 800, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	strip, ok := tt.Find("Tab bar")
	if !ok {
		t.Fatalf("texts %v: want the tab strip", tt.Texts())
	}
	if s.tabScroll.MaxX <= 0 {
		t.Fatalf("the strip does not overflow: MaxX=%.1f, the test opens too few tabs", s.tabScroll.MaxX)
	}
	// Park at the start so both wheels have room to move.
	s.tabScroll.X = 0
	tt.Frame()

	// A straight wheel notch scrolls sideways, at once: the handler takes
	// the notch, sets the offset and the frame the tester settles applies it.
	before := s.tabScroll.X
	tt.Wheel(strip.X+60, strip.Y+10, 0, 120)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if s.tabScroll.X <= before+1 {
		t.Fatalf("a straight wheel notch at dy=120 left the strip at %.1f, want past %.1f", s.tabScroll.X, before+1)
	}

	// Shift+wheel keeps mygo's native sideways route.
	before = s.tabScroll.X
	tt.Wheel(strip.X+60, strip.Y+10, 120, 0)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if s.tabScroll.X <= before+1 {
		t.Fatalf("a shifted notch at dx=120 left the strip at %.1f, want past %.1f", s.tabScroll.X, before+1)
	}

	// More straight notches in a row keep scrolling toward the end.
	s.tabScroll.X = 0
	tt.Frame()
	for range 30 {
		tt.Wheel(strip.X+60, strip.Y+10, 0, 120)
		tt.Advance(300 * time.Millisecond)
	}
	tt.Frame()
	if s.tabScroll.X < s.tabScroll.MaxX-0.5 {
		t.Fatalf("notches of dy=120 past the end left the strip at %.1f of %.1f, want it at the end", s.tabScroll.X, s.tabScroll.MaxX)
	}
}

// TestTabStripWheelComesBackFromTheEnd pins the route the first fix
// missed: opening a tab parks the strip at its end (ScrollIntoView), and
// from there the old "at the end pass everything" guard handed notches
// pointing back into the content to mygo's native sideways route, which
// a straight wheel's dy cannot move. The wheel stayed dead until the
// strip grew room forward again.
func TestTabStripWheelComesBackFromTheEnd(t *testing.T) {
	s, dir := newShell(t)
	var paths []string
	for i := range 14 {
		p := filepath.Join(dir, fmt.Sprintf("tab-%02d-with-a-longer-name.md", i))
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	s.OpenPaths(paths)
	tt := s.renderSize(t, 800, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	label, ok := tt.Find(filepath.Base(paths[len(paths)-1]))
	if !ok {
		t.Fatalf("texts %v: want the last tab's label", tt.Texts())
	}
	if s.tabScroll.MaxX <= 0 {
		t.Fatalf("the strip does not overflow: MaxX=%.1f", s.tabScroll.MaxX)
	}
	// A fresh tab's ScrollIntoView parks the strip at its end; pin it.
	if s.tabScroll.X < s.tabScroll.MaxX-0.5 {
		s.tabScroll.X = s.tabScroll.MaxX
		tt.Frame()
	}
	if s.tabScroll.X < s.tabScroll.MaxX-0.5 {
		t.Fatalf("the strip sits at %.1f of %.1f, want it parked at the end like a fresh tab leaves it", s.tabScroll.X, s.tabScroll.MaxX)
	}

	// The point sits on the last tabs themselves, as a user's pointer does.
	cx, cy := label.X+label.W/2, label.Y+label.H/2
	tt.Move(cx, cy)
	tt.Frame()

	// A notch pointing back into the content scrolls toward the start.
	before := s.tabScroll.X
	tt.Wheel(cx, cy, 0, -120)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if s.tabScroll.X >= before-1 {
		t.Fatalf("a notch of dy=-120 from the end left the strip at %.1f, want back past %.1f", s.tabScroll.X, before-1)
	}

	// And the way out still passes on: nothing to the left of the start.
	s.tabScroll.X = 0
	tt.Frame()
	tt.Wheel(cx, cy, 0, -120)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if s.tabScroll.X > 0.5 {
		t.Fatalf("a notch of dy=-120 at the start left the strip at %.1f, want it parked at 0", s.tabScroll.X)
	}
}
