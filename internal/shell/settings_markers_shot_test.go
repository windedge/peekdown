package shell

import (
	"os"
	"testing"
)

// TestSettingsMarkersRowShot captures the Files section with the Project
// Root Markers row hidden (current directory root) and shown (project
// root), for a human to compare. Skipped unless PEEKDOWN_SHOT is set, so
// the regular runs stay headless-clean.
func TestSettingsMarkersRowShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the markers row screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/markers-hidden.png")

	if err := tt.Click("Project Directory"); err != nil {
		t.Fatalf("no Project Directory choice in %q", tt.Texts())
	}
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/markers-shown.png")
}
