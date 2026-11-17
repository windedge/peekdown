package shell

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// openPaletteFor opens the theme popover from the bar's palette button
// and leaves its first frame drawn.
func openPaletteFor(t *testing.T, s *Shell, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("Accent theme"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
	if !s.paletteOpen {
		t.Fatalf("the palette trigger left the popover closed")
	}
	if _, ok := tt.Find("Theme petal"); !ok {
		t.Fatalf("no theme cards in the popover: %v", tt.Texts())
	}
}

// TestPaletteListsAllThemes checks the popover shows one card per accent
// theme, under the appearance segmented.
func TestPaletteListsAllThemes(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	for _, id := range settings.AccentThemeIDs {
		if _, ok := tt.Find("Theme " + id); !ok {
			t.Fatalf("no card for %q: %v", id, tt.Texts())
		}
	}
	if !tt.HasText("System") || !tt.HasText("Dark") {
		t.Fatalf("no appearance segmented in the popover: %v", tt.Texts())
	}
}

// TestPaletteAppearanceSitsOverTheThemes checks the sections stack in the
// order the user reads them: the appearance heading, its segmented and its
// note, then the accent grid below.
func TestPaletteAppearanceSitsOverTheThemes(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	heading, ok := tt.Find(strings.ToUpper(locales.PaletteAppearance()))
	if !ok {
		t.Fatalf("no appearance heading: %v", tt.Texts())
	}
	trackInfo, ok := tt.AccessByRole(ui.RoleRadioGroup, locales.PaletteAppearance())
	if !ok {
		t.Fatalf("no appearance track: %v", tt.Texts())
	}
	track := trackInfo.Bounds
	note, ok := tt.Find(locales.AppearanceNote())
	if !ok {
		t.Fatalf("no appearance note: %v", tt.Texts())
	}
	card, ok := tt.Find("Theme petal")
	if !ok {
		t.Fatalf("no petal card: %v", tt.Texts())
	}
	if heading.Y+heading.H > track.Y+1 {
		t.Fatalf("the heading ends at %v, the track starts at %v", heading.Y+heading.H, track.Y)
	}
	if track.Y+track.H > note.Y+1 {
		t.Fatalf("the track ends at %v, the note starts at %v", track.Y+track.H, note.Y)
	}
	if note.Y+note.H > card.Y+1 {
		t.Fatalf("the note ends at %v, the first card starts at %v", note.Y+note.H, card.Y)
	}
}

// TestPaletteCardNamesAreTranslated checks each card shows the theme's
// catalog name, not the key it is stored under: a missing message would
// otherwise fall back to the raw key.
func TestPaletteCardNamesAreTranslated(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	for _, th := range accentThemes {
		name := themeName(th)
		if name == "" || strings.HasPrefix(name, "accentTheme") {
			t.Fatalf("theme %q has no catalog name: %q", th.ID, name)
		}
		if !tt.HasText(name) {
			t.Fatalf("no card label %q: %v", name, tt.Texts())
		}
	}
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "accentTheme") {
			t.Fatalf("a card draws its key instead of its name: %q", text)
		}
	}
}

// TestPaletteGridIsTwoRowsOfFive checks the grid is 5x2: the fifth card
// stays on the first card's row and inside the panel, the sixth wraps.
func TestPaletteGridIsTwoRowsOfFive(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	panel, ok := tt.Find("Theme picker")
	if !ok {
		t.Fatalf("no theme picker panel: %v", tt.Texts())
	}
	card := func(id string) ui.Rect {
		t.Helper()
		r, ok := tt.Find("Theme " + id)
		if !ok {
			t.Fatalf("no card %q: %v", id, tt.Texts())
		}
		return r
	}
	first, fifth, sixth := card("petal"), card("tide"), card("apricot")
	if math.Abs(float64(first.Y-fifth.Y)) > 1 {
		t.Fatalf("the fifth card sits on row %v, want the first card's %v", fifth.Y, first.Y)
	}
	if fifth.X <= first.X {
		t.Fatalf("the fifth card starts at %v, not right of the first at %v", fifth.X, first.X)
	}
	if fifth.X+fifth.W > panel.X+panel.W {
		t.Fatalf("the fifth card ends at %v, past the panel's %v", fifth.X+fifth.W, panel.X+panel.W)
	}
	if sixth.Y <= first.Y+first.H {
		t.Fatalf("the sixth card sits on row %v, want below the first card's %v", sixth.Y, first.Y)
	}
	if math.Abs(float64(sixth.X-first.X)) > 1 {
		t.Fatalf("the sixth card starts at %v, want the first column's %v", sixth.X, first.X)
	}
}

// TestPaletteAppearanceSegmentsShareTheTrack checks the appearance
// segmented splits its track into equal thirds, flush with the track's
// edges: the chosen card spans a third instead of wrapping its label.
func TestPaletteAppearanceSegmentsShareTheTrack(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	trackInfo, ok := tt.AccessByRole(ui.RoleRadioGroup, locales.PaletteAppearance())
	if !ok {
		t.Fatalf("no appearance track: %v", tt.Texts())
	}
	track := trackInfo.Bounds
	labels := []string{"System", "Light", "Dark"}
	segs := make([]ui.Rect, len(labels))
	for i, label := range labels {
		info, ok := tt.AccessByRole(ui.RoleRadio, label)
		if !ok {
			t.Fatalf("no %s segment: %v", label, tt.Texts())
		}
		segs[i] = info.Bounds
	}
	for i, r := range segs {
		if math.Abs(float64(r.W-segs[0].W)) > 1 {
			t.Errorf("segment %d is %v wide, the first %v", i, r.W, segs[0].W)
		}
	}
	// The segments fill the track's content box, between its 3 DIP
	// padding and 2 DIP gaps.
	sum := segs[0].W + segs[1].W + segs[2].W
	if math.Abs(float64(sum-track.W)) > 14 {
		t.Errorf("the segments span %v of the track's %v", sum, track.W)
	}
	if off := segs[0].X - track.X; off > 6 {
		t.Errorf("the first segment starts %v into the track", off)
	}
	if off := track.X + track.W - segs[2].X - segs[2].W; off > 6 {
		t.Errorf("the last segment leaves %v of the track after it", off)
	}
	// Each label sits in the middle of its share.
	for i, label := range labels {
		text, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %s label: %v", label, tt.Texts())
		}
		mid, want := float64(text.X+text.W/2), float64(segs[i].X+segs[i].W/2)
		if math.Abs(mid-want) > 1 {
			t.Errorf("%s's label is centered at %v in %+v", label, mid, segs[i])
		}
	}
}

// TestPaletteCardAppliesAtOnce checks a meadow click writes the saved
// accent theme, persists it through a settings.json round trip, and
// keeps the popover open for the next choice.
func TestPaletteCardAppliesAtOnce(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	if err := tt.Click("Theme meadow"); err != nil {
		t.Fatalf("meadow card: %v", err)
	}
	tt.Frame()
	if s.Saved.AccentTheme != "meadow" {
		t.Fatalf("AccentTheme = %q, want meadow", s.Saved.AccentTheme)
	}
	if !s.paletteOpen {
		t.Fatalf("the card click closed the popover")
	}
	if _, ok := tt.Find("preview meadow"); !ok {
		t.Fatalf("the popover vanished after the card click")
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

// TestPaletteDarkSegmentChecksThemeSource checks the dark segment: the
// saved theme becomes Dark and the host's SetTheme sees it.
func TestPaletteDarkSegmentChecksThemeSource(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	// The segment carries its label text; the tester reaches it by it.
	if !tt.HasText("Dark") {
		t.Fatalf("no Dark segment: %v", tt.Texts())
	}
	tt.Click("Dark")
	tt.Frame()
	if s.Saved.Theme != settings.ThemeDark {
		t.Fatalf("Theme = %q, want Dark", s.Saved.Theme)
	}
	if src := ThemeSource(s.Saved.Theme); src != mygo.ThemeDark {
		t.Fatalf("ThemeSource = %q, want %q", src, mygo.ThemeDark)
	}
	if !s.paletteOpen {
		t.Fatalf("the segment click closed the popover")
	}
}

// TestPaletteEscapeCloses checks Esc closes the popover and a click on a
// card after reopening still applies.
func TestPaletteEscapeCloses(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if s.paletteOpen {
		t.Fatalf("Escape left the popover open")
	}
}

// TestPaletteSelectedCardOutline checks the chosen card marks itself: the
// preview's border renders in the window's accent, twice as wide as an
// unchosen card's hairline.
func TestPaletteSelectedCardOutline(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openPaletteFor(t, s, tt)

	chosen, ok := tt.Find("preview petal")
	if !ok {
		t.Fatalf("no petal preview: %v", tt.Texts())
	}
	plain, ok := tt.Find("preview mist")
	if !ok {
		t.Fatalf("no mist preview: %v", tt.Texts())
	}
	// The accent stroke is the 2 DIP Painter.Stroke over the preview's
	// paints, its center on the edge: the sampled pixel may sit half a
	// DIP into it, so the channels read within +-2 of the accent, and
	// the plain card's hairline keeps the same spot a different color.
	img := tt.Image()
	stroke := accentThemeByID(s.Saved.AccentTheme).of(false).Accent
	c1 := img.At(int(chosen.X)+1, int(chosen.Y+chosen.H/2))
	c2 := img.At(int(plain.X)+1, int(plain.Y+plain.H/2))
	r, g, b, _ := c1.RGBA()
	if absDiff(uint8(r>>8), stroke.R) > 2 || absDiff(uint8(g>>8), stroke.G) > 2 || absDiff(uint8(b>>8), stroke.B) > 2 {
		t.Fatalf("the chosen preview's edge shows %v, want the accent stroke %v +-2", c1, stroke)
	}
	if c1 == c2 {
		t.Fatalf("the chosen and plain previews' edges render the same: %v", c1)
	}
}
