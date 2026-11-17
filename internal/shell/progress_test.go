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
	return "C:/tmp/progress-body.png"
}()

// openLongDocScrollTo opens a document long enough to scroll, scrolls it
// fully and returns the tester with the settled frame: the path both
// progress tests take.
func openLongDocScrollTo(t *testing.T, s *Shell, tt *ui.Tester) *ui.Tester {
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
	tab := s.ActiveTab()
	tab.State.List.ScrollToEnd()
	tt.Frame()
	tt.Frame()
	return tt
}

// TestProgressBarStopsAtTheBodyEdge checks the fill's right edge against
// the body's right edge with the sidebar visible: at 100% the bar must
// stop at the body, not run under the sidebar or past the window edge.
func TestProgressBarStopsAtTheBodyEdge(t *testing.T) {
	s, _ := newShell(t)
	s.Sidebar.Visible = true
	s.SetSidebarWidth(200)
	tt := s.renderSize(t, 900, 600)
	tt = openLongDocScrollTo(t, s, tt)

	if tab := s.ActiveTab(); tab.State.Progress < 0.999 {
		t.Fatalf("progress after ScrollToEnd = %v, want the end", tab.State.Progress)
	}
	body, ok := tt.Find("Body")
	if !ok {
		t.Fatal("the body box has no committed bounds")
	}
	if body.W >= 900 {
		t.Fatalf("the body box is %v wide, the sidebar must have narrowed it", body.W)
	}

	// The accent paints the fill; count its pixels past the body's edge.
	img := tt.Image()
	theme := ui.LightTheme()
	accent := 0
	bx := int(body.X + body.W)
	for y := 0; y < 4; y++ {
		for x := bx; x < 900; x++ {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) == theme.Accent.R && uint8(g>>8) == theme.Accent.G && uint8(bl>>8) == theme.Accent.B && uint8(a>>8) == theme.Accent.A {
				accent++
			}
		}
	}
	if accent != 0 {
		t.Fatalf("%d accent pixels right of the body's edge x=%d in the bar's first rows: the fill overruns the body", accent, bx)
	}
	if progressShotPath != "" {
		testshot.WritePng(t, img, progressShotPath)
	}
}

// TestProgressBarStaysHiddenOnWelcome checks the bar draws nothing on
// the welcome page: no tab, no committed body, no fill.
func TestProgressBarStaysHiddenOnWelcome(t *testing.T) {
	s, _ := newShell(t)
	tt := s.render(t)
	if w := s.bodyWidth(nil); w != 0 {
		t.Fatalf("the welcome page's body width is %v, want 0", w)
	}
	tt.Frame()
	theme := ui.LightTheme()
	img := tt.Image()
	for y := 0; y < 4; y++ {
		for x := 0; x < 900; x++ {
			r, g, bl, a := img.RGBAAt(x, y).RGBA()
			if uint8(r>>8) == theme.Accent.R && uint8(g>>8) == theme.Accent.G && uint8(bl>>8) == theme.Accent.B && uint8(a>>8) == theme.Accent.A {
				t.Fatalf("accent pixel at %d,%d on the welcome page: the bar drew without a document", x, y)
			}
		}
	}
}
