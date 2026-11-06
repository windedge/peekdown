// Peekdown is a Markdown viewer, rewritten in Go on mygo's native UI.
package main

import (
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/shell"
)

// app glues the shell to the single window: it keeps the window handle and
// remembers an activation that arrived before the window existed.
type app struct {
	shell           *shell.Shell
	win             *mygo.Window
	activatePending bool
}

// view renders the shell.
func (a *app) view(c *ui.Context) { a.shell.View(c) }

// title is the window title, as the Rust implementation shows it.
func (a *app) title() string { return a.shell.Title() }

// open shows path in the window.
func (a *app) open(path string) {
	if err := a.shell.OpenFile(path); err != nil {
		log.Printf("peekdown: open %s: %v", path, err)
		return
	}
	if a.win != nil {
		a.win.SetTitle(a.title())
		a.win.SetRepresentedFile(path)
	}
}

// adopt takes the window and replays a pending activation.
func (a *app) adopt(w *mygo.Window) {
	a.win = w
	if w == nil {
		return
	}
	w.SetTitle(a.title())
	if t := a.shell.ActiveTab(); t != nil {
		w.SetRepresentedFile(t.Path)
	}
	if a.activatePending {
		a.activatePending = false
		w.Restore()
		w.Focus()
	}
}

// activate brings the window to the front, or remembers to do so once the
// window exists.
func (a *app) activate() {
	if a.win == nil {
		a.activatePending = true
		return
	}
	a.win.Restore()
	a.win.Focus()
}

// themeSource maps the saved theme to mygo's appearance source.
func themeSource(theme string) mygo.ThemeSource {
	switch theme {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

// loadSettings reads the settings from the user data directory.
func loadSettings() settings.Settings {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Printf("peekdown: settings: %v; using defaults", err)
		return settings.Default()
	}
	s, err := settings.Load(filepath.Join(dir, settings.FileName))
	if err != nil {
		log.Printf("peekdown: settings: %v; using defaults", err)
		return settings.Default()
	}
	return s
}

// newShell builds the shell with the platform implementations wired in.
func newShell() *shell.Shell {
	s := shell.New()
	s.Load = loadSettings
	s.Save = func(v settings.Settings) error {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return err
		}
		return settings.Save(filepath.Join(dir, settings.FileName), v)
	}
	s.SetTheme = mygo.Theme.SetSource
	s.Recent = mygo.App.RecentDocuments
	s.AddRecent = mygo.App.AddRecentDocument
	s.ClearRecent = mygo.App.ClearRecentDocuments
	s.OpenDialog = func() ([]string, error) {
		return mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:    "Open Markdown",
			Multiple: true,
			Filters: []mygo.FileFilter{
				{Name: "Markdown", Extensions: []string{"md", "markdown"}},
			},
		})
	}
	return s
}

func main() {
	mygo.App.SetName("Peekdown")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	s := newShell()
	s.Saved = s.Load()
	s.Draft = s.Saved
	s.SetSidebarWidth(float32(s.Saved.SidebarWidth))
	s.Sidebar.Visible = s.Saved.SidebarVisible
	s.SetTheme(themeSource(s.Saved.Theme))

	a := &app{shell: s}
	mygo.App.OnOpenFile(a.open)
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		a.activate()
	})
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:     a.title(),
			Width:     1024,
			Height:    768,
			MinWidth:  480,
			MinHeight: 320,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
		a.adopt(w)
		mygo.App.SetMenu(shell.BuildMenu(s))
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
