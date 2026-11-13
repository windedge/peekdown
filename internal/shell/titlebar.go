package shell

import (
	"github.com/egoist/mygo/ui"
)

// TitleBarHeight is the window's bar height in DIPs: a 30 DIP tab, the 9
// DIPs the strip's overlay scroll bar takes below it, and the 1 DIP
// hairline under the bar. The system draws its own controls in this band
// (Win11 caption buttons, macOS traffic lights), so the bar is also the
// room WindowOptions.TitleBarHeight reserves.
const TitleBarHeight = 44

// The bar's own spacing: titleBarClearance at its ends, titleBarGap
// between its groups, titleBarHairline its bottom border, titleBarButton
// the gap between two toolbar buttons.
const (
	titleBarClearance float32 = 8
	titleBarGap       float32 = 6
	titleBarHairline  float32 = 1
	titleBarButton    float32 = 2
)

// appLogoSize is the app's mark in the bar, in DIPs: the size a Windows
// caption icon takes.
const appLogoSize float32 = 18

// titleBar draws the window's bar in place of the system title bar: the
// app's mark and name, the tab strip, the toolbar and the menu button.
// The window controls stay the system's, drawn over the bar's ends, whose
// width TitleBar reports so the bar can pad them away.
//
// The row is the window's drag area (DragWindow, which mygo doubles as
// the maximize on a double click); the tabs and buttons inside it are
// interactive, so a press on them never reaches the drag.
func (s *Shell) titleBar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	h := max(bar.Height, float32(TitleBarHeight))
	ui.Row(c).Label("Title bar").Height(h).Shrink(0).
		Padding(0, titleBarClearance+bar.Right, 0, titleBarClearance+bar.Left).
		AlignItems(ui.Center).Gap(titleBarGap).
		// The bar is a recessed tray on the window's background: the
		// active tab lifts off it as a chip of the window's own color.
		Background(t.Surface).
		BorderWidth(0, 0, titleBarHairline, 0).BorderColor(t.Border).
		DragWindow().Children(func() {
		// The brand is the way back to Home: the whole mark and name is one
		// flat button, hovered and labelled, and its press takes the pointer
		// off the bar's drag (mygo drags from the deepest interactive node
		// that is not otherwise interactive, which the button is not).
		brand := ui.ButtonBase(c.Key("bar-brand")).Label("Home").
			Gap(6).Padding(3, 6).Radius(6).FocusRing(false)
		if brand.Hovered() {
			brand.Background(t.SurfaceHover)
		}
		brand.Tooltip("Home")
		brand.Children(func() {
			// 18 DIPs: the size a Windows caption icon takes, small enough
			// to sit in the band and large enough to read the P.
			if logo, err := appLogo(); err == nil {
				ui.Image(c, logo).Size(appLogoSize, appLogoSize).Label("App logo")
			}
			ui.Text(c, "Peekdown").FontSize(13).FontWeight(600).MaxLines(1)
		})
		if brand.Clicked() {
			s.ShowHome()
		}
		if len(s.Tabs) > 0 {
			s.tabStrip(c, h-titleBarHairline).Grow(1).MinWidth(0)
		} else {
			ui.Spacer(c)
		}
		s.titleBarButtons(c)
	})
}

// titleBarButtons renders the bar's toolbar: the commands of the menu
// bar, then the menu button itself, whose menu is the same tree. The
// labels differ from the status bar's on purpose: two controls of one
// frame sharing a name leave the input target and the tests guessing.
func (s *Shell) titleBarButtons(c *ui.Context) {
	ui.Row(c).Gap(titleBarButton).Shrink(0).AlignItems(ui.Center).Children(func() {
		iconButton(c, "bar-open", folderOpenIcon, "Open file", 24, 14, func() {
			if s.OpenDialog == nil {
				return
			}
			if paths, err := s.OpenDialog(); err == nil {
				s.OpenPaths(paths)
			}
		}).Tooltip("Open (Ctrl+O)")
		iconButton(c, "bar-find", searchIcon, "Find in document", 24, 14, s.OpenSearch).
			Tooltip("Find (Ctrl+F)")
		iconButton(c, "bar-files", folderTreeIcon, "Files panel", 24, 14, func() { s.SetSidebarTab(0) }).
			Tooltip("Files (Ctrl+Shift+E)")
		iconButton(c, "bar-outline", outlineIcon, "Outline panel", 24, 14, func() { s.SetSidebarTab(1) }).
			Tooltip("Outline (Ctrl+B)")
		iconButton(c, "bar-settings", gearIcon, "App settings", 24, 14, s.OpenSettings).
			Tooltip("Settings (Ctrl+,)")
		s.menuButton(c)
	})
}

// menuButton is the bar's entry into the app menu: the native menu bar is
// hidden together with the title bar, so the same specs open as a popup
// menu here.
func (s *Shell) menuButton(c *ui.Context) ui.Element {
	b := iconButton(c, "bar-menu", menuIcon, "Main menu", 24, 14, nil)
	b.Tooltip("Menu")
	b.Menu(func(m *ui.Menu) { drawMenu(c, m, MenuSpecs(s)) })
	return b
}
