package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

const (
	edgeToggleSize  = float32(24)
	edgeToggleInset = float32(10)
	edgeToggleTop   = float32(8)
)

// paneEdgeToggles pins the two panel switches inside the document card's
// top corners. The card's committed bounds follow the sidebar and outline
// splits, so the buttons ride its corners instead of the content area's
// edges. With no card there is nothing to pin them to.
func (s *Shell) paneEdgeToggles(c *ui.Context) {
	card, area := s.edgeToggleBounds(c)
	if card.W <= 0 || card.H <= 0 {
		return
	}
	x, y := card.X-area.X, card.Y-area.Y+edgeToggleTop
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).PassThrough().Children(func() {
		s.edgeButtonAt(c, x+edgeToggleInset, y, func() {
			s.panelEdgeButton(c, "pane-files-toggle", locales.BarFilesPanel(),
				locales.BarFilesTooltip(), folderTreeIcon,
				s.Sidebar.Visible, s.ToggleFiles)
		})
		s.edgeButtonAt(c, x+card.W-edgeToggleInset-edgeToggleSize, y, func() {
			s.panelEdgeButton(c, "pane-outline-toggle", locales.BarOutlinePanel(),
				locales.BarOutlineTooltip(), outlineIcon,
				s.Outline.Visible, s.ToggleOutline)
		})
	})
}

// edgeToggleBounds reads the card's committed bounds, with a first-frame
// estimate: handles only see the previous frame, and the buttons must be
// clickable as soon as a document opens.
func (s *Shell) edgeToggleBounds(c *ui.Context) (ui.Rect, ui.Rect) {
	tab := s.ActiveTab()
	if tab == nil || tab.Model == nil {
		return ui.Rect{}, ui.Rect{}
	}
	card := s.documentCard.Bounds(c)
	area := s.progressBody.Bounds(c)
	if card.W > 0 && card.H > 0 && area.W > 0 {
		return card, area
	}
	winW, _ := c.Size()
	area = ui.Rect{X: 0, Y: TitleBarHeight, W: winW}
	card = ui.Rect{X: 8, Y: TitleBarHeight + 8, W: winW - 16, H: 1}
	if s.Sidebar.Visible {
		card.X += s.Sidebar.Width + 1
		card.W -= s.Sidebar.Width + 1
	}
	if s.Outline.Visible && winW >= outlineMinWindowWidth {
		card.W -= s.Outline.Width + 1
	}
	return card, area
}

func (s *Shell) edgeButtonAt(c *ui.Context, x, y float32, draw func()) {
	ui.Box(c).Absolute().Left(x).Top(y).Size(edgeToggleSize, edgeToggleSize).Children(draw)
}

// panelEdgeButton draws one switch on the card surface: an accent glyph
// reads open, a muted one closed. The button has no background or rim of
// its own; hover briefly lights the widget surface. Expanded carries the
// state for screen readers and tests.
func (s *Shell) panelEdgeButton(c *ui.Context, key, label, tip string,
	icon *ui.SVG, open bool, onClick func()) {
	t := c.Theme()
	color := t.TextMuted
	if open {
		color = t.Accent
	}
	b := ui.ButtonBase(c.Key(key)).Label(label).Size(edgeToggleSize, edgeToggleSize).Padding(0).Radius(6).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		Expanded(open).Tooltip(tip)
	bg := ui.Transparent
	if b.Hovered() {
		bg = t.SurfaceHover
	}
	b.Background(bg).TextColor(color)
	b.Children(func() { ui.Icon(c, icon).Size(13, 13) })
	if b.Clicked() && onClick != nil {
		onClick()
	}
}
