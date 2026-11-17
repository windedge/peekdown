package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

func newShell(t *testing.T) (*Shell, string) {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	if err := os.WriteFile(a, []byte("# A\n\nfirst\n\n## A two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("# B\n\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New()
	// Auto refresh off: watcher tests turn it on themselves, the rest of
	// the suite must not race a live watcher goroutine.
	saved := settings.Default()
	saved.AutoRefresh = false
	recent := []string{}
	s.Load = func() settings.Settings { return saved }
	s.Save = func(v settings.Settings) error {
		saved = v
		return nil
	}

	var applied []string
	s.SetTheme = func(src mygo.ThemeSource) { applied = append(applied, string(src)) }
	s.Recent = func() []string { return append([]string(nil), recent...) }
	// The mygo store dedupes and moves the opened path to the front; a
	// stub that only prepends would list one path twice and panic on
	// duplicate row keys.
	s.AddRecent = func(p string) error {
		kept := make([]string, 0, len(recent)+1)
		for _, r := range recent {
			if r != p {
				kept = append(kept, r)
			}
		}
		recent = append(kept, p)
		return nil
	}
	s.ClearRecent = func() error {
		recent = nil
		return nil
	}
	s.OpenDialog = func() ([]string, error) { return []string{a, b}, nil }
	s.Saved = s.Load()
	s.Sidebar.Visible = false
	return s, dir
}

func (s *Shell) render(t *testing.T) *ui.Tester {
	t.Helper()
	return s.renderSize(t, 800, 600)
}

func (s *Shell) renderSize(t *testing.T, width, height int) *ui.Tester {
	t.Helper()
	return ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		s.View(c)
	}, width, height)
}

func TestTabOpenCloseAndSwitch(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)

	s.OpenPaths([]string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")})
	tt.Frame()
	if len(s.Tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(s.Tabs))
	}
	if s.Active != 1 || s.ActiveTab().Path != filepath.Join(dir, "b.md") {
		t.Fatalf("active = %d %v", s.Active, s.ActiveTab())
	}

	if err := tt.Click("a.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Active != 0 {
		t.Fatalf("active after click = %d, want 0", s.Active)
	}

	s.CloseTab(0)
	if len(s.Tabs) != 1 || s.Active != 0 || filepath.Base(s.ActiveTab().Path) != "b.md" {
		t.Fatalf("after close: %d tabs active %d", len(s.Tabs), s.Active)
	}
	s.CloseAll()
	if len(s.Tabs) != 0 || s.Active != -1 || s.ActiveTab() != nil {
		t.Fatalf("after close all: %d tabs active %d", len(s.Tabs), s.Active)
	}
	tt.Frame()
	if !tt.HasText("Welcome to Peekdown") {
		t.Fatalf("welcome not shown: %v", tt.Texts())
	}
}

func TestTabContextMenuActions(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)
	s.OpenPaths([]string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")})
	tt.Frame()

	if err := tt.RightClick("b.md"); err != nil {
		t.Fatal(err)
	}
	items := tt.Menu()
	if len(items) == 0 {
		t.Fatal("no context menu items")
	}
	if err := tt.ChooseMenuItem("Close Other Tabs"); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 1 || filepath.Base(s.Tabs[0].Path) != "b.md" {
		t.Fatalf("close others left %d tabs", len(s.Tabs))
	}
	if s.Active != 0 {
		t.Fatalf("active = %d, want 0", s.Active)
	}

	tt.Frame()
	if err := tt.RightClick("b.md"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Close All Tabs"); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 0 {
		t.Fatalf("close all left %d tabs", len(s.Tabs))
	}
}

// Reopening a file that already has a tab activates it instead of
// stacking a duplicate: one open, one tab, whatever the path's case
// the later call brings (Windows names the same file under any
// case). The disk content's change rereads; an unchanged disk keeps
// the model as it is.
func TestOpenSameFileReusesTab(t *testing.T) {
	s, dir := newShell(t)
	path := filepath.Join(dir, "a.md")
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.md")
	if err := os.WriteFile(other, []byte("# Other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(other); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 2 {
		t.Fatalf("tabs after setup = %d, want 2", len(s.Tabs))
	}

	// The same file again: the tab activates, no third one appends.
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 2 || s.Active != 0 || s.ActiveTab().Path != path {
		t.Fatalf("reopen: %d tabs, active %d (%v)", len(s.Tabs), s.Active, s.ActiveTab())
	}
	if s.Title() != path+" - Peekdown" {
		t.Fatalf("title = %q", s.Title())
	}

	// A case variant names the same file on Windows; it must land on
	// the existing tab everywhere (on Linux the fold is a no-op that
	// only fails to fire when a case-only alias is really two files).
	if runtime.GOOS == "windows" {
		if err := s.OpenFile(strings.ToUpper(path)); err != nil {
			t.Fatal(err)
		}
		if len(s.Tabs) != 2 || s.Active != 0 {
			t.Fatalf("case-variant reopen: %d tabs, active %d", len(s.Tabs), s.Active)
		}
	}

	// The disk content moved while the tab was open: the reopen rereads
	// it into the same tab.
	newer := "# A rewritten\n"
	if err := os.WriteFile(path, []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 2 || s.Active != 0 {
		t.Fatalf("reopen after change: %d tabs, active %d", len(s.Tabs), s.Active)
	}
	if got := s.ActiveTab().Model.Source(); got != newer {
		t.Fatalf("model after reopen: %q, want %q", got, newer)
	}

	// A different file still opens its own tab.
	cPath := filepath.Join(dir, "c.md")
	if err := os.WriteFile(cPath, []byte("# C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(cPath); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 3 || s.Active != 2 {
		t.Fatalf("new file: %d tabs, active %d", len(s.Tabs), s.Active)
	}
}

// A relative path with ./ segments names an open tab's file too: the
// cleanup before the comparison makes the reuse fire.
func TestOpenRelativePathReusesTab(t *testing.T) {
	s, dir := newShell(t)
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := s.OpenFile("./a.md"); err != nil {
		t.Fatal(err)
	}
	if len(s.Tabs) != 1 || s.Active != 0 {
		t.Fatalf("relative reopen: %d tabs, active %d", len(s.Tabs), s.Active)
	}
}

func TestTabFollowsActive(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)
	for _, name := range []string{"a.md", "b.md", "a.md", "b.md"} {
		s.OpenFile(filepath.Join(dir, name))
	}
	tt.Frame()
	if !s.synced || s.activeSeen != s.Active {
		t.Fatalf("not synced: synced=%v seen=%d active=%d", s.synced, s.activeSeen, s.Active)
	}

	s.Active = 0
	tt.Frame()
	if s.activeSeen != 0 {
		t.Fatalf("activeSeen = %d, want 0", s.activeSeen)
	}

	s.CloseTab(0)
	tt.Frame()
	if s.activeSeen != s.Active || s.Active < 0 || s.Active >= len(s.Tabs) {
		t.Fatalf("after close: seen=%d active=%d len=%d", s.activeSeen, s.Active, len(s.Tabs))
	}
}

func TestSidebarToggleAndWidth(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)
	s.OpenFile(filepath.Join(dir, "a.md"))
	tt.Frame()

	if s.Sidebar.Visible {
		t.Fatal("sidebar starts hidden in the test shell")
	}
	s.Sidebar.Visible = true
	tt.Frame()
	if !s.Sidebar.Visible {
		t.Fatal("show did not show the sidebar")
	}
	if !tt.HasText("Outline") {
		t.Fatalf("sidebar not rendered: %v", tt.Texts())
	}

	s.SetSidebarTab(1)
	tt.Frame()
	if s.Sidebar.Tab != 1 || !s.Sidebar.Visible {
		t.Fatalf("tab = %d visible = %v", s.Sidebar.Tab, s.Sidebar.Visible)
	}
	if !tt.HasText("A two") {
		t.Fatalf("outline missing heading: %v", tt.Texts())
	}

	s.SetSidebarWidth(10)
	if s.Sidebar.Width != 120 {
		t.Fatalf("width = %v, want 120", s.Sidebar.Width)
	}
	s.SetSidebarWidth(1000)
	if s.Sidebar.Width != 400 {
		t.Fatalf("width = %v, want 400", s.Sidebar.Width)
	}
	s.Sidebar.Visible = false
	if s.Sidebar.Visible {
		t.Fatal("hide did not hide the sidebar")
	}
}

func TestSettingsApplyImmediately(t *testing.T) {
	s, _ := newShell(t)
	var applied string
	s.SetTheme = func(src mygo.ThemeSource) { applied = string(src) }
	tt := s.renderSize(t, 800, 1000)
	s.OpenSettings()
	tt.Frame()
	if !tt.HasText("Settings") {
		t.Fatalf("dialog missing: %v", tt.Texts())
	}

	// Choosing Dark applies at once: the saved settings and the host
	// theme both hold it without any Save press.
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Saved.Theme != settings.ThemeDark {
		t.Fatalf("theme = %q right after the click, want Dark applied at once", s.Saved.Theme)
	}
	if applied != "dark" {
		t.Fatalf("host theme = %q, want dark applied immediately", applied)
	}

	// The layout segmented applies too.
	if err := tt.Click("Full Width"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.Saved.Layout != "full_width" {
		t.Fatalf("layout = %q after the click, want full_width", s.Saved.Layout)
	}
	if s.draftLayout != 1 {
		t.Fatalf("draftLayout = %d, want 1", s.draftLayout)
	}

	// Reopening the dialog reads the applied values.
	s.OpenSettings()
	if s.draftLayout != 1 {
		t.Fatalf("draftLayout after reopen = %d, want 1", s.draftLayout)
	}
	tt.Frame()
	if err := tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("dialog still open after Done")
	}
}

func TestSettingsSaveErrorKeepsDialogOpen(t *testing.T) {
	s, _ := newShell(t)
	s.Save = func(settings.Settings) error { return errors.New("disk full") }
	tt := s.renderSize(t, 800, 1000)
	s.OpenSettings()
	tt.Frame()

	// A control's change persists at once; a failing write shows its
	// error and leaves the dialog open with the choice standing.
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.SettingsOpen {
		t.Fatal("dialog closed after a failed save")
	}
	if !tt.HasText("disk full") {
		t.Fatalf("texts %q: want the save error", tt.Texts())
	}

	// Done still closes it.
	if err := tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("dialog still open after Done")
	}
}

// TestRestoreFromSettings walks the startup restore: the saved sidebar
// width, tab and visibility land on the shell and the theme reaches
// the host. An unknown tab name
// falls back to the explorer, a hidden sidebar stays hidden.
func TestRestoreFromSettings(t *testing.T) {
	s := New()
	var theme string
	s.SetTheme = func(source mygo.ThemeSource) { theme = string(source) }
	var saved settings.Settings
	saved.Theme = settings.ThemeDark
	saved.SidebarWidth = 333
	saved.SidebarTab = "outline"
	saved.SidebarVisible = true

	s.RestoreFromSettings(saved)
	if s.Sidebar.Width != 333 || s.Sidebar.Tab != 1 || !s.Sidebar.Visible {
		t.Fatalf("sidebar after restore: %+v", s.Sidebar)
	}
	if !reflect.DeepEqual(s.Saved, saved) {
		t.Fatalf("saved = %+v, want the restored %+v", s.Saved, saved)
	}
	if theme != "dark" {
		t.Fatalf("theme = %q, want dark", theme)
	}

	saved.SidebarTab = "nonsense"
	saved.SidebarVisible = false
	s2 := New()
	s2.RestoreFromSettings(saved)
	if s2.Sidebar.Tab != 0 || s2.Sidebar.Visible {
		t.Fatalf("fallback restore: tab %d visible %v, want the explorer hidden", s2.Sidebar.Tab, s2.Sidebar.Visible)
	}
}

func TestApplyKeepsSidebarVisibility(t *testing.T) {
	s, _ := newShell(t)
	s.Sidebar.Visible = true
	saved := s.Saved
	saved.SidebarVisible = false
	if err := s.Apply(saved); err != nil {
		t.Fatal(err)
	}
	if !s.Sidebar.Visible {
		t.Fatal("apply hid a visible sidebar")
	}
}

func TestWelcomeShowsRecent(t *testing.T) {
	s, dir := newShell(t)
	a := filepath.Join(dir, "a.md")
	if err := s.AddRecent(a); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	if !tt.HasText("Welcome to Peekdown") || !tt.HasText("a.md") {
		t.Fatalf("welcome texts: %v", tt.Texts())
	}

	if err := tt.Click("a.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(s.Tabs) != 1 || s.ActiveTab().Path != a {
		t.Fatalf("recent click did not open the file: %+v", s.Tabs)
	}

	s.CloseAll()
	tt.Frame()
	if err := tt.Click("Clear"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := s.Recent(); len(got) != 0 {
		t.Fatalf("recent = %v, want empty", got)
	}
	if tt.HasText("a.md") {
		t.Fatalf("recent list still rendered: %v", tt.Texts())
	}
}

func TestWelcomeRecentRowsAlignLeft(t *testing.T) {
	s, dir := newShell(t)
	// The rows' contents differ in width only if the directories do; a
	// centered ButtonBase row would indent each line by its own width.
	var names []string
	for _, d := range []string{"sub", "sub", "subsubsubsub"} {
		dir := filepath.Join(dir, d)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		name := d + ".md"
		names = append(names, name)
		if err := s.AddRecent(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	tt := s.render(t)

	var want float32
	for i, name := range names {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("recent row %q not rendered: %v", name, tt.Texts())
		}
		if i == 0 {
			want = r.X
			continue
		}
		if r.X != want {
			t.Fatalf("recent row %q starts at x %v, row 0 at %v", name, r.X, want)
		}
	}
}

func TestWindowTitle(t *testing.T) {
	s, dir := newShell(t)
	if s.Title() != "Peekdown" {
		t.Fatalf("title = %q", s.Title())
	}
	path := filepath.Join(dir, "a.md")
	if err := s.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if want := path + " - Peekdown"; s.Title() != want {
		t.Fatalf("title = %q, want %q", s.Title(), want)
	}
	s.CloseAll()
	s.emitTitle()
	if s.Title() != "Peekdown" {
		t.Fatalf("title = %q", s.Title())
	}
}

func TestTabCloseButtonDoesNotSwitch(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)
	s.OpenPaths([]string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")})
	tt.Frame()
	if err := tt.Click("Close a.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if len(s.Tabs) != 1 || filepath.Base(s.Tabs[0].Path) != "b.md" {
		t.Fatalf("close left %d tabs: %+v", len(s.Tabs), s.Tabs)
	}
	if s.Active != 0 || filepath.Base(s.ActiveTab().Path) != "b.md" {
		t.Fatalf("active = %d %v, want the untouched b.md", s.Active, s.ActiveTab())
	}
}

// TestIconButtonsLabeled keeps the decorative buttons reachable by their
// labels, since the icons carry no text of their own.
func TestIconButtonsLabeled(t *testing.T) {
	s, dir := newShell(t)
	tt := s.render(t)
	s.OpenFile(filepath.Join(dir, "a.md"))
	s.Sidebar.Visible = true
	tt.Frame()
	// The status bar is labels now, its zoom readout included: the
	// remaining icons live in the sidebar header and the title bar. The
	// tabs go before the close, which takes the sidebar off the window.
	for _, label := range []string{"Files", "Outline", "Close sidebar", "Find in document", "App settings"} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("%q is not the input target: %v", label, err)
		}
		tt.Frame()
		tt.Frame()
	}
	if !s.SettingsOpen {
		t.Fatal("settings button did not open the dialog")
	}
}

// searchShell opens a document with "first" twice, once upper case, and
// shows the search bar.
func searchShell(t *testing.T) (*Shell, *ui.Tester, *Tab) {
	s, dir := newShell(t)
	if err := os.WriteFile(filepath.Join(dir, "find.md"), []byte("# A\n\nfirst\n\n## A two\n\nsecond\n\nfirst FIRST\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "find.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	s.OpenSearch()
	tt.Frame()
	if !s.SearchOpen {
		t.Fatal("the search bar did not open")
	}
	return s, tt, s.ActiveTab()
}

// TestSearchBarOpensAndCloses walks the bar's life: Ctrl+F opens it, the
// query runs, and Escape in the field closes it with the highlights.
func TestSearchBarOpensAndCloses(t *testing.T) {
	s, tt, tab := searchShell(t)

	s.search.Query = "second"
	s.runSearch(tab)
	tt.Frame()
	if len(tab.Model.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(tab.Model.Matches))
	}
	if !tt.HasText("1/1") || !tt.HasText("Previous match") {
		t.Fatalf("bar not rendered: %v", tt.Texts())
	}

	// Escape in the field closes the bar and clears every tab's search:
	// OnTextKey takes the key, so the input's editor never clears it.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if s.SearchOpen {
		t.Fatal("Escape left the bar open")
	}
	if len(tab.Model.Matches) != 0 || tab.Model.Current != -1 || tab.Model.Query() != "" {
		t.Fatalf("Escape kept the search: %+v current %d", tab.Model.Matches, tab.Model.Current)
	}
	if tt.HasText("Previous match") {
		t.Fatalf("bar still rendered: %v", tt.Texts())
	}

	// Ctrl+F reopens it, empty again.
	s.OpenSearch()
	tt.Frame()
	if !s.SearchOpen || s.search.Query != "" {
		t.Fatalf("reopen: open=%v query=%q", s.SearchOpen, s.search.Query)
	}
}

// TestEscapeClosesSearchFromAnywhere presses Escape while the bar shows
// and the focus is not in the field: the window's shortcut closes it as
// the field's own key does. With the settings dialog up, the dialog
// takes Escape first (the overlay on top) and the search stands.
func TestEscapeClosesSearchFromAnywhere(t *testing.T) {
	s, tt, tab := searchShell(t)

	// A press on the document clears the field's focus: nothing around
	// the focus takes Escape, the window's shortcut does.
	if err := tt.Click("second"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.SearchOpen {
		t.Fatal("Escape away from the field left the bar open")
	}
	if len(tab.Model.Matches) != 0 || tab.Model.Query() != "" {
		t.Fatalf("Escape kept the search: %d matches query %q", len(tab.Model.Matches), tab.Model.Query())
	}

	s.OpenSearch()
	s.OpenSettings()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !s.SearchOpen {
		t.Fatal("Escape closed the search while the dialog was up")
	}
	if s.SettingsOpen {
		t.Fatal("Escape left the settings dialog open")
	}
}

// TestSearchQueryRunsSetSearch types into the field and reads the model:
// each change runs the search, Enter moves the current and wraps.
func TestSearchQueryRunsSetSearch(t *testing.T) {
	s, tt, tab := searchShell(t)

	// Typing goes to the field once it has the focus; the input asks
	// for it as the bar opens.
	tt.Type("seco")
	tt.Frame()
	if len(tab.Model.Matches) != 1 {
		t.Fatalf("matches of %q = %d, want 1", s.search.Query, len(tab.Model.Matches))
	}
	if tab.Model.Current != 0 || s.search.Count != "1/1" {
		t.Fatalf("current %d count %q, want 0 and 1/1", tab.Model.Current, s.search.Count)
	}

	tt.Type("nd")
	tt.Frame()
	if len(tab.Model.Matches) != 1 || tab.Model.Query() != "second" {
		t.Fatalf("after typing: %d matches of %q", len(tab.Model.Matches), tab.Model.Query())
	}

	// Enter wraps back to the first match, Shift+Enter forward again.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if tab.Model.Current != 0 || s.search.Count != "1/1" {
		t.Fatalf("after Enter: current %d count %q", tab.Model.Current, s.search.Count)
	}
	tt.TypeKey(ui.Shift, ui.KeyEnter, "")
	tt.Frame()
	if tab.Model.Current != 0 {
		t.Fatalf("Shift+Enter went to %d, want 0", tab.Model.Current)
	}

	// A query without matches empties them.
	s.search.Query = "gone"
	s.runSearch(tab)
	tt.Frame()
	if len(tab.Model.Matches) != 0 || tab.Model.Current != -1 || s.search.Count != "no results" {
		t.Fatalf("no matches: %d current %d count %q", len(tab.Model.Matches), tab.Model.Current, s.search.Count)
	}
}

// TestSearchToggleRegexCase flips the toggles and reads the state: a
// case-sensitive query drops matches, an invalid regex reports "bad
// pattern" instead of a count.
func TestSearchToggleRegexCase(t *testing.T) {
	s, tt, tab := searchShell(t)

	// The on fill is visible: sample inside the button's start padding,
	// clear of the glyph; the off state holds none of it, the on state
	// the selection's pale accent over the bar's surface.
	offFill := func(t *testing.T, name string) {
		t.Helper()
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("%s button missing", name)
		}
		got := pixelAt(tt.Image(), r.X+3, r.Y+r.H/2)
		// A click leaves the pointer parked on the button, so the off fill
		// may be the hover wash; neither is the selection's blue. The
		// bar's translucent panel lands composited, so the channels are
		// what a token match means.
		light := windowLight()
		if !channelClose(got, light.Surface, 1) && !channelClose(got, light.SurfaceHover, 1) {
			t.Fatalf("off %s fill %v, want surface or hover, not the on fill", name, got)
		}
	}
	onFill := func(t *testing.T, name string) {
		t.Helper()
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("%s button missing", name)
		}
		got := pixelAt(tt.Image(), r.X+3, r.Y+r.H/2)
		if channelClose(got, windowLight().Surface, 1) {
			t.Fatalf("on %s fill %v, indistinguishable from the surface", name, got)
		}
		// The on fill is the selection's pale accent laid over the bar's
		// surface: with an accent theme it takes that theme's hue, not
		// always blue, so the expectation rides the live token.
		want := over(windowLight().Selection, windowLight().Surface)
		if !channelClose(got, want, 2) {
			t.Fatalf("on %s fill %v, want the selection's %v over the surface", name, got, want)
		}
	}
	offFill(t, "Match case")
	offFill(t, "Regular expression")

	if err := tt.Click("Match case"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.search.CaseSensitive {
		t.Fatal("the case toggle did not turn on")
	}

	s.search.Query = "FIRST"
	s.runSearch(tab)
	tt.Frame()
	if len(tab.Model.Matches) != 1 {
		t.Fatalf("sensitive FIRST: %d matches, want the one upper case", len(tab.Model.Matches))
	}

	if err := tt.Click("Regular expression"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.search.Regex {
		t.Fatal("the regex toggle did not turn on")
	}

	// An invalid pattern finds nothing and the bar says so.
	s.search.Query = "("
	s.runSearch(tab)
	tt.Frame()
	if len(tab.Model.Matches) != 0 {
		t.Fatalf("bad pattern: %d matches", len(tab.Model.Matches))
	}
	if !tt.HasText("bad pattern") {
		t.Fatalf("count %q, want \"bad pattern\": %v", s.search.Count, tt.Texts())
	}

	// A valid regex counts again: every "first" holds "irs".
	s.search.Query = `f.rst`
	s.runSearch(tab)
	tt.Frame()
	if s.search.Count != "1/2" {
		t.Fatalf("regex count %q, want 1/2", s.search.Count)
	}

	// Both on: the pale accent fill shows, and toggling back off clears
	// it. The hover pass left the pointer on "bad pattern"'s old spot;
	// park it in the field clear of every button.
	in, _ := tt.Find("Find")
	tt.Move(in.X+in.W/2, in.Y+in.H/2)
	tt.Frame()
	onFill(t, "Match case")
	onFill(t, "Regular expression")
	tt.Click("Match case")
	tt.Click("Regular expression")
	tt.Frame()
	offFill(t, "Match case")
	offFill(t, "Regular expression")

	// The toggles reset when the bar closes.
	s.CloseSearch()
	tt.Frame()
	if s.search.CaseSensitive || s.search.Regex || s.search.Query != "" {
		t.Fatalf("close left state: %+v", s.search)
	}
}

// TestSearchFloatDoesNotShiftBody checks the float: the bar's texts sit
// near the body's top right, and the first body line's y is the same with
// the bar open and closed, so nothing pushes the document around.
func TestSearchFloatDoesNotShiftBody(t *testing.T) {
	s, tt, tab := searchShell(t)

	barText, ok := tt.Find("Previous match")
	if !ok {
		t.Fatalf("bar not rendered: %v", tt.Texts())
	}
	body, ok := tt.Find("second")
	if !ok {
		t.Fatalf("body not rendered: %v", tt.Texts())
	}
	if barText.Y > body.Y {
		t.Fatalf("float at y %.1f sits below the body's first line at %.1f", barText.Y, body.Y)
	}
	firstOpen := body.Y

	s.CloseSearch()
	tt.Frame()
	_, closed := tt.Find("second")
	if !closed {
		t.Fatal("body not rendered with the bar closed")
	}
	// Find reports the first match; the closed frame has no count or
	// hint texts in the way, so the same y means the body held still.
	_, openAgain := tt.Find("second")
	if !openAgain {
		t.Fatal("body lost")
	}
	if _, ok := tt.Find("Previous match"); ok {
		t.Fatal("bar still rendered after close")
	}
	// Reopen and compare the body's line: the float must not push it.
	s.OpenSearch()
	tt.Frame()
	again, ok := tt.Find("second")
	if !ok {
		t.Fatal("body not rendered after reopen")
	}
	if again.Y != firstOpen {
		t.Fatalf("body's first line at %.1f with the float open, was %.1f closed", again.Y, firstOpen)
	}
	if tab.Model == nil || !s.SearchOpen {
		t.Fatal("state lost")
	}
}

// TestSearchCountKeepsFloatWidth checks the float's width holds still as
// the count appears, changes and goes: the bar is right-anchored, so a
// count that sized itself to its text dragged the field and every button
// sideways on each search.
func TestSearchCountKeepsFloatWidth(t *testing.T) {
	s, tt, tab := searchShell(t)

	closeR, ok := tt.Find("Close search")
	if !ok {
		t.Fatalf("bar not rendered: %v", tt.Texts())
	}
	prevR, ok := tt.Find("Previous match")
	if !ok {
		t.Fatalf("bar not rendered: %v", tt.Texts())
	}

	// The count states the bar shows, in the order a query walks them.
	states := []struct {
		name  string
		query string
		regex bool
	}{
		{"1/3", "first", false},
		{"no results", "gone", false},
		{"bad pattern", "(", true},
	}
	for _, st := range states {
		s.search.Regex = st.regex
		s.search.Query = st.query
		s.runSearch(tab)
		tt.Frame()
		if s.search.Count != st.name {
			t.Fatalf("query %q: count %q, want %q", st.query, s.search.Count, st.name)
		}
		countR, ok := tt.Find(st.name)
		if !ok {
			t.Fatalf("count %q not rendered: %v", st.name, tt.Texts())
		}
		if countR.W != searchCountWidth {
			t.Fatalf("count %q is %.1f wide, want the fixed %.1f", st.name, countR.W, searchCountWidth)
		}
		cr, _ := tt.Find("Close search")
		pr, _ := tt.Find("Previous match")
		if cr.X != closeR.X || pr.X != prevR.X {
			t.Fatalf("count %q moved the buttons: close %.1f (was %.1f), prev %.1f (was %.1f)",
				st.name, cr.X, closeR.X, pr.X, prevR.X)
		}
	}

	// The longest count the bar can show still fits the fixed width, so
	// it stays whole instead of ending in an ellipsis.
	s.search.Regex = false
	s.search.Count = "12345/10000"
	tt.Frame()
	longR, ok := tt.Find("12345/10000")
	if !ok {
		t.Fatalf("long count not rendered: %v", tt.Texts())
	}
	if longR.W != searchCountWidth {
		t.Fatalf("long count is %.1f wide, want %.1f", longR.W, searchCountWidth)
	}
	if cr, _ := tt.Find("Close search"); cr.X != closeR.X {
		t.Fatalf("long count moved the close button to %.1f, want %.1f", cr.X, closeR.X)
	}

	// Closing the bar drops the count; the empty bar's buttons stay put.
	s.CloseSearch()
	tt.Frame()
	if _, ok := tt.Find("12345/10000"); ok {
		t.Fatal("the count survived the close")
	}
}

// TestSearchPlaceholderShowsWhileEmpty checks the input's placeholder:
// the field's own accessibility info holds "Find" in an empty field, and
// typing fills the value that paints over it. The access tree reports
// the placeholder whether or not the field has text; the paint drops it.
func TestSearchPlaceholderShowsWhileEmpty(t *testing.T) {
	s, tt, _ := searchShell(t)

	field, ok := tt.AccessByRole(ui.RoleTextField, "")
	if !ok {
		t.Fatalf("no text field in the bar: %v", tt.Texts())
	}
	if field.Placeholder != "Find" || field.Value != "" {
		t.Fatalf("empty field: placeholder %q value %q, want \"Find\" and \"\"", field.Placeholder, field.Value)
	}
	tt.Type("x")
	tt.Frame()
	field, ok = tt.AccessByRole(ui.RoleTextField, "")
	if !ok || field.Value != "x" {
		t.Fatalf("typed %q: field %+v ok=%v", s.search.Query, field, ok)
	}
}

// findPoint returns the center of the first element showing want.
func findPoint(tt *ui.Tester, want string) (float32, float32, error) {
	r, ok := tt.Find(want)
	if !ok {
		return 0, 0, fmt.Errorf("no element shows %q", want)
	}
	return r.X + r.W/2, r.Y + r.H/2, nil
}

// TestSearchFollowsTabSwitch walks the bar across tabs: the count shows
// the tab the search ran on, a switch shows the target tab's own search
// (empty after an edit dropped it), and switching back restores it.
func TestSearchFollowsTabSwitch(t *testing.T) {
	s, dir := newShell(t)
	if err := os.WriteFile(filepath.Join(dir, "find.md"), []byte("# A\n\nfirst\n\n## A two\n\nsecond\n\nfirst FIRST\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tt := s.render(t)
	if err := s.OpenFile(filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(filepath.Join(dir, "find.md")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	s.OpenSearch()
	tt.Frame()
	if !s.SearchOpen {
		t.Fatal("the search bar did not open")
	}
	tab := &s.Tabs[1]

	s.search.Query = "first"
	s.runSearch(tab)
	tt.Frame()
	if s.search.Count != "1/3" {
		t.Fatalf("count %q, want 1/3", s.search.Count)
	}

	// Clicking the other tab's strip button switches; the model edit
	// dropped tab A's search, so the bar's field and count empty out
	// with it. The strip's tab buttons carry no label of their own, so
	// the click goes by point, over the target tab's title.
	x, y, err := findPoint(tt, "b.md")
	if err != nil {
		t.Fatal(err)
	}
	tt.ClickAt(x, y)
	tt.Frame()
	b := s.ActiveTab()
	if b.Path == tab.Path {
		t.Fatal("the click did not switch tabs")
	}
	if s.search.Query != "" || s.search.Count != "" {
		t.Fatalf("after switch: query %q count %q, want empty", s.search.Query, s.search.Count)
	}
	if len(tab.Model.Matches) != 3 {
		t.Fatalf("tab A holds %d matches, want its 3 kept for the return", len(tab.Model.Matches))
	}
	if len(b.Model.Matches) != 0 {
		t.Fatalf("tab B holds %d matches without a search", len(b.Model.Matches))
	}

	// A search on B, then back to A: the field restores A's query and
	// the count A's matches, and B keeps its own.
	s.search.Query = "second"
	s.runSearch(b)
	tt.Frame()
	if s.search.Count != "1/1" {
		t.Fatalf("B count %q, want 1/1", s.search.Count)
	}
	x, y, err = findPoint(tt, "find.md")
	if err != nil {
		t.Fatal(err)
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if s.search.Query != "first" {
		t.Fatalf("field %q after switching back, want A's query %q", s.search.Query, "first")
	}
	if s.search.Count != "1/3" {
		t.Fatalf("count %q after switching back, want A's 1/3", s.search.Count)
	}
	if len(b.Model.Matches) != 1 || b.Model.Query() != "second" {
		t.Fatalf("B lost its search: %d matches %q", len(b.Model.Matches), b.Model.Query())
	}

	// With the bar closed a switch touches nothing: B keeps its search.
	s.CloseSearch()
	tt.Frame()
	x, y, err = findPoint(tt, "b.md")
	if err != nil {
		t.Fatal(err)
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if len(s.ActiveTab().Model.Matches) != 0 {
		t.Fatalf("close left matches: %d", len(s.ActiveTab().Model.Matches))
	}
}

// TestFirstHitScrollsIntoView types a fresh query and reads the view's
// scroll target: it lands on the first hit's block, as Next would.
func TestFirstHitScrollsIntoView(t *testing.T) {
	_, tt, tab := searchShell(t)
	tt.Frame()

	m := tab.Model
	// "first" hits every block, the last one last; the query is new, so
	// the view gets the first hit's block to scroll to.
	tt.Type("first")
	tt.Frame()
	if len(m.Matches) < 3 {
		t.Fatalf("matches of first: %d, want at least 3", len(m.Matches))
	}
	firstHit := m.Matches[0].Block
	if got := tab.State.ScrollTargetBlock; got != firstHit {
		t.Fatalf("scroll target %d, want the first hit's block %d", got, firstHit)
	}
	if m.Current != 0 {
		t.Fatalf("current %d, want the first hit", m.Current)
	}

	// Enter moves to the next hit and re-targets the scroll.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if m.Current != 1 {
		t.Fatalf("after Enter: current %d, want 1", m.Current)
	}
	if got := tab.State.ScrollTargetBlock; got != m.Matches[1].Block {
		t.Fatalf("scroll target %d, want the second hit's block %d", got, m.Matches[1].Block)
	}
}

// TestTitleEmittedOnTabChanges checks OnTitleChange fires as the active tab
// opens, switches or closes, so the host window title follows along.
func TestTitleEmittedOnTabChanges(t *testing.T) {
	s, dir := newShell(t)
	var titles []string
	s.OnTitleChange = func(title string) { titles = append(titles, title) }

	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	if err := s.OpenFile(a); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(b); err != nil {
		t.Fatal(err)
	}
	if len(titles) != 2 || titles[len(titles)-1] != b+" - Peekdown" {
		t.Fatalf("after open: %v, want last %q", titles, b+" - Peekdown")
	}

	// Clicking a tab rebinds Active at render time; the title follows on the frame.
	tt := s.render(t)
	tt.Frame()
	if err := tt.Click("a.md"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if titles[len(titles)-1] != a+" - Peekdown" {
		t.Fatalf("after switch: %v, want %q", titles, a+" - Peekdown")
	}

	s.CloseTab(0)
	if titles[len(titles)-1] != b+" - Peekdown" {
		t.Fatalf("after close: %v, want last %q", titles, b+" - Peekdown")
	}

	s.CloseAll()
	if titles[len(titles)-1] != "Peekdown" {
		t.Fatalf("after close all: %v, want last %q", titles, "Peekdown")
	}
}
