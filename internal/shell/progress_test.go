package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/testshot"
)

// progressShotPath is where the progress bar's screenshot lands, ""
// with the env unset: the shot is extra evidence, the assertions run
// headless either way.
var progressShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/progress-top.png"
}()

// openLongDoc opens a document long enough to scroll and settles its
// first frame. A negative fraction leaves it at the top.
func openLongDoc(t *testing.T, s *Shell, tt *ui.Tester, fraction float32) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "long.md")
	var b []byte
	b = append(b, "# Top\n\n"...)
	for i := range 60 {
		b = append(b, "Paragraph of the filler body, number "...)
		b = append(b, []byte{byte('0' + i/10), byte('0' + i%10)}...)
		b = append(b, ". It is long enough to scroll.\n\n"...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenPaths([]string{path})
	tt.Frame()
	if fraction >= 0 {
		tab := s.ActiveTab()
		tab.State.Scroll.Y = fraction * tab.State.Scroll.MaxY
	}
	tt.Frame()
	tt.Frame()
}

// areaProgressGeometry pins the bar to the content area's top edge, the
// window-level reading under the title bar: it starts at the area's left
// edge, right under the title bar, and runs across every pane.
func areaProgressGeometry(t *testing.T, tt *ui.Tester) (area, bar ui.Rect) {
	t.Helper()
	area, ok := tt.Find("Content area")
	if !ok {
		t.Fatalf("no content area in %q", tt.Texts())
	}
	bar, ok = tt.Find("Reading progress")
	if !ok {
		t.Fatalf("no reading progress bar in %q", tt.Texts())
	}
	if bar.Y < area.Y || bar.Y > area.Y+2 {
		t.Fatalf("bar at %v is not on the content area's top edge %v", bar, area)
	}
	if bar.X < area.X || bar.X > area.X+2 {
		t.Fatalf("bar at %v is not at the content area's left edge %v", bar, area)
	}
	return area, bar
}

// TestProgressBarSpansContentArea checks the fill's width and place
// against the content area: it grows with the scroll fraction, runs
// across the panes the splits carved out (so it is wider than the
// document card), and draws nothing past the area's right edge.
func TestProgressBarSpansContentArea(t *testing.T) {
	s, _ := newShell(t)
	s.Sidebar.Visible = true
	s.SetSidebarWidth(200)
	s.Outline.Visible = true
	tt := s.renderSize(t, 1400, 700)
	openLongDoc(t, s, tt, 0.5)

	tab := s.ActiveTab()
	if p := tab.State.Progress; p < 0.4 || p > 0.6 {
		t.Fatalf("progress at mid scroll = %v, want about 0.5", p)
	}
	area, mid := areaProgressGeometry(t, tt)
	card, ok := tt.Find("Document card")
	if !ok {
		t.Fatalf("no document card in %q", tt.Texts())
	}
	// The panes narrow the card; the bar belongs to the area, not to it.
	if area.W <= card.W+10 {
		t.Fatalf("content area %v is not wider than the card %v: the panes did not build", area, card)
	}
	if want := area.W * tab.State.Progress; mid.W < want-4 || mid.W > want+4 {
		t.Fatalf("bar width %v at progress %v, want about %v", mid.W, tab.State.Progress, want)
	}

	tab.State.List.ScrollToEnd()
	tt.Frame()
	tt.Frame()
	area, end := areaProgressGeometry(t, tt)
	if end.W <= mid.W {
		t.Fatalf("bar at the end is %v wide, not wider than mid scroll %v", end.W, mid.W)
	}
	if want := area.W; end.W < want-2 || end.W > want+2 {
		t.Fatalf("bar at the end is %v wide, want the content area's width %v", end.W, want)
	}

	// The accent must run across the file pane's columns on the bar's
	// rows, and stop at the area's right edge.
	img := tt.Image()
	accent := windowLight().Accent
	for x := int(area.X) + 1; x < int(card.X); x++ {
		if pixelAt(img, float32(x), area.Y+1) != accent {
			t.Fatalf("no accent at %d,%d: the bar stops before the file pane", x, int(area.Y)+1)
		}
	}
	for x := int(area.X+area.W) + 1; x < 1400; x++ {
		if pixelAt(img, float32(x), area.Y+1) == accent {
			t.Fatalf("accent pixel at %d,%d past the content area's right edge", x, int(area.Y)+1)
		}
	}
}

// TestProgressTopShot captures a mid-scroll frame where the fill is
// clearly shorter than the area, so its width and place can be inspected.
func TestProgressTopShot(t *testing.T) {
	if progressShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the progress bar screenshot")
	}
	s, _ := newShell(t)
	s.Sidebar.Visible = true
	s.SetSidebarWidth(200)
	s.Outline.Visible = true
	tt := s.renderSize(t, 1400, 700)
	openLongDoc(t, s, tt, 0.5)
	testshot.WritePng(t, tt.Image(), progressShotPath)
}

// TestProgressBarStaysHiddenOnWelcome checks the bar draws nothing on
// the welcome page: no tab, no area and no committed fill.
func TestProgressBarStaysHiddenOnWelcome(t *testing.T) {
	s, _ := newShell(t)
	tt := s.render(t)
	tt.Frame()
	if _, ok := tt.Find("Reading progress"); ok {
		t.Fatalf("the welcome page drew a progress bar: %q", tt.Texts())
	}
	img := tt.Image()
	accent := windowLight().Accent
	for y := 0; y < 4; y++ {
		for x := 0; x < 900; x++ {
			if pixelAt(img, float32(x), float32(y)) == accent {
				t.Fatalf("accent pixel at %d,%d on the welcome page: the bar drew without a document", x, y)
			}
		}
	}
}
