package view

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/mermaid"
)

// The mermaid block's limits: an error line truncates past this, and a
// diagram taller than this clamps by its aspect ratio.
const (
	mermaidErrLimit  = 80
	mermaidMaxHeight = 800
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
// below 2x the SVG's text turns soft. It caps at mermaid.MaxScale: pixel
// count grows with the square of the scale, and past 2.5x the sharpness
// a zoom gains is not worth the memory.
func mermaidScale(c *ui.Context) float64 {
	return min(max(float64(c.Zoom()), 2.0), mermaid.MaxScale)
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
// clamped at mermaidMaxHeight.
func mermaidImage(c *ui.Context, b *ui.Bitmap) {
	ui.Column(c).AlignSelf(ui.Stretch).MarginY(6).AlignItems(ui.Center).Children(func() {
		w, h := ui.ImageSize(b)
		if h > mermaidMaxHeight {
			w *= mermaidMaxHeight / h
			h = mermaidMaxHeight
		}
		ui.Image(c, b).MaxWidth(w).MaxHeight(h)
	})
}

// mermaidErrorLine says why a diagram kept its source: one muted red
// line, truncated, above the source block that follows it.
func mermaidErrorLine(c *ui.Context, err error) {
	msg := []rune(err.Error())
	if len(msg) > mermaidErrLimit {
		msg = append(msg[:mermaidErrLimit:mermaidErrLimit], 0x2026)
	}
	ui.Text(c, string(msg)).FontSize(12).TextColor(c.Theme().Danger).MarginY(2)
}
