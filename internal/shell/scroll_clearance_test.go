package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// barClearance is the room a text row keeps from a scroll container's
// right edge: mygo's scroll bar paints 6 DIPs wide with a 3 DIP inset, so
// anything closer than 12 DIPs sits under the thumb while it shows.
const barClearance = 12

// TestWelcomeRecentClearsScrollbar pins the welcome list's geometry: the
// list is wide enough for a whole path, it keeps to the window's margins,
// and no recent row's text runs under the overlay scroll bar.
func TestWelcomeRecentClearsScrollbar(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want float32
	}{
		{1400, 880, recentWidth},
		{520, 880, 520 - 2*welcomeMargin},
	} {
		s, dir := newShell(t)
		paths := make([]string, 0, 10)
		for i := range 10 {
			p := filepath.Join(dir, fmt.Sprintf("doc%02d.md", i))
			if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, p)
		}
		s.Recent = func() []string { return append([]string(nil), paths...) }
		tt := s.renderSize(t, tc.w, tc.h)
		tt.Frame()

		list, ok := tt.Find("Recent Files list")
		if !ok {
			t.Fatalf("%d wide: texts %v: want the recent list", tc.w, tt.Texts())
		}
		// Wide enough for a path, capped at recentWidth, and never wider
		// than the window's own margins leave.
		if list.W != tc.want {
			t.Fatalf("%d wide: the list is %v wide, want %v", tc.w, list.W, tc.want)
		}
		if center := list.X + list.W/2; center < float32(tc.w)/2-1 || center > float32(tc.w)/2+1 {
			t.Fatalf("%d wide: the list centers at %v, want %v", tc.w, center, float32(tc.w)/2)
		}

		for _, p := range paths {
			row, ok := tt.Find("Recent row " + p)
			if !ok {
				t.Fatalf("%d wide: texts %v: want the row of %s", tc.w, tt.Texts(), p)
			}
			// Rows scrolled out of view report a clipped box: the ones the
			// frame shows carry the geometry.
			if row.Y < list.Y || row.Y+row.H > list.Y+list.H {
				continue
			}
			// FillWidth: the row reaches the container's right edge, so the
			// text's clearance is the row's own padding.
			if row.X+row.W > list.X+list.W+0.5 {
				t.Fatalf("%d wide: the row ends at %v, past the list's %v", tc.w, row.X+row.W, list.X+list.W)
			}
			for _, text := range []string{filepath.Base(p), filepath.Dir(p)} {
				r, ok := tt.Find(text)
				if !ok {
					t.Fatalf("%d wide: texts %v: want %q", tc.w, tt.Texts(), text)
				}
				if got := row.X + row.W - (r.X + r.W); got < barClearance {
					t.Fatalf("%d wide: %q ends %v DIPs before the row's edge, want >= %d (the bar's strip)",
						tc.w, text, got, barClearance)
				}
				if got := list.X + list.W - (r.X + r.W); got < barClearance {
					t.Fatalf("%d wide: %q ends %v DIPs before the list's edge, want >= %d",
						tc.w, text, got, barClearance)
				}
			}
		}
	}
}

// TestTabBarClearsScrollbar keeps the strip's text above the horizontal
// bar: the tabs scroll sideways in the title bar, whose bottom 9 DIPs the
// bar overlays, so the labels must end above them (barClearance, the rule
// the rows keep to).
func TestTabBarClearsScrollbar(t *testing.T) {
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
	tt.Frame()

	bar, ok := tt.Find("Tab bar")
	if !ok {
		t.Fatalf("texts %v: want the tab bar", tt.Texts())
	}
	label, ok := tt.Find(filepath.Base(paths[0]))
	if !ok {
		t.Fatalf("texts %v: want the first tab's label", tt.Texts())
	}
	if got := bar.Y + bar.H - (label.Y + label.H); got < barClearance {
		t.Fatalf("the tab label ends %v DIPs above the strip's bottom, want >= %d (the bar's strip)", got, barClearance)
	}
}
