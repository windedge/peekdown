package shell

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/locales"
)

// The tab badge is a fixed square before the label: it keeps its size and
// color while the label beside it is the part that truncates.
const (
	tabBadgeSize   float32 = 18
	tabBadgeGap    float32 = 8
	tabBadgeRadius float32 = 4
	tabBadgeFont   float32 = 9
	// Rust caps a tab's label at 150 DIPs; ours also carries an 18 DIP
	// badge, so 132 keeps the whole chip about as wide as Rust's.
	tabTextMaxWidth float32 = 132
)

// tabBadgePalette is the fixed set of muted, dark badge fills.
var tabBadgePalette = [...]ui.Color{
	ui.RGBA(0x7c, 0x5c, 0xbf, 255),
	ui.RGBA(0x4e, 0x79, 0xa7, 255),
	ui.RGBA(0x59, 0xa1, 0x4f, 255),
	ui.RGBA(0xd4, 0x71, 0x3c, 255),
	ui.RGBA(0xb2, 0x3a, 0x5b, 255),
	ui.RGBA(0x2f, 0x9e, 0x8f, 255),
	ui.RGBA(0xa8, 0x76, 0x2a, 255),
	ui.RGBA(0x5a, 0x6b, 0x9e, 255),
}

// tabBadgeColor maps a file name to the same fill on every render. FNV-1a
// spreads names that share an initial across the palette.
func tabBadgeColor(path string) ui.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(filepath.Base(path)))
	return tabBadgePalette[h.Sum32()%uint32(len(tabBadgePalette))]
}

// tabBadgeInitial is the badge's glyph: the name's first letter, upper
// cased, or its first character when it is not a letter.
func tabBadgeInitial(path string) string {
	for _, r := range filepath.Base(path) {
		return string(unicode.ToUpper(r))
	}
	return "?"
}

// tabShadow is the active tab's shadow on the bar's tray, per theme: dark
// panels need a heavier hand for the lift to read at all. Single tight
// layer keeps it a subtle sticker, not a floating card.
var (
	tabShadowLight = ui.RGBA(0, 0, 0, 0.06)
	tabShadowDark  = ui.RGBA(0, 0, 0, 0.25)
)

// tabStripHeight is a tab's height. The strip is taller (the title bar's
// content height): the tabs sit at its top, and the overlay scroll bar
// they scroll with paints in the room below them instead of over their
// bottom edge.
const tabStripHeight float32 = 25

// The strip's bar is a hairline: a 4 DIP thumb in an 8 DIP hit area.
// The wider hit area keeps the thin thumb draggable.
const (
	tabScrollbarThumbWidth float32 = 4
	tabScrollbarTrackWidth float32 = 8
)

// tabScrollbarStyle keeps the strip's bar thin and muted. Only this
// container gets the style, so the document, outline and tree keep theirs.
func tabScrollbarStyle(t *ui.Theme) ui.ScrollbarStyle {
	return ui.ScrollbarStyle{
		TrackWidth: tabScrollbarTrackWidth,
		ThumbWidth: tabScrollbarThumbWidth,
		Thumb:      t.Scrollbar.Alpha(0.45),
		Hover:      t.Scrollbar.Alpha(1.4),
		Active:     t.Scrollbar.Alpha(1.8),
	}
}

// tabStrip renders the scrollable tab strip into the title bar and keeps
// the active tab visible. h is the height of the strip's box, which the
// caller takes from the bar: the tabs keep to tabStripHeight and the
// bar's overlay scroll bar to the rest.
func (s *Shell) tabStrip(c *ui.Context, h float32) ui.Element {
	t := c.Theme()
	// Elements join their parent at creation, so the tab list has to be
	// built inside the scroll container that holds it: built earlier it
	// would land beside the strip and leave the bar empty.
	var activeTab ui.Element
	// was is the tab the frame started on: a keyboard pick on the strip
	// moves Active without a click, and both routes leave Home.
	was := s.Active
	clicked := -1
	// The bar is a recessed tray on the window's background (titleBar),
	// so the active tab floats as a chip of the window's color and the
	// idle ones melt into the tray. The strip clips sideways; the tabs
	// stack from its top, so the bottom clearance the bar leaves is
	// free for the scroll bar.
	strip := ui.ScrollHorizontal(c).Label("Tab bar").Height(h).TrackScroll(&s.tabScroll).
		ScrollbarStyle(tabScrollbarStyle(t))
	// A straight wheel's notch scrolls dy, and mygo only takes a notch
	// for the axes a container scrolls (wheelMoves), so on a sideways
	// strip it dies unheard. The handler turns a notch toward the strip's
	// free room into sideways scrolling; at either end a notch pointing
	// past the content passes on, as mygo would without a handler. The
	// old "at the end pass everything" guard also passed notches pointing
	// back into the content, whose native route on a sideways strip is
	// dead: from the end (where a fresh tab leaves the strip) the wheel
	// could never scroll back.
	strip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll || ev.DY == 0 || ev.DX != 0 {
			return false
		}
		at, max := s.tabScroll.X, s.tabScroll.MaxX
		if ev.DY > 0 && at >= max-0.5 || ev.DY < 0 && at <= 0.5 {
			return false
		}
		if ev.Momentum || ev.Precise {
			s.tabScroll.X = at + ev.DY
			return true
		}
		s.tabScroll.ScrollTo(at+ev.DY, 0, ui.Animation{})
		return true
	})
	strip.Children(func() {
		parts := ui.TabsBase(c, &s.Active, len(s.Tabs))
		parts.List.Gap(4).AlignItems(ui.Start).Children(func() {
			// Close requests are collected so the loop keeps iterating over the original slice.
			closeAt := -1
			reveal := ""

			// The menu's Chosen fires on a later frame, so actions re-locate the tab by
			// path instead of trusting the index captured now.
			for i := range s.Tabs {
				i := i
				path := s.Tabs[i].Path
				tab := parts.Tab(i)
				// The neutral weight is inherited by the badge and close icon;
				// the label sets its own weight per state.
				tab.Height(tabStripHeight).Padding(3, 10).Radius(7).FontWeight(400)
				// Home keeps the tabs but no active one: the body is the
				// welcome page, so a highlight would point at a tab that is not
				// on screen.
				if i == s.Active && !s.Home {
					activeTab = tab
					shadow := tabShadowLight
					if t.Dark {
						shadow = tabShadowDark
					}
					tab.Background(t.Background).TextColor(t.Text).
						Shadow(0, 1, 3, 0, shadow)
				} else {
					if tab.Hovered() {
						tab.Background(t.SurfaceHover)
					}
					tab.TextColor(t.TextMuted)
				}
				tab.Children(func() {
					ui.Row(c).Gap(tabBadgeGap).AlignItems(ui.Center).Children(func() {
						name := filepath.Base(path)
						badge := ui.Box(c).Role(ui.RoleImage).Label("Tab badge: "+name).
							Size(tabBadgeSize, tabBadgeSize).Radius(tabBadgeRadius).
							Background(tabBadgeColor(name)).Shrink(0).
							AlignItems(ui.Center).Justify(ui.Center)
						badge.Children(func() {
							ui.Text(c, tabBadgeInitial(name)).FontSize(tabBadgeFont).
								FontWeight(700).TextColor(ui.RGBA(255, 255, 255, 255)).MaxLines(1)
						})
						// Reserving the bolder label's advance keeps the chip's
						// width fixed when the active state changes the weight.
						// The same cap applies in both states, long or short.
						boldW, _ := c.MeasureText(0, ui.Span{Text: name, Size: 13, Weight: 500})
						label := ui.Text(c, name).FontSize(13).SingleLine().Ellipsis("...").
							Width(min(boldW+1, tabTextMaxWidth)).Shrink(0)
						if i == s.Active && !s.Home {
							label.FontWeight(500)
						}
						close := ui.ButtonBase(c.Key(fmt.Sprintf("close-%s", path))).
							Label(locales.CloseTabNamed(filepath.Base(path))).
							Size(20, 20).Padding(0).Radius(4).
							AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
						if close.Hovered() {
							close.Background(t.Danger).TextColor(t.AccentText)
						}
						close.Children(func() { ui.Icon(c, closeIcon).Size(10, 10) })
						if close.Clicked() {
							closeAt = i
						}
					})
				})
				tab.Tooltip(s.Tabs[i].Path)
				if tab.Clicked() {
					clicked = i
				}
				tab.ContextMenu(func(m *ui.Menu) {
					if m.Item(locales.TabRevealInSidebar()).Chosen() {
						reveal = path
					}
					if m.Item(locales.TabOpenInFileManager()).Chosen() {
						s.OpenInFileManager(path)
					}
					if m.Item(locales.TabCopyFilePath()).Chosen() {
						c.WriteClipboard(path)
					}
					m.Separator()
					if m.Item(locales.MenuCloseTab()).Chosen() {
						if idx := s.tabIndex(path); idx >= 0 {
							s.CloseTab(idx)
						}
					}
					if m.Item(locales.TabCloseOtherTabs()).Chosen() {
						if idx := s.tabIndex(path); idx >= 0 {
							s.CloseOthers(idx)
						}
					}
					if m.Item(locales.TabCloseAllTabs()).Chosen() {
						s.CloseAll()
					}
				})
			}

			if reveal != "" {
				s.RevealFile(reveal)
			}
			if closeAt >= 0 {
				s.CloseTab(closeAt)
			}
		})
	})

	// TabsBase's own choose runs after this build, so the pick is applied
	// here: the click on the tab already active changes nothing in Active,
	// and it still has to leave Home.
	if clicked >= 0 {
		s.showTab(clicked)
	} else if s.Active != was {
		s.showTab(s.Active)
	}

	if len(s.Tabs) > 0 && (!s.synced || s.activeSeen != s.Active) {
		if s.Active >= 0 && s.Active < len(s.Tabs) {
			s.tabSwitched(&s.Tabs[s.Active])
			activeTab.ScrollIntoView()
			s.activeSeen = s.Active
			s.synced = true
		}
		s.emitTitle()
	}
	return strip
}
