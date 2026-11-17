package shell

import (
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// windowLight and windowDark are the widget themes the shell draws
// headlessly with: the frame's native theme with the default petal
// tokens pressed on. Pixel assertions compare against these, not the
// raw Light/DarkTheme, once the token layer drives the window.
func windowLight() *ui.Theme {
	return applyAccentTheme(ui.LightTheme(), accentThemeByID(settings.AccentThemeDefault).of(false))
}

func windowDark() *ui.Theme {
	return applyAccentTheme(ui.DarkTheme(), accentThemeByID(settings.AccentThemeDefault).of(true))
}

// The pixel difference of two colors' channels, for assertions that only
// ask for a visible change of hue.
func channelSum(a, b ui.Color) int {
	return absDiff(a.R, b.R) + absDiff(a.G, b.G) + absDiff(a.B, b.B)
}

func absDiff(a, b uint8) int {
	x, y := int(a), int(b)
	if x > y {
		return x - y
	}
	return y - x
}

// channelClose reports whether two colors' channels sit within tol,
// alpha aside: the shell's translucent fills land on the frame already
// composited (A=255) with the token's channels kept to a rounding step.
func channelClose(a, b ui.Color, tol int) bool {
	return absDiff(a.R, b.R) <= tol && absDiff(a.G, b.G) <= tol && absDiff(a.B, b.B) <= tol
}

// over composites fg over bg the way the rasterizer lays a translucent
// fill onto the opaque frame beneath it.
func over(fg, bg ui.Color) ui.Color {
	a := float32(fg.A) / 255
	mix := func(f, b uint8) uint8 {
		v := float32(f)*a + float32(b)*(1-a)
		return uint8(math.Round(float64(v)))
	}
	return ui.Color{R: mix(fg.R, bg.R), G: mix(fg.G, bg.G), B: mix(fg.B, bg.B), A: 255}
}

// TestAccentThemeTables lists every theme the settings name, with
// non-zero key colors in both appearances, and a hue that flips between
// the light and the dark set.
func TestAccentThemeTables(t *testing.T) {
	for _, id := range settings.AccentThemeIDs {
		th := accentThemeByID(id)
		if th.ID != id {
			t.Fatalf("accentThemeByID(%q) = %q", id, th.ID)
		}
		for _, dark := range []bool{false, true} {
			k := th.of(dark)
			if k.BgTop == (ui.Color{}) || k.Accent == (ui.Color{}) {
				t.Fatalf("%s dark=%v: BgTop or Accent zero", id, dark)
			}
			for i, c := range k.Syntax {
				if c == (ui.Color{}) {
					t.Fatalf("%s dark=%v: syntax token %d zero", id, dark, i)
				}
			}
		}
		if channelSum(th.Light.Accent, th.Dark.Accent) < 40 {
			t.Fatalf("%s: light accent %v and dark accent %v barely differ", id, th.Light.Accent, th.Dark.Accent)
		}
		if channelSum(th.Light.BgTop, th.Dark.BgTop) < 40 {
			t.Fatalf("%s: light BgTop %v and dark BgTop %v barely differ", id, th.Light.BgTop, th.Dark.BgTop)
		}
	}
}

// TestAccentThemePairsCoverSettings: the color table carries a row for
// every id the settings persist, in the same order.
func TestAccentThemePairsCoverSettings(t *testing.T) {
	if len(accentThemes) != len(settings.AccentThemeIDs) {
		t.Fatalf("accentThemes = %d rows, settings list %d ids", len(accentThemes), len(settings.AccentThemeIDs))
	}
	for i, id := range settings.AccentThemeIDs {
		if accentThemes[i].ID != id {
			t.Fatalf("accentThemes[%d] = %q, settings list %q", i, accentThemes[i].ID, id)
		}
	}
}

// TestAccentThemeByIDUnknownFallsBackToPetal: an id the table does not
// carry draws the default.
func TestAccentThemeByIDUnknownFallsBackToPetal(t *testing.T) {
	petal := accentThemeByID(settings.AccentThemeDefault)
	for _, id := range []string{"", "nope", "PETAL", "petal "} {
		if got := accentThemeByID(id); got.ID != petal.ID {
			t.Fatalf("accentThemeByID(%q) = %q, want the petal fallback", id, got.ID)
		}
	}
	if petal.ID != settings.AccentThemeDefault {
		t.Fatalf("fallback = %q, want the settings default %q", petal.ID, settings.AccentThemeDefault)
	}
}

// TestApplyAccentThemePressesTokensOnTheme: the widget theme takes the
// accent theme's accent, danger and attention, with the selection and
// focus derived from the accent; the light/dark dimension follows the
// theme it presses onto.
func TestApplyAccentThemePressesTokensOnTheme(t *testing.T) {
	meadow := accentThemeByID("meadow")
	for _, dark := range []bool{false, true} {
		base := ui.LightTheme()
		if dark {
			base = ui.DarkTheme()
		}
		k := meadow.of(dark)
		got := applyAccentTheme(base, k)
		if got.Dark != dark {
			t.Fatalf("dark = %v, want %v", got.Dark, dark)
		}
		if got.Accent != k.Accent {
			t.Fatalf("accent %v, want %v", got.Accent, k.Accent)
		}
		if got.Danger != k.Danger || got.Warning != k.Attention {
			t.Fatalf("danger/warning %v/%v, want %v/%v", got.Danger, got.Warning, k.Danger, k.Attention)
		}
		if got.Selection.A == 0 || got.Focus.A == 0 {
			t.Fatalf("selection %v or focus %v unset", got.Selection, got.Focus)
		}
		if got.AccentText.A == 0 {
			t.Fatalf("accent text unset")
		}
	}
}

// TestAccentSettingsRoundTrip: Save writes the field, Load reads it
// back; a value the build does not know normalizes to the default.
func TestAccentSettingsRoundTrip(t *testing.T) {
	path := t.TempDir() + "/settings.json"

	saved := settings.Default()
	saved.AccentTheme = "meadow"
	if err := settings.Save(path, saved); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccentTheme != "meadow" {
		t.Fatalf("accent theme = %q, want meadow", got.AccentTheme)
	}

	broken := []byte(`{"version":1,"accent_theme":"soup"}`)
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccentTheme != settings.AccentThemeDefault {
		t.Fatalf("unknown accent theme = %q, want %q", got.AccentTheme, settings.AccentThemeDefault)
	}
}

// TestAccentThemeDrivesFramePixels renders the shell over an opened
// document in two accent themes and reads the pure gradient at the left
// edge: the rasterizer lays the bands' linear mixes down as runs whose
// step boundaries keep the stop's own color for a stretch, so 240 rides
// BgMid's run and 420 BgLow's (the second band ends there), within a
// rounding step of the table -- and the two themes' stops must not be
// interchangeable.
func TestAccentThemeDrivesFramePixels(t *testing.T) {
	probe := func(t *testing.T, id string) (mid, bottom ui.Color) {
		t.Helper()
		s, dir := newShell(t)
		s.Saved.AccentTheme = id
		s.Sidebar.Visible = false
		s.OpenPaths([]string{dir + "/a.md"})
		tt := s.renderSize(t, 800, 600)
		tt.Frame()
		img := tt.Image()
		// The window's gradient paints behind the bars, so the probes
		// read the left edge, clear of every surface the shell draws;
		// 575 would sit in the status bar's translucent panel.
		return pixelAt(img, 2, 240), pixelAt(img, 2, 420)
	}

	pm, pb := probe(t, settings.AccentThemeDefault)
	mm, mb := probe(t, "meadow")

	// A theme swap must move the gradient by a visible step: 100 times
	// the per-channel sum, so the assertions compare stops, not rounding
	// noise at the probe points.
	if channelSum(pm, mm)*100 < 300 {
		t.Fatalf("mid band: petal %v vs meadow %v, too close", pm, mm)
	}
	if channelSum(pb, mb)*100 < 300 {
		t.Fatalf("bottom band: petal %v vs meadow %v, too close", pb, mb)
	}
	// Each probe reads its band's stop from the token table.
	for _, c := range []struct {
		name string
		got  ui.Color
		stop ui.Color
	}{
		{"petal mid", pm, accentThemeByID(settings.AccentThemeDefault).Light.BgMid},
		{"meadow mid", mm, accentThemeByID("meadow").Light.BgMid},
		{"petal bottom", pb, accentThemeByID(settings.AccentThemeDefault).Light.BgLow},
		{"meadow bottom", mb, accentThemeByID("meadow").Light.BgLow},
	} {
		if !channelClose(c.got, c.stop, 1) {
			t.Fatalf("%s probe %v, want the table's %v within a rounding step", c.name, c.got, c.stop)
		}
	}
}
