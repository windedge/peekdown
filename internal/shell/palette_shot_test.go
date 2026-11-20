package shell

import (
	"os"
	"testing"

	"github.com/windedge/peekdown/internal/testshot"
)

// paletteShotCropMargin is the slack around the grid's own box so the
// caption and the card strokes stay inside the crop.
const paletteShotCropMargin = 14

// TestPaletteShot writes the accent cards as they now appear on the
// settings' Appearance page: the whole dialog and a magnified grid, then
// the same grid after a meadow pick. Skipped unless PEEKDOWN_SHOT is set.
func TestPaletteShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the palette screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1440, 880)
	s.OpenSettings()
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/palette-grid-settings.png")

	writeGridCrop := func(path string) {
		t.Helper()
		grid, ok := tt.Find("Theme grid")
		if !ok {
			t.Fatalf("no theme grid: %v", tt.Texts())
		}
		crop := testshot.Crop(tt.Image(),
			int(grid.X)-paletteShotCropMargin, int(grid.Y)-paletteShotCropMargin,
			int(grid.W)+2*paletteShotCropMargin, int(grid.H)+2*paletteShotCropMargin)
		testshot.WritePng(t, testshot.Scale3x(crop), path)
	}
	writeGridCrop("C:/tmp/palette-grid.png")

	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card: %v", err)
	}
	tt.Frame()
	writeGridCrop("C:/tmp/palette-meadow.png")
}
