package shell

import (
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
	if err := s.OpenFile(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if s.Title() != "a.md - Peekdown" {
		t.Fatalf("title = %q", s.Title())
	}
	s.CloseAll()
	if s.Title() != "Peekdown" {
		t.Fatalf("title = %q", s.Title())
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
