package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

// TitleBarHeight is the window's bar height in DIPs: the tab row, the
// room its overlay scroll bar takes below it, and the 1 DIP hairline
// under the bar. The system draws its own controls in this band
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
// menu, files switch, brand, tab strip and right-hand tools.
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
		// The menu takes the window's corner, ahead of the files switch;
		// the row's inset keeps it clear of the system's controls.
		s.menuButton(c)
		s.barToggleButton(c, "bar-files", folderTreeIcon, locales.BarFilesPanel(),
			locales.BarFilesTooltip(), s.Sidebar.Visible, s.ToggleFiles)
		// The brand is the way back to Home: the whole mark and name is one
		// flat button, hovered and labelled, and its press takes the pointer
		// off the bar's drag (mygo drags from the deepest interactive node
		// that is not otherwise interactive, which the button is not).
		brand := ui.ButtonBase(c.Key("bar-brand")).Label(locales.BarHome()).
			Gap(6).Padding(3, 6).Radius(6).FocusRing(false)
		if brand.Hovered() {
			brand.Background(t.SurfaceHover)
		}
		brand.Tooltip(locales.BarHome())
		brand.Children(func() {
			// 18 DIPs: the size a Windows caption icon takes, small enough
			// to sit in the band and large enough to read the P.
			if logo, err := appLogo(); err == nil {
				ui.Image(c, logo).Size(appLogoSize, appLogoSize).Label("App logo")
			}
			ui.Text(c, locales.AppTitle()).FontSize(13).FontWeight(600).MaxLines(1)
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

// titleBarButtons renders the bar's right-hand tools; the menu button
// sits at the bar's left end. The labels differ from the status bar's on
// purpose: two controls of one frame sharing a name leave the input
// target and the tests guessing.
func (s *Shell) titleBarButtons(c *ui.Context) {
	ui.Row(c).Gap(titleBarButton).Shrink(0).AlignItems(ui.Center).Children(func() {
		iconButton(c, "bar-open", folderOpenIcon, locales.BarOpenFile(), 24, 14, func() {
			if s.OpenDialog == nil {
				return
			}
			if paths, err := s.OpenDialog(); err == nil {
				s.OpenPaths(paths)
			}
		}).Tooltip(locales.BarOpenTooltip())
		iconButton(c, "bar-find", searchIcon, locales.BarFind(), 24, 14, s.OpenSearch).
			Tooltip(locales.BarFindTooltip())
		iconButton(c, "bar-settings", gearIcon, locales.BarSettingsPanel(), 24, 14, s.OpenSettings).
			Tooltip(locales.BarSettingsTooltip())
		// The outline switch ends the tool group, just before the
		// system caption buttons the window itself draws.
		s.barToggleButton(c, "bar-outline", outlineIcon, locales.BarOutlinePanel(),
			locales.BarOutlineTooltip(), s.Outline.Visible, s.ToggleOutline)
	})
}

// menuButton is the bar's entry into the app menu: the native menu bar is
// hidden together with the title bar, so the same specs open as a popup
// menu here.
func (s *Shell) menuButton(c *ui.Context) ui.Element {
	b := iconButton(c, "bar-menu", menuIcon, locales.BarMainMenu(), 24, 14, nil)
	b.Tooltip(locales.BarMenuTooltip())
	b.Menu(func(m *ui.Menu) { drawMenu(c, m, MenuSpecs(s)) })
	return b
}

// barToggleButton is the bar's panel switch: an open panel wears the
// accent-tinted selection surface, the prototype's .ibtn.active state.
func (s *Shell) barToggleButton(c *ui.Context, key any, icon *ui.SVG, label, tip string,
	open bool, onClick func()) ui.Element {
	t := c.Theme()
	k := s.accentColors(c)
	b := ui.ButtonBase(c.Key(key)).Label(label).Size(24, 24).Padding(0).Radius(5).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		Expanded(open).Tooltip(tip)
	bg, color := ui.Transparent, t.TextMuted
	if open {
		bg, color = k.PanelSel, k.Accent
	} else if b.Hovered() {
		bg = t.SurfaceHover
	}
	b.Background(bg).TextColor(color)
	b.Children(func() { ui.Icon(c, icon).Size(14, 14).TextColor(color) })
	if b.Clicked() && onClick != nil {
		onClick()
	}
	return b
}
