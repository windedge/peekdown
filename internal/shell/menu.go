package shell

import "github.com/egoist/mygo"

// BuildMenu wires the native application menu to the shell.
func BuildMenu(s *Shell) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "Open...", Accelerator: "CmdOrCtrl+O", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if s.OpenDialog == nil {
					return
				}
				paths, err := s.OpenDialog()
				if err != nil {
					return
				}
				s.OpenPaths(paths)
				if w != nil {
					w.Invalidate()
				}
			}},
			mygo.Separator(),
			{Label: "Close Tab", Accelerator: "CmdOrCtrl+W", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.CloseTab(s.Active)
				if w != nil {
					w.Invalidate()
				}
			}},
			{Label: "Next Tab", Accelerator: "CmdOrCtrl+Tab", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if len(s.Tabs) == 0 {
					return
				}
				s.Active = (s.Active + 1) % len(s.Tabs)
				if w != nil {
					w.Invalidate()
				}
			}},
			{Label: "Previous Tab", Accelerator: "CmdOrCtrl+Shift+Tab", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				if len(s.Tabs) == 0 {
					return
				}
				s.Active = (s.Active - 1 + len(s.Tabs)) % len(s.Tabs)
				if w != nil {
					w.Invalidate()
				}
			}},
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Toggle Sidebar", Accelerator: "CmdOrCtrl+B", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.ToggleSidebar()
				if w != nil {
					w.Invalidate()
				}
			}},
			mygo.Separator(),
			{Label: "Zoom In", Accelerator: "CmdOrCtrl+Plus", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.ZoomIn()
				if w != nil {
					w.Invalidate()
				}
			}},
			{Label: "Zoom Out", Accelerator: "CmdOrCtrl+-", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.ZoomOut()
				if w != nil {
					w.Invalidate()
				}
			}},
			{Label: "Actual Size", Accelerator: "CmdOrCtrl+0", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.ZoomReset()
				if w != nil {
					w.Invalidate()
				}
			}},
		}},
		{Label: "Peekdown", Submenu: []*mygo.MenuItem{
			{Label: "Settings...", Accelerator: "CmdOrCtrl+,", Click: func(_ *mygo.MenuItem, w *mygo.Window) {
				s.OpenSettings()
				if w != nil {
					w.Invalidate()
				}
			}},
		}},
	})
}
