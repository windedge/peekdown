// Package settings stores the user's preferences as JSON in the app's user
// data directory. It replaces the Rust implementation's config.toml; there
// is no migration.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// FileName is the file the settings live in, inside the user data directory.
const FileName = "settings.json"

// currentVersion is the schema this build writes. A file of a newer version
// is left alone rather than being overwritten with settings this build
// cannot represent.
const currentVersion = 1

// The theme values this build stores, spelled as the settings dialog lists
// them. Files written before the choices went Title Case carry the lowercase
// spellings; Load maps those onto these.
const (
	ThemeAuto  = "Auto"
	ThemeLight = "Light"
	ThemeDark  = "Dark"
)

// Settings are the preferences the settings dialog edits. The JSON keys keep
// the Rust implementation's names where the meaning is the same, so the file
// stays recognizable.
type Settings struct {
	Version int `json:"version"`

	Theme          string  `json:"theme"`  // Auto, Light or Dark
	Layout         string  `json:"layout"` // centered or full_width
	ScrollSpeed    float64 `json:"scroll_speed"`
	FontFamily     string  `json:"font_family"`
	FontSize       float64 `json:"font_size"`
	MonoFontFamily string  `json:"mono_font_family"`
	MonoFontSize   float64 `json:"mono_font_size"`
	ShowFPS        bool    `json:"show_fps"`
	InertiaScroll  bool    `json:"inertia_scroll"`

	ExplorerRootMode   string   `json:"explorer_root_mode"` // project_root (default) or current_dir
	ExplorerSortMode   string   `json:"explorer_sort_mode"` // name_asc, name_desc, time_desc or time_asc
	ProjectRootMarkers []string `json:"project_root_markers"`
	ExpandedDirs       []string `json:"expanded_dirs"`
	AutoRefresh        bool     `json:"auto_refresh"`
	SidebarWidth       float64  `json:"sidebar_width"`
	SidebarTab         string   `json:"sidebar_tab"` // explorer or outline
	SidebarVisible     bool     `json:"sidebar_visible"`
}

// Default returns the settings of a first run.
func Default() Settings {
	return Settings{
		Version:          1,
		Theme:            ThemeAuto,
		Layout:           "centered",
		ScrollSpeed:      1,
		FontSize:         16,
		MonoFontSize:     13,
		InertiaScroll:    true,
		ExplorerRootMode: "project_root",
		ExplorerSortMode: "name_asc",
		ProjectRootMarkers: []string{
			"Cargo.toml",
			"package.json",
			"pyproject.toml",
			"go.mod",
			"pom.xml",
			"build.gradle",
			"build.gradle.kts",
			"CMakeLists.txt",
		},
		AutoRefresh:  true,
		SidebarWidth: 200,
		SidebarTab:   "explorer",
	}
}

// Load reads path, or returns the defaults when it does not exist. Keys this
// version does not know are ignored, as the Rust implementation ignored them
// when it saved.
func Load(path string) (Settings, error) {
	s := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return Default(), err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		bad := path + ".bad"
		_ = os.Rename(path, bad)
		return Default(), fmt.Errorf("%s: %w (the file was moved to %s)", path, err, bad)
	}
	if s.Version > currentVersion {
		return Default(), fmt.Errorf("%s: settings version %d is newer than %d", path, s.Version, currentVersion)
	}
	s.Theme = normalizeTheme(s.Theme)
	return s, nil
}

// normalizeTheme reads an older file's theme as this build spells it. The
// match ignores case, so a file that says "auto" or "AUTO" loads as Auto;
// a value this build does not know stays as it was, and ThemeSource treats
// it as the system appearance.
func normalizeTheme(theme string) string {
	switch strings.ToLower(theme) {
	case "", "auto":
		return ThemeAuto
	case "light":
		return ThemeLight
	case "dark":
		return ThemeDark
	}
	return theme
}

// Save writes s to path, replacing the file atomically so a crash cannot
// leave a half-written settings file.
func Save(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := replace(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// replace renames from to to. Windows refuses to replace a file that
// antivirus software or an indexer has open, so it tries again for a while.
func replace(from, to string) error {
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		if runtime.GOOS != "windows" || (!errors.Is(err, syscall.Errno(32)) && !errors.Is(err, syscall.Errno(5))) {
			return err
		}
		time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
	}
	return err
}
