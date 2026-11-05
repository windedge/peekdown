// Peekdown is a Markdown viewer, rewritten in Go on mygo's native UI.
package main

import (
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// app is the state the window shows. Files opened before the window exists,
// from the command line for example, wait in pending.
type app struct {
	settings settings.Settings
	active   string
	pending  []string
	win      *mygo.Window
	ready    bool
}

// view builds the window's interface from the app's state.
func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Text(c, "Peekdown").FontSize(28).Bold()
		if a.active == "" {
			ui.Text(c, "Markdown viewer").FontSize(14).TextColor(t.TextMuted)
			return
		}
		ui.Text(c, filepath.Base(a.active)).FontSize(20)
		ui.Text(c, a.active).FontSize(12).TextColor(t.TextMuted)
	})
}

// title is the window title: the file path and the app name, as the Rust
// implementation shows it.
func (a *app) title() string {
	if a.active == "" {
		return "Peekdown"
	}
	return a.active + " - Peekdown"
}

// open shows path in the window, recording it once the app is ready.
func (a *app) open(path string) {
	a.active = path
	if a.ready {
		a.recordRecent(path)
	} else {
		a.pending = append(a.pending, path)
	}
	if a.win != nil {
		a.win.SetTitle(a.title())
	}
}

// recordRecent adds path to the app's and the system's recent documents.
func (a *app) recordRecent(path string) {
	if err := mygo.App.AddRecentDocument(path); err != nil {
		log.Printf("peekdown: recent documents: %v", err)
	}
}

// windowReady adopts the window: it records the files opened before it
// existed, then shows the last of them.
func (a *app) windowReady(w *mygo.Window) {
	a.win, a.ready = w, true
	for _, p := range a.pending {
		a.recordRecent(p)
	}
	a.pending = nil
	w.SetTitle(a.title())
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
		log.Printf("peekdown: settings: %v", err)
		return settings.Default()
	}
	s, err := settings.Load(filepath.Join(dir, settings.FileName))
	if err != nil {
		log.Printf("peekdown: settings: %v", err)
		return settings.Default()
	}
	return s
}

func main() {
	mygo.App.SetName("Peekdown")
	s := loadSettings()
	mygo.Theme.SetSource(themeSource(s.Theme))

	a := &app{settings: s}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnOpenFile(a.open)
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		if a.win != nil {
			a.win.Restore()
			a.win.Focus()
		}
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
		a.windowReady(w)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
