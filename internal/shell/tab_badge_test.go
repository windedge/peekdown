package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/testshot"
	"github.com/windedge/peekdown/locales"
)

// TestTabBadgeHelpers pins the mapping the badge draws: a name always
// picks the same palette entry, and its glyph is the basename's first
// character, upper cased when it is a letter.
func TestTabBadgeHelpers(t *testing.T) {
	for _, tc := range []struct{ path, initial string }{
		{"/docs/meeting-notes.md", "M"},
		{"/docs/readme.md", "R"},
		{"/docs/9-guide.md", "9"},
		{"/docs/_draft.md", "_"},
		{"/docs/.hidden.md", "."},
		{"", "."},
	} {
		if got := tabBadgeInitial(tc.path); got != tc.initial {
			t.Errorf("tabBadgeInitial(%q) = %q, want %q", tc.path, got, tc.initial)
		}
		if c := tabBadgeColor(tc.path); c != tabBadgeColor(tc.path) || c == (ui.Color{}) {
			t.Errorf("tabBadgeColor(%q) is unstable or empty: %v", tc.path, c)
		}
	}
	if c := tabBadgeColor("/a/meeting-notes.md"); c != tabBadgeColor("/b/meeting-notes.md") {
		t.Error("the same basename in two directories picks different badge colors")
	}
	seen := map[ui.Color]bool{}
	for _, name := range []string{"meeting-notes.md", "readme.md", "guide.md", "a.md", "b.md"} {
		seen[tabBadgeColor(name)] = true
	}
	if len(seen) < 2 {
		t.Error("the badge palette never varies with the file name")
	}
	for _, c := range tabBadgePalette {
		if c.A != 255 {
			t.Errorf("palette color %v is not opaque", c)
		}
	}
}

// TestTabBadgesRender draws two tabs and checks their badge geometry and
// pixels: the badge is a fixed square before the label, its fill comes
// from the name's hash, and it leaves the label and close button clear.
func TestTabBadgesRender(t *testing.T) {
	s, dir := newShell(t)
	long := filepath.Join(dir, "meeting-notes-with-an-extraordinarily-long-filename.md")
	short := filepath.Join(dir, "readme.md")
	for _, p := range []string{long, short} {
		if err := os.WriteFile(p, []byte("# "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.OpenPaths([]string{long, short})
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	if tabBadgeColor(long) == tabBadgeColor(short) {
		t.Fatal("the two test files happen to share a badge color")
	}
	for _, path := range []string{long, short} {
		name := filepath.Base(path)
		badgeRect, ok := tt.Find("Tab badge: " + name)
		if !ok {
			t.Fatalf("texts %v: want the badge for %q", tt.Texts(), name)
		}
		if badgeRect.W != tabBadgeSize || badgeRect.H != tabBadgeSize {
			t.Fatalf("%q's badge is %vx%v, want %vx%v", name, badgeRect.W, badgeRect.H, tabBadgeSize, tabBadgeSize)
		}
		if got := pixelAt(tt.Image(), badgeRect.X+2, badgeRect.Y+badgeRect.H/2); !channelClose(got, tabBadgeColor(path), 1) {
			t.Fatalf("%q's badge paints %v, want %v", name, got, tabBadgeColor(path))
		}
		label, ok := tt.Find(name)
		if !ok {
			t.Fatalf("texts %v: want the %q label", tt.Texts(), name)
		}
		if gap := label.X - (badgeRect.X + badgeRect.W); gap < tabBadgeGap-0.5 {
			t.Fatalf("%q's badge ends at %v but its label starts at %v, gap %v", name, badgeRect.X+badgeRect.W, label.X, gap)
		}
		if label.Y+label.H > badgeRect.Y+badgeRect.H || label.Y < badgeRect.Y {
			t.Fatalf("%q's label %v is not beside its badge %v", name, label, badgeRect)
		}
		// The close button is a 20 DIP square in the tab's right padding;
		// the label must end before it, and the badge stays inside the tab.
		var tab ui.Rect
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleTab && strings.Contains(a.Name, name) {
				tab = a.Bounds
			}
		}
		if tab.W == 0 {
			t.Fatalf("access: want the %q tab", name)
		}
		if badgeRect.X < tab.X || badgeRect.X+badgeRect.W > tab.X+tab.W {
			t.Fatalf("%q's badge %v sits outside its tab %v", name, badgeRect, tab)
		}
		if label.X+label.W > tab.X+tab.W-10-20+0.5 {
			t.Fatalf("%q's label %v runs into the close button in %v", name, label, tab)
		}
		initial := strings.ToUpper(name[:1])
		found := false
		for _, text := range tt.Texts() {
			if text == initial {
				found = true
			}
		}
		if !found {
			t.Fatalf("texts %v: want the badge's %q glyph", tt.Texts(), initial)
		}
	}
}

// TestTitleBarToolIconLabels keeps the bar's tool icons' commands named:
// changing an SVG path must not lose a button or its label. The two
// panel switches sit at the bar's ends alongside its three tools.
func TestTitleBarToolIconLabels(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 900, 600)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()

	labels := []string{
		locales.BarFilesPanel(), locales.BarOpenFile(), locales.BarFind(),
		locales.BarSettingsPanel(), locales.BarOutlinePanel(),
	}
	count := 0
	for _, a := range tt.AccessAll() {
		if a.Role != ui.RoleButton {
			continue
		}
		for _, label := range labels {
			if a.Name == label {
				count++
			}
		}
	}
	if count != len(labels) {
		t.Fatalf("%d of the %d title bar tool buttons are named: %v", count, len(labels), tt.Texts())
	}
	for _, label := range labels {
		if _, ok := tt.Access(label); !ok {
			t.Errorf("no accessible %q button", label)
		}
	}
}

// TestTabsBadgeShot writes the two-tab bar for a visual check: a short
// name and a long one, both with their colored letter badge. Skipped
// unless PEEKDOWN_SHOT is set, so regular runs stay headless-only.
func TestTabsBadgeShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the tab badge screenshot")
	}
	s, dir := newShell(t)
	long := filepath.Join(dir, "meeting-notes-with-an-extraordinarily-long-filename.md")
	short := filepath.Join(dir, "r.md")
	for _, p := range []string{short, long} {
		if err := os.WriteFile(p, []byte("# "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.OpenFile(p); err != nil {
			t.Fatal(err)
		}
	}
	tt := s.renderSize(t, 1440, 880)
	tt.SetTitleBar(ui.TitleBar{Height: TitleBarHeight, Right: 138})
	tt.Frame()
	testshot.WritePng(t, tt.Image(), "C:/tmp/tabs-badges.png")

	bar, ok := tt.Find("Title bar")
	if !ok {
		t.Fatalf("texts %v: want the title bar", tt.Texts())
	}
	crop := testshot.Crop(tt.Image(), 0, 0, int(bar.W), int(bar.H))
	testshot.WritePng(t, testshot.Scale3x(crop), "C:/tmp/tabs-badges-bar.png")
}
