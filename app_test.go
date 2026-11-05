package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestTitle(t *testing.T) {
	a := &app{}
	if got := a.title(); got != "Peekdown" {
		t.Fatalf("empty title: %q", got)
	}
	path := filepath.Join("docs", "readme.md")
	a.open(path)
	if got, want := a.title(), path+" - Peekdown"; got != want {
		t.Fatalf("title: got %q, want %q", got, want)
	}
}

func TestViewShowsTheOpenFile(t *testing.T) {
	a := &app{}
	path := filepath.Join("docs", "readme.md")
	a.open(path)
	tt := ui.NewTester(a.view, 640, 400)
	if !tt.HasText(filepath.Base(path)) {
		t.Fatalf("texts %q: want the file name", tt.Texts())
	}
	if !tt.HasText(path) {
		t.Fatalf("texts %q: want the path", tt.Texts())
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
