package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/shell"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	s := shell.New()
	s.Load = settings.Default
	s.Save = func(settings.Settings) error { return nil }
	s.SetTheme = func(mygo.ThemeSource) {}
	s.Saved = s.Load()
	s.Draft = s.Saved
	return &app{shell: s}
}

func writeDoc(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTitle(t *testing.T) {
	a := newTestApp(t)
	if got := a.title(); got != "Peekdown" {
		t.Fatalf("empty title: %q", got)
	}
	path := writeDoc(t, t.TempDir(), "readme.md", "# Hi\n")
	a.open(path)
	if got, want := a.title(), path+" - Peekdown"; got != want {
		t.Fatalf("title: got %q, want %q", got, want)
	}
}

func TestViewShowsTheOpenFile(t *testing.T) {
	a := newTestApp(t)
	path := writeDoc(t, t.TempDir(), "readme.md", "# Hi\n\nthere\n")
	a.open(path)
	tt := ui.NewTester(a.view, 640, 400)
	if !tt.HasText("readme.md") {
		t.Fatalf("texts %q: want the tab title", tt.Texts())
	}
	if !tt.HasText("Hi") {
		t.Fatalf("texts %q: want the document heading", tt.Texts())
	}
}

func TestOpenMissingFileKeepsTheTabs(t *testing.T) {
	a := newTestApp(t)
	path := writeDoc(t, t.TempDir(), "readme.md", "# Hi\n")
	a.open(path)
	a.open(filepath.Join(t.TempDir(), "missing.md"))
	if got := len(a.shell.Tabs); got != 1 {
		t.Fatalf("tabs = %d, want 1", got)
	}
	if got := a.title(); got != path+" - Peekdown" {
		t.Fatalf("title = %q", got)
	}
}

// TestDropOpenFiltersMarkdown checks the drop entry point: markdown
// opens, other files are skipped, and it is safe before the window is.
func TestDropOpenFiltersMarkdown(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	img := filepath.Join(dir, "pic.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := writeDoc(t, dir, "dropped.md", "# Dropped\n")
	a.dropOpen([]string{img, md})
	if len(a.shell.Tabs) != 1 || a.shell.Tabs[0].Path != md {
		t.Fatalf("drop opened %v, want %q", a.shell.Tabs, md)
	}
	if got := a.title(); got != md+" - Peekdown" {
		t.Fatalf("title = %q", got)
	}
}

func TestActivateBeforeTheWindow(t *testing.T) {
	a := newTestApp(t)
	a.activate()
	if !a.activatePending {
		t.Fatal("activation before the window was forgotten")
	}
	a.adopt(nil)
	if a.win != nil || !a.activatePending {
		t.Fatalf("adopt(nil) changed state: win %v pending %v", a.win, a.activatePending)
	}
}

// TestTitleChangeFollowsTheTabs drives the shell the way main does: one
// app, one shell, the shell's OnTitleChange pointed at the app. Opening
// two files, closing one and closing all must emit the title of the
// active tab each time, which the old double-app wiring dropped.
func TestTitleChangeFollowsTheTabs(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	aPath := writeDoc(t, dir, "a.md", "# A\n")
	bPath := writeDoc(t, dir, "b.md", "# B\n")
	var titles []string
	a.shell.OnTitleChange = func(title string) {
		titles = append(titles, title)
		a.OnTitleChange(title) // win == nil: skipped silently
	}

	if err := a.shell.OpenFile(aPath); err != nil {
		t.Fatal(err)
	}
	if err := a.shell.OpenFile(bPath); err != nil {
		t.Fatal(err)
	}
	if len(titles) != 2 || titles[1] != bPath+" - Peekdown" {
		t.Fatalf("after open: %v, want last %q", titles, bPath+" - Peekdown")
	}
	a.shell.CloseTab(0)
	if len(titles) != 3 || titles[2] != bPath+" - Peekdown" {
		t.Fatalf("after close: %v, want last %q", titles, bPath+" - Peekdown")
	}
	a.shell.CloseAll()
	if len(titles) != 4 || titles[3] != "Peekdown" {
		t.Fatalf("after close all: %v, want last %q", titles, "Peekdown")
	}
}

// TestOpenSecondInstanceOpensMarkdown walks the second instance's args:
// the markdown paths open, the rest are skipped, and the activation is
// remembered while the window is still missing.
func TestOpenSecondInstanceOpensMarkdown(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	md := writeDoc(t, dir, "asked.md", "# Asked\n")
	img := filepath.Join(dir, "pic.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	a.openSecondInstance([]string{md, img})
	if len(a.shell.Tabs) != 1 || a.shell.Tabs[0].Path != md {
		t.Fatalf("second instance opened %v, want only %q", a.shell.Tabs, md)
	}
	if !a.activatePending {
		t.Fatal("the activation was not remembered")
	}

	// A no-args second instance still asks for the activation.
	a.adopt(nil)
	a.openSecondInstance(nil)
	if !a.activatePending {
		t.Fatal("the activation of a no-args second instance was dropped")
	}
}

func TestThemeSource(t *testing.T) {
	cases := map[string]mygo.ThemeSource{
		"":      mygo.ThemeSystem,
		"auto":  mygo.ThemeSystem,
		"light": mygo.ThemeLight,
		"dark":  mygo.ThemeDark,
		"other": mygo.ThemeSystem,
	}
	for in, want := range cases {
		if got := themeSource(in); got != want {
			t.Errorf("themeSource(%q) = %q, want %q", in, got, want)
		}
	}
}
