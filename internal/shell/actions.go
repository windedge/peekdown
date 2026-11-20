package shell

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The View menu's document commands: scrolling, a manual reload and the
// two panel toggles.

// ScrollToStart scrolls the active document to its first block, as Ctrl+Home
// does. No document, nothing to do.
func (s *Shell) ScrollToStart() {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil || tab.Model.Len() == 0 {
		return
	}
	tab.State.List.ScrollTo(0, ui.Start)
}

// ScrollToEnd scrolls the active document to its end, as Ctrl+End does.
// No document, nothing to do.
func (s *Shell) ScrollToEnd() {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil || tab.Model.Len() == 0 {
		return
	}
	tab.State.List.ScrollToEnd()
}

// RefreshActiveTab rereads the active tab's file from disk, as F5 and Ctrl+R
// do. It works with auto refresh off: the watcher may be idle, the reader
// still wants the file as it is now.
func (s *Shell) RefreshActiveTab() {
	s.reloadTab(s.ActiveTab())
}

// ToggleOutline flips the right-hand panel, independently of the files
// pane on the left.
func (s *Shell) ToggleOutline() { s.SetOutlineVisible(!s.Outline.Visible) }

// ToggleFiles flips the left-hand file pane and leaves the outline be.
func (s *Shell) ToggleFiles() { s.SetFilesVisible(!s.Sidebar.Visible) }

// OpenInFileManager reveals path in the OS file manager, selecting the file
// where the platform can. mygo's shell covers the three platforms
// (explorer.exe /select, on Windows, Finder's reveal on macOS, the file
// manager's ShowItems over D-Bus on Linux). ShowItemInFolder, when set,
// replaces it: headless tests have no app to hand it to.
func (s *Shell) OpenInFileManager(path string) {
	if path == "" {
		return
	}
	if s.ShowItemInFolder != nil {
		s.ShowItemInFolder(path)
		return
	}
	mygo.Shell.ShowItemInFolder(path)
}
