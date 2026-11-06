package shell

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// tabBar renders the scrollable tab strip and keeps the active tab visible.
func (s *Shell) tabBar(c *ui.Context) {
	t := c.Theme()
	parts := ui.TabsBase(c, &s.Active, len(s.Tabs))
	ui.Row(c).Height(32).Padding(0, 4).Background(t.Background).Children(func() {
		ui.ScrollHorizontal(c).Children(func() {
			parts.List.Gap(0).Children(func() {
				// Close requests are collected so the loop keeps iterating over the original slice.
				closeAt := -1
				reveal := false

				// The menu's Chosen fires on a later frame, so actions re-locate the tab by
				// path instead of trusting the index captured now.
				for i := range s.Tabs {
					i := i
					path := s.Tabs[i].Path
					tab := parts.Tab(i)
					if i == s.Active {
						tab.Background(t.Surface).Radius(4)
					}
					tab.Padding(4, 10)
					tab.Children(func() {
						ui.Row(c).Gap(6).Children(func() {
							ui.Text(c, filepath.Base(s.Tabs[i].Path)).FontSize(13)
							iconButton(c, fmt.Sprintf("close-%s", path), closeIcon, "Close "+filepath.Base(path), 18, 10, func() { closeAt = i })
						})
					})
					tab.Tooltip(s.Tabs[i].Path)
					tab.ContextMenu(func(m *ui.Menu) {
						if m.Item("Reveal in Sidebar").Chosen() {
							reveal = true
						}
						if m.Item("Copy File Path").Chosen() {
							c.WriteClipboard(path)
						}
						m.Separator()
						if m.Item("Close Tab").Chosen() {
							if idx := s.tabIndex(path); idx >= 0 {
								s.CloseTab(idx)
							}
						}
						if m.Item("Close Other Tabs").Chosen() {
							if idx := s.tabIndex(path); idx >= 0 {
								s.CloseOthers(idx)
							}
						}
						if m.Item("Close All Tabs").Chosen() {
							s.CloseAll()
						}
					})
				}

				if reveal {
					s.SetSidebarTab(0)
				}
				if closeAt >= 0 {
					s.CloseTab(closeAt)
				}
			})
		})
	})

	if len(s.Tabs) > 0 && (!s.synced || s.activeSeen != s.Active) {
		if s.Active >= 0 && s.Active < len(s.Tabs) {
			parts.Tab(s.Active).ScrollIntoView()
			s.activeSeen = s.Active
			s.synced = true
		}
	}
}
