package shell

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// tabBackground samples the tab holding label: a few DIPs left of its text,
// inside the tab's padding, where the tab's own background shows and no
// glyph does.
func tabBackground(t *testing.T, tt *ui.Tester, label string) ui.Color {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("texts %v: want the %q tab", tt.Texts(), label)
	}
	return pixelAt(tt.Image(), r.X-6, r.Y+r.H/2)
}

// TestBrandOpensHome keeps the tabs and shows the welcome page: a click on
// the bar's brand returns to the greeting with both tabs still in the strip
// and none of them read active, and a click on a tab comes back to it.
func TestBrandOpensHome(t *testing.T) {
	s, dir := newShell(t)
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	s.OpenPaths([]string{a, b})
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	// The brand is one control: the mark and the name sit inside it, it
	// fits the bar, and it is wide enough for the name it shows.
	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar", tt.Texts())
	}
	home, ok := tt.Find("Home")
	if !ok {
		t.Fatalf("texts %v: want the brand's button", tt.Texts())
	}
	logo, ok := tt.Find("App logo")
	if !ok {
		t.Fatalf("texts %v: want the app's mark", tt.Texts())
	}
	name, ok := tt.Find("Peekdown")
	if !ok {
		t.Fatalf("texts %v: want the app's name", tt.Texts())
	}
	if home.H > bar.H {
		t.Fatalf("the brand is %v high, taller than the bar's %v", home.H, bar.H)
	}
	if home.W < name.W {
		t.Fatalf("the brand is %v wide, narrower than its name %v", home.W, name.W)
	}
	for _, r := range []ui.Rect{logo, name} {
		if r.X < home.X || r.X+r.W > home.X+home.W+0.5 ||
			r.Y < home.Y || r.Y+r.H > home.Y+home.H+0.5 {
			t.Fatalf("%v sits outside the brand's box %v", r, home)
		}
	}

	// Before Home the active tab (b.md, the last opened) takes the
	// window's background and the idle one the raised surface: the sample
	// tells the two apart, so its verdict after Home means something.
	light := ui.LightTheme()
	if got := tabBackground(t, tt, "b.md"); got != light.Background {
		t.Fatalf("the active tab is %v, want the window's %v", got, light.Background)
	}
	if got := tabBackground(t, tt, "a.md"); got != light.Surface {
		t.Fatalf("the idle tab is %v, want the raised %v", got, light.Surface)
	}

	if err := tt.Click("Home"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.Home {
		t.Fatal("the brand's click left Home off")
	}
	if !tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %v: want the welcome page", tt.Texts())
	}
	if tt.HasText("first") || tt.HasText("second") {
		t.Fatalf("texts %v: a document still shows under Home", tt.Texts())
	}
	if len(s.Tabs) != 2 {
		t.Fatalf("Home dropped tabs: %d left", len(s.Tabs))
	}
	for _, label := range []string{"a.md", "b.md"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("texts %v: want the %q tab kept", tt.Texts(), label)
		}
		if got := tabBackground(t, tt, label); got != light.Surface {
			t.Fatalf("the %q tab is %v under Home, want the idle %v", label, got, light.Surface)
		}
	}

	// A click on the second tab returns to it, although Home left it the
	// active one: the click itself is what leaves the welcome page.
	if err := tt.Click("b.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Home {
		t.Fatal("the tab's click left Home on")
	}
	if !tt.HasText("second") {
		t.Fatalf("texts %v: want the second document's body", tt.Texts())
	}
	if tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %v: the welcome page stayed under the document", tt.Texts())
	}
	if got := tabBackground(t, tt, "b.md"); got != light.Background {
		t.Fatalf("the clicked tab is %v, want the active %v", got, light.Background)
	}
}

// TestHomeLeavesOnOpen checks the other way out: opening a file, by the
// path or by a recent row, returns to the document.
func TestHomeLeavesOnOpen(t *testing.T) {
	s, dir := newShell(t)
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	s.OpenPaths([]string{a, b})
	tt := s.render(t)
	tt.Frame()
	s.ShowHome()
	tt.Frame()
	if !tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %v: want the welcome page", tt.Texts())
	}

	// The recent row leads back to the tab it names.
	if err := tt.Click("Recent row " + a); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Home {
		t.Fatal("the recent row's click left Home on")
	}
	if s.ActiveTab() == nil || s.ActiveTab().Path != a {
		t.Fatalf("active %v, want %q", s.ActiveTab(), a)
	}
	if !tt.HasText("first") || tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %v: want a.md's body", tt.Texts())
	}

	// OpenFile is the same route: the shell leaves Home on it too.
	s.ShowHome()
	tt.Frame()
	if err := s.OpenFile(b); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Home {
		t.Fatal("OpenFile left Home on")
	}
	if !tt.HasText("second") {
		t.Fatalf("texts %v: want b.md's body", tt.Texts())
	}
}

// TestHomeAfterClosingEveryTab keeps the empty window and Home on one view:
// closing the last tab lands on the same welcome page.
func TestHomeAfterClosingEveryTab(t *testing.T) {
	s, dir := newShell(t)
	s.OpenPaths([]string{filepath.Join(dir, "a.md")})
	tt := s.render(t)
	tt.Frame()
	s.ShowHome()
	tt.Frame()

	s.CloseAll()
	tt.Frame()
	if !tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %v: want the welcome page after the last close", tt.Texts())
	}
	if _, ok := tt.Find("Peekdown"); !ok {
		t.Fatalf("texts %v: want the bar's brand on the empty page", tt.Texts())
	}
	// Opening again from there leaves Home, which the empty window keeps
	// on: the flag must not stick.
	if err := s.OpenFile(filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Home {
		t.Fatal("opening a file left Home on")
	}
	if !tt.HasText("second") {
		t.Fatalf("texts %v: want b.md's body", tt.Texts())
	}
}
