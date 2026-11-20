package shell

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// TestSettingsNavigationGeometry checks the rail's shape: three page
// items inside a 216 DIP card, the first one filled with PanelSel and
// colored with the accent, and a hovered one tinted with SurfaceHover.
func TestSettingsNavigationGeometry(t *testing.T) {
	const w, h = 1400, 880
	s, _ := newShell(t)
	tt := s.renderSize(t, w, h)
	bare := settingsBareFrame(t, w, h)
	s.OpenSettings()
	tt.Frame()

	rail, ok := tt.Find("Settings rail")
	if !ok {
		t.Fatalf("no settings rail in %q", tt.Texts())
	}
	if rail.W != settingsRailWidth {
		t.Fatalf("rail is %v wide, want %v", rail.W, settingsRailWidth)
	}
	if len(settingsNavItems) != 3 {
		t.Fatalf("%d rail items, want 3", len(settingsNavItems))
	}
	for i, want := range []string{"Appearance", "Reading", "About"} {
		if got := settingsNavItems[i].title(); got != want {
			t.Fatalf("rail item %d is %q, want %q", i, got, want)
		}
		if _, ok := tt.Find("Settings nav " + want); !ok {
			t.Fatalf("no %q nav item in %q", want, tt.Texts())
		}
	}
	k := accentThemeByID(settings.AccentThemeDefault).of(false)
	probeSel := func(name string, want ui.Color) {
		t.Helper()
		r, ok := tt.Find("Settings nav " + name)
		if !ok {
			t.Fatalf("no %q nav item in %q", name, tt.Texts())
		}
		if r.X < rail.X || r.X+r.W > rail.X+rail.W {
			t.Fatalf("%q nav item %v runs outside the rail %v", name, r, rail)
		}
		x, y := r.X+r.W-4, r.Y+r.H/2
		got := pixelAt(tt.Image(), x, y)
		if !channelClose(got, want, 2) {
			t.Fatalf("%q nav pixel at %.0f,%.0f = %v, want %v", name, x, y, got, want)
		}
	}
	face := func(x, y float32) ui.Color {
		return over(k.Card, over(k.CardFocused, over(k.Backdrop, pixelAt(bare.Image(), x, y))))
	}

	var lastY float32
	for _, item := range settingsNavItems {
		r, ok := tt.Find("Settings nav " + item.title())
		if !ok {
			t.Fatalf("no %q nav item in %q", item.title(), tt.Texts())
		}
		if r.H < 34 {
			t.Fatalf("%q nav item is only %v high", item.title(), r.H)
		}
		if r.Y <= lastY {
			t.Fatalf("%q nav item at %v is not below the previous at %v", item.title(), r.Y, lastY)
		}
		lastY = r.Y
	}
	first := settingsNavItems[0].title()
	firstRect, _ := tt.Find("Settings nav " + first)
	fx, fy := firstRect.X+firstRect.W-4, firstRect.Y+firstRect.H/2
	probeSel(first, over(k.PanelSel, face(fx, fy)))

	// A hovered, unchosen row takes the surface tint; the chosen row keeps
	// its PanelSel fill.
	second := settingsNavItems[1].title()
	secondRect, _ := tt.Find("Settings nav " + second)
	sx, sy := secondRect.X+secondRect.W-4, secondRect.Y+secondRect.H/2
	tt.Move(sx, sy)
	tt.Frame()
	probeSel(second, over(k.SurfaceHover, face(sx, sy)))
	tt.Move(firstRect.X+firstRect.W-4, fy)
	tt.Frame()
	probeSel(first, over(k.PanelSel, face(fx, fy)))
}

// TestSettingsNavTextIsRegular pins the rail's page names to the inherited
// regular weight; mygo's Tester does not expose text styles, so this reads
// the rail's rendering source.
func TestSettingsNavTextIsRegular(t *testing.T) {
	src, err := os.ReadFile("settings.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(src), "func (s *Shell) settingsRail(")
	if start < 0 {
		t.Fatal("no settingsRail in settings.go")
	}
	rest := string(src[start:])
	end := strings.Index(rest, "\nfunc ")
	if end < 0 {
		t.Fatal("settingsRail has no end")
	}
	rail := rest[:end]
	if !strings.Contains(rail, "ui.Text(c, item.title()).FontSize(14)") {
		t.Fatal("the rail's page titles no longer use the regular text style")
	}
	if strings.Contains(rail, "FontWeight(") {
		t.Fatal("a rail page title still sets a non-regular font weight")
	}
}

// TestSettingsNavigationSwitchesPage is the behavioral check: a rail
// click replaces the detail header and card, and the controls that
// belonged to the old page disappear. If the click did nothing, every
// page assertion below fails.
func TestSettingsNavigationSwitchesPage(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	s.OpenSettings()
	tt.Frame()

	if head, ok := tt.AccessByRole(ui.RoleHeading, "Settings header"); !ok {
		t.Fatalf("no page heading in %q", tt.Texts())
	} else if head.Bounds.X < float32(300) {
		t.Fatalf("the heading %v sits in the rail", head.Bounds)
	}
	if !tt.HasText(settingsNavItems[0].detail()) {
		t.Fatalf("no Appearance subtitle in %q", tt.Texts())
	}
	if _, ok := tt.Find("Appearance card"); !ok {
		t.Fatalf("no Appearance card in %q", tt.Texts())
	}
	if s.settingsSection != 0 {
		t.Fatalf("the first section is %d, want 0", s.settingsSection)
	}

	for _, tc := range []struct {
		page string
		want int
		row  string
		gone string
	}{
		{"Reading", 1, "Project Root Markers", "Theme"},
		{"About", 2, "About card", "Project Root Markers"},
		{"Appearance", 0, "Theme", "About card"},
	} {
		if err := tt.Click("Settings nav " + tc.page); err != nil {
			t.Fatalf("no %q rail item: %v", tc.page, err)
		}
		tt.Frame()
		if s.settingsSection != tc.want {
			t.Fatalf("section after choosing %s = %d, want %d", tc.page, s.settingsSection, tc.want)
		}
		if head, ok := tt.AccessByRole(ui.RoleHeading, "Settings header"); !ok {
			t.Fatalf("after choosing %s, no page heading in %q", tc.page, tt.Texts())
		} else if head.Bounds.X < float32(300) {
			t.Fatalf("after choosing %s, the heading %v sits in the rail", tc.page, head.Bounds)
		}
		if detail := settingsNavItems[tc.want].detail(); !tt.HasText(detail) {
			t.Fatalf("after choosing %s, no page subtitle %q in %q", tc.page, detail, tt.Texts())
		}
		if _, ok := tt.Find(tc.page + " card"); !ok {
			t.Fatalf("after choosing %s, no card in %q", tc.page, tt.Texts())
		}
		if _, ok := tt.Find(tc.row); !ok {
			t.Fatalf("after choosing %s, no %q row in %q", tc.page, tc.row, tt.Texts())
		}
		if _, ok := tt.Find(tc.gone); ok {
			t.Fatalf("after choosing %s, the old %q row is still drawn", tc.page, tc.gone)
		}
	}
}

// TestSettingsReadingCardRowOrder pins the merged page: the file rows
// come after the reading rows, not between them.
func TestSettingsReadingCardRowOrder(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "Reading")

	var lastY float32
	for _, label := range []string{
		locales.ContentFontSize(),
		locales.CodeFontSize(),
		locales.SettingsScrollSpeed(),
		locales.SettingsInertiaScroll(),
		locales.SettingsAutoRefresh(),
		locales.SettingsExplorerRoot(),
		locales.SettingsProjectRootMarkers(),
	} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q row in %q", label, tt.Texts())
		}
		if r.Y <= lastY {
			t.Fatalf("%q row at %v is not below the previous at %v", label, r.Y, lastY)
		}
		lastY = r.Y
	}
}

// TestSettingsRightPaneScrollsIndependently checks that only the detail
// column moves on a wheel, and that choosing another page resets its
// scroll to the top while the rail keeps its place.
func TestSettingsRightPaneScrollsIndependently(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 760, 400)
	openSettingsSection(t, tt, s, "Reading")

	scroll, ok := tt.Find("Settings fields")
	if !ok {
		t.Fatalf("no fields scroller in %q", tt.Texts())
	}
	rail, ok := tt.Find("Settings rail")
	if !ok {
		t.Fatalf("no rail in %q", tt.Texts())
	}
	if scroll.X <= rail.X+rail.W {
		t.Fatalf("the fields scroller %v does not start right of the rail %v", scroll, rail)
	}
	before, ok := tt.Find("Reading card")
	if !ok {
		t.Fatalf("no Reading card in %q", tt.Texts())
	}

	tt.Scroll(scroll.X+scroll.W/2, scroll.Y+scroll.H/2, 0, 2000)
	tt.Frame()
	if s.settingsScroll.Y <= 0 {
		t.Fatalf("the fields scroller stayed at %v after a wheel", s.settingsScroll.Y)
	}
	after, _ := tt.Find("Reading card")
	if after.Y >= before.Y-10 {
		t.Fatalf("the card stayed at %v after scrolling from %v", after.Y, before.Y)
	}
	still, _ := tt.Find("Settings rail")
	if still.Y != rail.Y {
		t.Fatalf("the rail moved with the fields: %v to %v", rail.Y, still.Y)
	}

	if err := tt.Click("Settings nav About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.settingsScroll.Y != 0 {
		t.Fatalf("switching pages left the field scroll at %v, want the top", s.settingsScroll.Y)
	}
	card, ok := tt.Find("About card")
	if !ok {
		t.Fatalf("no About card in %q", tt.Texts())
	}
	if card.Y < scroll.Y-1 {
		t.Fatalf("the new page starts at %v, above the scroller %v", card.Y, scroll)
	}
}

// TestSettingsPanelHeightConstant pins the modal's height to the window,
// not to the chosen page: switching sections must not resize the panel.
func TestSettingsPanelHeightConstant(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1400, 880}, {760, 620}} {
		s, _ := newShell(t)
		tt := s.renderSize(t, size.w, size.h)
		s.OpenSettings()
		tt.Frame()

		want := float32(min(settingsMaxHeight, size.h-48))
		for _, page := range []string{"Appearance", "Reading", "About", "Appearance"} {
			if err := tt.Click("Settings nav " + page); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			panel, ok := tt.Find("Settings panel")
			if !ok {
				t.Fatalf("%dx%d: no panel after choosing %s in %q", size.w, size.h, page, tt.Texts())
			}
			if diff := panel.H - want; diff > 0.5 || diff < -0.5 {
				t.Fatalf("%dx%d %s: panel height %v, want a fixed %v", size.w, size.h, page, panel.H, want)
			}
		}
	}
}

// TestSettingsAboutContent checks the About page's version, links and
// license, including that a link uses the shell's external-link hook.
func TestSettingsAboutContent(t *testing.T) {
	s, _ := newShell(t)
	var opened []string
	s.OpenURL = func(url string) { opened = append(opened, url) }
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "About")

	for _, text := range []string{
		locales.AppTitle(),
		"v" + s.appVersion(),
		locales.SettingsAboutIntro(),
		repoURL,
		aboutIssuesURL,
		aboutChangelogURL,
		"MIT",
		runtime.Version(),
		mygo.Version,
	} {
		if !tt.HasText(text) {
			t.Fatalf("About page is missing %q in %q", text, tt.Texts())
		}
	}
	if _, ok := tt.Find("About card"); !ok {
		t.Fatalf("no About card in %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleLink, "About link "+locales.SettingsAboutRepository()); !ok {
		t.Fatalf("no repository link in %q", tt.Texts())
	}
	if err := tt.Click("About link " + locales.SettingsAboutRepository()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(opened) != 1 || opened[0] != repoURL {
		t.Fatalf("link click opened %v, want [%s]", opened, repoURL)
	}
	if !s.SettingsOpen {
		t.Fatal("clicking a link closed the settings dialog")
	}
}

// TestSettingsSaveErrorOnCurrentPage keeps the error line on the page
// whose control failed to write, instead of returning the reader to the
// first page on every error.
func TestSettingsSaveErrorOnCurrentPage(t *testing.T) {
	s, _ := newShell(t)
	s.Save = func(settings.Settings) error { return errors.New("save probe") }
	tt := s.renderSize(t, 1400, 880)
	openSettingsSection(t, tt, s, "Appearance")

	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Show FPS")
	if !ok {
		t.Fatalf("no Show FPS switch in %q", tt.Texts())
	}
	clickSwitch(tt, sw)
	tt.Frame()
	if s.settingsSection != 0 {
		t.Fatalf("section after a failed write = %d, want 0", s.settingsSection)
	}
	if !tt.HasText("save probe") {
		t.Fatalf("the error line is missing from %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleHeading, "Settings header"); !ok {
		t.Fatalf("the page heading vanished after a failed write: %q", tt.Texts())
	}
}
