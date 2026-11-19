package shell

import (
	"math"
	"strings"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/settings"
	"github.com/windedge/peekdown/locales"
)

// The theme picker popover's fixed measures, mojian's theme-popover's:
// a 540 DIP panel over a 2x5 card grid, the cards' own preview sizes.
const (
	paletteWidth   float32 = 540
	palettePad     float32 = 12
	paletteGap     float32 = 8
	paletteBorder  float32 = 1
	palettePreview float32 = 64
	// palettePrevPad is .thm-prev's padding, the gradient frame the
	// miniature window is inset by; palettePrevRadius its corner.
	palettePrevPad    float32 = 7
	palettePrevRadius float32 = 7
	// paletteWinPad is .thm-win's horizontal padding, paletteWinRadius
	// its corner.
	paletteWinPad    float32 = 6
	paletteWinRadius float32 = 5
	// The preview's inner rows, measured from the window's top edge as
	// .thm-aa, .thm-line and .thm-dots stack: Aa's 10 DIP line 5 DIPs in,
	// the line's own 5 DIP margin and 4 DIPs, then the dots' 8 DIP margin.
	paletteAARow   float32 = 5
	paletteLineRow float32 = 20
	paletteDotsRow float32 = 32
	paletteLine    float32 = 4
	paletteCaret   float32 = 8
	paletteDot     float32 = 5
	// The dots row spans the window's content, split into eight dots and
	// the seven gaps between them, as mojian's PaintPreview draws it.
	paletteDotGap float32 = 3
	paletteDots           = 8
	// paletteAa and paletteName are the preview's two type sizes, .thm-aa's
	// and .thm-name's; paletteLineSpan is .thm-line's width, 62% of the
	// window's content, the caret standing at its end.
	paletteAa       float32 = 10
	paletteName     float32 = 11.5
	paletteMark     float32 = 9
	paletteLineSpan float32 = 0.62
	// paletteNameGap is what .thmcard's flex gap leaves between the
	// preview and the name; paletteNameRow is the name line's height.
	paletteNameGap float32 = 8
	paletteNameRow float32 = 15
)

// themeCardWidth is one card's width in the bar's popover; a card's
// height is its preview scaled with the width, plus the gap and name.
// The width is floored: five cards and their four gaps must clear the
// panel's content box, which the border and padding have thinned.
var themeCardWidth = float32(math.Floor(float64(paletteWidth-2*paletteBorder-2*palettePad-4*paletteGap) / 5))

// themeGridMinWidth is the smallest card that still reads like a
// miniature window. Below it the grid drops from five columns to four
// instead of shrinking the previews into unreadable strips.
const themeGridMinWidth = 80

// openPalette toggles the theme popover: a second press on the trigger
// closes it, and an open one closes before the trigger's popover builds
// again, so the same click never keeps it alive.
func (s *Shell) openPalette() {
	s.paletteOpen = !s.paletteOpen
}

// palettePopover draws the theme picker as a popover anchored below the
// bar's palette trigger: the appearance segmented over the ten accent
// themes as window-miniature cards, so the light-or-dark choice comes
// before the accent. A click applies the choice at once
// through persist, as the settings dialog's fields do, and keeps the
// popover open; Esc or a press outside closes it.
func (s *Shell) palettePopover(c *ui.Context, trigger ui.Element) {
	if !s.paletteOpen || !trigger.Valid() {
		return
	}
	t := c.Theme()

	// The picker builds in its own Overlay, after the title bar and before
	// the settings dialog, so mygo's build-order stacking leaves it over
	// the bar and under the dialog, as the font pickers' layer is. Its Esc
	// and outside presses close it, as PopoverBase's do.
	ui.Overlay(c, func() {
		panel := ui.Box(c).Role(ui.RolePopup).Label("Theme picker").
			AttachTo(trigger, ui.AnchorBottomEnd, ui.AnchorTopEnd).
			Width(paletteWidth).Padding(2, palettePad, palettePad).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Border).
			Radius(10).Shadow(0, 24, 60, -8, dialogShadow(t)).Clip()
		panel.Children(func() {
			s.paletteAppearanceSection(c)
			s.paletteThemeSection(c)
		})
		if panel.OverlayShortcut(0, ui.KeyEscape) || panel.PressedOutside() {
			s.paletteOpen = false
		}
	})
}

// paletteThemeSection draws the THEME heading and the 2x5 card grid.
// The section takes no horizontal padding of its own: the panel's
// padding already insets it, and a second one would leave the row too
// narrow for five cards.
func (s *Shell) paletteThemeSection(c *ui.Context) {
	paletteHeading(c, locales.PaletteTitle(), 10)
	s.themeGrid(c, paletteWidth-2*paletteBorder-2*palettePad)
}

// themeGrid draws the ten cards in a wrapping row. The bar's popover and
// the settings page both build it from the same preview, scaled to the
// width each has; the choice reads the same in either place.
func (s *Shell) themeGrid(c *ui.Context, avail float32) {
	_, cardW := themeGridMetrics(avail)
	ui.Row(c).FillWidth().Wrap().Gap(paletteGap).Label("Theme grid").Children(func() {
		for _, th := range accentThemes {
			themeCard(c, th, s.Saved.AccentTheme == th.ID, cardW, func() { s.setAccentTheme(th.ID) })
		}
	})
}

// themeGridMetrics keeps five columns whenever their cards stay legible,
// shrinks them proportionally otherwise, and falls back to four in the
// narrow settings column. Cards never grow past the popover's measure.
func themeGridMetrics(avail float32) (int, float32) {
	cols := 5
	cardW := (avail - float32(cols-1)*paletteGap) / float32(cols)
	if cardW < themeGridMinWidth {
		cols = 4
		cardW = (avail - float32(cols-1)*paletteGap) / float32(cols)
	}
	if cardW > themeCardWidth {
		cardW = themeCardWidth
	}
	return cols, max(cardW, 40)
}

// setAccentTheme applies a card's pick at once, as the settings' other
// fields do; both pickers go through it.
func (s *Shell) setAccentTheme(id string) {
	s.Saved.AccentTheme = id
	s.persist()
}

// themeCard draws one accent theme as a window miniature: the theme's
// gradient, an inner card with "Aa", a fake path line and the eight
// syntax dots, and the theme's name under it. The chosen card's preview
// takes a 2 DIP outline in the window's own accent, and its name the
// accent with it, as mojian marks the pick.
func themeCard(c *ui.Context, th AccentTheme, chosen bool, cardW float32, onPick func()) {
	t := c.Theme()
	k := th.of(t.Dark)
	prevH := palettePreview * cardW / themeCardWidth
	card := ui.ButtonBase(c.Key("thm-"+th.ID)).Label("Theme "+th.ID).
		Width(cardW).Height(prevH+paletteNameGap+paletteNameRow).Padding(6, 6, 5).
		Radius(8).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
	if card.Hovered() {
		card.Background(t.SurfaceHover)
	}
	card.Children(func() {
		ui.Column(c).Width(cardW - 12).Gap(paletteNameGap).AlignItems(ui.Center).Children(func() {
			prev := ui.Box(c).Label("preview "+th.ID).Size(cardW-12, prevH).
				Radius(palettePrevRadius).Clip()
			prev.Draw(func(p *ui.Painter, r ui.Rect) {
				paintThemePreview(p, r, k, t.Dark, t.Accent, t.Border, chosen)
			})
			weight, color := 400, t.TextMuted
			if chosen {
				weight, color = 600, t.Accent
			}
			ui.Text(c, themeName(th)).FontSize(paletteName).FontWeight(weight).TextColor(color).
				SingleLine().MaxLines(1)
		})
	})
	if card.Clicked() {
		onPick()
	}
}

// paintThemePreview paints one card's window miniature: the theme's own
// background, the lifted document face with its "Aa", path line and
// caret, the eight syntax dots, and the chosen card's accent stroke.
// mygo paints a box's border under its Draw and only clips children, so
// the strokes go here, over the fills.
func paintThemePreview(p *ui.Painter, r ui.Rect, k AccentColors, dark bool, accent, border ui.Color, chosen bool) {
	text := previewText(dark)
	p.Clip(r, palettePrevRadius, func() {
		paintAccentBackground(p, r, k)
		win := ui.Rect{
			X: r.X + palettePrevPad, Y: r.Y + palettePrevPad,
			W: r.W - 2*palettePrevPad, H: r.H - 2*palettePrevPad,
		}
		p.Shadow(win, paletteWinRadius, 0, 1, 2, 0, rgba(0, 0, 0, 0.10))
		p.Fill(win, k.CardFocused, paletteWinRadius)
		x, content := win.X+paletteWinPad, win.W-2*paletteWinPad
		p.RichText(x, win.Y+paletteAARow, 0,
			ui.Span{Text: "Aa", Size: paletteAa, Weight: 600, Color: text})
		lineY := win.Y + paletteLineRow
		mark := ui.Span{Text: "~/", Size: paletteMark, Color: text.Alpha(0.5)}
		markW, _ := p.MeasureText(0, mark)
		p.RichText(x, lineY-4, 0, mark)
		barX := x + markW + 2
		barW := content * paletteLineSpan
		p.Fill(ui.Rect{X: barX, Y: lineY, W: barW, H: paletteLine}, text.Alpha(0.22), 2)
		p.Fill(ui.Rect{X: barX + barW, Y: lineY - 2, W: 2, H: paletteCaret}, k.Accent, 1)
		dotW := (content - (paletteDots-1)*paletteDotGap) / paletteDots
		for i := 0; i < paletteDots; i++ {
			d := ui.Rect{X: x + float32(i)*(dotW+paletteDotGap), Y: win.Y + paletteDotsRow, W: dotW, H: paletteDot}
			p.Fill(d, previewDotColor(k, text, i), 2.5)
		}
	})
	if chosen {
		p.Stroke(r, accent, palettePrevRadius, 2)
	} else {
		p.Stroke(r, border, palettePrevRadius, 1)
	}
}

// paletteAppearanceSection draws the appearance heading, the three-way
// segmented bound to the saved Theme, and the faint note under it.
func (s *Shell) paletteAppearanceSection(c *ui.Context) {
	paletteHeading(c, locales.PaletteAppearance(), 10)
	s.appearancePick = themeSegmentIndex(s.Saved.Theme)
	seg := ui.SegmentedBase(c.Key("palette-appearance"), &s.appearancePick, len(themeLabels))
	s.styleSegmented(c, seg.Track, locales.PaletteAppearance())
	seg.Track.Children(func() {
		labels := themeChoiceLabels()
		for n, label := range labels {
			c.Key("palette-appearance-" + themeLabels[n])
			s.segment(c, seg.Segment(n), n, s.appearancePick, label)
		}
	})
	// FillEqually shares the track's width among the segments it already
	// holds, so it has to follow their construction.
	seg.Track.FillEqually().Margin(12, 0, 0)
	// The dialog's theme row's own commit: write the label the segments
	// chose once the input has updated the index, and persist.
	seg.Track.OnChange(func() {
		s.Saved.Theme = themeLabels[s.appearancePick]
		s.persist()
	})
	ui.Text(c, locales.AppearanceNote()).FontSize(11).TextColor(faintText(c)).
		Margin(6, 0, 0)
}

// paletteHeading draws a section's label: mojian's uppercase caption,
// tracked out over the cards or the segmented. The settings' inline grid
// uses a slightly larger caption than the popover's.
func paletteHeading(c *ui.Context, label string, size float32) {
	ui.Text(c, strings.ToUpper(label)).FontSize(size).FontWeight(650).
		LetterSpacing(0.5).TextColor(faintText(c)).Padding(10, 0, 4)
}

// dialogShadow is the shadow the popover floats on: mojian's
// --shadow-dialog at its 0 24 60 -8 geometry, a warm near-black in a
// light window and plain black in a dark one, which the theme's Border
// alone is too faint for at this blur.
func dialogShadow(t *ui.Theme) ui.Color {
	if t.Dark {
		return rgba(0, 0, 0, 0.55)
	}
	return rgba(30, 20, 30, 0.22)
}

// themeName is one card's label: the theme carries its catalog key, so
// the name follows the locale rather than showing the key itself.
func themeName(th AccentTheme) string {
	return locales.Catalog.Format(i18n.Locale(), th.NameKey, nil)
}

// themeSegmentIndex maps the saved Theme onto the popover's segmented's
// index, as themeSegmented's choice does.
func themeSegmentIndex(theme string) int {
	switch theme {
	case settings.ThemeLight:
		return 1
	case settings.ThemeDark:
		return 2
	}
	return 0
}

// faintText is the faint tier the section headings and note draw in: the
// theme carries no faint color, so TextMuted fades a step toward
// transparent.
func faintText(c *ui.Context) ui.Color {
	return c.Theme().TextMuted.Alpha(0.75)
}

// previewText is the preview's text color: the tokens carry no text
// color, so the miniature draws near-black in a light window and
// near-white in a dark one, as mojian's --pv-text does.
func previewText(dark bool) ui.Color {
	if dark {
		return ui.RGB(243, 240, 244)
	}
	return ui.RGB(24, 24, 27)
}

// previewDotColor is the i-th of the eight dots: the theme's foreground
// first, then its six syntax colors, and the gray that closes the row,
// the order mojian's terminal palette lists them in.
func previewDotColor(k AccentColors, text ui.Color, i int) ui.Color {
	switch {
	case i == 0:
		return text
	case i <= len(k.Syntax):
		return k.Syntax[i-1]
	default:
		return k.Syntax[5]
	}
}
