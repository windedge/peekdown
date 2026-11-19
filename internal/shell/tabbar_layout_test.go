package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTitleBarHoldsTheTabs pins the bar's geometry: one 44 DIP row at the
// window's top carrying the app's mark, its name and the tab strip, and
// the tabs clear of the strip's overlay scroll bar.
func TestTitleBarHoldsTheTabs(t *testing.T) {
	s, dir := newShell(t)
	p := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(p, []byte("# Document\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{p})
	tt := s.renderSize(t, 800, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar", tt.Texts())
	}
	if bar.Y != 0 || bar.H != TitleBarHeight {
		t.Fatalf("the title bar is at y=%v h=%v, want y=0 h=%d", bar.Y, bar.H, TitleBarHeight)
	}

	// The app's mark is its own icon, a picture rather than a glyph: a
	// caption sized square at the bar's start, with the name right of it.
	logo, ok := tt.Find("App logo")
	if !ok {
		t.Fatalf("texts %v: want the app's mark", tt.Texts())
	}
	if logo.W < 16 || logo.W > 20 || logo.H < 16 || logo.H > 20 {
		t.Fatalf("the app's mark is %vx%v, want 16..20 DIPs", logo.W, logo.H)
	}
	if logo.X < bar.X || logo.Y < bar.Y || logo.Y+logo.H > bar.Y+bar.H {
		t.Fatalf("the app's mark %v sits outside the bar %v", logo, bar)
	}

	// The app's name rides the bar, left of the strip.
	name, ok := tt.Find("Peekdown")
	if !ok {
		t.Fatalf("texts %v: want the app's name", tt.Texts())
	}
	if name.X < 0 || name.X+name.W > bar.W {
		t.Fatalf("the app's name at %v sits outside the bar %v", name, bar)
	}
	if name.X < logo.X+logo.W {
		t.Fatalf("the app's name at %v overlaps its mark, which ends at %v", name, logo.X+logo.W)
	}

	// The strip is in the bar, not on a row of its own above the body.
	strip, ok := tt.Find("Tab bar")
	if !ok {
		t.Fatalf("texts %v: want the tab strip", tt.Texts())
	}
	if strip.Y < bar.Y || strip.Y+strip.H > bar.Y+bar.H {
		t.Fatalf("the strip %v sits outside the bar %v", strip, bar)
	}
	if strip.X <= name.X {
		t.Fatalf("the strip starts at %v, left of the app's name at %v", strip.X, name.X)
	}

	label, ok := tt.Find(filepath.Base(p))
	if !ok {
		t.Fatalf("texts %v: want the tab's label", tt.Texts())
	}
	if label.Y < strip.Y || label.Y+label.H > strip.Y+strip.H {
		t.Fatalf("the tab label spans y=%v..%v, outside the strip's %v..%v",
			label.Y, label.Y+label.H, strip.Y, strip.Y+strip.H)
	}
	// The strip scrolls sideways, and its overlay bar paints its last 9
	// DIPs: the label has to end above them.
	if got := strip.Y + strip.H - (label.Y + label.H); got < 9 {
		t.Fatalf("the tab label ends %v DIPs above the strip's bottom, want >= 9", got)
	}

	// The body starts under the bar, not under a second row.
	head, ok := tt.Find("Document")
	if !ok {
		t.Fatalf("texts %v: want the body's heading", tt.Texts())
	}
	if head.Y < bar.Y+bar.H {
		t.Fatalf("the body starts at y=%v, above the bar's bottom %v", head.Y, bar.Y+bar.H)
	}
}

// TestTitleBarButtonsAndTheirClearance walks the bar's toolbar: every
// control is in the bar, and none runs under the system's window controls
// whatever room they take (none, as in the tester's default, or the 138
// DIPs Win11 draws).
func TestTitleBarButtonsAndTheirClearance(t *testing.T) {
	controls := []string{"Open file", "Find in document", "App settings", "Main menu"}
	for _, tc := range []struct {
		bar  ui.TitleBar
		text string
	}{
		{ui.TitleBar{}, "no window controls"},
		{ui.TitleBar{Height: TitleBarHeight, Right: 138}, "138 DIPs of controls"},
	} {
		s, dir := newShell(t)
		if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
			t.Fatal(err)
		}
		tt := s.renderSize(t, 900, 600)
		tt.SetTitleBar(tc.bar)
		tt.Frame()

		bar, ok := tt.Find("Title bar")
		if !ok {
			t.Fatalf("%s: texts %v: want the title bar", tc.text, tt.Texts())
		}
		if bar.H < TitleBarHeight {
			t.Fatalf("%s: the bar is %v high, want >= %d", tc.text, bar.H, TitleBarHeight)
		}
		limit := bar.X + bar.W - tc.bar.Right
		prevX := float32(-1)
		for _, label := range controls {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s: texts %v: want the %q control", tc.text, tt.Texts(), label)
			}
			if r.W != 24 || r.H != 24 {
				t.Fatalf("%s: %q is %vx%v, want 24x24", tc.text, label, r.W, r.H)
			}
			if r.Y < bar.Y || r.Y+r.H > bar.Y+bar.H {
				t.Fatalf("%s: %q at %v sits outside the bar %v", tc.text, label, r, bar)
			}
			if r.X+r.W > limit+0.5 {
				t.Fatalf("%s: %q ends at %v, past the window controls' edge %v", tc.text, label, r.X+r.W, limit)
			}
			if prevX >= 0 && r.X < prevX {
				t.Fatalf("%s: %q starts at %v, left of the control before it at %v", tc.text, label, r.X, prevX)
			}
			prevX = r.X
		}
	}
}

// TestTitleBarOnTheWelcomePage keeps the bar on the page without a
// document: a window with a hidden title bar has no other place to drag.
func TestTitleBarOnTheWelcomePage(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 800, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar on the welcome page", tt.Texts())
	}
	if bar.Y != 0 || bar.H != TitleBarHeight {
		t.Fatalf("the welcome page's bar is at y=%v h=%v, want y=0 h=%d", bar.Y, bar.H, TitleBarHeight)
	}
	welcome, ok := tt.Find("Welcome to Peekdown")
	if !ok {
		t.Fatalf("texts %v: want the greeting", tt.Texts())
	}
	if welcome.Y < bar.Y+bar.H {
		t.Fatalf("the greeting at %v starts above the bar's bottom %v", welcome, bar.Y+bar.H)
	}
	if _, ok := tt.Find("Main menu"); !ok {
		t.Fatalf("texts %v: want the menu button without a document too", tt.Texts())
	}
}

// TestTitleBarTabsScroll walks a strip wider than the room it has: the
// last tab starts out of the bar and a sideways scroll brings it in, so
// the bar clips the strip rather than pushing the toolbar out.
func TestTitleBarTabsScroll(t *testing.T) {
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

	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar", tt.Texts())
	}
	menu, ok := tt.Find("Main menu")
	if !ok {
		t.Fatalf("texts %v: want the menu button", tt.Texts())
	}
	strip, ok := tt.Find("Tab bar")
	if !ok {
		t.Fatalf("texts %v: want the tab strip", tt.Texts())
	}
	if menu.X+menu.W > bar.X+bar.W-138 {
		t.Fatalf("the toolbar ends at %v, under the window controls", menu.X+menu.W)
	}
	if strip.X+strip.W > menu.X {
		t.Fatalf("the strip ends at %v, past the toolbar's start %v", strip.X+strip.W, menu.X)
	}
	first, ok := tt.Find(filepath.Base(paths[0]))
	if !ok {
		t.Fatalf("texts %v: want the first tab", tt.Texts())
	}
	// The active tab (the last one) is scrolled into view; the first has
	// left the strip.
	if first.X+first.W > bar.W+0.5 && first.X > menu.X {
		t.Fatalf("the first tab at %v overlaps the toolbar: the strip did not clip", first)
	}
	tabsSeen := 0
	for _, p := range paths {
		if r, ok := tt.Find(filepath.Base(p)); ok && r.W > 0 {
			tabsSeen++
		}
	}
	if tabsSeen == len(paths) {
		t.Fatalf("all %d tabs show in a 900 DIP bar: the strip did not scroll", len(paths))
	}
	if tabsSeen == 0 {
		t.Fatalf("no tab of the strip shows: %v", tt.Texts())
	}

	// A sideways scroll over the strip brings the first tabs back in.
	tt.Scroll(strip.X+20, strip.Y+10, -4000, 0)
	tt.Frame()
	if r, ok := tt.Find(filepath.Base(paths[0])); !ok || r.W <= 0 {
		t.Fatalf("the first tab at %v did not scroll back in", r)
	}
}
