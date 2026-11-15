package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statusbarShotPath is where the status bar's screenshot lands, skipped
// with the env unset: the regular runs stay headless-clean.
var statusbarShotPath = func() string {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		return ""
	}
	return "C:/tmp/statusbar-clean.png"
}()

// TestStatusBarShot captures the bar for a human to look at: after the
// icon buttons went, the strip should hold only the zoom readout, right
// aligned. Skipped unless PEEKDOWN_SHOT is set.
func TestStatusBarShot(t *testing.T) {
	if statusbarShotPath == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the status bar screenshot")
	}
	s, dir := newShell(t)
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
	writePng(t, tt.Image(), statusbarShotPath)
}
