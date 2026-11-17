package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// statusbarShotPath is where the status bar's screenshot lands, skipped
// with the env unset: the regular runs stay headless-clean.
var statusbarShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/statusbar-fps-left.png"
}()

// TestStatusBarShot captures the bar for a human to look at: with the FPS
// readout on, it should open the strip at the left edge with the zoom
// readout at the right. Skipped unless PEEKDOWN_SHOT is set.
func TestStatusBarShot(t *testing.T) {
	if statusbarShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the status bar screenshot")
	}
	s, dir := newShell(t)
	s.Saved.ShowFPS = true

	var b strings.Builder
	for range 30 {
		b.WriteString("A paragraph long enough to fill the reading column naturally.\n\n")
	}
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# Status bar\n\n"+b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.Frame()
	// Let the readout measure a window before the shot: an empty box would
	// show the room but not the text.
	tt.Advance(2 * time.Second)
	writePng(t, tt.Image(), statusbarShotPath)
}
