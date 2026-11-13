package shell

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The project's pages the Help menu opens: the README carries the
// Keyboard Shortcuts section the second one lands on.
const (
	repoURL      = "https://github.com/windedge/peekdown"
	shortcutsURL = "https://github.com/windedge/peekdown#keyboard-shortcuts"
)

// repaint asks the window for the frame the menu's action changed state
// for; a nil window (a test, a menu of no window) has none to ask.
func repaint(w *mygo.Window) {
	if w != nil {
		w.Invalidate()
	}
}

// MenuSpec is one item of the app menu, free of either menu system: the
// native menubar (BuildMenu) and the title bar's menu button (drawMenu)
// both render these, so the two entries into the same commands cannot
// drift apart. Sep is a line between groups; Sub a submenu, whose own
// items are the same spec.
type MenuSpec struct {
	Label string
	Accel string // "CmdOrCtrl+O"; empty for none
	Sep   bool
	// Run does the command. The native menu hands over the window it was
	// opened from, the drawn one nil: it repaints with its frame's
	// context instead.
	Run func(*mygo.Window)
	Sub []MenuSpec
}

// MenuSpecs is the app menu's one definition: the commands the Rust
// version binds as keys, so they stay discoverable in the menu bar.
//
// The chords live here only: mygo gives each item one accelerator, and a
// duplicate label reads as two actions, so a second chord of an action
// (Ctrl+R, Ctrl+F4, Ctrl++) rides the window shortcuts in View instead
// (RegisterShortcuts).
func MenuSpecs(s *Shell) []MenuSpec {
	closeTab := func(w *mygo.Window) {
		s.CloseTab(s.Active)
		repaint(w)
	}
	refresh := func(w *mygo.Window) {
		s.RefreshActiveTab()
		repaint(w)
	}
	return []MenuSpec{
		{Label: "File", Sub: []MenuSpec{
			{Label: "Open...", Accel: "CmdOrCtrl+O", Run: func(w *mygo.Window) {
				if s.OpenDialog == nil {
					return
				}
				paths, err := s.OpenDialog()
				if err != nil {
					return
				}
				s.OpenPaths(paths)
				repaint(w)
			}},
			{Sep: true},
			{Label: "Close Tab", Accel: "CmdOrCtrl+W", Run: closeTab},
			{Label: "Next Tab", Accel: "CmdOrCtrl+Tab", Run: func(w *mygo.Window) {
				if len(s.Tabs) == 0 {
					return
				}
				s.showTab((s.Active + 1) % len(s.Tabs))
				repaint(w)
			}},
			{Label: "Previous Tab", Accel: "CmdOrCtrl+Shift+Tab", Run: func(w *mygo.Window) {
				if len(s.Tabs) == 0 {
					return
				}
				s.showTab((s.Active - 1 + len(s.Tabs)) % len(s.Tabs))
				repaint(w)
			}},
		}},
		{Label: "Edit", Sub: []MenuSpec{
			// The document's selection is the app's own model, so Copy
			// and Select All live here; the menu accelerator is the only
			// key route that works without focus in a text field.
			{Label: "Copy", Accel: "CmdOrCtrl+C", Run: func(w *mygo.Window) {
				s.CopySelection(nil)
				repaint(w)
			}},
			{Label: "Select All", Accel: "CmdOrCtrl+A", Run: func(w *mygo.Window) {
				s.SelectAll()
				repaint(w)
			}},
		}},
		{Label: "View", Sub: []MenuSpec{
			{Label: "Find", Accel: "CmdOrCtrl+F", Run: func(w *mygo.Window) {
				s.OpenSearch()
				repaint(w)
			}},
			{Label: "Find Next", Accel: "F3", Run: func(w *mygo.Window) {
				s.NextSearchMatch()
				repaint(w)
			}},
			{Label: "Find Previous", Accel: "Shift+F3", Run: func(w *mygo.Window) {
				s.PrevSearchMatch()
				repaint(w)
			}},
			{Sep: true},
			{Label: "Scroll to Top", Accel: "CmdOrCtrl+Home", Run: func(w *mygo.Window) {
				s.ScrollToStart()
				repaint(w)
			}},
			{Label: "Scroll to Bottom", Accel: "CmdOrCtrl+End", Run: func(w *mygo.Window) {
				s.ScrollToEnd()
				repaint(w)
			}},
			{Sep: true},
			{Label: "Toggle Outline", Accel: "CmdOrCtrl+B", Run: func(w *mygo.Window) {
				s.ToggleOutline()
				repaint(w)
			}},
			{Label: "Toggle Files", Accel: "CmdOrCtrl+Shift+E", Run: func(w *mygo.Window) {
				s.ToggleFiles()
				repaint(w)
			}},
			{Sep: true},
			{Label: "Refresh", Accel: "F5", Run: refresh},
			{Sep: true},
			// Ctrl+=, the chord Rust binds; Ctrl++ rides the window
			// shortcuts (it presses Ctrl+Shift+= on most layouts).
			{Label: "Zoom In", Accel: "CmdOrCtrl+=", Run: func(w *mygo.Window) {
				s.ZoomIn()
				repaint(w)
			}},
			{Label: "Zoom Out", Accel: "CmdOrCtrl+-", Run: func(w *mygo.Window) {
				s.ZoomOut()
				repaint(w)
			}},
			// Reset Zoom, not Actual Size: the zoom scales the document's
			// text, and 100 percent is the settings' sizes.
			{Label: "Reset Zoom", Accel: "CmdOrCtrl+0", Run: func(w *mygo.Window) {
				s.ZoomReset()
				repaint(w)
			}},
		}},
		{Label: "Help", Sub: []MenuSpec{
			{Label: "Peekdown on GitHub", Run: func(*mygo.Window) {
				s.OpenLink(repoURL)
			}},
			{Label: "Keyboard Shortcuts", Run: func(*mygo.Window) {
				s.OpenLink(shortcutsURL)
			}},
		}},
		// Settings and Exit are the app's own commands, not a submenu's:
		// they sit where a reader looks for them, under the three menus.
		{Sep: true},
		{Label: "Settings...", Accel: "CmdOrCtrl+,", Run: func(w *mygo.Window) {
			s.OpenSettings()
			repaint(w)
		}},
		{Sep: true},
		// Quitting is closing the window: the app is one window, and the
		// host ends the run loop when its last window goes.
		{Label: "Exit", Accel: "CmdOrCtrl+Q", Run: func(w *mygo.Window) {
			s.CloseWindow()
			repaint(w)
		}},
	}
}

// OpenLink opens a page of the project in the system's browser.
func (s *Shell) OpenLink(url string) {
	if s.OpenURL != nil {
		s.OpenURL(url)
	}
}

// BuildMenu renders the app menu for the native menubar: the chords stay
// the one place a shortcut is registered, which is what makes them work
// on macOS's system menu bar and on the Alt/F10 popup of Windows and
// Linux, where mygo hides the bar of a window without a title bar.
func BuildMenu(s *Shell) *mygo.Menu {
	return mygo.NewMenu(append(appMenuItems(), nativeItems(MenuSpecs(s))...))
}

// appMenuItems is the slot macOS's menu bar puts the application's own
// menu in, ahead of the app's menus: mygo installs the items as given, so
// without it the File menu would sit under the app's name and the bar
// would carry neither Quit nor Settings. Windows and Linux have no such
// slot, and the drawn menu (drawMenu) is the same tree either way.
func appMenuItems() []*mygo.MenuItem {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return []*mygo.MenuItem{{Role: mygo.RoleAppMenu}}
}

// nativeItems renders specs as native menu items.
func nativeItems(specs []MenuSpec) []*mygo.MenuItem {
	items := make([]*mygo.MenuItem, 0, len(specs))
	for _, sp := range specs {
		switch {
		case sp.Sep:
			items = append(items, mygo.Separator())
		case len(sp.Sub) > 0:
			items = append(items, &mygo.MenuItem{Label: sp.Label, Submenu: nativeItems(sp.Sub)})
		default:
			run := sp.Run
			items = append(items, &mygo.MenuItem{Label: sp.Label, Accelerator: sp.Accel,
				Click: func(_ *mygo.MenuItem, w *mygo.Window) {
					if run != nil {
						run(w)
					}
				}})
		}
	}
	return items
}

// drawMenu renders specs into a menu drawn in the window, printing the
// chords the native menu registers: the title bar's menu button is the
// same tree, so a chord shown there is the chord that works.
func drawMenu(c *ui.Context, m *ui.Menu, specs []MenuSpec) {
	for _, sp := range specs {
		if sp.Sep {
			m.Separator()
			continue
		}
		if len(sp.Sub) > 0 {
			sub := sp.Sub
			m.Submenu(sp.Label, func(m *ui.Menu) { drawMenu(c, m, sub) })
			continue
		}
		item := m.Item(sp.Label)
		if mods, key, ok := shortcut(sp.Accel); ok {
			item.Shortcut(mods, key)
		}
		if run := sp.Run; run != nil && item.Chosen() {
			run(nil)
			c.Invalidate()
		}
	}
}

// shortcut turns a native accelerator into the modifiers and key a drawn
// menu item prints: the two menu systems spell the same chord
// differently ("CmdOrCtrl+Shift+E" and Cmd|Shift plus KeyE), and a chord
// only one of them knows would show in the menu bar but not in the title
// bar's menu.
func shortcut(accel string) (ui.Modifiers, ui.Key, bool) {
	if accel == "" {
		return 0, 0, false
	}
	parts := strings.Split(accel, "+")
	var mods ui.Modifiers
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "CmdOrCtrl", "CommandOrControl", "Cmd":
			// Cmd is the platform's own: Command on macOS, Control
			// elsewhere, as CmdOrCtrl reads.
			mods |= ui.Cmd
		case "Ctrl":
			mods |= ui.Ctrl
		case "Alt":
			mods |= ui.Alt
		case "Shift":
			mods |= ui.Shift
		case "Super":
			mods |= ui.Super
		default:
			return 0, 0, false
		}
	}
	key, ok := keyNamed(parts[len(parts)-1])
	if !ok {
		return 0, 0, false
	}
	return mods, key, true
}

// keyNamed maps the key names of accelerators back to their keys.
func keyNamed(name string) (ui.Key, bool) {
	if name == "" {
		return 0, false
	}
	if len(name) == 1 {
		switch c := name[0]; {
		case c >= 'a' && c <= 'z':
			return ui.KeyA + ui.Key(c-'a'), true
		case c >= 'A' && c <= 'Z':
			return ui.KeyA + ui.Key(c-'A'), true
		case c >= '0' && c <= '9':
			return ui.Key0 + ui.Key(c-'0'), true
		}
	}
	if n, ok := strings.CutPrefix(name, "F"); ok {
		if i, err := strconv.Atoi(n); err == nil && i >= 1 && i <= 12 {
			return ui.KeyF1 + ui.Key(i-1), true
		}
	}
	key, ok := keysByName[name]
	return key, ok
}

// keysByName are the named keys accelerators use.
var keysByName = map[string]ui.Key{
	"Enter":     ui.KeyEnter,
	"Escape":    ui.KeyEscape,
	"Backspace": ui.KeyBackspace,
	"Tab":       ui.KeyTab,
	"Space":     ui.KeySpace,
	"Delete":    ui.KeyDelete,
	"Insert":    ui.KeyInsert,
	"Home":      ui.KeyHome,
	"End":       ui.KeyEnd,
	"PageUp":    ui.KeyPageUp,
	"PageDown":  ui.KeyPageDown,
	"Left":      ui.KeyLeft,
	"Right":     ui.KeyRight,
	"Up":        ui.KeyUp,
	"Down":      ui.KeyDown,
	"-":         ui.KeyMinus,
	"=":         ui.KeyEqual,
	",":         ui.KeyComma,
	".":         ui.KeyPeriod,
	"/":         ui.KeySlash,
	";":         ui.KeySemicolon,
	"'":         ui.KeyQuote,
	"[":         ui.KeyBracketLeft,
	"]":         ui.KeyBracketRight,
	"\\":        ui.KeyBackslash,
	"`":         ui.KeyBackquote,
}
