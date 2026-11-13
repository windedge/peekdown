package view

import "github.com/egoist/mygo/ui"

// The document's icons, drawn as paths the ui paints: the view cannot
// reach the shell's icon set, and a font glyph reads differently on every
// platform. currentColor follows the TextColor the element sets.

var (
	// copyIcon is two overlapping sheets: the code block's copy button.
	copyIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M15 5.5V5a2 2 0 00-2-2H6a2 2 0 00-2 2v7a2 2 0 002 2h.5"/></svg>`))

	// checkIcon is the copied state of that button, and the mark inside a
	// checked task item's box.
	checkIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7"/></svg>`))

	// chevronUpIcon and chevronDownIcon are the frontmatter card's fold
	// toggle: the state the card is in, not the action the click takes.
	chevronUpIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 15l6-6 6 6"/></svg>`))

	chevronDownIcon = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>`))
)
