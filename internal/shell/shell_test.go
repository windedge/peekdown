package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	saved := settings.Default()
	recent := []string{}
	s.Load = func() settings.Settings { return saved }
	s.Save = func(v settings.Settings) error {
		saved = v
		return nil
	}
	s.SetTheme = func(mygo.ThemeSource) {}
	s.Recent = func() []string { return append([]string(nil), recent...) }
	s.AddRecent = func(p string) error {
		recent = append([]string{p}, recent...)
		return nil
	}
	s.ClearRecent = func() error {
		recent = nil
		return nil
	}
	s.OpenDialog = func() ([]string, error) { return []string{a, b}, nil }
	s.Saved = s.Load()
	s.Draft = s.Saved
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
	s.ToggleSidebar()
	tt.Frame()
	if !s.Sidebar.Visible {
		t.Fatal("toggle did not show the sidebar")
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
	s.ToggleSidebar()
	if s.Sidebar.Visible {
		t.Fatal("toggle did not hide the sidebar")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s, _ := newShell(t)
	tt := s.renderSize(t, 800, 1000)
	s.OpenSettings()
	tt.Frame()
	if !tt.HasText("Settings") {
		t.Fatalf("dialog missing: %v", tt.Texts())
	}

	s.Draft.Theme = "dark"
	s.Draft.Layout = "full_width"
	s.Draft.ScrollSpeed = 2
	s.markersText = " go.mod , package.json "
	tt.Frame()

	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("dialog still open after save")
	}
	if s.Saved.Theme != "dark" || s.Saved.Layout != "full_width" || s.Saved.ScrollSpeed != 2 {
		t.Fatalf("saved = %+v", s.Saved)
	}
	want := []string{"go.mod", "package.json"}
	if len(s.Saved.ProjectRootMarkers) != len(want) {
		t.Fatalf("markers = %v, want %v", s.Saved.ProjectRootMarkers, want)
	}
	for i := range want {
		if s.Saved.ProjectRootMarkers[i] != want[i] {
			t.Fatalf("markers = %v, want %v", s.Saved.ProjectRootMarkers, want)
		}
	}

	// Reopening the dialog starts from the saved values again.
	s.OpenSettings()
	if s.Draft.Theme != "dark" || s.Draft.Layout != "full_width" {
		t.Fatalf("draft = %+v", s.Draft)
	}
	if s.draftLayout != 1 {
		t.Fatalf("draftLayout = %d, want 1", s.draftLayout)
	}
	tt.Frame()
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SettingsOpen {
		t.Fatal("dialog still open after cancel")
	}
	if s.Draft.Theme != s.Saved.Theme {
		t.Fatalf("cancel did not restore the draft: %+v", s.Draft)
	}
}

func TestSettingsSaveErrorKeepsDialogOpen(t *testing.T) {
	s, _ := newShell(t)
	s.Save = func(settings.Settings) error { return errors.New("disk full") }
	tt := s.renderSize(t, 800, 1000)
	s.OpenSettings()
	tt.Frame()

	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.SettingsOpen {
		t.Fatal("dialog closed after a failed save")
	}
	if !tt.HasText("disk full") {
		t.Fatalf("texts %q: want the save error", tt.Texts())
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
	for _, label := range []string{"Close sidebar", "Files", "Outline", "Settings"} {
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

func TestZoomClamps(t *testing.T) {
	s, _ := newShell(t)
	for i := 0; i < 40; i++ {
		s.ZoomIn()
	}
	if s.Zoom != 3 {
		t.Fatalf("zoom in = %v, want 3", s.Zoom)
	}
	for i := 0; i < 40; i++ {
		s.ZoomOut()
	}
	if s.Zoom != 0.5 {
		t.Fatalf("zoom out = %v, want 0.5", s.Zoom)
	}
	s.ZoomReset()
	if s.Zoom != 1 {
		t.Fatalf("zoom reset = %v, want 1", s.Zoom)
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

	// The toggles reset when the bar closes.
	s.CloseSearch()
	if s.search.CaseSensitive || s.search.Regex || s.search.Query != "" {
		t.Fatalf("close left state: %+v", s.search)
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
