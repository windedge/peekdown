package shell

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/assets"
)

var (
	closeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18"/></svg>`))

	chevronUpIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 15l6-6 6 6"/></svg>`))

	chevronDownIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>`))

	chevronRightIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6l6 6-6 6"/></svg>`))

	folderIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2z"/></svg>`))

	// The bar's icons follow the prototype's set: a plain folder for Open,
	// a split panel for Files, left-aligned text lines for Outline and
	// sliders for Settings.
	folderOpenIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 5h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1z"/></svg>`))

	fileIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4"/></svg>`))

	docIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4"/><path d="M9 12h6M9 16h4"/></svg>`))

	folderTreeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/></svg>`))

	outlineIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 6H3M17 12H3M21 18H3"/></svg>`))

	searchIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>`))

	// Lucide palette: the theme picker's own mark, the accent themes'
	// trigger on the bar.
	paletteIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3a9 9 0 1 0 .4 18H14a2.5 2.5 0 0 0 0-5h-1a2 2 0 0 1 0-4h6.2A3.8 3.8 0 0 0 23 8.2C22.2 5 17.6 3 12 3z"/><circle cx="7.5" cy="10.5" r="1" fill="currentColor" stroke="none"/><circle cx="11.5" cy="7" r="1" fill="currentColor" stroke="none"/><circle cx="15.5" cy="8.5" r="1" fill="currentColor" stroke="none"/></svg>`))

	gearIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 8h10M18 8h2M4 16h4M12 16h8"/><circle cx="16" cy="8" r="2"/><circle cx="10" cy="16" r="2"/></svg>`))

	menuIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 7h16M4 12h16M4 17h16"/></svg>`))

	// The explorer header sorts the tree; the arrow reads at the small
	// ghost-button size where a full word would not fit.
	sortIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h9M4 12h6M4 17h3"/><path d="M17 5v13m0 0l3-3m-3 3l-3-3"/></svg>`))

	checkIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7"/></svg>`))
)

// appLogo is the title bar's mark: the app's own icon, not a glyph, so the
// bar and the task switcher show one picture. It decodes once, as 1024
// pixels are not worth decoding on every frame.
var appLogo = sync.OnceValues(func() (*ui.Bitmap, error) {
	return ui.DecodeBitmap(assets.AppIcon)
})

// iconButton is a flat icon button: no border, no background until hovered.
// size is the hit area, iconSize the glyph, and label what assistive
// technology reads, since the icon itself carries no text.
func iconButton(c *ui.Context, key any, icon *ui.SVG, label string, size, iconSize float32, onClick func()) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label).Size(size, size).Padding(0).Radius(5)
	b.AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, icon).Size(iconSize, iconSize) })
	if b.Clicked() && onClick != nil {
		onClick()
	}
	return b
}
