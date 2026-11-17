// Peekdown is a Markdown viewer, rewritten in Go on mygo's native UI.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/shell"
	"github.com/windedge/peekdown/locales"
)

// app glues the shell to the single window: it keeps the window handle and
// remembers an activation that arrived before the window existed.
type app struct {
	shell           *shell.Shell
	win             *mygo.Window
	activatePending bool
}

// view renders the shell.
func (a *app) view(c *ui.Context) {
	// A watched file's changes reload before the frame reads the model:
	// the view never sees a half-swapped document.
	a.shell.ApplyWatchReloads()
	a.shell.View(c)
}

// title is the window title, as the Rust implementation shows it.
func (a *app) title() string { return a.shell.Title() }

// open shows path in the window.
func (a *app) open(path string) {
	if err := a.shell.OpenFile(path); err != nil {
		log.Printf("peekdown: open %s: %v", path, err)
		return
	}
	a.showOpen()
}

// showOpen mirrors the shell's tab state on the window: the title, the
// represented file, a repaint.
func (a *app) showOpen() {
	if a.win == nil {
		return
	}
	a.win.SetTitle(a.title())
	if t := a.shell.ActiveTab(); t != nil {
		a.win.SetRepresentedFile(t.Path)
	}
	a.win.Invalidate()
}

// adopt takes the window and replays a pending activation.
func (a *app) adopt(w *mygo.Window) {
	a.win = w
	if w == nil {
		return
	}
	// The window controls are the system's, drawn in the app's bar: a dark
	// bar under the default light ones would show light glyphs on it.
	a.syncWindowButtons()
	a.showOpen()
	// Files dropped on the window open as tabs, the markdown ones only.
	w.OnFileDrop(func(e *mygo.FileDropEvent) {
		a.dropOpen(e.Paths)
	})
	if a.activatePending {
		a.activatePending = false
		w.Restore()
		w.Focus()
	}
}

// dropOpen opens the markdown paths of a drop and mirrors the new tab
// on the window.
func (a *app) dropOpen(paths []string) {
	a.shell.OpenDrop(paths)
	a.showOpen()
}

// openSecondInstance opens the markdown paths a second instance handed
// over, then brings the window up.
func (a *app) openSecondInstance(args []string) {
	for _, p := range args {
		if shell.IsMarkdown(p) {
			a.open(p)
		}
	}
	a.activate()
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

// OnTitleChange mirrors the shell's title on the window as the tabs
// change; before the window exists there is nothing to mirror.
func (a *app) OnTitleChange(title string) {
	if a.win != nil {
		a.win.SetTitle(title)
		a.win.Invalidate()
	}
}

// syncWindowButtons draws the system's window controls in the bar's
// colors, from the theme the shell holds: the light or dark the bar
// paints, not the system's appearance the window defaults to.
func (a *app) syncWindowButtons() {
	if a.win == nil {
		return
	}
	a.win.SetWindowButtonsTheme(shell.ThemeSource(a.shell.Saved.Theme))
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
func newShell(a *app) *shell.Shell {
	s := shell.New()
	s.Load = loadSettings
	s.Save = func(v settings.Settings) error {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return err
		}
		return settings.Save(filepath.Join(dir, settings.FileName), v)
	}
	s.SetTheme = func(src mygo.ThemeSource) {
		mygo.Theme.SetSource(src)
		// The bar's background follows the app's appearance; the system's
		// controls over it keep their own, so they are told separately.
		a.syncWindowButtons()
	}
	s.Recent = mygo.App.RecentDocuments
	s.AddRecent = mygo.App.AddRecentDocument
	s.ClearRecent = mygo.App.ClearRecentDocuments
	s.OpenDialog = func() ([]string, error) {
		return mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:    locales.DialogOpenMarkdown(),
			Multiple: true,
			Filters:  []mygo.FileFilter{{Name: locales.FilterMarkdown(), Extensions: shell.MarkdownExtensions()}},
		})
	}
	s.OnTitleChange = a.OnTitleChange
	// The Help menu's pages open in the system's browser; quitting closes
	// the window, which ends the run loop, as the system's close button
	// does.
	s.OpenURL = func(url string) { mygo.Shell.OpenExternal(url) }
	s.AppLocale = mygo.App.Locale
	s.SetAppLocale = mygo.App.SetLocale
	s.CloseWindow = func() {
		if a.win != nil {
			a.win.Close()
		}
	}
	// The watcher reloads a tab off the main thread; the window only
	// repaints when told, so the reload must ask for a frame.
	s.OnWatchReload = func() {
		if a.win != nil {
			a.win.Invalidate()
		}
	}
	// A parse failure over an opened file is logged and shown in the
	// window's title area via the title; the empty tab still opens, so
	// the app never loses the window. A toast needs a frame context,
	// which the shell's Content wires per tab. The shell also holds the
	// error for its own Title until the next successful open.
	s.OnOpenError = func(msg string) {
		if a.win != nil {
			a.win.Invalidate()
		}
	}
	return s
}

func main() {
	// A portable install registers itself with --register and exits; it
	// must not reach the single-instance lock or the GUI.
	for _, arg := range os.Args[1:] {
		if arg == "--register" {
			if err := registerFileAssociations(); err != nil {
				fmt.Fprintln(os.Stderr, "peekdown: register:", err)
				os.Exit(1)
			}
			fmt.Println("Registered Peekdown for .md and .markdown files.")
			return
		}
	}

	// The locale file lives in the user data directory; reading it needs
	// the app's name and paths, so this runs before anything asks the
	// locale.
	mygo.App.SetName("Peekdown")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	a := &app{}
	s := newShell(a)
	a.shell = s
	s.RestoreFromSettings(s.Load())

	mygo.App.OnOpenFile(a.open)
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		a.openSecondInstance(args)
	})
	mygo.App.OnLocaleChanged(func(locale string) {
		// The native menu takes its labels at build time and the title bar
		// reads the messages every frame: rebuild the menu, the window
		// repaints on its own.
		mygo.App.SetMenu(shell.BuildMenu(a.shell))
		if a.win != nil {
			a.win.SetTitle(a.title())
		}
	})
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:          a.title(),
			Width:          1024,
			Height:         768,
			MinWidth:       480,
			MinHeight:      320,
			StateKey:       "main",
			TitleBarStyle:  mygo.TitleBarHidden,
			TitleBarHeight: shell.TitleBarHeight,
			Content:        ui.View(a.view),
		})
		a.adopt(w)
		mygo.App.SetMenu(shell.BuildMenu(s))
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
