package shell

import (
	"path/filepath"
	"testing"

	"github.com/windedge/peekdown/internal/settings"
)

// TestAccentCardAppliesAtOncePersists checks that picking an accent on the
// settings page saves it without a Save press and survives a round trip.
func TestAccentCardAppliesAtOncePersists(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	if _, ok := tt.Find("Theme grid"); !ok {
		t.Fatalf("no accent grid on the Appearance page: %v", tt.Texts())
	}

	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card: %v", err)
	}
	tt.Frame()
	if s.Saved.AccentTheme != "meadow" {
		t.Fatalf("AccentTheme = %q, want meadow", s.Saved.AccentTheme)
	}
	if _, ok := tt.Find("Theme grid"); !ok {
		t.Fatalf("the grid vanished after the choice: %v", tt.Texts())
	}

	path := filepath.Join(t.TempDir(), settings.FileName)
	saved := s.Saved
	if err := settings.Save(path, saved); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccentTheme != "meadow" {
		t.Fatalf("round trip: accent theme = %q, want meadow", got.AccentTheme)
	}
}

// TestThemePreviewPaletteFollowsTheAccent pins the dot order and text
// tiers paintThemePreview draws inside each accent card.
func TestThemePreviewPaletteFollowsTheAccent(t *testing.T) {
	k := accentThemeByID("meadow").of(false)
	light := previewText(false)
	for i := 0; i < paletteDots; i++ {
		expected := k.Syntax[5]
		if i == 0 {
			expected = light
		} else if i <= len(k.Syntax) {
			expected = k.Syntax[i-1]
		}
		if got := previewDotColor(k, light, i); got != expected {
			t.Errorf("dot %d is %v, want %v", i, got, expected)
		}
	}
	if previewText(true) == light {
		t.Error("dark and light previews use the same text color")
	}
}
