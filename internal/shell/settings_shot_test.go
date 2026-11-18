package shell

import (
	"os"
	"testing"
)

// TestSettingsThemeSegmentedShot captures the theme segmented with Auto
// chosen and then Dark, so a human can check the choices read "Auto",
// "Light", "Dark" and the chosen segment lifts. Skipped unless
// PEEKDOWN_SHOT is set, so the regular runs stay headless-clean.
func TestSettingsThemeSegmentedShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the theme segmented screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/theme-segmented-auto.png")

	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("the Dark segment is not clickable: %v", err)
	}
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/theme-segmented-dark.png")
}

// TestSettingsCardsShot captures the dialog's card layers and backdrop
// together, for a human check against the document card's language.
func TestSettingsCardsShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the settings card screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-cards.png")
}
