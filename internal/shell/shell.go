package shell

import (
	"context"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/mermaid"
	"github.com/windedge/peekdown/internal/selection"
	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/view"
	"github.com/windedge/peekdown/locales"
)

// Tab is one open document.
type Tab struct {
	Path  string
	Model *doc.Model
	State view.State
}

// Shell is the application shell: tabs, sidebar, settings dialog and menus.
//
// Every platform specific operation is injected as a func so headless tests
// never touch mygo.App / mygo.Dialog / mygo.Theme.
type Shell struct {
	Tabs   []Tab
	Active int
	// Home shows the welcome page while the tabs stay open: the click on
	// the bar's brand returns there, and any tab activation leaves it.
	// The tabs keep their models, so nothing reloads on the way back.
	Home bool
	// ContentHook, when set, runs at the head of Content with the frame's
	// Context: tests read element Bounds out of band with it. Production
	// wiring leaves it nil.
	ContentHook func(*ui.Context)
	// watchCancel stops the auto-refresh watcher, watchDone closes once
	// its watches released, watchKick carries the tabs' paths to the loop
	// after a change and watchHits names the files the watcher saw change
	// for the next frame to reload on the main thread.
	watchCancel context.CancelFunc
	watchDone   chan struct{}
	watchKick   chan []string
	watchHits   chan string
	Sidebar     struct {
		Width   float32
		Visible bool
	}
	Outline struct {
		Width   float32
		Visible bool
	}
	tree         treeState
	SettingsOpen bool
	Saved        settings.Settings
	// settingsSection is the page the two-column dialog shows, and
	// settingsScroll keeps that page's own place in its scroll.
	settingsSection int
	settingsScroll  ui.ScrollState
	// rows counts the rows the settings dialog's card draws, for the
	// hairline between them; persistTimer holds the debounced persist a
	// text-like control armed.
	rows         int
	persistTimer time.Time
	// fontPickOpen shows the content font's popover, fontPickMonoOpen the
	// mono one; fontSearch is the open one's field text. fontList is
	// the virtual list's state across frames.
	fontPickOpen     bool
	fontPickMonoOpen bool
	fontSearch       string
	fontList         ui.ListState
	// paletteOpen shows the theme picker's popover under the bar's
	// palette button; appearancePick is its segmented's local index
	// (0 system, 1 light, 2 dark) across frames.
	paletteOpen    bool
	appearancePick int
	// Zoom is the document text's runtime zoom in percent, 100 drawing at
	// the settings' font sizes. It is session state only: Rust writes its
	// zoom into the config's font sizes, this build keeps settings.json
	// untouched (ticket #25).
	Zoom    int
	saveErr string
	// fps measures the frames the window draws for the status bar's
	// optional readout; it is idle, and pulls no frames, while the switch
	// is off.
	fps fpsMeter
	// widthSaveTimer is when a sidebar drag's width lands in the
	// settings: armed by a divider move, cleared once saved.
	widthSaveTimer time.Time

	// SearchOpen shows the search bar; search is its state.
	SearchOpen bool
	search     searchBar

	// progressBody is the content area's identity: the reading progress
	// bar reads its committed bounds, so the fill spans the whole area
	// under the title bar, across the file pane, the document and the
	// outline, however the splits divide it.
	progressBody ui.Handle
	// documentCard is the reading card's identity: the corner toggles
	// pin themselves to its committed bounds as the splits resize.
	documentCard ui.Handle

	// bgRect is the gradient's root rect, which the divider cover's
	// columns repaint against; gripHover remembers which gutter the
	// pointer sits in between the hotspot's rebuild and the split's
	// DrawOver, which runs after the subtree's own paint.
	bgRect    ui.Rect
	gripHover [2]bool

	// mermaidCache renders the mermaid blocks of every tab: one app-wide
	// LRU, shared across tabs and theme or zoom changes.
	mermaidCache *mermaid.Cache

	// Load reads the user's settings; nil leaves the defaults.
	Load          func() settings.Settings
	Save          func(settings.Settings) error
	SetTheme      func(mygo.ThemeSource)
	Recent        func() []string
	AddRecent     func(string) error
	ClearRecent   func() error
	OpenDialog    func() ([]string, error)
	OnTitleChange func(string)
	// OnWatchReload repaints the window after the watcher reloaded a
	// tab; the host also toasts here. Nil skips it.
	OnWatchReload func()
	// OnOpenError reports a parse failure over an opened file: the host
	// toasts it. Nil logs only.
	OnOpenError func(string)
	// ShowItemInFolder reveals a file in the OS file manager, selecting
	// it; nil uses mygo's system shell. Tests stub it: a headless run has
	// no app to hand the call to.
	ShowItemInFolder func(string)
	// OpenURL opens the Help menu's pages in the browser; nil skips it.
	OpenURL func(string)
	// AppVersion supplies the About page's version; nil uses mygo's build version.
	AppVersion func() string
	// CloseWindow quits the app: the host closes its window, which ends
	// the run loop. Nil skips it.
	CloseWindow func()
	// AppLocale reads the language the app shows, SetAppLocale chooses
	// one ("" follows the system); the host maps both onto mygo.App, so
	// tests and a headless run stay off the app's main loop. Nil reads
	// as the system choice, and choosing does nothing.
	AppLocale    func() string
	SetAppLocale func(string)

	// tabScroll tracks the tab strip's sideways offset: the strip's wheel
	// handler scrolls with it, and the strip's ScrollIntoView glides against
	// the notches it holds.
	tabScroll ui.ScrollState

	// openError is the last failed open's parse error; the window title
	// carries it until the next successful open clears it.
	openError string

	activeSeen int
	synced     bool

	// Frame scratch state for the settings dialog bindings.
	draftLayout   int
	draftExplorer int
	markersText   string
}

// New returns a shell with sane defaults.
func New() *Shell {
	s := &Shell{
		Load:         settings.Default,
		Zoom:         zoomReset,
		mermaidCache: mermaid.NewCache(16),
	}
	s.Active = -1
	s.Sidebar.Width = 200
	s.Sidebar.Visible = true
	s.Outline.Width = outlineWidthDefault
	s.Outline.Visible = true
	return s
}

// OpenFile shows path as a tab: an existing tab over the same file is
// activated and reread if the disk content moved (the watcher may be
// off), otherwise the file parses into a new tab.
func (s *Shell) OpenFile(path string) error {
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	// The open dialog and the tree hand back different cases for the
	// same file; Windows paths differ in case for the same file, so the
	// dedupe folds case. Clean also strips ./ and trailing separators a
	// drag or a forwarded command line may carry.
	if i := s.tabIndex(abs); i >= 0 {
		tab := &s.Tabs[i]
		s.showTab(i)
		// reloadTab compares the disk bytes against the model and keeps
		// the tab untouched when they match.
		s.reloadTab(tab)
		if s.AddRecent != nil {
			_ = s.AddRecent(tab.Path)
		}
		s.openError = ""
		s.emitTitle()
		return nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	// Math extensions stay off (ticket #17): TeX shows as plain text, as
	// the Rust version shows it; LaTeX typesetting is later work.
	// A failed parse still opens a tab, over an empty document with the
	// error toasted: never leave the user without the app.
	m, parseErr := doc.NewWith(string(data), parser.GFM|parser.ExtFootnotes)
	if parseErr != nil {
		log.Printf("peekdown: parse %s: %v", path, parseErr)
		if s.OnOpenError != nil {
			s.OnOpenError(parseErr.Error())
		}
	}
	s.Tabs = append(s.Tabs, Tab{Path: abs, Model: m})
	if parseErr != nil {
		// The error title sticks until the next successful open; a plain
		// emitTitle here would overwrite it at once.
		s.openError = parseErr.Error()
	} else {
		s.openError = ""
	}
	s.showTab(len(s.Tabs) - 1)
	if s.AddRecent != nil {
		_ = s.AddRecent(abs)
	}
	s.syncWatch()
	s.emitTitle()
	return nil
}

// OpenPaths opens every readable path and keeps going past failures.
func (s *Shell) OpenPaths(paths []string) {
	for _, p := range paths {
		_ = s.OpenFile(p)
	}
}

// OpenDrop opens the markdown files of a drop, skipping the rest: a
// window's file drop names directories and other files too.
func (s *Shell) OpenDrop(paths []string) {
	for _, p := range paths {
		if IsMarkdown(p) {
			_ = s.OpenFile(p)
		}
	}
}

// freeHeapThreshold is the closed document size over which closing a tab
// asks the runtime to hand the freed pages to the OS right away. Go's
// scavenger returns them lazily, so without this the working set in the
// task manager stays at the closed document's peak for a while.
const freeHeapThreshold = 1 << 20

// modelBytes estimates the memory a tab's document holds: the source with
// its parse, twice over, the parse caching the text several ways. It gates
// debug.FreeOSMemory, a full GC with a sweep, over small documents.
func modelBytes(t *Tab) int {
	if t == nil || t.Model == nil {
		return 0
	}
	return 2 * len(t.Model.Source())
}

// releaseTabs zeroes the closed tabs the slice holds and, when any held
// a big document, hands the freed heap to the OS once for the whole
// batch: a scavenger run per tab would stutter the frame for seconds.
// Main thread only, so no sweep can race the next operation's
// allocations. It reports whether it ran the scavenger.
func (s *Shell) releaseTabs(tabs ...Tab) bool {
	big := false
	for i := range tabs {
		if modelBytes(&tabs[i]) >= freeHeapThreshold {
			big = true
		}
	}
	clear(tabs)
	if big {
		debug.FreeOSMemory()
	}
	return big
}

// CloseTab closes tab i and picks a sensible neighbour as the new active tab.
func (s *Shell) CloseTab(i int) {
	if i < 0 || i >= len(s.Tabs) {
		return
	}
	closed := s.Tabs[i]
	s.Tabs = append(s.Tabs[:i], s.Tabs[i+1:]...)
	s.clearTabTail()
	s.releaseTabs(closed)
	switch {
	case len(s.Tabs) == 0:
		s.Active = -1
	case s.Active > i:
		s.Active--
	case s.Active == i:
		if i >= len(s.Tabs) {
			s.Active = len(s.Tabs) - 1
		}
	}
	if s.Active >= len(s.Tabs) {
		s.Active = len(s.Tabs) - 1
	}
	s.syncWatch()
	s.emitTitle()
}

// CloseOthers keeps only tab i.
func (s *Shell) CloseOthers(i int) {
	if i < 0 || i >= len(s.Tabs) {
		return
	}
	closed := make([]Tab, 0, len(s.Tabs)-1)
	for k := range s.Tabs {
		if k != i {
			closed = append(closed, s.Tabs[k])
		}
	}
	keep := s.Tabs[i]
	s.Tabs = []Tab{keep}
	s.Active = 0
	s.releaseTabs(closed...)
	s.syncWatch()
	s.emitTitle()
}

// CloseAll removes every tab.
func (s *Shell) CloseAll() {
	closed := s.Tabs
	s.Tabs = nil
	s.Active = -1
	s.releaseTabs(closed...)
	s.syncWatch()
	s.emitTitle()
}

// clearTabTail zeroes the slots of the tabs slice past its length: the
// shift in CloseTab leaves the closed tab in the backing array's tail,
// where the GC keeps tracing it, and the closed document's memory never
// frees (by-stars.md: 56 MB held after the tab closed).
func (s *Shell) clearTabTail() {
	tail := s.Tabs[len(s.Tabs):cap(s.Tabs)]
	clear(tail)
}

// emitTitle reports the window title to the host, once the tab state
// settled. A failed open's error title holds until the next successful
// open clears it.
func (s *Shell) emitTitle() {
	if s.OnTitleChange != nil {
		s.OnTitleChange(s.Title())
	}
}

// ShowHome shows the welcome page and keeps the open tabs.
func (s *Shell) ShowHome() { s.Home = true }

// showTab points the shell at tab i and leaves the welcome page: every
// path that activates a tab goes through it, so none of them forgets.
func (s *Shell) showTab(i int) {
	if i < 0 || i >= len(s.Tabs) {
		return
	}
	s.Active = i
	s.Home = false
}

// ActiveTab returns the active tab or nil.
func (s *Shell) ActiveTab() *Tab {
	if s.Active < 0 || s.Active >= len(s.Tabs) {
		return nil
	}
	return &s.Tabs[s.Active]
}

// Title is the window title. The title of the last failed open shows
// until the next successful open replaces it.
func (s *Shell) Title() string {
	if s.openError != "" {
		return locales.CouldNotReadDocument(s.openError) + " - " + locales.AppTitle()
	}
	if t := s.ActiveTab(); t != nil {
		return t.Path + " - " + locales.AppTitle()
	}
	return locales.AppTitle()
}

// CopySelection writes the active tab's selected text to the clipboard
// and reports whether there was any; the menu's Copy and Ctrl+C run it.
// write is the host's clipboard writer, so tests stub it; nil uses
// mygo's system clipboard, callable from any goroutine.
func (s *Shell) CopySelection(write func(string)) bool {
	tab := s.ActiveTab()
	if tab == nil || !tab.State.HasSelection() {
		return false
	}
	text := tab.State.SelectionText(tab.Model)
	if text == "" {
		return false
	}
	if write == nil {
		mygo.Clipboard.WriteText(text)
	} else {
		write(text)
	}
	return true
}

// SelectAll selects the active tab's whole document, as Ctrl+A does:
// the selection model's All, from the first run's start to the last's
// end. It reports whether a document was there to select.
func (s *Shell) SelectAll() bool {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil {
		return false
	}
	sel := selection.All(tab.Model)
	tab.State.SetSelection(sel.Anchor, sel.Caret)
	return true
}

// tabIndex returns the index of the first tab with path.
func (s *Shell) tabIndex(path string) int {
	for i := range s.Tabs {
		if sameFile(s.Tabs[i].Path, path) {
			return i
		}
	}
	return -1
}

// sameFile reports whether an existing tab already shows path's file.
// Windows names one file under any letter case, and the tree, the
// dialog and a forwarded command line need not agree with the case the
// first open recorded, so the match folds there. macOS's default APFS
// and every Linux filesystem are case sensitive, where a.md and A.md
// are two files; folding there would merge them. Clean strips the ./
// and trailing separators a drag may carry.
func sameFile(tabPath, path string) bool {
	tabPath, path = filepath.Clean(tabPath), filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(tabPath, path)
	}
	return tabPath == path
}

// SetFilesVisible makes sure the file pane is on screen, for the bar
// button and the reveal commands.
func (s *Shell) SetFilesVisible(v bool) { s.Sidebar.Visible = v }

// SetOutlineVisible shows or hides the right-hand outline panel.
func (s *Shell) SetOutlineVisible(v bool) { s.Outline.Visible = v }

// SetSidebarWidth clamps the sidebar width to a usable range.
func (s *Shell) SetSidebarWidth(w float32) {
	const minW, maxW float32 = 120, 400
	if w < minW {
		w = minW
	}
	if w > maxW {
		w = maxW
	}
	s.Sidebar.Width = w
}

// The outline's width, its first-run value and its drag limits.
const (
	outlineWidthDefault = float32(232)
	outlineWidthMin     = float32(160)
	outlineWidthMax     = float32(360)
)

// SetOutlineWidth clamps the outline width to a usable range.
func (s *Shell) SetOutlineWidth(w float32) {
	s.Outline.Width = min(max(w, outlineWidthMin), outlineWidthMax)
}

// widthSaveDelay waits out a drag: the divider moves every frame while
// held, and only the width it stops on belongs in settings.json, the
// way mygo's own window state settles before it writes.
const widthSaveDelay = 300 * time.Millisecond

// outlineMinWindowWidth is the window width below which the outline
// gives its room back to the document, without changing what was saved.
const outlineMinWindowWidth = 1100

// persistPaneWidths records both dividers through the settings, straight
// into Saved and disk like the tree's expanded dirs: the widths are
// instant state, not settings-dialog fields.
func (s *Shell) persistPaneWidths() {
	s.Saved.SidebarWidth = float64(s.Sidebar.Width)
	s.Saved.OutlineWidth = float64(s.Outline.Width)
	if s.Save != nil {
		_ = s.Save(s.Saved)
	}
}

// settleWidthSave debounces either divider's drag into one settings
// write. It runs once per frame, after both splits were built.
func (s *Shell) settleWidthSave(c *ui.Context, changed bool) {
	if changed {
		s.widthSaveTimer = c.Now().Add(widthSaveDelay)
	}
	if s.widthSaveTimer.IsZero() {
		return
	}
	if now := c.Now(); now.Before(s.widthSaveTimer) {
		c.After(s.widthSaveTimer.Sub(now))
		return
	}
	s.widthSaveTimer = time.Time{}
	s.persistPaneWidths()
}

// RestoreFromSettings puts the saved settings on the shell at startup:
// the panes' widths and visibility, and the theme. It does not persist;
// the host mirrors the theme on the window.
func (s *Shell) RestoreFromSettings(t settings.Settings) {
	s.Saved = t
	s.SetSidebarWidth(float32(t.SidebarWidth))
	s.SetOutlineWidth(float32(t.OutlineWidth))
	s.Sidebar.Visible = t.SidebarVisible
	s.Outline.Visible = t.OutlineVisible
	s.applyAutoRefresh()
	if s.SetTheme != nil {
		s.SetTheme(ThemeSource(t.Theme))
	}
}

// The zoom's limits and step, in percent. Rust's Ctrl+= adds 2px to the
// persisted font_size and Ctrl+0 puts the sizes back (config.rs's
// zoom_font_size / reset_font_size, workspace/mod.rs's handlers); a 2px
// step on the 16px base is 12.5 percent. This build keeps a runtime
// multiplier instead, so settings.json stays untouched and the status
// bar can read whole percents: 5 percent steps span 50 to 300 percent,
// which on the default 16px base is the 8 to 48px Rust clamps to.
const (
	zoomReset = 100
	zoomStep  = 5
	zoomMin   = 50
	zoomMax   = 300
)

// defaultFontSize and defaultMonoFontSize back the zoom when the saved
// settings name no size: the same values a first run loads.
const (
	defaultFontSize     = 16
	defaultMonoFontSize = 13
)

// ZoomIn increases the document text's zoom one step.
func (s *Shell) ZoomIn() { s.setZoom(s.zoomPercent() + zoomStep) }

// ZoomOut decreases the document text's zoom one step.
func (s *Shell) ZoomOut() { s.setZoom(s.zoomPercent() - zoomStep) }

// ZoomReset restores 100 percent: the settings' font sizes.
func (s *Shell) ZoomReset() { s.setZoom(zoomReset) }

// setZoom clamps the zoom to its range and stores it.
func (s *Shell) setZoom(percent int) {
	s.Zoom = min(max(percent, zoomMin), zoomMax)
}

// zoomPercent is Zoom with an unset zero standing for 100 percent, so a
// Shell built by hand still draws at the settings' sizes.
func (s *Shell) zoomPercent() int {
	if s.Zoom <= 0 {
		return zoomReset
	}
	return s.Zoom
}

// ZoomLabel is the status bar's readout of the zoom, as in "125%".
func (s *Shell) ZoomLabel() string { return strconv.Itoa(s.zoomPercent()) + "%" }

// baseFonts returns the settings' font sizes, the base the zoom scales: a
// zero setting means the same defaults a first run loads.
func (s *Shell) baseFonts() (body, mono float32) {
	body = float32(s.Saved.FontSize)
	if body <= 0 {
		body = defaultFontSize
	}
	mono = float32(s.Saved.MonoFontSize)
	if mono <= 0 {
		mono = defaultMonoFontSize
	}
	return body, mono
}

// zoomFactor is the zoom as a multiplier: 1 at 100 percent.
func (s *Shell) zoomFactor() float64 { return float64(s.zoomPercent()) / 100 }

// fontSizes returns the body and mono sizes the document draws at: the
// settings' sizes scaled by the zoom. Headings, tables and the code's
// panel ride the two through the view's own ratios (ticket #19).
func (s *Shell) fontSizes() (body, mono float32) {
	body, mono = s.baseFonts()
	f := float32(s.zoomFactor())
	return body * f, mono * f
}

// Apply persists t and syncs the host theme.
func (s *Shell) Apply(t settings.Settings) error {
	s.Saved = t
	// The root mode, the markers and the sort mode all decide what the
	// tree lists: drop the listings so the next frame reads them again.
	s.invalidateTree()
	if t.SidebarWidth > 0 {
		s.SetSidebarWidth(float32(t.SidebarWidth))
	}
	s.applyAutoRefresh()
	s.SetOutlineWidth(float32(t.OutlineWidth))
	if s.SetTheme != nil {
		s.SetTheme(ThemeSource(t.Theme))
	}
	s.markersText = strings.Join(t.ProjectRootMarkers, ", ")
	if s.Save != nil {
		return s.Save(t)
	}
	return nil
}

// OpenSettings opens the dialog: the controls read Saved, and the frame
// bindings (segmented indexes, the markers text) start from it.
func (s *Shell) OpenSettings() {
	s.draftLayout = layoutIndex(s.Saved.Layout)
	s.draftExplorer = explorerIndex(s.Saved.ExplorerRootMode)
	s.markersText = strings.Join(s.Saved.ProjectRootMarkers, ", ")
	s.settingsScroll.Y = 0
	s.paletteOpen = false
	s.SettingsOpen = true
}

// CloseSettings dismisses the dialog and drops its transient picker state.
// Every setting was already applied when its control changed.
func (s *Shell) CloseSettings() {
	s.fontPickOpen, s.fontPickMonoOpen = false, false
	s.SettingsOpen = false
	s.saveErr = ""
}

// RegisterShortcuts binds the chords the menu bar cannot carry to the
// window's shortcuts: mygo gives a menu item one accelerator, and a
// second item would print the same label twice. Ctrl+R reloads as the
// browser does, Ctrl+F4 is the window manager's close chord, and Ctrl++
// (Ctrl+Shift+=) is zoom in's second key, as Ctrl+Plus. Escape closes
// the search bar wherever the focus is; a key the focused field or an
// overlay (menu, dialog) takes never reaches the window, so it cannot
// fight the dialog's own Escape either.
func (s *Shell) RegisterShortcuts(c *ui.Context) {
	c.OnWindowShortcut(ui.Ctrl, ui.KeyR, func() {
		s.RefreshActiveTab()
		c.Invalidate()
	})
	c.OnWindowShortcut(0, ui.KeyEscape, func() {
		if s.SearchOpen && !s.SettingsOpen {
			s.CloseSearch()
			c.Invalidate()
		}
	})
	c.OnWindowShortcut(ui.Ctrl, ui.KeyF4, func() {
		s.CloseTab(s.Active)
		c.Invalidate()
	})
	c.OnWindowShortcut(ui.Ctrl|ui.Shift, ui.KeyEqual, func() {
		s.ZoomIn()
		c.Invalidate()
	})
}

// View renders the whole shell.
func (s *Shell) View(c *ui.Context) {
	s.trackFrames(c)
	s.settlePersist(c)
	s.RegisterShortcuts(c)
	// The theme with the accent tokens pressed on it goes to the frame
	// first: the bars and the welcome page below read it back through
	// c.Theme(). The window's own background is painted, not filled, so
	// the root keeps its (now matched) color under the gradient.
	k := s.accentColors(c)
	c.SetTheme(s.applyAccentTheme(c, k))
	ui.Column(c).Fill().
		Draw(func(p *ui.Painter, r ui.Rect) {
			s.bgRect = r
			paintAccentBackground(p, r, k)
		}).
		Children(func() {
			// The bar is the window's title bar, so it draws on the welcome
			// page too: without it a window with a hidden title bar has no
			// place to drag from and no room for the window controls.
			s.titleBar(c)
			// The welcome page backs both the empty window and Home, so the two
			// paths render the same view; the status bar stays off it.
			if s.Home || len(s.Tabs) == 0 {
				s.welcome(c)
				return
			}
			// The search bar floats over the body's top right inside the clipped
			// body container, so opening it never pushes the document around.
			// The body's root answers LeftToRight: the RTL split below
			// mirrors what it holds, and only the pane order, not the
			// document's padding sides or scrollbar edge, may follow it.
			body := func() {
				ui.Box(c).Fill().Direction(ui.LeftToRight).Children(func() {
					ui.Box(c).Fill().Label("Body").Children(func() {
						s.content(c)
					})
					if s.SearchOpen {
						s.searchView(c)
					}
					s.jumpButtons(c)
				})
			}
			// The body grows into the room the bars leave instead of asking
			// for the whole column: a Fill here is a hundred percent of the
			// window, which with a long document's natural height overflowing
			// its floor carries the status bar off the bottom edge. Grow takes
			// the free space and drops the content-based minimum (mygo's
			// flexLayout leaves a growing item's minimum at zero), so the
			// document's list meets a bounded box and scrolls inside it.
			// The left split is outermost: its divider runs the full body
			// height, while the right one only shares the reading column.
			winW, _ := c.Size()
			var widthChanged bool
			withOutline := func() {
				if !s.Outline.Visible || winW < outlineMinWindowWidth {
					ui.Box(c).Grow(1).Clip().Children(func() { body() })
					return
				}
				s.SetOutlineWidth(s.Outline.Width)
				// The outline is the split's first pane, so *size widths it;
				// Direction docks that pane at the window's right edge. It
				// must go in the build chain, before the panes build: set on
				// a built node it would swap logical edges under an already
				// committed layout (direction.go's setDir).
				//
				// The split flips the drag delta only when it reads RTL from
				// the window around it (split.go's handle.rtl()); under an
				// LTR window its own Direction(RightToLeft) does not reach
				// the flip, so a left drag narrows the first pane. Route
				// *size through a mirror that undoes each move around the
				// width the frame began at, and write the mirrored value
				// back so the handle's overshoot resumes from where the
				// divider really stands; keyboard moves on a focused divider
				// ride the same path.
				w := s.Outline.Width
				split := ui.Split(c, &w,
					func() { s.outlinePanel(c) }, body).Direction(ui.RightToLeft).Grow(1)
				if split.Changed() {
					before := s.Outline.Width
					moved := 2*before - w
					s.SetOutlineWidth(moved)
					w = 2*s.Outline.Width - moved
					widthChanged = s.Outline.Width != before
				}
			}
			// The progress bar floats over the content area itself, outside
			// the panes: a window-level reading under the title bar, so the
			// fill spans every pane's width instead of one pane's. The box
			// wraps the whole area the panes divide, and binds the handle the
			// fill measures against.
			area := func() {
				ui.Box(c).Grow(1).Clip().Label("Content area").Bind(&s.progressBody).Children(func() {
					widthChanged = s.paneGripHotspots(c, winW) || widthChanged
					if s.Sidebar.Visible {
						s.SetSidebarWidth(s.Sidebar.Width)
						// One DrawOver per element: a second call would replace
						// the first, so this pass draws both gutters, the left
						// boundary and (while the outline shows) the right one.
						split := ui.Split(c, &s.Sidebar.Width, func() { s.sidebar(c) }, withOutline).Grow(1).
							DrawOver(func(p *ui.Painter, r ui.Rect) {
								s.paneGrip(c, p, r, 0)
								if s.Outline.Visible && winW >= outlineMinWindowWidth {
									s.paneGrip(c, p, r, 1)
								}
							})
						widthChanged = split.Changed() || widthChanged
					} else {
						ui.Box(c).Grow(1).Clip().Children(func() { withOutline() })
					}
					s.readingProgress(c)
					s.paneEdgeToggles(c)
				})
			}
			area()
			s.settleWidthSave(c, widthChanged)
			s.statusBar(c)
		})
	if s.SettingsOpen {
		s.settingsDialog(c)
	}
}

// accentCodeTheme colors code blocks by the accent theme: mygo's
// GitHub default for the frame's appearance, with the theme's six
// tokens over it. mojian keeps petal's DefaultID editor tokens whole,
// so petal's document stays pixel-equal; every other theme recolors
// the tokens alone.
func (s *Shell) accentCodeTheme(c *ui.Context) *editor.Theme {
	th := editor.DefaultTheme(c.Theme())
	k := s.accentColors(c)
	tok := func(kind editor.TokenKind, i int) { th.Tokens[kind].Color = k.Syntax[i] }
	tok(editor.TokenKeyword, 0)
	tok(editor.TokenType, 1)
	tok(editor.TokenFunction, 2)
	tok(editor.TokenNumber, 3)
	tok(editor.TokenString, 4)
	tok(editor.TokenComment, 5)
	return th
}

// accentTheme is the saved accent theme, resolved once. Empty and
// unknown ids fall back to the default paper.
func (s *Shell) accentTheme() AccentTheme {
	return accentThemeByID(s.Saved.AccentTheme)
}

// accentColors is the accent theme's colors for the frame's appearance:
// the dark side of the table in a dark window, the light one otherwise.
func (s *Shell) accentColors(c *ui.Context) AccentColors {
	return s.accentTheme().of(c.Theme().Dark)
}

// applyAccentTheme gives the frame the widget theme with the accent
// theme's tokens pressed onto a copy of it, as mojian's windowTheme
// builds on mygo's theme: the frame's light or dark still rules, only
// the hue tokens change. It returns the theme the frame draws with.
func (s *Shell) applyAccentTheme(c *ui.Context, k AccentColors) *ui.Theme {
	return applyAccentTheme(c.Theme(), k)
}

// Content renders the active document.
func (s *Shell) Content(c *ui.Context) {
	if s.ContentHook != nil {
		s.ContentHook(c)
	}
	t := c.Theme()
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, locales.NoDocumentOpen()).TextColor(t.TextMuted)
		})
		return
	}
	// The images resolve against the document's directory, and a tab
	// opened elsewhere drops its loader here, so every frame tops it up.
	if tab.State.LoadImage == nil {
		tab.State.LoadImage = markdown.FileImages(filepath.Dir(tab.Path))
	}
	// The card's copy button writes the clipboard and tells the user;
	// set each frame so a tab opened elsewhere finds it in place.
	tab.State.CopyFrontmatter = func(raw string) {
		c.WriteClipboard(raw)
		c.Toast(locales.CopiedFrontmatter())
	}
	// A code block's copy button, the same clipboard and a toast of its own.
	tab.State.CopyText = func(text string) {
		c.WriteClipboard(text)
		c.Toast(locales.CopiedCode())
	}
	// The code highlight follows the accent theme: its six tokens come
	// from the same table as the window's colors, petal keeping the GitHub
	// palette from mojian's DefaultID special case.
	tab.State.CodeTheme = s.accentCodeTheme(c)
	// The document area's context menu: the commands act on the open
	// document, whichever row the click landed on. Rebound every frame like
	// the loaders, so the closures carry this frame's context.
	tab.State.Menu = &view.MenuActions{
		HasSelection: tab.State.HasSelection,
		Copy:         func() { s.CopySelection(c.WriteClipboard) },
		SelectAll:    func() { s.SelectAll() },
		Find:         func() { s.OpenSearch() },
		Refresh:      func() { s.RefreshActiveTab() },
		Reveal:       func() { s.RevealFile(tab.Path) },
		OpenInFileManager: func() {
			s.OpenInFileManager(tab.Path)
		},
		CopyPath: func() {
			c.WriteClipboard(tab.Path)
			c.Toast(locales.CopiedPath())
		},
	}
	// The diagrams render through the shared cache, keyed by source,
	// theme and scale: a theme switch or a zoom past 2x re-renders once
	// and every tab shares the result.
	tab.State.LoadMermaid = func(src string, theme mermaid.Theme, scale float64) (image.Image, error) {
		return s.mermaidCache.Render(src, theme, scale)
	}
	// Links open in the system's browser; a failure toasts. Set each
	// frame like the loaders, so a tab opened elsewhere finds it too.
	// LinksWired marks the tab as shell-drawn: the outline reads it to
	// gate the scroll-follow highlight on a view that is really wired,
	// not on OpenURL happening to be set.
	tab.State.OpenURL = func(url string) {
		if err := mygo.Shell.OpenExternal(url); err != nil {
			c.Toast(locales.CouldNotOpenLink())
		}
	}
	// A link that names a local file opens here, as a tab of its own;
	// the view hands the raw destination over, anchor tail included.
	tab.State.OpenLocalLink = func(dest string) {
		s.openLocalLink(tab.Path, dest)
	}
	tab.State.LinksWired = true
	// The fonts and families ride the saved settings, topped up every
	// frame like the loaders: a settings save lands on the next frame
	// without further wiring, and a tab opened elsewhere finds them too.
	// The zoom scales the settings' sizes rather than the whole window:
	// a listener holding a small window at 125% gets a bigger page, not a
	// bigger toolbar (ticket #25).
	tab.State.FontSize, tab.State.MonoFontSize = s.fontSizes()
	tab.State.Zoom = s.zoomFactor()
	tab.State.FontFamily = s.Saved.FontFamily
	tab.State.MonoFontFamily = s.Saved.MonoFontFamily
	// The wheel rides the saved scroll settings, topped up every frame
	// like the loaders: a save lands on the next frame without further
	// wiring, and a tab opened elsewhere finds them too. The multiplier
	// and the inertia switch live in the view's own wheel handler
	// (Handler's InputScroll branch); the touchpad keeps the engine's
	// unmultiplied drag.
	tab.State.ScrollSpeed = float32(s.Saved.ScrollSpeed)
	tab.State.NoInertia = !s.Saved.InertiaScroll
	// The document cards the gradient: an 8 DIP gap around it lets the
	// theme show through, while the inner padding keeps the old reading
	// margins. The list reaches the card's right edge, so its overlay
	// scrollbar hugs that edge instead of the window's.
	k := s.accentColors(c)
	ui.Box(c).Fill().Padding(8).Children(func() {
		ui.Box(c).Fill().Background(k.CardFocused).Label("Document card").
			Bind(&s.documentCard).
			Border(1, cardRim(c.Theme(), true)).Radius(10).Clip().
			Shadow(0, 1, 2, 0, cardShadow(c.Theme(), true)).
			Shadow(0, 6, 22, -2, cardShadow(c.Theme(), true)).
			Padding(8, 0, 8, 16).Children(func() {
			// Mirrors the Rust reader: centered caps the text column at 900
			// DIPs, full_width lets it follow the window. The column itself
			// spans the card either way, so the bar keeps hugging the card's
			// edge; the view caps and centers the lines inside it.
			if s.Saved.Layout == "full_width" {
				tab.State.ContentWidth = 0
			} else {
				tab.State.ContentWidth = centeredWidth
			}
			ui.Column(c).Grow(1).AlignSelf(ui.Stretch).Children(func() {
				view.Frontmatter(c, tab.Model, &tab.State)
				view.ScrollView(c, tab.Model, &tab.State)
			})
		})
	})
}

func (s *Shell) content(c *ui.Context) { s.Content(c) }

// readingProgress draws the 2px reading progress bar across the content
// area's top edge, under the title bar and above every pane: the fill
// spans the fraction the active tab's view is scrolled through, in the
// accent color, over the file pane, the document and the outline. The
// area's bounds set the width, so the fill spans the whole reading area
// however the splits divide it. A document with no scroll room draws
// none, as the Rust version's max_scroll test does.
func (s *Shell) readingProgress(c *ui.Context) {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil || tab.State.Progress <= 0 {
		return
	}
	w := s.progressBody.Bounds(c).W
	if w <= 1 {
		return
	}
	ui.Box(c).Absolute().Top(0).Left(0).Height(2).
		Width(max(0, min(tab.State.Progress, 1)) * w).
		Background(c.Theme().Accent).ZIndex(5).Label("Reading progress")
}

// jumpButtons are the scroll-to-top and scroll-to-bottom floating
// buttons over the body's bottom right corner, above the status bar:
// the button whose end the view already stands at hides, as the Rust
// version's is_at_top / is_at_bottom do. They float over the document,
// clear of the search bar's top right corner.
func (s *Shell) jumpButtons(c *ui.Context) {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil {
		return
	}
	st := &tab.State
	if st.AtTop && st.AtBottom {
		return
	}
	col := ui.Column(c).Absolute().Bottom(24).Right(24).Gap(8).ZIndex(10)
	col.Children(func() {
		if !st.AtTop {
			iconButton(c, "jump-top", chevronUpIcon, locales.JumpTop(), 24, 14, s.ScrollToStart)
		}
		if !st.AtBottom {
			iconButton(c, "jump-bottom", chevronDownIcon, locales.JumpBottom(), 24, 14, s.ScrollToEnd)
		}
	})
}

// openLocalLink resolves dest, as a document's link wrote it, against
// base's directory and shows the target as a tab: an absolute path
// stands as it is, anything else joins the document's directory. Only
// markdown opens - directories and other files silently do nothing,
// as do unreadable ones - so an image link never yanks the reader out
// of the document. A "#anchor" tail scrolls the target file to that
// heading once its model is ready; a missing anchor lands on top.
func (s *Shell) openLocalLink(base, dest string) {
	path, anchor, _ := strings.Cut(dest, "#")
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(base), path)
	}
	path = filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(path), ".md") {
		return
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return
	}
	abs := path
	if a, err := filepath.Abs(abs); err == nil {
		abs = a
	}
	if err := s.OpenFile(abs); err != nil {
		return
	}
	if anchor == "" {
		return
	}
	// The reopened tab may be an old one whose model already holds the
	// heading; the fresh one gets its model this call. Resolve at once
	// when possible, else leave the slug pending: the view retries it
	// per frame and drops it if no heading shows up.
	tab := s.ActiveTab()
	if tab == nil {
		return
	}
	if block, ok := tab.Model.AnchorBySlug(anchor); ok {
		tab.State.JumpToBlock(block)
		return
	}
	tab.State.ResetPendingAnchor(anchor)
}

func (s *Shell) welcome(c *ui.Context) {
	t := c.Theme()
	// The greeting, the button and the list's header take some 260 DIPs of
	// the window; the list scrolls in the rest, up to its 300 DIP cap, so
	// a short window still shows the list whole instead of running its
	// last rows past the bottom edge.
	winW, winH := c.Size()
	recentMax := min(300, max(winH-welcomeRoom, 80))
	// The list takes recentWidth where the window has the room, and gives
	// way to the window's own margins below that: a fixed width would spill
	// past a narrow window's edge.
	recentW := max(min(recentWidth, winW-2*welcomeMargin), 120)
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.Text(c, locales.WelcomeTitle()).FontSize(24).Bold()
		ui.Text(c, locales.WelcomeSubtitle()).FontSize(12).TextColor(t.TextMuted)
		if ui.PrimaryButton(c, locales.WelcomeOpenFile()).Clicked() {
			if s.OpenDialog != nil {
				if paths, err := s.OpenDialog(); err == nil {
					s.OpenPaths(paths)
				}
			}
		}
		ui.Text(c, locales.WelcomeDropHint()).FontSize(11).TextColor(t.TextMuted)

		var recent []string
		if s.Recent != nil {
			recent = s.Recent()
		}
		if len(recent) == 0 {
			return
		}
		ui.Column(c).Gap(6).Width(recentW).Children(func() {
			// FillWidth, not Fill: the column is as high as its rows, so a
			// hundred percent of that height would have each row take the
			// whole list's height and the next one start below the window.
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.RecentFiles()).FontSize(11).Bold().TextColor(t.TextMuted)
				if ui.Button(c, locales.Clear()).Clicked() && s.ClearRecent != nil {
					_ = s.ClearRecent()
				}
			})
			// The list caps its height so a long Recent cannot push the
			// greeting off the window; the overflow scrolls. Its width is
			// fixed, so the bar's strip and the rows' padding line up.
			ui.Scroll(c).Label("Recent Files list").MaxHeight(recentMax).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for _, p := range recent {
						path := p
						// ButtonBase centers its children; a recent row's text block
						// must hug the row's left edge or each line starts elsewhere.
						row := ui.ButtonBase(c.Key("recent-row-"+path)).Label("Recent row "+path).FillWidth().Justify(ui.Start).Padding(8, recentRowPadEnd).Radius(6)
						if row.Hovered() {
							row.Background(t.SurfaceHover)
						}
						row.Children(func() {
							// Grow, not shrink to the text: the block takes the row's
							// width, so MaxLines cuts the path at the row's padding
							// instead of letting it run under the scroll bar.
							ui.Column(c).Grow(1).AlignItems(ui.Start).Children(func() {
								ui.Text(c, filepath.Base(path)).FontSize(13).MaxLines(1)
								ui.Text(c, filepath.Dir(path)).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
							})
						})
						if row.Clicked() {
							_ = s.OpenFile(path)
						}
					}
				})
			})
		})
	})
}

// trackFrames feeds the FPS readout and asks for the next frame while it
// shows: the readout counts real frames, so it keeps them coming. With the
// switch off, or on the welcome page where the bar is not drawn, nothing is
// sampled and no frame is pulled, so it costs nothing.
func (s *Shell) trackFrames(c *ui.Context) {
	if !s.Saved.ShowFPS || s.Home || len(s.Tabs) == 0 {
		s.fps.reset()
		return
	}
	s.fps.sample(c.Now())
	c.AnimationFrame()
}

// welcomeRoom is the height the welcome page spends above its recent
// list: the greeting, the open button, the hint and the list's caption,
// with their gaps.
const welcomeRoom = 260

// recentWidth is the recent list's widest: a folder path ends well left
// of the list's scroll bar instead of being cut at it, and the list still
// sits as a column under the greeting. welcomeMargin is the space the
// list keeps from the window's edges in a window narrower than that.
const (
	recentWidth   = 560
	welcomeMargin = 16
)

// centeredWidth is the text column's widest in the centered layout: the
// Rust reader's 900 DIPs, the length a line of prose stays readable at.
// The list, and its scroll bar, still span the window.
const centeredWidth = float32(900)

// recentRowPadEnd is a recent row's right padding. The list's overlay
// scroll bar paints 6 DIPs wide with a 3 DIP inset, so the row's text
// must end at least 12 DIPs before the container's right edge to clear
// it.
const recentRowPadEnd = 14

// statusBar renders the strip under the body: the FPS readout at the left
// edge, the zoom readout at the right, the room between them empty.
func (s *Shell) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(28).Padding(2, 8).Gap(6).Background(t.Surface).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		// The readout is a label, not a control: the menu's chords drive
		// the zoom, so the zoom line stays a label at the bar's other end.
		//
		// The readout keeps the box fpsWidth wide whichever text it shows, so
		// the zoom label never slides as the text changes.
		if text := s.fps.label(); text != "" {
			// Muted even when a frame was dropped: a stall shows in the
			// meter, not as an error colour in the corner.
			ui.Text(c, text).FontSize(11).TextColor(t.TextMuted).Width(fpsWidth).Shrink(0).
				MaxLines(1).Tooltip(locales.FpsTooltip())
		} else if s.Saved.ShowFPS && !s.Home && len(s.Tabs) > 0 {
			// Hold the room the readout takes before its first window fills:
			// the zoom label must not jump when the text arrives.
			ui.Box(c).Width(fpsWidth).Shrink(0)
		}
		ui.Box(c).Grow(1)
		ui.Text(c, s.ZoomLabel()).FontSize(11).TextColor(t.TextMuted).MarginX(6).
			Tooltip(locales.ZoomTooltip())
	})
}

// ThemeSource maps the saved theme to mygo's appearance source. The match
// ignores case: settings files written before the choices were Title Case
// carry "auto", "light" and "dark".
func ThemeSource(theme string) mygo.ThemeSource {
	switch strings.ToLower(theme) {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

func layoutIndex(layout string) int {
	if layout == "full_width" {
		return 1
	}
	return 0
}

func layoutName(i int) string {
	if i == 1 {
		return "full_width"
	}
	return "centered"
}

func explorerIndex(mode string) int {
	if mode == "project_root" {
		return 1
	}
	return 0
}

func explorerName(i int) string {
	if i == 1 {
		return "project_root"
	}
	return "current_dir"
}
