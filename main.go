// Peekdown is a Markdown viewer, rewritten in Go on mygo's native UI.
package main

import (
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
)

// app is the state the window shows. Its fields are only touched on the main
// thread, as the mygo callbacks that call open, activate and adopt are.
type app struct {
	active          string
	pending         []string // files that arrived before the window existed
	win             *mygo.Window
	ready           bool
	activatePending bool
	addRecent       func(string) error
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
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	a.active = path
	if a.ready {
		a.recordRecent(path)
	} else {
		a.pending = append(a.pending, path)
	}
	if a.win != nil {
		a.win.SetTitle(a.title())
		a.win.SetRepresentedFile(path)
	}
}

// recordRecent adds path to the app's and the system's recent documents.
func (a *app) recordRecent(path string) {
	if a.addRecent == nil {
		return
	}
	if err := a.addRecent(path); err != nil {
		log.Printf("peekdown: recent documents: %v", err)
	}
}

// adopt takes the window: it records the files that arrived before it
// existed, then shows the last of them.
func (a *app) adopt(w *mygo.Window) {
	a.win, a.ready = w, true
	for _, p := range a.pending {
		a.recordRecent(p)
	}
	a.pending = nil
	if w == nil {
		return
	}
	w.SetTitle(a.title())
	if a.active != "" {
		w.SetRepresentedFile(a.active)
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

func main() {
	mygo.App.SetName("Peekdown")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	s := loadSettings()
	mygo.Theme.SetSource(themeSource(s.Theme))

	a := &app{addRecent: mygo.App.AddRecentDocument}
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
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
