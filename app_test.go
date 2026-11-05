package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func abs(t *testing.T, path string) string {
	t.Helper()
	got, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTitle(t *testing.T) {
	a := &app{}
	if got := a.title(); got != "Peekdown" {
		t.Fatalf("empty title: %q", got)
	}
	path := filepath.Join("docs", "readme.md")
	a.open(path)
	if got, want := a.title(), abs(t, path)+" - Peekdown"; got != want {
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
	if !tt.HasText(abs(t, path)) {
		t.Fatalf("texts %q: want the path", tt.Texts())
	}
}

func TestPendingFilesRecordedWhenTheWindowArrives(t *testing.T) {
	var got []string
	a := &app{addRecent: func(path string) error {
		got = append(got, path)
		return nil
	}}
	a.open(filepath.Join("docs", "a.md"))
	a.open(filepath.Join("docs", "b.md"))
	if a.ready {
		t.Fatal("ready before the window")
	}
	a.adopt(nil)
	if !a.ready || len(a.pending) != 0 {
		t.Fatalf("after adopt: ready %v, pending %v", a.ready, a.pending)
	}
	want := []string{abs(t, filepath.Join("docs", "a.md")), abs(t, filepath.Join("docs", "b.md"))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
	a.open(filepath.Join("docs", "c.md"))
	if want := []string{want[0], want[1], abs(t, filepath.Join("docs", "c.md"))}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after the window opened: %v, want %v", got, want)
	}
}

func TestActivateBeforeTheWindow(t *testing.T) {
	a := &app{}
	a.activate()
	if !a.activatePending {
		t.Fatal("activation before the window was forgotten")
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
