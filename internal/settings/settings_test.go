package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	s := Default()
	if s.Version != 1 || s.Theme != "auto" || s.Layout != "centered" {
		t.Fatalf("defaults: %+v", s)
	}
	if s.FontSize != 16 || s.MonoFontSize != 13 || s.ScrollSpeed != 1 {
		t.Fatalf("default sizes: %+v", s)
	}
	if !s.InertiaScroll || !s.AutoRefresh || s.SidebarWidth != 200 || s.SidebarTab != "explorer" {
		t.Fatalf("default behavior: %+v", s)
	}
	if len(s.ProjectRootMarkers) == 0 {
		t.Fatal("default project root markers are empty")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("load missing: got %+v, want defaults", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	want := Default()
	want.Theme = "dark"
	want.FontFamily = "Georgia"
	want.FontSize = 18
	want.ExpandedDirs = []string{"/a", "/b"}
	want.SidebarVisible = true
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: got %+v, want %+v", got, want)
	}
}

func TestUnknownKeysIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(`{"theme":"light","something_new":42}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "light" || got.FontSize != 16 || got.Layout != "centered" {
		t.Fatalf("unknown keys: %+v", got)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid JSON: want an error")
	}
}

func TestSaveReplacesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	second := Default()
	second.Theme = "dark"
	if err := Save(path, second); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("save left %d files: %v", len(entries), entries)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "dark" {
		t.Fatalf("theme after the second save: %q", got.Theme)
	}
}
