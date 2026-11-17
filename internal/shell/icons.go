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

	folderOpenIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 012-2h4l2 2h6a2 2 0 012 2v2"/><path d="M3 9h17l-2 10H5z"/></svg>`))

	fileIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4"/></svg>`))

	docIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4"/><path d="M9 12h6M9 16h4"/></svg>`))

	// Lucide folder-tree: a folder over tree branches, the file tree's
	// own mark, distinct from the open-folder icon of Open file.
	folderTreeIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 10a1 1 0 01-1-1V6a1 1 0 011-1h-2.5a1 1 0 01-.8-.4l-.9-1.2A1 1 0 0015 3h-2a1 1 0 00-1 1v5a1 1 0 001 1z"/><path d="M20 21a1 1 0 01-1-1v-3a1 1 0 011-1h-2.9a1 1 0 01-.88-.55l-.42-.85a1 1 0 00-.92-.6H13a1 1 0 00-1 1v5a1 1 0 001 1z"/><path d="M3 5a2 2 0 002 2h3"/><path d="M3 3v13a2 2 0 002 2h3"/></svg>`))

	// Lucide list-tree: indented levels off a spine, the hierarchy an
	// outline panel shows.
	outlineIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 5h13"/><path d="M13 12h8"/><path d="M13 19h8"/><path d="M3 10a2 2 0 002 2h3"/><path d="M3 5v12a2 2 0 002 2h3"/></svg>`))

	searchIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>`))

	// Lucide palette: the theme picker's own mark, the accent themes'
	// trigger on the bar.
	paletteIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3a9 9 0 1 0 .4 18H14a2.5 2.5 0 0 0 0-5h-1a2 2 0 0 1 0-4h6.2A3.8 3.8 0 0 0 23 8.2C22.2 5 17.6 3 12 3z"/><circle cx="7.5" cy="10.5" r="1" fill="currentColor" stroke="none"/><circle cx="11.5" cy="7" r="1" fill="currentColor" stroke="none"/><circle cx="15.5" cy="8.5" r="1" fill="currentColor" stroke="none"/></svg>`))

	// A cog with its center hole, not a sun: a circle ringed by rays reads
	// as a brightness toggle, which is what this icon used to look like.
	gearIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>`))

	menuIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 7h16M4 12h16M4 17h16"/></svg>`))

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
