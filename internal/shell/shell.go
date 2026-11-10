package shell

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/mermaid"
	"github.com/windedge/peekdown/internal/selection"
	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/internal/view"
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
		Tab     int // 0 files, 1 outline
		Visible bool
	}
	SettingsOpen bool
	Draft        settings.Settings
	Saved        settings.Settings
	Zoom         float64
	saveErr      string

	// SearchOpen shows the search bar; search is its state.
	SearchOpen bool
	search     searchBar

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

	// openError is the last failed open's parse error; the window title
	// carries it until the next successful open clears it.
	openError string

	activeSeen int
	synced     bool

	// Frame scratch state for the settings dialog bindings.
	draftLayout      int
	draftExplorer    int
	markersText      string
	lastActiveOnOpen int
}

// New returns a shell with sane defaults.
func New() *Shell {
	s := &Shell{
		Load:         settings.Default,
		Zoom:         1,
		mermaidCache: mermaid.NewCache(16),
	}
	s.Active = -1
	s.Sidebar.Width = 200
	s.Sidebar.Visible = true
	return s
}

// OpenFile reads path, parses it and appends it as a new tab.
func (s *Shell) OpenFile(path string) error {
	data, err := os.ReadFile(path)
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
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}
	s.Tabs = append(s.Tabs, Tab{Path: abs, Model: m})
	if parseErr != nil {
		// The error title sticks until the next successful open; a plain
		// emitTitle here would overwrite it at once.
		s.openError = parseErr.Error()
	} else {
		s.openError = ""
	}
	s.Active = len(s.Tabs) - 1
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
		return "Could not read this document: " + s.openError + " - Peekdown"
	}
	if t := s.ActiveTab(); t != nil {
		return t.Path + " - Peekdown"
	}
	return "Peekdown"
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

// tabIndex returns the index of the first tab with path, or -1.
func (s *Shell) tabIndex(path string) int {
	for i := range s.Tabs {
		if s.Tabs[i].Path == path {
			return i
		}
	}
	return -1
}

// ToggleSidebar flips sidebar visibility.
func (s *Shell) ToggleSidebar() {
	s.Sidebar.Visible = !s.Sidebar.Visible
}

// SetSidebarTab selects the sidebar segment and shows the sidebar.
func (s *Shell) SetSidebarTab(i int) {
	if i != 0 && i != 1 {
		return
	}
	s.Sidebar.Tab = i
	s.Sidebar.Visible = true
}

// SidebarTabIndex maps a saved sidebar tab name to the segment index:
// an unknown name is the explorer.
func SidebarTabIndex(tab string) int {
	return sidebarTabIndex(tab)
}

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

// RestoreFromSettings puts the saved settings on the shell at startup:
// the draft, the sidebar's width, tab and visibility, and the theme. It
// does not persist; the host mirrors the theme on the window.
func (s *Shell) RestoreFromSettings(t settings.Settings) {
	s.Saved = t
	s.Draft = t
	s.SetSidebarWidth(float32(t.SidebarWidth))
	s.Sidebar.Visible = t.SidebarVisible
	s.Sidebar.Tab = sidebarTabIndex(t.SidebarTab)
	s.applyAutoRefresh()
	if s.SetTheme != nil {
		s.SetTheme(themeSource(t.Theme))
	}
}

// ZoomIn increases the content zoom.
func (s *Shell) ZoomIn() {
	s.Zoom += 0.1
	if s.Zoom > 3 {
		s.Zoom = 3
	}
}

// ZoomOut decreases the content zoom.
func (s *Shell) ZoomOut() {
	s.Zoom -= 0.1
	if s.Zoom < 0.5 {
		s.Zoom = 0.5
	}
}

// ZoomReset restores 100 percent.
func (s *Shell) ZoomReset() { s.Zoom = 1 }

// Apply persists t and syncs the draft plus the host theme.
func (s *Shell) Apply(t settings.Settings) error {
	s.Saved = t
	s.Draft = t
	if t.SidebarWidth > 0 {
		s.SetSidebarWidth(float32(t.SidebarWidth))
	}
	s.applyAutoRefresh()
	s.Sidebar.Tab = sidebarTabIndex(t.SidebarTab)
	if s.SetTheme != nil {
		s.SetTheme(themeSource(t.Theme))
	}
	s.markersText = strings.Join(t.ProjectRootMarkers, ", ")
	if s.Save != nil {
		return s.Save(t)
	}
	return nil
}

// OpenSettings opens the dialog with a pristine draft.
func (s *Shell) OpenSettings() {
	s.Draft = s.Saved
	s.draftLayout = layoutIndex(s.Draft.Layout)
	s.draftExplorer = explorerIndex(s.Draft.ExplorerRootMode)
	s.markersText = strings.Join(s.Draft.ProjectRootMarkers, ", ")
	s.SettingsOpen = true
}

// View renders the whole shell.
func (s *Shell) View(c *ui.Context) {
	c.SetZoom(float32(s.Zoom))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		if len(s.Tabs) == 0 {
			s.welcome(c)
			return
		}
		s.tabBar(c)
		body := func() {
			if s.SearchOpen {
				s.searchView(c)
			}
			s.content(c)
		}
		if s.Sidebar.Visible {
			s.SetSidebarWidth(s.Sidebar.Width)
			ui.Split(c, &s.Sidebar.Width, func() { s.sidebar(c) }, body).Fill()
		} else {
			ui.Box(c).Fill().Children(func() { body() })
		}
		s.statusBar(c)
	})
	if s.SettingsOpen {
		s.settingsDialog(c)
	}
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
			ui.Text(c, "No document open").TextColor(t.TextMuted)
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
		c.Toast("Copied frontmatter")
	}
	// The diagrams render through the shared cache, keyed by source,
	// theme and scale: a theme switch or a zoom past 2x re-renders once
	// and every tab shares the result.
	tab.State.LoadMermaid = func(src string, theme mermaid.Theme, scale float64) (image.Image, error) {
		return s.mermaidCache.Render(src, theme, scale)
	}
	// The right edge is 0 so the list's own scrollbar sits at the window's
	// edge instead of floating 24 DIPs in; the rows give the text its
	// clearance on the right themselves.
	ui.Box(c).Fill().Padding(16, 0, 16, 24).Children(func() {
		// Mirrors the Rust reader: centered caps the column at 900 DIPs,
		// full_width lets it follow the window; the left padding keeps the
		// text off the window's edge and the rows pad the right.
		col := ui.Column(c).Grow(1)
		if s.Saved.Layout == "full_width" {
			col.AlignSelf(ui.Stretch)
		} else {
			col.MaxWidth(900).AlignSelf(ui.Center)
		}
		col.Children(func() {
			view.Frontmatter(c, tab.Model, &tab.State)
			view.ScrollView(c, tab.Model, &tab.State)
		})
	})
}

func (s *Shell) content(c *ui.Context) { s.Content(c) }

func (s *Shell) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, "Welcome to Peekdown").FontSize(24).Bold()
		if ui.Button(c, "Open File").Clicked() {
			if s.OpenDialog != nil {
				if paths, err := s.OpenDialog(); err == nil {
					s.OpenPaths(paths)
				}
			}
		}
		ui.Text(c, "Drop a Markdown file here").FontSize(12).TextColor(t.TextMuted)

		var recent []string
		if s.Recent != nil {
			recent = s.Recent()
		}
		if len(recent) == 0 {
			return
		}
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Recent").FontSize(12).TextColor(t.TextMuted)
			for _, p := range recent {
				path := p
				if ui.Button(c, filepath.Base(path)).Clicked() {
					_ = s.OpenFile(path)
				}
			}
			if ui.Button(c, "Clear").Clicked() && s.ClearRecent != nil {
				_ = s.ClearRecent()
			}
		})
	})
}

func (s *Shell) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(26).Padding(2, 6).Gap(6).Background(t.Surface).Children(func() {
		iconButton(c, "status-files", folderIcon, "Files", 24, 14, func() { s.SetSidebarTab(0) })
		iconButton(c, "status-outline", chevronDownIcon, "Outline", 24, 14, func() { s.SetSidebarTab(1) })
		ui.Box(c).Grow(1)
		iconButton(c, "status-settings", gearIcon, "Settings", 24, 14, func() { s.OpenSettings() })
		ui.Text(c, fmt.Sprintf("%d FPS", 0)).FontSize(11).TextColor(t.TextMuted) // TODO: real frame rate counter
	})
}

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

func sidebarTabIndex(tab string) int {
	if tab == "outline" {
		return 1
	}
	return 0
}

func sidebarTabName(i int) string {
	if i == 1 {
		return "outline"
	}
	return "explorer"
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
