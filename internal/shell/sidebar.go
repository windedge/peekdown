package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// sidebar renders the file list or the outline of the active tab.
func (s *Shell) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Row(c).Padding(6, 6).Gap(6).Children(func() {
			ui.Segmented(c, &s.Sidebar.Tab, "Files", "Outline")
			ui.Box(c).Grow(1)
			iconButton(c, "close-sidebar", closeIcon, "Close sidebar", 24, 12, func() { s.Sidebar.Visible = false })
		})
		if s.Sidebar.Tab == 0 {
			s.filesPane(c)
			return
		}
		s.outlinePane(c)
	})
}

func (s *Shell) filesPane(c *ui.Context) {
	t := c.Theme()
	tab := s.ActiveTab()
	ui.Column(c).Fill().Padding(0, 10).Gap(2).Children(func() {
		if tab == nil {
			ui.Text(c, "No file open").FontSize(12).TextColor(t.TextMuted)
			return
		}
		dir := s.explorerRoot(tab.Path)
		entries, err := readSortedEntries(dir, s.Saved.ExplorerSortMode)
		if err != nil {
			ui.Text(c, err.Error()).FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Text(c, filepath.Base(dir)).FontSize(11).TextColor(t.TextMuted)
		for _, e := range entries {
			if e.IsDir() || !IsMarkdown(e.Name()) {
				continue
			}
			name := e.Name()
			path := filepath.Join(dir, name)
			if path == tab.Path {
				ui.Text(c, name).FontSize(11).TextColor(t.TextMuted)
				continue
			}
			if ui.Button(c, name).Width(200).Height(20).Clicked() {
				_ = s.OpenFile(path)
			}
		}
	})
}

// explorerRoot is the directory the file pane shows: the document's own
// directory, or the project root found upward from it when the settings
// ask for one. Without a root it falls back to the document's directory.
func (s *Shell) explorerRoot(path string) string {
	dir := filepath.Dir(path)
	if s.Saved.ExplorerRootMode != "project_root" {
		return dir
	}
	if root := findProjectRoot(dir, s.Saved.ProjectRootMarkers); root != "" {
		return root
	}
	return dir
}

// findProjectRoot walks from dir upward to the first directory holding
// any marker, a plain stat per marker per level, as the Rust version
// does. Returns "" when none matches above dir.
func findProjectRoot(dir string, markers []string) string {
	for {
		for _, m := range markers {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// readSortedEntries lists dir and orders it as the sort setting asks:
// directories first, then by name or modification time. The pane reads
// the directory every frame (a known leftover), so the sort runs there
// too; the lists are small, and entries sort without allocating.
func readSortedEntries(dir, mode string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	byTime := mode == "time_asc" || mode == "time_desc"
	if byTime {
		type stamp struct {
			entry os.DirEntry
			mod   time.Time
		}
		rows := make([]stamp, 0, len(entries))
		for _, e := range entries {
			mod := time.Time{}
			if info, err := e.Info(); err == nil {
				mod = info.ModTime()
			}
			rows = append(rows, stamp{e, mod})
		}
		slices.SortFunc(rows, func(a, b stamp) int {
			if a.entry.IsDir() != b.entry.IsDir() {
				if a.entry.IsDir() {
					return -1
				}
				return 1
			}
			if c := a.mod.Compare(b.mod); c != 0 {
				if mode == "time_desc" {
					return -c
				}
				return c
			}
			return strings.Compare(a.entry.Name(), b.entry.Name())
		})
		out := make([]os.DirEntry, len(rows))
		for i, v := range rows {
			out[i] = v.entry
		}
		return out, nil
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}
			return 1
		}
		c := strings.Compare(a.Name(), b.Name())
		if mode == "name_desc" {
			return -c
		}
		return c
	})
	return entries, nil
}

func (s *Shell) outlinePane(c *ui.Context) {
	t := c.Theme()
	tab := s.ActiveTab()
	ui.Column(c).Fill().Padding(0, 10).Gap(2).Children(func() {
		if tab == nil || tab.Model == nil {
			ui.Text(c, "No headings").FontSize(12).TextColor(t.TextMuted)
			return
		}
		headings := tab.Model.Headings()
		if len(headings) == 0 {
			ui.Text(c, "No headings").FontSize(12).TextColor(t.TextMuted)
			return
		}
		for _, h := range headings {
			indent := float32(h.Level-1) * 12
			ui.Row(c).Padding(0, 0).Children(func() {
				ui.Box(c).Width(indent)
				ui.Text(c, h.Text).FontSize(12)
			})
		}
	})
}

// MarkdownExtensions is the extension set IsMarkdown accepts, for the
// host's open dialog filters to agree with it.
func MarkdownExtensions() []string {
	return []string{"md", "markdown", "mdown", "mkd"}
}

// IsMarkdown reports whether name looks like a Markdown document, by
// its extension: the open dialog, the file pane and the drop filter
// agree on the set.
func IsMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range MarkdownExtensions() {
		if ext == "."+e {
			return true
		}
	}
	return false
}
