package shell

import (
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
				closeOthers := -1
				closeAll := false
				reveal := -1

				for i := range s.Tabs {
					i := i
					tab := parts.Tab(i)
					if i == s.Active {
						tab.Background(t.Surface).Radius(4)
					}
					tab.Padding(4, 10)
					tab.Children(func() {
						ui.Row(c).Gap(6).Children(func() {
							ui.Text(c, filepath.Base(s.Tabs[i].Path)).FontSize(13)
							if ui.Button(c, "").Width(16).Height(16).Children(func() {
								ui.Icon(c, closeIcon).Size(10, 10)
							}).Clicked() {
								closeAt = i
							}
						})
					})
					tab.Tooltip(s.Tabs[i].Path)
					tab.ContextMenu(func(m *ui.Menu) {
						if m.Item("Reveal in Sidebar").Chosen() {
							reveal = i
						}
						if m.Item("Copy File Path").Chosen() {
							c.WriteClipboard(s.Tabs[i].Path)
						}
						m.Separator()
						if m.Item("Close Tab").Chosen() {
							closeAt = i
						}
						if m.Item("Close Other Tabs").Chosen() {
							closeOthers = i
						}
						if m.Item("Close All Tabs").Chosen() {
							closeAll = true
						}
					})
				}

				if reveal >= 0 {
					s.SetSidebarTab(0)
				}
				switch {
				case closeAll:
					s.CloseAll()
				case closeOthers >= 0:
					s.CloseOthers(closeOthers)
				case closeAt >= 0:
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
