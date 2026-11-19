package shell

import (
	"os"
	"testing"

	"github.com/windedge/peekdown/internal/testshot"
)

// TestFontPickerScreenshot writes the popover's screenshots for a human
// to look at; skipped unless PEEKDOWN_SHOT is set, so the regular test
// runs stay headless-clean. The hover pass moves the pointer over the
// list, so the overlay scroll bar shows and the shot proves the check
// clear of it.
func TestFontPickerScreenshot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the popover screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	if err := tt.Click("Settings nav Reading"); err != nil {
		t.Fatalf("Reading rail item: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Choose Content Font"); err != nil {
		t.Fatalf("content font trigger: %v", err)
	}
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/fontpicker-popover.png")

	// Hover the list: the bar shows while the pointer is over the
	// container, and at the top its thumb stands beside the checked
	// row's check, the overlap the user saw.
	panel, _ := tt.Find("Font picker")
	tt.Move(panel.X+panel.W/2, panel.Y+panel.H/2)
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/fontpicker-popover-scrollbar.png")

	tt.Type("ari")
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/fontpicker-popover-filtered.png")
}
