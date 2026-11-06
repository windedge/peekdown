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
