package shell

import (
	"os"
	"path/filepath"
	"strings"

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
		dir := filepath.Dir(tab.Path)
		entries, err := os.ReadDir(dir)
		if err != nil {
			ui.Text(c, err.Error()).FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Text(c, filepath.Base(dir)).FontSize(11).TextColor(t.TextMuted)
		for _, e := range entries {
			if e.IsDir() || !isMarkdown(e.Name()) {
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

func isMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}
