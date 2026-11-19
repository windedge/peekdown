package shell

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/testshot"
	"github.com/windedge/peekdown/locales"
)

func titleBarToggleShell(t *testing.T, w, h int, files, outline bool) (*Shell, *ui.Tester) {
	t.Helper()
	s, _ := newShell(t)
	if err := s.OpenFile(cardShotDoc(t)); err != nil {
		t.Fatal(err)
	}
	s.Sidebar.Visible = files
	s.Outline.Visible = outline
	tt := s.renderSize(t, w, h)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	tt.Frame()
	return s, tt
}

// TestTitleBarPanelTogglesAtTheEnds pins the prototype's placement: the
// files switch leads the brand, the outline switch trails the tools,
// and both stops clear the system caption area on the right.
func TestTitleBarPanelTogglesAtTheEnds(t *testing.T) {
	_, tt := titleBarToggleShell(t, 1200, 600, true, true)
	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("no title bar: %v", tt.Texts())
	}
	logo, ok := tt.Find("App logo")
	if !ok {
		t.Fatalf("no brand mark: %v", tt.Texts())
	}
	menu, ok := tt.Find("Main menu")
	if !ok {
		t.Fatalf("no menu button: %v", tt.Texts())
	}
	files, okFiles := tt.Access(locales.BarFilesPanel())
	outline, okOutline := tt.Access(locales.BarOutlinePanel())
	if !okFiles || !okOutline {
		t.Fatalf("bar toggles missing: %v", tt.Texts())
	}
	for _, a := range []ui.AccessInfo{files, outline} {
		if a.Role != ui.RoleButton {
			t.Fatalf("%q is role %v, want a button", a.Name, a.Role)
		}
		if a.Bounds.W != 24 || a.Bounds.H != 24 {
			t.Fatalf("%q is %vx%v, want 24x24", a.Name, a.Bounds.W, a.Bounds.H)
		}
		if a.Bounds.Y < bar.Y || a.Bounds.Y+a.Bounds.H > bar.Y+bar.H {
			t.Fatalf("%q at %v sits outside the bar %v", a.Name, a.Bounds, bar)
		}
		if !a.Expanded {
			t.Fatalf("%q does not read expanded with its panel open", a.Name)
		}
	}
	if files.Description != locales.BarFilesTooltip() || outline.Description != locales.BarOutlineTooltip() {
		t.Fatalf("tooltips %q / %q, want the panel shortcuts", files.Description, outline.Description)
	}
	if files.Bounds.X < bar.X || files.Bounds.X+files.Bounds.W > logo.X {
		t.Fatalf("files switch %v is not ahead of the brand at %v", files.Bounds, logo)
	}
	if outline.Bounds.X < menu.X+menu.W {
		t.Fatalf("outline switch %v does not trail the menu at %v", outline.Bounds, menu)
	}
	if limit := bar.X + bar.W - 138; outline.Bounds.X+outline.Bounds.W > limit+0.5 {
		t.Fatalf("outline switch ends at %v, under the window controls at %v", outline.Bounds.X+outline.Bounds.W, limit)
	}
}

// TestTitleBarPanelTogglesFlipAndReadState checks both directions and the
// active face: the accent-tinted PanelSel fill and accent glyph mean
// open; the bare bar means closed.
func TestTitleBarPanelTogglesFlipAndReadState(t *testing.T) {
	s, tt := titleBarToggleShell(t, 1200, 600, true, false)
	th := windowLight()
	k := accentThemeByID(settings.AccentThemeDefault).Light
	active := over(k.PanelSel, th.Surface)
	countPixels := func(btn ui.Rect, want ui.Color) int {
		n := 0
		for y := int(btn.Y) + 4; y < int(btn.Y)+20; y++ {
			for x := int(btn.X) + 4; x < int(btn.X)+20; x++ {
				if channelClose(pixelAt(tt.Image(), float32(x), float32(y)), want, 1) {
					n++
				}
			}
		}
		return n
	}
	for _, c := range []struct {
		name   string
		active bool
	}{
		{locales.BarFilesPanel(), true},
		{locales.BarOutlinePanel(), false},
	} {
		btn, ok := tt.Find(c.name)
		if !ok {
			t.Fatalf("no %q button", c.name)
		}
		want := th.Surface
		glyph := th.TextMuted
		if c.active {
			want, glyph = active, k.Accent
		}
		if got := pixelAt(tt.Image(), btn.X+3, btn.Y+3); !channelClose(got, want, 2) {
			t.Fatalf("%q face %v, want %v (active=%v)", c.name, got, want, c.active)
		}
		if n := countPixels(btn, glyph); n < 3 {
			t.Fatalf("%q glyph has %d %v pixels, want at least 3", c.name, n, glyph)
		}
	}

	if err := tt.Click(locales.BarFilesPanel()); err != nil {
		t.Fatal(err)
	}
	tt.Move(600, 300)
	tt.Frame()
	tt.Frame()
	if s.Sidebar.Visible {
		t.Fatal("the files switch must close the pane")
	}
	if a, ok := tt.Access(locales.BarFilesPanel()); !ok || a.Expanded {
		t.Fatal("the closed files switch must read unexpanded")
	}
	btn, ok := tt.Find(locales.BarFilesPanel())
	if !ok {
		t.Fatal("files switch missing after closing")
	}
	if got := pixelAt(tt.Image(), btn.X+3, btn.Y+3); !channelClose(got, th.Surface, 2) {
		t.Fatalf("closed files switch face %v, want the bar's %v", got, th.Surface)
	}
	if n := countPixels(btn, th.TextMuted); n < 3 {
		t.Fatalf("closed glyph has %d muted pixels, want at least 3", n)
	}

	if err := tt.Click(locales.BarOutlinePanel()); err != nil {
		t.Fatal(err)
	}
	tt.Move(600, 300)
	tt.Frame()
	tt.Frame()
	if !s.Outline.Visible || s.Sidebar.Visible {
		t.Fatal("the outline switch must open its pane and leave files alone")
	}
	if a, ok := tt.Access(locales.BarOutlinePanel()); !ok || !a.Expanded {
		t.Fatal("the open outline switch must read expanded")
	}
	btn, ok = tt.Find(locales.BarOutlinePanel())
	if !ok {
		t.Fatal("outline switch missing after opening")
	}
	if got := pixelAt(tt.Image(), btn.X+3, btn.Y+3); !channelClose(got, active, 2) {
		t.Fatalf("open outline switch face %v, want %v", got, active)
	}
	if err := tt.Click(locales.BarOutlinePanel()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if s.Outline.Visible {
		t.Fatal("the outline switch must close its pane on the second click")
	}
}

// TestTitleBarPanelTogglesOnTheWelcomePage keeps both switches reachable
// without a document: they belong to the bar, not the reading card.
func TestTitleBarPanelTogglesOnTheWelcomePage(t *testing.T) {
	s, _ := newShell(t)
	s.Sidebar.Visible = false
	s.Outline.Visible = false
	tt := s.renderSize(t, 1200, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	for _, label := range []string{locales.BarFilesPanel(), locales.BarOutlinePanel()} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("%q is not an input target on the welcome page: %v", label, err)
		}
		tt.Frame()
		tt.Frame()
	}
	if !s.Sidebar.Visible || !s.Outline.Visible {
		t.Fatalf("welcome switches: files %v outline %v, want both open", s.Sidebar.Visible, s.Outline.Visible)
	}
}

func TestTitleBarTogglesShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the title bar toggle screenshots")
	}
	_, open := titleBarToggleShell(t, 1440, 900, true, true)
	writePng(t, open.Image(), "C:/tmp/titlebar-toggles.png")
	btn, ok := open.Access(locales.BarOutlinePanel())
	if !ok {
		t.Fatalf("outline switch missing: %v", open.Texts())
	}
	crop := testshot.Crop(open.Image(), int(btn.Bounds.X)-96, int(btn.Bounds.Y)-8, 128, 40)
	testshot.WritePng(t, testshot.Scale3x(crop), "C:/tmp/outline-icon-tas.png")

	_, closed := titleBarToggleShell(t, 1440, 900, false, false)
	writePng(t, closed.Image(), "C:/tmp/titlebar-toggles-closed.png")
}

func TestOutlineCardNoBorderShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the outline card screenshot")
	}
	_, tt := titleBarToggleShell(t, 1200, 700, false, true)
	card, ok := tt.Find("Outline card")
	if !ok {
		t.Fatalf("no outline card: %v", tt.Texts())
	}
	crop := testshot.Crop(tt.Image(), int(card.X)-12, int(card.Y)+70, 40, 90)
	testshot.WritePng(t, testshot.Scale3x(crop), "C:/tmp/outline-no-border.png")
}
