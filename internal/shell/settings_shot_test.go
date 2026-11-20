package shell

import (
	"os"
	"strings"
	"testing"
)

// TestSettingsThemeSegmentedShot captures the theme segmented with Auto
// chosen and then Dark, so a human can check the choices read "System",
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

// TestSettingsChineseShot captures the Appearance page in zh-Hans, so the
// translated theme, layout, language and accent names can be checked at
// once. Skipped unless PEEKDOWN_SHOT is set.
func TestSettingsChineseShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the Chinese settings screenshot")
	}
	useLocale(t, "zh-Hans")
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-zh.png")
}

// TestSettingsCardsShot captures the two-column dialog's card layers and
// backdrop together, for a human check against the document card's
// language.
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

// TestSettingsTwoColumnShot captures the rail with Appearance chosen and
// the detail column after a rail click to Reading. Both pages should show
// one header and one card, with Close still at the panel's foot.
func TestSettingsTwoColumnShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the two-column screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-two-column.png")

	if err := tt.Click("Settings nav Reading"); err != nil {
		t.Fatalf("Reading rail item: %v", err)
	}
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-two-column-reading.png")
}

// TestSettingsFixedSectionsShot captures the three pages at one fixed
// height, plus a narrow window, for a visual check that only the page's
// content changes when the reader switches sections.
func TestSettingsFixedSectionsShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the fixed-height settings screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	for _, page := range []string{"Appearance", "Reading", "About"} {
		if page != "Appearance" {
			if err := tt.Click("Settings nav " + page); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		writePng(t, tt.Image(), "C:/tmp/settings-fixed-"+strings.ToLower(page)+".png")
	}

	narrow, _ := newShell(t)
	ntt := narrow.renderSize(t, 760, 620)
	openSettingsSection(t, ntt, narrow, "About")
	writePng(t, ntt.Image(), "C:/tmp/settings-fixed-narrow.png")
}

// TestSettingsThreeNavShot captures the three-item rail with the merged
// Reading page, where the explorer rows follow the reading rows.
func TestSettingsThreeNavShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the three-item navigation screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "Reading")
	writePng(t, tt.Image(), "C:/tmp/settings-three-nav.png")
}

// TestSettingsTwoColumnNarrowShot captures the dialog in a 760 DIP
// window, where the panel is clamped, to check that neither column runs
// past the panel and Close stays visible.
func TestSettingsTwoColumnNarrowShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the narrow screenshot")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 760, 620)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-two-column-narrow.png")
}
