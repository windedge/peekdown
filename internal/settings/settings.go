// Package settings stores the user's preferences as JSON in the app's user
// data directory. It replaces the Rust implementation's config.toml; there
// is no migration.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the file the settings live in, inside the user data directory.
const FileName = "settings.json"

// Settings are the preferences the settings dialog edits. The JSON keys keep
// the Rust implementation's names where the meaning is the same, so the file
// stays recognizable.
type Settings struct {
	Version int `json:"version"`

	Theme          string  `json:"theme"`  // auto, light or dark
	Layout         string  `json:"layout"` // centered or full_width
	ScrollSpeed    float64 `json:"scroll_speed"`
	FontFamily     string  `json:"font_family"`
	FontSize       float64 `json:"font_size"`
	MonoFontFamily string  `json:"mono_font_family"`
	MonoFontSize   float64 `json:"mono_font_size"`
	ShowFPS        bool    `json:"show_fps"`
	InertiaScroll  bool    `json:"inertia_scroll"`

	ExplorerRootMode   string   `json:"explorer_root_mode"` // current_dir or project_root
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
		Theme:            "auto",
		Layout:           "centered",
		ScrollSpeed:      1,
		FontSize:         16,
		MonoFontSize:     13,
		InertiaScroll:    true,
		ExplorerRootMode: "current_dir",
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
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
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
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
