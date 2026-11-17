package shell

import (
	"os"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/testshot"
)

// paletteShotCropMargin is the slack around the popover's own box in the
// crop, so its border and the shadow it floats on stay inside the frame.
const paletteShotCropMargin = 22

// openPaletteForShot opens the theme picker from the bar and leaves its
// frame drawn.
func openPaletteForShot(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("Accent theme"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
}

// TestPaletteShot writes the theme picker's screenshots for a human to
// look at: the popover over the light window, a meadow click tinting the
// window's gradient, the dark segment darkening the whole window and the
// popover, and a meadow click in the dark window. Skipped unless
// PEEKDOWN_SHOT is set, so the regular runs stay headless-clean.
func TestPaletteShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the palette screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1440, 880)
	// newShell stubs the host's theme setter, so wire it to the tester's
	// dark flag: the real host repaints the window, while the headless
	// window needs the call spelled out.
	s.SetTheme = func(src mygo.ThemeSource) { tt.SetDark(src == mygo.ThemeDark) }
	openPaletteForShot(t, tt)
	testshot.WritePng(t, tt.Image(), "C:/tmp/palette-open.png")

	// The popover's own box, not the whole window: the crop enlarged by 3
	// so the cards' strokes and the dots are countable by eye.
	panel, ok := tt.Find("Theme picker")
	if !ok {
		t.Fatalf("no theme picker panel: %v", tt.Texts())
	}
	crop := testshot.Crop(tt.Image(),
		int(panel.X)-paletteShotCropMargin, int(panel.Y)-paletteShotCropMargin,
		int(panel.W)+2*paletteShotCropMargin, int(panel.H)+2*paletteShotCropMargin)
	testshot.WritePng(t, testshot.Scale3x(crop), "C:/tmp/palette-popover.png")

	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card: %v", err)
	}
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/palette-meadow.png")

	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("the Dark segment is not clickable: %v", err)
	}
	tt.Frame()
	// Back to the default accent, so this shot shows the dark window and
	// the next one its meadow repaint as two distinct states.
	if err := tt.Click("Theme petal"); err != nil {
		t.Fatalf("petal card in the dark window: %v", err)
	}
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/palette-dark.png")

	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card in the dark window: %v", err)
	}
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/palette-meadow-dark.png")
}
