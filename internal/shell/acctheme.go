// Package shell holds the application shell: tabs, sidebar, settings
// dialog and menus. This file is the accent-theme token layer: the ten
// hue families of the mojian theme-switch design, in both appearances,
// driving the window's gradient, the accent the widgets use and the code
// highlight's tokens.
package shell

import (
	"github.com/egoist/mygo/ui"
)

// AccentTheme is one accent theme in both appearances. The values come
// verbatim from the theme-switch design's token tables, recomputed by
// its one-shot script from mojian's oklch pipeline; the numbers are not
// to be retuned here.
type AccentTheme struct {
	ID      string
	NameKey string
	// Light and Dark are the tokens in the two appearances; the window
	// reads Light or Dark by the frame's theme.
	Light AccentColors
	Dark  AccentColors
}

// AccentColors are the window-level colors of an accent theme in one
// appearance. The gradient runs from the top through mid and low to the
// bottom, with a horizontal tint overlaid; the syntax colors color code
// blocks' tokens.
type AccentColors struct {
	// BgTop, BgMid, BgLow and BgBottom are the window gradient's stops
	// from the top down; BgTint overlays a horizontal wash toward the
	// window's right edge, transparent where a theme has none.
	BgTop, BgMid, BgLow, BgBottom, BgTint ui.Color

	// Accent is the theme's own color: selected rows, focus rings, the
	// progress bar and links take it.
	Accent ui.Color
	// Danger and Attention color destructive actions and warnings.
	Danger, Attention ui.Color
	// Panel and Card are the floating surfaces; PanelSel the fill a
	// selected row takes; SurfaceHover the fill a hovered one takes.
	Panel, Card, CardFocused, PanelSel, SurfaceHover ui.Color
	// Backdrop dims the window behind a dialog; it follows the window's
	// appearance rather than the accent hue, as mojian's Backdrop does.
	Backdrop ui.Color
	// Syntax are the code highlight's tokens: keyword, type, function,
	// number, string and comment.
	Syntax [6]ui.Color
}

// petalID is mojian's own DefaultID. Its set keeps the literal palette
// mojian's theme.go has always drawn, including the DefaultID special
// case, independent of this build's default.
const petalID = "petal"

func rgba(r, g, b uint8, a float32) ui.Color { return ui.RGBA(r, g, b, a) }

func syntaxColors(kw, ty, fn, num, str, cm string) [6]ui.Color {
	return [6]ui.Color{
		ui.Hex(kw), ui.Hex(ty), ui.Hex(fn), ui.Hex(num), ui.Hex(str), ui.Hex(cm),
	}
}

// accentThemes lists the picker's order, paper first, with the remaining
// hue families grouped by warmth.
var accentThemes = []AccentTheme{
	{
		ID: "paper", NameKey: "accentThemePaper",
		Light: AccentColors{
			BgTop: ui.Hex("#f6f6f7"), BgMid: ui.Hex("#f7f7f8"), BgLow: ui.Hex("#f8f8f8"), BgBottom: ui.Hex("#f8f8f8"),
			BgTint: ui.Transparent,
			Accent: ui.Hex("#466fbd"), Danger: ui.Hex("#b54e5a"), Attention: ui.Hex("#a2812d"),
			Panel: rgba(255, 255, 255, 0.97), Card: rgba(255, 255, 255, 0.78), CardFocused: rgba(255, 255, 255, 0.97),
			PanelSel: rgba(70, 111, 189, 0.12), SurfaceHover: ui.Hex("#f2f2f2"),
			Syntax: syntaxColors("#b54e5a", "#a2812d", "#8557a4", "#216ccc", "#1f895b", "#7a7a7a"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#141416"), BgMid: ui.Hex("#151517"), BgLow: ui.Hex("#161618"), BgBottom: ui.Hex("#161618"),
			BgTint: rgba(90, 120, 200, 0.10),
			Accent: ui.Hex("#7daafd"), Danger: ui.Hex("#e17c85"), Attention: ui.Hex("#dcb968"),
			Panel: rgba(44, 44, 44, 0.97), Card: rgba(28, 28, 30, 0.58), CardFocused: rgba(36, 36, 39, 0.95),
			PanelSel: rgba(125, 170, 253, 0.18), SurfaceHover: ui.Hex("#373737"),
			Syntax: syntaxColors("#e17c85", "#dcb968", "#bc8fdb", "#5c9ffb", "#61c191", "#9b9b9b"),
		},
	},
	{
		ID: "petal", NameKey: "accentThemePetal",
		Light: AccentColors{
			BgTop: ui.Hex("#f7f2f6"), BgMid: ui.Hex("#f4eff4"), BgLow: ui.Hex("#efeaf1"), BgBottom: ui.Hex("#e9e5ed"),
			BgTint: rgba(235, 222, 232, 0.50),
			Accent: ui.Hex("#a84e7c"), Danger: ui.Hex("#d92d20"), Attention: ui.Hex("#f59e0b"),
			Panel: rgba(251, 249, 251, 0.97), Card: rgba(255, 255, 255, 0.72), CardFocused: rgba(255, 255, 255, 0.96),
			PanelSel: rgba(168, 78, 124, 0.12), SurfaceHover: ui.Hex("#eeecee"),
			Syntax: syntaxColors("#cf222e", "#953800", "#8250df", "#0550ae", "#0a3069", "#6e7781"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#241d25"), BgMid: ui.Hex("#1f1a21"), BgLow: ui.Hex("#1b181d"), BgBottom: ui.Hex("#18151a"),
			BgTint: rgba(120, 72, 100, 0.18),
			Accent: ui.Hex("#e887b6"), Danger: ui.Hex("#f97066"), Attention: ui.Hex("#fbbf24"),
			Panel: rgba(35, 32, 38, 0.97), Card: rgba(25, 23, 28, 0.58), CardFocused: rgba(37, 35, 41, 0.95),
			PanelSel: rgba(232, 135, 182, 0.18), SurfaceHover: ui.Hex("#2f2c32"),
			Syntax: syntaxColors("#ff7b72", "#ffa657", "#d2a8ff", "#79c0ff", "#a5d6ff", "#8b949e"),
		},
	},
	{
		ID: "mist", NameKey: "accentThemeMist",
		Light: AccentColors{
			BgTop: ui.Hex("#e3e9f4"), BgMid: ui.Hex("#ebeff6"), BgLow: ui.Hex("#f1f2f3"), BgBottom: ui.Hex("#f2f1ec"),
			BgTint: rgba(210, 225, 248, 0.75),
			Accent: ui.Hex("#3772bb"), Danger: ui.Hex("#af545c"), Attention: ui.Hex("#9e833c"),
			Panel: rgba(250, 252, 255, 0.97), Card: rgba(254, 254, 255, 0.78), CardFocused: rgba(254, 255, 255, 0.97),
			PanelSel: rgba(55, 114, 187, 0.12), SurfaceHover: ui.Hex("#edeff3"),
			Syntax: syntaxColors("#af545c", "#9e833c", "#835b9f", "#256ec4", "#30875f", "#6f7884"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#0c1628"), BgMid: ui.Hex("#0d1827"), BgLow: ui.Hex("#111b23"), BgBottom: ui.Hex("#111b23"),
			BgTint: rgba(46, 91, 166, 0.30),
			Accent: ui.Hex("#70adfb"), Danger: ui.Hex("#db8087"), Attention: ui.Hex("#d7bb74"),
			Panel: rgba(37, 44, 52, 0.97), Card: rgba(25, 28, 34, 0.58), CardFocused: rgba(33, 36, 42, 0.95),
			PanelSel: rgba(112, 173, 251, 0.18), SurfaceHover: ui.Hex("#30373f"),
			Syntax: syntaxColors("#db8087", "#d7bb74", "#b992d5", "#5fa1f3", "#6bbf94", "#949ba5"),
		},
	},
	{
		ID: "meadow", NameKey: "accentThemeMeadow",
		Light: AccentColors{
			BgTop: ui.Hex("#e2eee5"), BgMid: ui.Hex("#ebf2e7"), BgLow: ui.Hex("#f3f2e2"), BgBottom: ui.Hex("#f5f0d6"),
			BgTint: rgba(214, 240, 222, 0.75),
			Accent: ui.Hex("#448335"), Danger: ui.Hex("#b4552b"), Attention: ui.Hex("#988629"),
			Panel: rgba(251, 253, 250, 0.97), Card: rgba(254, 255, 254, 0.78), CardFocused: rgba(254, 255, 254, 0.97),
			PanelSel: rgba(68, 131, 53, 0.12), SurfaceHover: ui.Hex("#eef1ed"),
			Syntax: syntaxColors("#b4552b", "#988629", "#945295", "#1776ab", "#248b27", "#717b6f"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#081c0d"), BgMid: ui.Hex("#101d0c"), BgLow: ui.Hex("#1b1c0b"), BgBottom: ui.Hex("#1b1c0b"),
			BgTint: rgba(6, 113, 50, 0.30),
			Accent: ui.Hex("#7dbf6e"), Danger: ui.Hex("#df825c"), Attention: ui.Hex("#d0bf66"),
			Panel: rgba(39, 46, 37, 0.97), Card: rgba(26, 29, 28, 0.58), CardFocused: rgba(34, 36, 36, 0.95),
			PanelSel: rgba(125, 191, 110, 0.18), SurfaceHover: ui.Hex("#323930"),
			Syntax: syntaxColors("#df825c", "#d0bf66", "#cb8acb", "#5aa7db", "#63c561", "#969d94"),
		},
	},
	{
		ID: "dusk", NameKey: "accentThemeDusk",
		Light: AccentColors{
			BgTop: ui.Hex("#e4dbf3"), BgMid: ui.Hex("#efdeee"), BgLow: ui.Hex("#f6e3dc"), BgBottom: ui.Hex("#f7e6cf"),
			BgTint: rgba(246, 206, 222, 0.80),
			Accent: ui.Hex("#8e57a5"), Danger: ui.Hex("#c14059"), Attention: ui.Hex("#b67702"),
			Panel: rgba(253, 251, 254, 0.97), Card: rgba(255, 254, 255, 0.78), CardFocused: rgba(255, 254, 255, 0.97),
			PanelSel: rgba(142, 87, 165, 0.12), SurfaceHover: ui.Hex("#f1eef2"),
			Syntax: syntaxColors("#c14059", "#b67702", "#a332b6", "#4d67c1", "#258a41", "#7d7380"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#190f2e"), BgMid: ui.Hex("#260f24"), BgLow: ui.Hex("#2d140b"), BgBottom: ui.Hex("#2d140b"),
			BgTint: rgba(125, 65, 138, 0.30),
			Accent: ui.Hex("#ca91e3"), Danger: ui.Hex("#ed7183"), Attention: ui.Hex("#f2af54"),
			Panel: rgba(47, 41, 51, 0.97), Card: rgba(29, 27, 33, 0.58), CardFocused: rgba(37, 35, 41, 0.95),
			PanelSel: rgba(202, 145, 227, 0.18), SurfaceHover: ui.Hex("#3a343e"),
			Syntax: syntaxColors("#ed7183", "#f2af54", "#db74ed", "#7d9af0", "#64c376", "#9e98a2"),
		},
	},
	{
		ID: "tide", NameKey: "accentThemeTide",
		Light: AccentColors{
			BgTop: ui.Hex("#dbebef"), BgMid: ui.Hex("#e3eff1"), BgLow: ui.Hex("#ecf3f0"), BgBottom: ui.Hex("#f2f2e8"),
			BgTint: rgba(194, 232, 238, 0.75),
			Accent: ui.Hex("#008695"), Danger: ui.Hex("#b35347"), Attention: ui.Hex("#968737"),
			Panel: rgba(249, 253, 254, 0.97), Card: rgba(253, 255, 255, 0.78), CardFocused: rgba(254, 255, 255, 0.97),
			PanelSel: rgba(0, 134, 149, 0.12), SurfaceHover: ui.Hex("#ebf1f2"),
			Syntax: syntaxColors("#b35347", "#968737", "#8559a1", "#1b75ad", "#2b885c", "#687b7e"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#001b22"), BgMid: ui.Hex("#001e1f"), BgLow: ui.Hex("#0b1e1b"), BgBottom: ui.Hex("#0b1e1b"),
			BgTint: rgba(0, 112, 128, 0.30),
			Accent: ui.Hex("#00c2d2"), Danger: ui.Hex("#df8073"), Attention: ui.Hex("#cebf70"),
			Panel: rgba(32, 47, 48, 0.97), Card: rgba(24, 29, 32, 0.58), CardFocused: rgba(31, 37, 40, 0.95),
			PanelSel: rgba(0, 194, 210, 0.18), SurfaceHover: ui.Hex("#2b3a3b"),
			Syntax: syntaxColors("#df8073", "#cebf70", "#bb90d8", "#5ca6dd", "#67c091", "#909e9f"),
		},
	},
	{
		ID: "apricot", NameKey: "accentThemeApricot",
		Light: AccentColors{
			BgTop: ui.Hex("#f5e1d6"), BgMid: ui.Hex("#f8e7dc"), BgLow: ui.Hex("#f8eedf"), BgBottom: ui.Hex("#f6f0da"),
			BgTint: rgba(252, 212, 194, 0.78),
			Accent: ui.Hex("#ac571d"), Danger: ui.Hex("#d02f00"), Attention: ui.Hex("#b97600"),
			Panel: rgba(255, 251, 249, 0.97), Card: rgba(255, 254, 254, 0.78), CardFocused: rgba(255, 254, 254, 0.97),
			PanelSel: rgba(172, 87, 29, 0.12), SurfaceHover: ui.Hex("#f3eeec"),
			Syntax: syntaxColors("#d02f00", "#b97600", "#a44688", "#426bbd", "#5a831b", "#82746c"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#2a1207"), BgMid: ui.Hex("#291507"), BgLow: ui.Hex("#25190a"), BgBottom: ui.Hex("#25190a"),
			BgTint: rgba(151, 62, 22, 0.30),
			Accent: ui.Hex("#ec915c"), Danger: ui.Hex("#fd6945"), Attention: ui.Hex("#f5ae3a"),
			Panel: rgba(51, 41, 35, 0.97), Card: rgba(31, 27, 27, 0.58), CardFocused: rgba(38, 35, 36, 0.95),
			PanelSel: rgba(236, 145, 92, 0.18), SurfaceHover: ui.Hex("#3e342e"),
			Syntax: syntaxColors("#fd6945", "#f5ae3a", "#dd80be", "#749ded", "#8fbb59", "#a39892"),
		},
	},
	{
		ID: "aurora", NameKey: "accentThemeAurora",
		Light: AccentColors{
			BgTop: ui.Hex("#dbefe8"), BgMid: ui.Hex("#e1ebf2"), BgLow: ui.Hex("#e8e4f4"), BgBottom: ui.Hex("#f0e6f1"),
			BgTint: rgba(198, 238, 224, 0.75),
			Accent: ui.Hex("#008966"), Danger: ui.Hex("#c44412"), Attention: ui.Hex("#998700"),
			Panel: rgba(249, 253, 251, 0.97), Card: rgba(254, 255, 254, 0.78), CardFocused: rgba(254, 255, 254, 0.97),
			PanelSel: rgba(0, 137, 102, 0.12), SurfaceHover: ui.Hex("#ecf1ee"),
			Syntax: syntaxColors("#c44412", "#998700", "#944aab", "#0077bb", "#009136", "#6b7c76"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#001f13"), BgMid: ui.Hex("#06192b"), BgLow: ui.Hex("#1e122a"), BgBottom: ui.Hex("#1e122a"),
			BgTint: rgba(0, 115, 75, 0.30),
			Accent: ui.Hex("#3bc59e"), Danger: ui.Hex("#f0754e"), Attention: ui.Hex("#d2c049"),
			Panel: rgba(34, 47, 42, 0.97), Card: rgba(24, 29, 30, 0.58), CardFocused: rgba(32, 37, 38, 0.95),
			PanelSel: rgba(59, 197, 158, 0.18), SurfaceHover: ui.Hex("#2d3a35"),
			Syntax: syntaxColors("#f0754e", "#d2c049", "#ca84e1", "#38a9ea", "#15cb6d", "#919e99"),
		},
	},
	{
		ID: "dune", NameKey: "accentThemeDune",
		Light: AccentColors{
			BgTop: ui.Hex("#ede4d8"), BgMid: ui.Hex("#f1e9de"), BgLow: ui.Hex("#f4eee4"), BgBottom: ui.Hex("#f3efe8"),
			BgTint: rgba(242, 222, 194, 0.70),
			Accent: ui.Hex("#996700"), Danger: ui.Hex("#a0624c"), Attention: ui.Hex("#a1813f"),
			Panel: rgba(253, 252, 250, 0.97), Card: rgba(255, 254, 254, 0.78), CardFocused: rgba(255, 255, 254, 0.97),
			PanelSel: rgba(153, 103, 0, 0.12), SurfaceHover: ui.Hex("#f1efed"),
			Syntax: syntaxColors("#a0624c", "#a1813f", "#8d5d7d", "#517198", "#667d50", "#7c776e"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#22190b"), BgMid: ui.Hex("#211a0e"), BgLow: ui.Hex("#201b12"), BgBottom: ui.Hex("#201b12"),
			BgTint: rgba(135, 79, 0, 0.30),
			Accent: ui.Hex("#d5a13c"), Danger: ui.Hex("#cb8e77"), Attention: ui.Hex("#dbb976"),
			Panel: rgba(47, 43, 36, 0.97), Card: rgba(29, 28, 28, 0.58), CardFocused: rgba(37, 35, 36, 0.95),
			PanelSel: rgba(213, 161, 60, 0.18), SurfaceHover: ui.Hex("#3a362f"),
			Syntax: syntaxColors("#cb8e77", "#dbb976", "#c493b3", "#82a1c7", "#9cb485", "#9e9a93"),
		},
	},
	{
		ID: "plain", NameKey: "accentThemePlain",
		Light: AccentColors{
			BgTop: ui.Hex("#eceef4"), BgMid: ui.Hex("#eceef4"), BgLow: ui.Hex("#eceef4"), BgBottom: ui.Hex("#eceef4"),
			BgTint: ui.Transparent,
			Accent: ui.Hex("#466fbd"), Danger: ui.Hex("#b54e5a"), Attention: ui.Hex("#a2812d"),
			Panel: rgba(255, 255, 255, 0.97), Card: rgba(255, 255, 255, 0.78), CardFocused: rgba(255, 255, 255, 0.97),
			PanelSel: rgba(70, 111, 189, 0.12), SurfaceHover: ui.Hex("#f2f2f2"),
			Syntax: syntaxColors("#b54e5a", "#a2812d", "#8557a4", "#216ccc", "#1f895b", "#7a7a7a"),
		},
		Dark: AccentColors{
			BgTop: ui.Hex("#141418"), BgMid: ui.Hex("#141418"), BgLow: ui.Hex("#141418"), BgBottom: ui.Hex("#141418"),
			BgTint: ui.Transparent,
			Accent: ui.Hex("#7daafd"), Danger: ui.Hex("#e17c85"), Attention: ui.Hex("#dcb968"),
			Panel: rgba(44, 44, 44, 0.97), Card: rgba(28, 28, 30, 0.58), CardFocused: rgba(36, 36, 39, 0.95),
			PanelSel: rgba(125, 170, 253, 0.18), SurfaceHover: ui.Hex("#373737"),
			Syntax: syntaxColors("#e17c85", "#dcb968", "#bc8fdb", "#5c9ffb", "#61c191", "#9b9b9b"),
		},
	},
}

// accentThemeByID resolves an accent theme's id, unknown and empty ids
// falling back to the default paper, as mojian's Resolve does.
func accentThemeByID(id string) AccentTheme {
	for i := range accentThemes {
		if accentThemes[i].ID == id {
			return accentThemes[i]
		}
	}
	return accentThemes[0]
}

// AccentThemeIDs lists the accent themes' ids in the design's order.
func AccentThemeIDs() []string {
	ids := make([]string, len(accentThemes))
	for i, t := range accentThemes {
		ids[i] = t.ID
	}
	return ids
}

// of returns the colors of one accent theme in one appearance. The
// backdrop is shared by every accent theme, so it is filled in here.
func (a AccentTheme) of(dark bool) AccentColors {
	if dark {
		c := a.Dark
		c.Backdrop = rgba(0, 0, 0, 0.45)
		return c
	}
	c := a.Light
	c.Backdrop = rgba(28, 16, 26, 0.18)
	return c
}

// applyAccentTheme copies the frame's theme over with the accent
// theme's tokens, as mojian's UI() recolors mygo's widget theme: the
// accent and its pressed and hover states, the text selection and the
// focus ring, and the surfaces the shell paints itself. The light/dark
// dimension stays with the frame's theme; only the hue tokens change.
func applyAccentTheme(t *ui.Theme, k AccentColors) *ui.Theme {
	u := *t
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	u.Accent = k.Accent
	if u.Dark {
		u.AccentHover = k.Accent.Mix(white, 0.2)
		u.AccentPressed = k.Accent.Mix(black, 0.15)
		u.Selection = k.Accent.Alpha(0.4)
		u.Focus = k.Accent.Alpha(0.6)
	} else {
		u.AccentHover = k.Accent.Mix(black, 0.12)
		u.AccentPressed = k.Accent.Mix(black, 0.25)
		u.Selection = k.Accent.Alpha(0.25)
		u.Focus = k.Accent.Alpha(0.55)
	}
	u.AccentText = accentTextOf(k.Accent)
	u.Danger = k.Danger
	u.Warning = k.Attention
	u.Surface = k.Panel
	u.SurfaceHover = k.SurfaceHover
	// The tokens carry no text color, so the pressed face steps to the
	// hover's rather than a mix against text, as mojian's UI() does.
	u.SurfacePressed = k.SurfaceHover
	u.Background = k.BgBottom
	return &u
}

// accentTextOf is black on a light accent and white on a dark one, read
// from the accent's luminance so a pale accent keeps its labels
// readable, as mojian's accentText does.
func accentTextOf(c ui.Color) ui.Color {
	lum := 0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)
	if lum > 165 {
		return ui.RGB(24, 24, 27)
	}
	return ui.RGB(255, 255, 255)
}

// cardRim is mojian's card border token: a white hairline on the
// focused document card, a fainter one on the translucent side panels.
func cardRim(t *ui.Theme, focused bool) ui.Color {
	if t.Dark {
		if focused {
			return rgba(255, 255, 255, 0.14)
		}
		return rgba(255, 255, 255, 0.06)
	}
	if focused {
		return rgba(255, 255, 255, 1)
	}
	return rgba(255, 255, 255, 0.85)
}

// cardShadow is mojian's shadow token for the two card layers.
func cardShadow(t *ui.Theme, focused bool) ui.Color {
	if t.Dark {
		if focused {
			return rgba(0, 0, 0, 0.45)
		}
		return rgba(0, 0, 0, 0.28)
	}
	if focused {
		return rgba(48, 36, 48, 0.12)
	}
	return rgba(48, 36, 48, 0.07)
}

// paintAccentBackground paints the window's background below the bars
// and the document: three vertical gradient bands from the top through
// mid and low to the bottom, then the horizontal tint over the whole
// rect toward the right edge. The band heights and the tint's fade are
// mojian's PaintBackground.
func paintAccentBackground(p *ui.Painter, r ui.Rect, k AccentColors) {
	h1, h2 := r.H*0.38, r.H*0.32
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: h1 + 1},
		ui.LinearGradient{From: k.BgTop, To: k.BgMid, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + h1, W: r.W, H: h2 + 1},
		ui.LinearGradient{From: k.BgMid, To: k.BgLow, Angle: 180}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + h1 + h2, W: r.W, H: r.H - h1 - h2},
		ui.LinearGradient{From: k.BgLow, To: k.BgBottom, Angle: 180}, 0)
	clear := k.BgTint
	clear.A = 0
	p.FillGradient(r, ui.LinearGradient{From: clear, To: k.BgTint, Angle: 90, Start: 0.35, End: 1}, 0)
}
