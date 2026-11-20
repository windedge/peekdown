package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// TestZoomClamps walks the zoom's limits: the steps stop at 300 percent
// above and 50 below, and the reset returns 100.
func TestZoomClamps(t *testing.T) {
	s, _ := newShell(t)
	for range 100 {
		s.ZoomIn()
	}
	if s.Zoom != zoomMax {
		t.Fatalf("zoom in = %d, want %d", s.Zoom, zoomMax)
	}
	for range 100 {
		s.ZoomOut()
	}
	if s.Zoom != zoomMin {
		t.Fatalf("zoom out = %d, want %d", s.Zoom, zoomMin)
	}
	s.ZoomReset()
	if s.Zoom != zoomReset || s.ZoomLabel() != "100%" {
		t.Fatalf("zoom reset = %d %q, want 100 percent", s.Zoom, s.ZoomLabel())
	}
}

// zoomFrame records what the content saw during the last frame: the
// window's own zoom and size. A Context only answers inside its own frame,
// so the hook reads them out then.
type zoomFrame struct {
	seen bool
	zoom float32
	w, h float32
}

// zoomShell opens a one-paragraph document and renders it, measuring the
// frame for the window assertions.
func zoomShell(t *testing.T) (*Shell, *ui.Tester, *zoomFrame, string) {
	t.Helper()
	s, dir := newShell(t)
	const body = "One short line of body text."
	path := filepath.Join(dir, "zoom.md")
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	frame := &zoomFrame{}
	s.ContentHook = func(c *ui.Context) {
		frame.seen = true
		frame.zoom = c.Zoom()
		frame.w, frame.h = c.Size()
	}
	tt := s.renderSize(t, 900, 640)
	tt.Frame()
	if !frame.seen {
		t.Fatal("the frame never reached the content")
	}
	return s, tt, frame, body
}

func nearDIP(a, b float32) bool { return a-b < 0.5 && b-a < 0.5 }

// TestZoomScalesTextNotWindow walks Ctrl+= and Ctrl++: the body draws
// bigger, the window's own zoom and the chrome's boxes stay put, and the
// status bar reads the zoom.
func TestZoomScalesTextNotWindow(t *testing.T) {
	const w, h = 900, 640
	s, tt, frame, body := zoomShell(t)

	tab := s.ActiveTab()
	if !tt.HasText("100%") {
		t.Fatalf("texts %v: the status bar lacks the zoom readout", tt.Texts())
	}
	if tab.State.FontSize != 16 {
		t.Fatalf("body at 100 percent = %v, want the 16px setting", tab.State.FontSize)
	}
	textBefore, ok := paragraphRect(tt, body)
	if !ok {
		t.Fatalf("texts %v: want the body line", tt.Texts())
	}
	chromeBefore, ok := tt.Find("100%")
	if !ok {
		t.Fatalf("texts %v: want the status bar's zoom readout", tt.Texts())
	}

	m := BuildMenu(s)
	chooseMenuItem(t, m, "Zoom In", "CmdOrCtrl+=")
	tt.Frame()
	if s.Zoom != 105 || !tt.HasText("105%") {
		t.Fatalf("after Ctrl+= zoom %d, texts %v: want 105 percent", s.Zoom, tt.Texts())
	}
	if tab.State.FontSize <= 16 {
		t.Fatalf("body after Ctrl+= = %v, want it bigger than 16", tab.State.FontSize)
	}

	for range 4 {
		tt.TypeKey(ui.Ctrl|ui.Shift, ui.KeyEqual, "")
	}
	tt.Frame()
	if s.Zoom != 125 || !tt.HasText("125%") {
		t.Fatalf("after Ctrl++ zoom %d, texts %v: want 125 percent", s.Zoom, tt.Texts())
	}
	// 125 percent of the default 16px body and 13px mono: the headings,
	// the tables and the code ride these through the view's own ratios.
	if tab.State.FontSize != 20 || tab.State.MonoFontSize != 16.25 {
		t.Fatalf("sizes at 125 percent: body %v mono %v, want 20 and 16.25",
			tab.State.FontSize, tab.State.MonoFontSize)
	}
	textAfter, ok := paragraphRect(tt, body)
	if !ok {
		t.Fatalf("texts %v: want the body line after the zoom", tt.Texts())
	}
	if textAfter.H <= textBefore.H {
		t.Fatalf("body line %v tall at 125 percent, %v at 100: the text did not grow", textAfter.H, textBefore.H)
	}

	// The window itself did not zoom: the frame's own zoom stays 1, its
	// size stays the window's, and the status bar's readout draws in the
	// same box.
	if frame.zoom != 1 {
		t.Fatalf("window zoom = %v during the zoomed frame, want 1", frame.zoom)
	}
	if frame.w != w || frame.h != h {
		t.Fatalf("frame size = %vx%v, want the window's %dx%d", frame.w, frame.h, w, h)
	}
	chromeAfter, ok := tt.Find("125%")
	// The readout is end-aligned: "125%" and "100%" have different glyph
	// advances, so only their anchor and line box must stay put.
	if !ok || !nearDIP(chromeAfter.Y, chromeBefore.Y) || !nearDIP(chromeAfter.H, chromeBefore.H) ||
		!nearDIP(chromeAfter.X+chromeAfter.W, chromeBefore.X+chromeBefore.W) {
		t.Fatalf("chrome box %v after the zoom, %v before: the window scaled", chromeAfter, chromeBefore)
	}

	chooseMenuItem(t, m, "Reset Zoom", "CmdOrCtrl+0")
	tt.Frame()
	if s.Zoom != 100 || !tt.HasText("100%") {
		t.Fatalf("after Ctrl+0 zoom %d, texts %v: want 100 percent", s.Zoom, tt.Texts())
	}
	if tab.State.FontSize != 16 || tab.State.MonoFontSize != 13 {
		t.Fatalf("sizes after the reset: body %v mono %v, want the settings' 16 and 13",
			tab.State.FontSize, tab.State.MonoFontSize)
	}
	textReset, ok := paragraphRect(tt, body)
	if !ok || textReset.H != textBefore.H {
		t.Fatalf("body line %v after the reset, %v before: it did not return", textReset, textBefore)
	}
}

// TestZoomNotPersisted checks the zoom stays a session state: no save runs,
// and the settings still name the base sizes.
func TestZoomNotPersisted(t *testing.T) {
	s, tt, _, _ := zoomShell(t)
	saves := 0
	s.Save = func(v settings.Settings) error {
		saves++
		return nil
	}
	m := BuildMenu(s)
	tt.TypeKey(ui.Ctrl|ui.Shift, ui.KeyEqual, "")
	chooseMenuItem(t, m, "Zoom Out", "CmdOrCtrl+-")
	chooseMenuItem(t, m, "Reset Zoom", "CmdOrCtrl+0")
	tt.Frame()
	if saves != 0 {
		t.Fatalf("%d saves after the zoom, want the zoom out of settings.json", saves)
	}
	if s.Saved.FontSize != 16 {
		t.Fatalf("settings font size %v after the zoom, want 16", s.Saved.FontSize)
	}
}
