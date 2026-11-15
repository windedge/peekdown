package view

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/mermaid"
	"github.com/windedge/peekdown/locales"
)

// The mermaid block's limits: an error line truncates past this, and a
// diagram taller than this clamps by its aspect ratio, the Rust
// version's 100..1200 container clamp.
const (
	mermaidErrLimit  = 80
	mermaidMaxHeight = 1200
	// mermaidErrorTint is how much of the danger color the error panel's
	// face takes: the Rust version's danger background, toned to a tint
	// the text stays readable over.
	mermaidErrorTint = 0.1
)

// isMermaid reports a fenced block the fence names mermaid. The language
// is the fence info's first word, as the parser gives it.
func isMermaid(lang string) bool { return strings.EqualFold(lang, "mermaid") }

// mermaidTheme maps the frame's appearance to the diagram's palette.
func mermaidTheme(c *ui.Context) mermaid.Theme {
	if c.Theme().Dark {
		return mermaid.ThemeDark
	}
	return mermaid.ThemeDefault
}

// mermaidScale renders diagrams at no less than twice their intrinsic
// size, so the picture stays crisp on high-DPI displays and under zoom;
// below 2x the SVG's text turns soft. It follows the text's zoom past
// that and caps at mermaid.MaxScale: pixel count grows with the square of
// the scale, and past 2.5x the sharpness a zoom gains is not worth the
// memory.
func mermaidScale(st *State) float64 {
	zoom := st.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return min(max(zoom, 2.0), mermaid.MaxScale)
}

// mermaidBitmap returns the bitmap of the diagram, making it once per
// key: NewBitmap converts pixels and uploads a texture, so remaking it
// every frame doubles the render's cost. The view caches by the same
// key the render cache uses; entries never leave until the State does,
// which is bounded by the diagrams a document holds. Render errors come
// back as a nil bitmap and the error.
func (st *State) mermaidBitmap(src string, theme mermaid.Theme, scale float64) (*ui.Bitmap, error) {
	key := mermaid.Key(src, theme, scale)
	if b, ok := st.bitmapCache[key]; ok {
		return b, nil
	}
	img, err := st.LoadMermaid(src, theme, scale)
	if err != nil {
		// The loader's own LRU caches the error; do not double-cache a
		// failure the cache already holds.
		return nil, err
	}
	b := ui.NewBitmap(img)
	if st.bitmapCache == nil {
		st.bitmapCache = map[string]*ui.Bitmap{}
	}
	st.bitmapCache[key] = b
	return b, nil
}

// mermaidImage lays the diagram out: centered, its aspect ratio kept,
// clamped at mermaidMaxHeight, over the Rust version's muted hint about
// the SVG. A wider diagram than the column scrolls sideways, the shape
// code blocks keep, instead of squeezing the picture illegibly; tall
// ones still clamp. The hint is plain text: this renderer paints a
// bitmap, so it has no SVG file to open, the Rust version's link.
func (st *State) mermaidImage(c *ui.Context, b *ui.Bitmap) {
	tk := st.tokens(c)
	ui.Column(c).AlignSelf(ui.Stretch).MarginY(6).Gap(4).AlignItems(ui.Center).Children(func() {
		w, h := ui.ImageSize(b)
		if h > mermaidMaxHeight {
			w *= mermaidMaxHeight / h
			h = mermaidMaxHeight
		}
		ui.ScrollHorizontal(c).Label("Mermaid diagram").AlignSelf(ui.Stretch).
			Direction(ui.LeftToRight).AlignItems(ui.Center).Children(func() {
			ui.Image(c, b).MinWidth(w).MaxWidth(w).Width(w).MaxHeight(h).Shrink(0)
		})
		ui.Text(c, locales.MermaidRendered()).FontSize(12).
			TextColor(tk.muted)
	})
}

// mermaidErrorPanel says why a diagram kept its source: a danger panel
// with the header and the truncated error, the Rust version's render
// error branch, over the source block that follows it.
func mermaidErrorPanel(c *ui.Context, err error) {
	t := c.Theme()
	msg := []rune(err.Error())
	if len(msg) > mermaidErrLimit {
		msg = append(msg[:mermaidErrLimit:mermaidErrLimit], 0x2026)
	}
	ui.Column(c).Background(t.Background.Mix(t.Danger, mermaidErrorTint)).
		Radius(t.Radius).Padding(12, 12).MarginY(2).AlignSelf(ui.Stretch).Children(func() {
		ui.Text(c, locales.MermaidFailed()).FontWeight(600).TextColor(t.Danger)
		ui.Text(c, string(msg)).FontSize(12).TextColor(t.Text).Margin(4, 0, 0, 0)
	})
}
