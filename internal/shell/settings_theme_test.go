package shell

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// themeCardRect finds one inline card and fails with the drawn texts when
// the grid forgot it.
func themeCardRect(t *testing.T, tt *ui.Tester, id string) ui.Rect {
	t.Helper()
	r, ok := tt.Find("Theme " + id)
	if !ok {
		t.Fatalf("no card for %q in %q", id, tt.Texts())
	}
	return r
}

// useLocale points the catalog at locale for one test; mygo's own
// language stays the one TestMain fixed.
func useLocale(t *testing.T, locale string) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return locale })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
}

// TestSettingsThemeLabelsFollowLocale checks the Appearance row shows the
// three themes in the active language while storing their fixed values.
func TestSettingsThemeLabelsFollowLocale(t *testing.T) {
	useLocale(t, "zh-Hans")
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	for _, label := range []string{
		locales.AppearanceSystem(), locales.AppearanceLight(), locales.AppearanceDark(),
	} {
		if !tt.HasText(label) {
			t.Fatalf("no %q on the Chinese Appearance page: %q", label, tt.Texts())
		}
	}
	if tt.HasText("Auto") || tt.HasText("System") {
		t.Fatalf("the Appearance page still shows storage values: %q", tt.Texts())
	}

	for _, tc := range []struct{ stored, shown string }{
		{settings.ThemeAuto, locales.AppearanceSystem()},
		{settings.ThemeLight, locales.AppearanceLight()},
		{settings.ThemeDark, locales.AppearanceDark()},
	} {
		info, ok := tt.AccessByRole(ui.RoleRadio, tc.shown)
		if !ok {
			t.Fatalf("no %q radio in %q", tc.shown, tt.Texts())
		}
		r := info.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		if s.Saved.Theme != tc.stored {
			t.Fatalf("after clicking %q, theme = %q, want %q", tc.shown, s.Saved.Theme, tc.stored)
		}
	}
}

// TestSettingsInlineThemeGrid checks the Appearance page shows all ten
// accent cards at once: a caption and a wrapped grid inside its card,
// five in the first row, no picker row or trigger left behind.
func TestSettingsInlineThemeGrid(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	if _, ok := tt.Find(locales.SettingsThemeColor() + " picker"); ok {
		t.Fatalf("the settings still draw a theme picker trigger: %q", tt.Texts())
	}
	if _, ok := tt.Find("Theme Color grid"); ok {
		t.Fatalf("the settings still draw a theme popover: %q", tt.Texts())
	}
	if !s.SettingsOpen {
		t.Fatal("the inline grid changed the dialog's open state")
	}
	if got := s.rows; got != 4 {
		t.Fatalf("Appearance draws %d setting rows, want four (the grid is a block)", got)
	}

	caption, ok := tt.Find(strings.ToUpper(locales.SettingsThemeColor()))
	if !ok {
		t.Fatalf("no Theme Color caption in %q", tt.Texts())
	}
	grid, ok := tt.Find("Theme grid")
	if !ok {
		t.Fatalf("no inline theme grid in %q", tt.Texts())
	}
	theme, ok := tt.Find(locales.SettingsTheme())
	if !ok {
		t.Fatalf("no Theme row in %q", tt.Texts())
	}
	layout, ok := tt.Find(locales.SettingsLayout())
	if !ok {
		t.Fatalf("no Layout row in %q", tt.Texts())
	}
	card, ok := tt.Find("Appearance card")
	if !ok {
		t.Fatalf("no Appearance card in %q", tt.Texts())
	}
	if caption.Y <= theme.Y || grid.Y < caption.Y+caption.H-1 || layout.Y <= grid.Y+grid.H {
		t.Fatalf("the caption %v or grid %v is out of order: Theme %v, Layout %v", caption, grid, theme, layout)
	}
	if grid.X < card.X || grid.X+grid.W > card.X+card.W+1 ||
		grid.Y+grid.H > card.Y+card.H+1 {
		t.Fatalf("the grid %v runs outside the Appearance card %v", grid, card)
	}

	for _, id := range settings.AccentThemeIDs {
		r := themeCardRect(t, tt, id)
		if r.X < grid.X || r.X+r.W > grid.X+grid.W+1 ||
			r.Y < grid.Y || r.Y+r.H > grid.Y+grid.H+1 {
			t.Fatalf("card %q at %v runs outside the grid %v", id, r, grid)
		}
		if r.X+r.W > rowLimit(t, tt, "Appearance card")+0.5 {
			t.Fatalf("card %q ends at %v, past the card's row limit", id, r.X+r.W)
		}
		if !tt.HasText(themeName(accentThemeByID(id))) {
			t.Fatalf("card %q has no translated name", id)
		}
	}

	cols, cardW := themeGridMetrics(settingsThemeGridWidth(1400))
	if cols != 5 || cardW > themeCardWidth+0.5 {
		t.Fatalf("wide grid metrics = %d columns of %v, want 5 of at most %v", cols, cardW, themeCardWidth)
	}
	first, second, fifth, sixth := themeCardRect(t, tt, "paper"), themeCardRect(t, tt, "petal"),
		themeCardRect(t, tt, "dusk"), themeCardRect(t, tt, "tide")
	for i, id := range settings.AccentThemeIDs {
		r := themeCardRect(t, tt, id)
		if i < 5 && math.Abs(float64(r.Y-first.Y)) > 1 {
			t.Fatalf("card %q at %v is not on the first row at %v", id, r, first.Y)
		}
		if i >= 5 && math.Abs(float64(r.Y-sixth.Y)) > 1 {
			t.Fatalf("card %q at %v is not on the second row at %v", id, r, sixth.Y)
		}
		if i > 0 && i%5 != 0 {
			prev := themeCardRect(t, tt, settings.AccentThemeIDs[i-1])
			if r.X <= prev.X+prev.W || math.Abs(float64(r.X-prev.X-(cardW+paletteGap))) > 1 {
				t.Fatalf("card %q at %v overlaps or misspaces %v", id, r, prev)
			}
		}
	}
	if second.X <= first.X || fifth.X <= first.X || sixth.Y <= first.Y+first.H {
		t.Fatalf("the grid is not 5x2: %v, %v, %v, %v", first, second, fifth, sixth)
	}
}

// TestSettingsInlineThemeSelection checks the chosen card's 2 DIP accent
// stroke on the preview, while an unchosen card only gets a hairline.
func TestSettingsInlineThemeSelection(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	chosen, ok := tt.Find("preview " + s.Saved.AccentTheme)
	if !ok {
		t.Fatalf("no chosen preview in %q", tt.Texts())
	}
	plain, ok := tt.Find("preview petal")
	if !ok {
		t.Fatalf("no petal preview in %q", tt.Texts())
	}
	stroke := accentThemeByID(s.Saved.AccentTheme).of(false).Accent
	img := tt.Image()
	c1 := pixelAt(img, chosen.X+1, chosen.Y+chosen.H/2)
	c2 := pixelAt(img, plain.X+1, plain.Y+plain.H/2)
	if !channelClose(c1, stroke, 2) {
		t.Fatalf("the chosen preview's edge is %v, want the accent stroke %v", c1, stroke)
	}
	if c1 == c2 {
		t.Fatalf("the chosen and plain previews' edges render the same: %v", c1)
	}
}

// TestSettingsInlineThemeAppliesAtOnce is the behavioral check: clicking a
// card in the page writes and saves the accent, recolors the window behind
// the dialog and moves the accent stroke to the chosen card, with no Save
// press and no popover.
func TestSettingsInlineThemeAppliesAtOnce(t *testing.T) {
	s, _ := newShell(t)
	saves := 0
	save := s.Save
	s.Save = func(v settings.Settings) error {
		saves++
		return save(v)
	}
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	// The bare gradient shows below the dialog's scrim at the left edge;
	// the accent swap must recolor those pixels without a Save press.
	before := pixelAt(tt.Image(), 2, 840)
	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card: %v", err)
	}
	tt.Frame()
	if got := s.Saved.AccentTheme; got != "meadow" {
		t.Fatalf("AccentTheme = %q, want meadow", got)
	}
	if saves == 0 {
		t.Fatal("the card click never persisted the choice")
	}
	after := pixelAt(tt.Image(), 2, 840)
	if channelSum(before, after) < 10 {
		t.Fatalf("the window's gradient stayed %v after switching to meadow, want a hue change", after)
	}
	if _, ok := tt.Find("Theme grid"); !ok {
		t.Fatalf("the grid vanished after the choice: %q", tt.Texts())
	}
	if _, ok := tt.Find("Theme Color grid"); ok {
		t.Fatalf("a click opened a popover: %q", tt.Texts())
	}

	meadow, ok := tt.Find("preview meadow")
	if !ok {
		t.Fatalf("no meadow preview after the choice: %q", tt.Texts())
	}
	accent := accentThemeByID("meadow").of(false).Accent
	if got := pixelAt(tt.Image(), meadow.X+1, meadow.Y+meadow.H/2); !channelClose(got, accent, 2) {
		t.Fatalf("meadow's preview edge is %v, want the accent stroke %v", got, accent)
	}
	paper, _ := tt.Find("preview paper")
	if got := pixelAt(tt.Image(), paper.X+1, paper.Y+paper.H/2); channelClose(got, accent, 2) {
		t.Fatalf("the old paper preview still draws the accent stroke: %v", got)
	}
}

// TestSettingsThemeGridNarrow checks the 760 DIP window falls back to four
// columns without overflowing the card or overlapping the next row.
func TestSettingsThemeGridNarrow(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 760, 620)
	s.OpenSettings()
	tt.Frame()

	cols, cardW := themeGridMetrics(settingsThemeGridWidth(760))
	if cols != 4 {
		t.Fatalf("narrow grid has %d columns, want 4", cols)
	}
	if cardW < themeGridMinWidth || cardW > themeCardWidth {
		t.Fatalf("narrow card width %v is outside [%v, %v]", cardW, themeGridMinWidth, themeCardWidth)
	}
	grid, ok := tt.Find("Theme grid")
	if !ok {
		t.Fatalf("no inline grid in %q", tt.Texts())
	}
	if _, ok := tt.Find("Appearance card"); !ok {
		t.Fatalf("no Appearance card in %q", tt.Texts())
	}
	limit := rowLimit(t, tt, "Appearance card")
	firstRow := themeCardRect(t, tt, "paper")
	fifth := themeCardRect(t, tt, "dusk")
	if math.Abs(float64(fifth.Y-firstRow.Y)) < 1 {
		t.Fatalf("the fifth card %v did not wrap below the first %v", fifth, firstRow)
	}
	for i, id := range settings.AccentThemeIDs {
		r := themeCardRect(t, tt, id)
		if r.X < grid.X-0.5 || r.X+r.W > grid.X+grid.W+1 || r.X+r.W > limit+0.5 {
			t.Fatalf("card %q at %v runs outside the grid %v or the limit %v", id, r, grid, limit)
		}
		if r.Y < grid.Y || r.Y+r.H > grid.Y+grid.H+1 {
			t.Fatalf("card %q at %v runs outside the grid's height %v", id, r, grid)
		}
		if i > 0 && i%cols != 0 {
			prev := themeCardRect(t, tt, settings.AccentThemeIDs[i-1])
			if r.X <= prev.X+prev.W || math.Abs(float64(r.X-prev.X-(cardW+paletteGap))) > 1 {
				t.Fatalf("card %q at %v overlaps or misspaces %v", id, r, prev)
			}
		}
	}
}

// TestSettingsNewLayoutShots writes the three review images: the inline
// grid, the merged Reading page, and the narrow window's columns.
func TestSettingsNewLayoutShots(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the new settings screenshots")
	}
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-inline-theme.png")

	if err := tt.Click("Settings nav Reading"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	writePng(t, tt.Image(), "C:/tmp/settings-reading.png")

	narrow, _ := newShell(t)
	ntt := narrow.renderSize(t, 760, 620)
	narrow.OpenSettings()
	ntt.Frame()
	writePng(t, ntt.Image(), "C:/tmp/settings-narrow-theme.png")
}
