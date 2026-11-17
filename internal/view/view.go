// Package view renders a parsed document as virtual list rows: one row per
// top-level block, styled spans for the text, highlight for the selection
// and the search, and an anchor that scrolls a heading into view.
package view

import (
	"cmp"
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/htmlx"
	"github.com/windedge/peekdown/internal/mermaid"
	"github.com/windedge/peekdown/internal/selection"
	"github.com/windedge/peekdown/locales"
)

// State is the state of one document view: its list, the selection, the
// search's matches and the anchor to scroll to. Keep one per open tab.
type State struct {
	// List is the virtual list of the document's blocks.
	List ui.ListState

	// ContentWidth caps the text column and centers it while the list,
	// and its overlay scrollbar, keep spanning the window: the bar stays
	// at the window's edge and only the lines that would grow too long
	// for reading pull in. 0 leaves the column at the list's width.
	ContentWidth float32

	// Selection is the text selection over the document's blocks.
	Selection selectionRange
	// Caret blinks at the selection's caret while the view has the focus.
	Focused bool

	// dragging marks a press that selects text: the element that took the
	// press gets the pointer's moves and its release wherever they go.
	dragging bool
	// press is where the drag began.
	press selection.Point
	// pressXY is the press's window position and moved reports whether a
	// drag moved past the click's threshold: a release inside it is a
	// click, which opens a link the press landed on.
	pressXY      ui.Point
	clickedALink bool

	// rowHandles bind each block's row as it builds, so the handler's
	// hit reads the committed frame's rectangles out of band, after the
	// frame and across scrolls; ctx is the frame's context they take.
	rowHandles map[int]*ui.Handle
	ctx        *ui.Context

	// Matches are the search's matches, in document order; Current is the
	// one the user is on, -1 for none. The view's overlays consume them;
	// ScrollView syncs them from the model each frame, and a caller that
	// renders rows without ScrollView may fill them itself. A match
	// addresses a run of a block's text, in runes.
	Matches []doc.Match
	Current int

	// FrontCollapsed folds the frontmatter card to its header row. It
	// lives per tab, so each file keeps its own fold; session only, never
	// saved to settings.
	FrontCollapsed bool

	// Anchor scrolls the heading of this slug into view; AnchorVersion
	// asks for the same anchor again.
	Anchor        string
	AnchorVersion uint64
	anchorDone    string

	// PendingAnchor holds a cross-document anchor a file link asked for
	// before its document was known to hold the heading: each frame
	// retries, a hit promotes the slug to Anchor, a miss counts against
	// pendingAnchorTries and then drops, so a dead anchor cannot jump on
	// a later edit that happens to add the heading.
	PendingAnchor       string
	pendingAnchorMisses int

	// ScrollTargetBlock is the block the search found the current match
	// in, which the view scrolls into view; -1 for none. The search bar
	// sets it as the current match moves, and ScrollView consumes it: a
	// value survives one scroll, so the next sets it again.
	ScrollTargetBlock int
	scrollTargetDone  int

	// Scroll tracks the list's offset: ScrollView wires the list to it,
	// and the container writes Y and MaxY back as it lays out and
	// scrolls. Progress, AtTop and AtBottom read it; a caller that builds
	// its own list may wire it the same way. The shell draws the reading
	// progress bar and the jump buttons from these, the window-level
	// views of one scroll.
	Scroll ui.ScrollState

	// ScrollSpeed multiplies the wheel's travel, the scroll_speed
	// setting; 0 draws as 1. The handler applies it to its own wheel
	// notches (touchpad drags ride mygo's engine unmultiplied), so it
	// only reaches the view when the shell injects it each frame. 1
	// passes notches to the engine's own glide.
	ScrollSpeed float32
	// NoInertia is the inertia_scroll setting, unset: the handler takes
	// wheel notches and scrolls them at once, mygo's glide being without
	// a per-container switch to turn it off.
	NoInertia bool
	// Inertia is the wheel glide ScrollView advances each frame while the
	// handler turns notches into velocity.
	Inertia InertiaScroll

	// Progress, AtTop and AtBottom are the last frame's scroll place,
	// refreshed by ScrollView: the fraction 0..1 the list is scrolled
	// through, and whether it stands at either end, where the Rust
	// version hides its jump buttons. 0 progress with MaxY 0 is a list
	// with no room to scroll, so short documents draw an empty bar.
	Progress float32
	AtTop    bool
	AtBottom bool

	// OpenURL opens a link's destination in the browser; nil makes the
	// links inert. The view calls it with http/https/mailto only; an
	// anchor (#slug) scrolls the heading into view instead.
	OpenURL func(string)
	// LinksWired reports that OpenURL is in place: the shell sets it
	// when it injects OpenURL each frame. The outline reads it to skip
	// the scroll-follow work before the first frame wired a tab, where
	// OpenURL alone was a guess: a caller may leave OpenURL nil on
	// purpose, which is "inert", not "not yet wired".
	LinksWired bool

	// OpenLocalLink runs when a release lands on a link that is no
	// anchor and no web scheme: a file path, whose "#fragment" tail
	// rides along raw. The view knows no document directory, so the
	// shell resolves the destination against the open file and shows it
	// as a tab; nil leaves such links inert.
	OpenLocalLink func(string)

	// AnchorOf returns the anchor of top-level block i, as the model
	// makes them, so the view can find the row to scroll to.
	AnchorOf func(i int) string
	// anchorModel is the model AnchorOf came from, when the shell wired
	// it: a linked image's "#anchor" resolves through it. A bare State
	// without it leaves the anchor inert.
	anchorModel anchorIndex

	// LoadImage loads the image at url; nil shows its description.
	LoadImage func(url string) (ui.ImageSource, error)

	// CopyFrontmatter runs when the frontmatter card's copy button is
	// clicked, with the block's Raw text; nil hides the button. The shell
	// writes the clipboard and toasts; the view keeps out of the host.
	CopyFrontmatter func(string)

	// CopyText runs when a code block's copy button is clicked, with the
	// block's source text; nil hides the buttons. The shell writes the
	// clipboard and toasts, as it does for the frontmatter.
	CopyText func(string)

	// Menu holds the document area's context menu commands; nil leaves the
	// area without a menu. The shell injects them, so the view keeps out of
	// the clipboard, the search bar and the file manager.
	Menu *MenuActions
	// copiedBlock and copiedAt hold the block whose copy button shows that
	// it copied, and when; a zero copiedAt is none.
	copiedBlock int
	copiedAt    time.Time
	// copyHandles are the copy buttons' rectangles, which the handler
	// reads out of band to leave their presses to the buttons.
	copyHandles map[int]*ui.Handle

	// FontSize is the body's size, from the font_size setting; 0 draws
	// at defaultBodySize. The shell injects it every frame, so a save
	// re-renders without further wiring.
	FontSize float32
	// MonoFontSize is the size of the code's monospaced text, the code
	// blocks, the inline code and the frontmatter's; 0 draws at
	// defaultMonoSize. The shell injects it every frame too.
	MonoFontSize float32
	// FontFamily is the prose family, from the font_family setting; ""
	// keeps the theme's default. Empty is the common case: mygo resolves
	// an element's family, a span's, against the system's list and falls
	// back to its sans.
	FontFamily string
	// MonoFontFamily is the code's family, from mono_font_family; ""
	// keeps "monospace".
	MonoFontFamily string
	// Zoom is the shell's runtime text zoom as a multiplier, 1 at 100
	// percent; 0 draws as 1. The sizes above are already scaled by it, so
	// only the mermaid diagrams, rendered rather than shaped, read it:
	// they follow the text into a sharper raster. It is not a window zoom
	// (ticket #25).
	Zoom float64

	// LoadMermaid renders a mermaid diagram to a picture; nil shows
	// mermaid blocks as plain source code. The shell wires the shared
	// LRU cache here; the view stays out of when and where it renders.
	LoadMermaid func(src string, theme mermaid.Theme, scale float64) (image.Image, error)
	// imageCache caches the sources the loader returned, by URL.
	imageCache map[string]ui.ImageSource
	// bitmapCache caches the mermaid bitmaps the loader made, by the
	// mermaid.Key of source, theme and scale, so a bitmap is built once
	// per key instead of once a frame.
	bitmapCache map[string]*ui.Bitmap

	// layouts cache the shaped text of the blocks the pointer has hit.
	layouts map[layoutKey]*textLayout

	// docVersion is the model's Version when the caches were built: a
	// new document leaves them stale, so the handler drops them and the
	// selection, as the Rust version does.
	docVersion uint64

	// Theme of the view's code: the window's by default.
	CodeTheme *editor.Theme
}

// InertiaScroll is the wheel's velocity, in DIPs per second, decaying as the
// Rust viewer's does: a flick keeps the page moving after the wheel stops,
// which mygo's per-notch ease does not.
type InertiaScroll struct {
	Velocity float32
	last     time.Time // zero until the first step
	wroteY   float32   // the Y the inertia last wrote, to spot foreign moves
}

// The constants of the Rust version's InertiaScrollState (src/text/state.rs),
// kept at its values and named after it, so a flick rides the same curve.
const (
	inertiaMinVelocity     float32 = 10   // MIN_VELOCITY, below which it stops
	inertiaFriction        float64 = 0.94 // 1 - FRICTION, per 1/60s
	inertiaMaxVelocity     float32 = 8000 // MAX_VELOCITY, the clamp
	inertiaVelocityScale   float32 = 8    // VELOCITY_MULTIPLIER
	inertiaSmoothThreshold float32 = 8000 // SMOOTH_THRESHOLD, times the speed
	inertiaLinearDecel     float32 = 8000 // LINEAR_DECEL, DIPs per second
	// A step longer than this is a stall (a hidden window, a slow frame),
	// not a flick: capped so the page does not leap in one frame.
	inertiaMaxStep = 100 * time.Millisecond
	// A first step has no previous frame to measure from: one frame, the
	// Rust version's 1/60s, rather than a zero-time gap.
	inertiaFirstStep = time.Second / 60
)

// active reports whether the velocity still moves the view.
func (in *InertiaScroll) active() bool {
	return in.Velocity >= inertiaMinVelocity || in.Velocity <= -inertiaMinVelocity
}

// stop ends the glide: the next one starts its own clock. The offset the
// last step wrote stays, for the next impulse to re-sync.
func (in *InertiaScroll) stop() {
	in.Velocity, in.last = 0, time.Time{}
}

// impulse adds a wheel notch's dy, as the Rust add_impulse does, the speed
// multiplier riding it in here and nowhere else: the same direction
// accumulates, the opposite one keeps 30% of the old velocity. y is the
// offset the glide starts from, which the frame loop compares to spot moves
// it did not make; only a start from rest takes it, so a notch that keeps a
// glide running does not reset the clock another frame already advanced.
func (in *InertiaScroll) impulse(dy, speed, y float32) {
	if !in.active() {
		in.wroteY, in.last = y, time.Time{}
	}
	if speed <= 0 {
		speed = 1
	}
	impulse := dy * inertiaVelocityScale * speed
	if in.Velocity == 0 || (in.Velocity > 0) == (impulse > 0) {
		in.Velocity += impulse
	} else {
		in.Velocity = in.Velocity*0.3 + impulse
	}
	in.Velocity = max(-inertiaMaxVelocity, min(in.Velocity, inertiaMaxVelocity))
}

// step advances the glide by dt and returns the distance to scroll: the
// pre-decay velocity's, as the Rust update multiplies it before decaying.
// Two decays meet the Rust curve: exponential above the smooth threshold,
// which scales with the speed setting as its does, and linear below it for
// a crisp stop.
func (in *InertiaScroll) step(dt time.Duration, speed float32) float32 {
	if speed <= 0 {
		speed = 1
	}
	secs := float32(dt.Seconds())
	distance := in.Velocity * secs
	abs := max(in.Velocity, -in.Velocity)
	if abs > inertiaSmoothThreshold*speed {
		in.Velocity *= float32(math.Pow(inertiaFriction, float64(secs*60)))
	} else if decel := inertiaLinearDecel * secs; abs <= decel {
		in.Velocity = 0
	} else if in.Velocity > 0 {
		in.Velocity -= decel
	} else {
		in.Velocity += decel
	}
	// A velocity below the floor lands nothing more: the Rust version stops
	// there and drops the step's distance, so the page does not creep.
	if in.Velocity < inertiaMinVelocity && in.Velocity > -inertiaMinVelocity {
		in.stop()
		return 0
	}
	return distance
}

// MenuActions are the document area's context menu commands, bound by the
// shell to the active tab. A nil func does nothing; HasSelection greys Copy
// out when there is nothing to copy.
type MenuActions struct {
	HasSelection      func() bool
	Copy              func()
	SelectAll         func()
	Find              func()
	Refresh           func()
	Reveal            func()
	OpenInFileManager func()
	CopyPath          func()
}

// selectionRange is the selection as the view sees it: a half-open run of
// each block's text, in runes.
type selectionRange struct {
	start, end selection.Point
	set        bool
}

// Point is a block and a rune offset in its text.
type Point = selection.Point

// Match is one search match: block i's text, runes from Start to End.
// The view's and the doc's search agree on the shape, so the model's
// state can feed the view's overlays directly.
type Match = doc.Match

// SetSelection stores the selection the view draws.
func (st *State) SetSelection(anchor, caret selection.Point) {
	st.Selection = selectionRange{start: anchor, end: caret, set: true}
	if caret.Before(anchor) {
		st.Selection.start, st.Selection.end = st.Selection.end, st.Selection.start
	}
}

// ClearSelection collapses the selection.
func (st *State) ClearSelection() { st.Selection.set = false }

// Handler returns the input handler for the document area. ScrollView
// wires it to its list already; a caller that builds its own list may
// attach it there. It returns false for events it does not
// take, scrolling among them. The events' position is relative to the
// element wired, which is to wrap the list without padding of its own.
func (st *State) Handler(m *doc.Model) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputScroll:
			// The wheel dies on a vertical-only list unless a handler
			// turns it (the engine only takes a notch for the axes the
			// container scrolls), so the handler turns it here: into the
			// view's own glide while inertia is on, into an offset write
			// while it is off. Shift swaps the axis sideways like the
			// engine does, and a sideways wheel keeps its route.
			//
			// A notch that moves nothing keeps its route too: the strip
			// handlers and the list see a zero notch as they always did.
			if ev.DY == 0 {
				return false
			}
			// The body's own shifts keep their routes: the code block and
			// the wide table scroll sideways on DX, and Shift turns the
			// wheel sideways.
			if ev.DX != 0 || ev.Mods&ui.Shift != 0 {
				return false
			}
			// A touchpad's drags, and the momentum the platform adds after
			// them, follow the fingers: the engine glides those itself, and
			// a glide of ours would fight it. A drag arriving mid-flick
			// hands the ride over, so the flick ends.
			if ev.Precise || ev.Momentum {
				st.Inertia.stop()
				return false
			}
			// Inertia on: the notch becomes velocity, and the view's own
			// loop rides it out, decaying as the Rust viewer does. The
			// multiplier rides the impulse, once, so a speed past 1 glides
			// further rather than landing at once.
			if !st.NoInertia {
				st.Inertia.impulse(ev.DY, st.ScrollSpeed, st.Scroll.Y)
				return true
			}
			// The inertia setting off turns the engine's wheel glide off:
			// mygo glides every notch and has no per-container switch, so
			// the handler scrolls the notch at once instead, the instant
			// ride the setting asks for. Windows never reports momentum,
			// so this branch covers the whole wheel there.
			speed := st.ScrollSpeed
			if speed <= 0 {
				speed = 1
			}
			// A direct offset write: TrackScroll lands it next layout, and
			// the wheelless list takes it from there. Scrolling at once
			// rather than gliding keeps the multiplier predictable under
			// repeated notches; a touchpad drag never reaches this branch.
			st.Scroll.Y = max(0, min(st.Scroll.Y+ev.DY*speed, st.Scroll.MaxY))
			return true
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			if st.copyButtonAt(ev) {
				// The press belongs to the copy button, not to a selection
				// drag: leaving it undelivered lets the button take it.
				return false
			}
			st.invalidateStale(m)
			// A press above the window's top comes clamped below zero
			// (the engine keeps the element hit), as the first row.
			p, ok := st.pointAt(m, ev, true)
			if !ok {
				return false
			}
			st.press = p
			st.pressXY = st.listXY(ev)
			st.clickedALink = st.linkAt(m, ev) != ""
			st.dragging = true
			switch {
			case ev.Clicks == 2:
				s := selection.SelectWord(p, runText(m, p.Block))
				st.SetSelection(s.Anchor, s.Caret)
			case ev.Clicks >= 3:
				s := selection.SelectLine(p, runText(m, p.Block))
				st.SetSelection(s.Anchor, s.Caret)
			default:
				st.SetSelection(p, p)
			}
			return true
		case ui.InputPointerMove:
			if !st.dragging {
				return false
			}
			p, ok := st.pointAt(m, ev, false)
			if !ok {
				return false
			}
			// Past the click's threshold this is a drag: the release then
			// selects and opens nothing.
			d := st.listXY(ev)
			if dx, dy := d.X-st.pressXY.X, d.Y-st.pressXY.Y; dx*dx+dy*dy > clickThreshold*clickThreshold {
				st.clickedALink = false
			}
			st.SetSelection(st.press, p)
			return true
		case ui.InputPointerUp:
			if !st.dragging || ev.Button != 0 {
				return false
			}
			st.dragging = false
			if st.clickedALink {
				st.openLinkAt(m, ev)
			}
			return true
		}
		return false
	}
}

// invalidateStale drops the layout cache and the selection when the
// model was scanned again: block indices keep their meaning but name
// other text, so the old selection points nowhere and the shaped text
// lies about the new one. The version covers every Set and every fresh
// parse.
func (st *State) invalidateStale(m *doc.Model) {
	if v := m.Version(); v != st.docVersion {
		st.layouts = nil
		st.Selection.set = false
		st.Matches = nil
		st.Current = -1
		st.copyHandles = nil
		st.docVersion = v
	}
}

// copyButtonAt reports whether the pointer event lands on a code block's
// copy button. The view's handler takes every press over the list, so it
// has to let the buttons have their own.
func (st *State) copyButtonAt(ev ui.InputEvent) bool {
	if st.ctx == nil || len(st.copyHandles) == 0 {
		return false
	}
	p := st.listXY(ev)
	for _, h := range st.copyHandles {
		r := h.Bounds(st.ctx)
		if p.X >= r.X && p.X <= r.X+r.W && p.Y >= r.Y && p.Y <= r.Y+r.H {
			return true
		}
	}
	return false
}

// clickThreshold is the drag distance in DIPs under which a press and
// release still count as a click, one that opens a link.
const clickThreshold = float32(4)

// listXY returns the event's position in window coordinates: the
// handler's events are the list's, and the list's rectangle turns the
// local point into a window one.
func (st *State) listXY(ev ui.InputEvent) ui.Point {
	if st.ctx == nil {
		return ui.Point{X: ev.X, Y: ev.Y}
	}
	listRect := st.List.Handle.Bounds(st.ctx)
	return ui.Point{X: listRect.X + ev.X, Y: listRect.Y + ev.Y}
}

// linkAt returns the link destination the pointer event lands on, or "":
// the point's block and rune offset read out of band like the selection's,
// and the block's inlines name the link that run position sits in. Images
// wrapped in one link answer the outer link's destination, which the walk
// never marks.
func (st *State) linkAt(m *doc.Model, ev ui.InputEvent) string {
	p, ok := st.pointAt(m, ev, false)
	if !ok {
		return ""
	}
	if dest := m.LinkTargetOf(p.Block); dest != "" {
		return dest
	}
	return linkDestOf(m, p.Block, p.Offset)
}

// openLinkAt acts on the link the release landed on: it hits the
// document's links through the pointer, then dispatches the destination.
func (st *State) openLinkAt(m *doc.Model, ev ui.InputEvent) {
	st.dispatchLink(st.linkAt(m, ev))
}

// dispatchLink acts on dest: an anchor scrolls its heading into view, a
// safe URL the view hands to OpenURL, the shell then opens it in the
// browser, and anything else is a file link the shell resolves against
// the document's directory. Unsafe schemes and unknown anchors do
// nothing.
func (st *State) dispatchLink(dest string) {
	if dest == "" {
		return
	}
	if anchor, ok := strings.CutPrefix(dest, "#"); ok {
		if anchor == "" {
			return
		}
		if block, ok := st.anchorLookup(anchor); ok {
			st.ScrollTargetBlock = block
		}
		return
	}
	if urlSafe(dest) {
		if st.OpenURL != nil {
			st.OpenURL(dest)
		}
		return
	}
	// Neither anchor nor scheme: a local path, markdown or not. Whether
	// the target exists is the shell's call, which knows the document's
	// directory and what it can open.
	if st.OpenLocalLink != nil {
		st.OpenLocalLink(dest)
	}
}

// urlSafe reports whether the view may hand url to OpenURL: http, https
// and mailto, the set the HTML whitelist keeps; javascript:, file:, data:
// and friends open nothing.
func urlSafe(url string) bool {
	l := strings.ToLower(url)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "mailto:")
}

// linkDestOf returns the destination of the link whose text covers rune
// offset off of block i's run, or "" when the offset sits outside any
// link. The walk flattens the inlines to the run text the offsets
// address, so it is the one authority for which runes a link owns.
func linkDestOf(m *doc.Model, i, off int) string {
	var dest string
	walk(m.Inlines(i), style{}, func(text string, kind runKind, link bool, d string) {
		if link && d != "" && off >= 0 && off < runeLen(text) {
			dest = d
		}
		// Every run advances the offset, link or not: the run text the
		// offsets address is all the runs joined.
		off -= runeLen(text)
	})
	return dest
}

// pointAt returns the document point under the handler's event and
// whether the view has rows to hit: the rows' handles give each block's
// rectangle of the committed frame, in window coordinates, the block's
// shaped text the rune. The event's position is the handler element's,
// the list's; the list's own rectangle turns it into a window point.
// Above or below the rows the point clamps to the nearest visible row's
// ends; a press clamps from above the window too, whose point the engine
// passes on below zero.
func (st *State) pointAt(m *doc.Model, ev ui.InputEvent, press bool) (selection.Point, bool) {
	if st.ctx == nil || len(st.rowHandles) == 0 {
		return selection.Point{}, false
	}
	listRect := st.List.Handle.Bounds(st.ctx)
	type hit struct {
		block int
		rect  ui.Rect
	}
	rows := make([]hit, 0, len(st.rowHandles))
	for block, handle := range st.rowHandles {
		rect := handle.Bounds(st.ctx)
		if rect.W <= 0 && rect.H <= 0 {
			continue
		}
		rows = append(rows, hit{block, rect})
	}
	if len(rows) == 0 {
		return selection.Point{}, false
	}
	slices.SortFunc(rows, func(a, b hit) int { return cmp.Compare(a.rect.Y, b.rect.Y) })
	x, y := listRect.X+ev.X, listRect.Y+ev.Y
	first, last := rows[0], rows[len(rows)-1]
	if y < first.rect.Y || press && y < 0 {
		return selection.Point{Block: first.block}, true
	}
	if y >= last.rect.Y+last.rect.H {
		return selection.Point{Block: last.block, Offset: runeLen(runText(m, last.block))}, true
	}
	for _, row := range rows {
		if y < row.rect.Y+row.rect.H {
			return selection.Point{Block: row.block, Offset: st.offsetAt(m, row.block, row.rect, listRect, x, y)}, true
		}
	}
	return selection.Point{Block: last.block, Offset: runeLen(runText(m, last.block))}, true
}

// offsetAt returns the rune offset of the window point in a row's block:
// the shaped run text maps the point into the text, past its ends to
// them.
func (st *State) offsetAt(m *doc.Model, block int, rect, listRect ui.Rect, x, y float32) int {
	text := runText(m, block)
	width := rect.W
	if width <= 0 {
		width = 10000
	}
	// A heading shapes at its level's size, the body at its setting;
	// rows move between the two as the document edits, so the size rides
	// the key.
	size := st.bodySize()
	if m.Block(block).Kind == parser.Heading {
		size = headingSize(st.bodySize(), m.Block(block).Level)
	}
	layout := st.layout(block, text, width, size)
	if layout == nil {
		return 0
	}
	u16 := layout.IndexAt(ui.Point{X: x - rect.X, Y: y - rect.Y})
	return runeOffset(text, u16)
}

// runeOffset converts a UTF-16 offset in text to a rune offset, which is
// what the selection addresses.
func runeOffset(text string, u16 int) int {
	return utf8.RuneCountInString(text[:ui.UTF16ByteOffset(text, u16)])
}

func runeLen(text string) int { return len([]rune(text)) }

// ScrollView builds the whole document: a virtual list of the blocks. The
// list keeps its own scroll place; use State.List to scroll it, and the
// Handler is wired to the list here, so dragging over the view selects
// text while the wheel and the keys still scroll. Each row binds its
// block's handle, which the handler's hit reads. The model's search
// state is the authority here: it syncs the view's copy each frame, so
// SetSearch and ClearSearch on the model show up without further wiring.
func ScrollView(c *ui.Context, m *doc.Model, st *State) {
	st.Matches = m.Matches
	st.Current = m.Current
	st.anchorModel = m
	// The glide writes the offset before the list is built, so its
	// TrackScroll takes the new place in this frame: a write after the list
	// exists reaches the container only after the list laid its rows out,
	// which writes the old place back.
	st.glide(c)
	list := ui.List(c, &st.List, m.Len(), func(r int) {
		st.Row(c, m, r)
	}).Grow(1).TrackScroll(&st.Scroll)
	st.updateScrollPlace(c, list)
	// The handler takes the pointer for the drag selection and the wheel
	// for its glide, and returns false for the rest, so the list keeps
	// scrolling what the handler leaves it: touchpad drags, sideways
	// wheels, and every notch the inertia setting is off for.
	list.HandleInput(st.Handler(m))
	if st.Menu != nil {
		list.ContextMenu(func(menu *ui.Menu) { st.documentMenu(menu) })
	}
	if st.AnchorOf == nil {
		st.AnchorOf = anchorsOf(m)
	}
	st.scrollPendingAnchor(m)
	st.scrollPendingMatch(m)
}

// glide advances the wheel's inertia by this frame's step and writes the
// distance to the offset, then asks for the frame that steps again. A move
// of the offset it did not write wins: anything else that scrolled the view
// since its last write (a jump, a List command, the keys, a thumb drag, a
// reload) ends the glide before the two fight.
func (st *State) glide(c *ui.Context) {
	if !st.Inertia.active() {
		return
	}
	if st.Scroll.Y != st.Inertia.wroteY {
		st.Inertia.stop()
		return
	}
	now := c.Now()
	dt := now.Sub(st.Inertia.last)
	if st.Inertia.last.IsZero() {
		dt = inertiaFirstStep
	} else if dt <= 0 {
		// The clock did not move since the last step, as a second build of
		// one frame: no time passed, so nothing moves.
		c.AnimationFrame()
		return
	}
	dt = min(dt, inertiaMaxStep)
	st.Inertia.last = now
	distance := st.Inertia.step(dt, st.ScrollSpeed)
	if distance != 0 {
		next := st.Scroll.Y + distance
		// Below a zero MaxY the container has not laid the content out yet,
		// and MaxY is still 0: clamping there would pin the offset at the
		// top and stop a glide that is only starting.
		if st.Scroll.MaxY > 0 {
			next = max(0, min(next, st.Scroll.MaxY))
		}
		if next == st.Scroll.Y {
			// The edge holds the view: nothing more of this glide lands.
			st.Inertia.stop()
		} else {
			st.Scroll.Y, st.Inertia.wroteY = next, next
		}
	}
	// The engine stops asking for frames once the event that started the
	// glide is handled: the glide asks for its own.
	c.AnimationFrame()
}

// updateScrollPlace reads the scroll place the last laid-out frame left
// in st.Scroll into the fractions the shell draws from: the bar's fill
// and the jump buttons' hiding. The values lag one frame behind the list
// they read, as the Rust version's read of scroll_offset does its own
// list; a frame's worth of lag at a wheel notch is invisible.
func (st *State) updateScrollPlace(c *ui.Context, list ui.Element) {
	if st.Scroll.MaxY <= 0 {
		st.Progress, st.AtTop, st.AtBottom = 0, true, true
		return
	}
	st.Progress = max(0, min(st.Scroll.Y/st.Scroll.MaxY, 1))
	st.AtTop = st.Scroll.Y <= 0.5
	st.AtBottom = st.Scroll.MaxY-st.Scroll.Y <= 0.5
}

// documentMenu fills the document area's context menu, in the Rust
// version's order: the editing commands, then the search and the reload,
// then the commands that act on the open file. The whole document is the
// menu's subject, wherever the click landed. The items stay the same with
// a nil action, which is what keeps the choice's place among them stable.
func (st *State) documentMenu(menu *ui.Menu) {
	a := st.Menu
	copyOn := a.HasSelection != nil && a.HasSelection()
	if menu.Item(locales.DocCopy()).Disabled(!copyOn).Chosen() {
		callAction(a.Copy)
	}
	if menu.Item(locales.DocSelectAll()).Chosen() {
		callAction(a.SelectAll)
	}
	menu.Separator()
	if menu.Item(locales.DocFind()).Chosen() {
		callAction(a.Find)
	}
	if menu.Item(locales.DocRefresh()).Chosen() {
		callAction(a.Refresh)
	}
	menu.Separator()
	if menu.Item(locales.DocRevealInSidebar()).Chosen() {
		callAction(a.Reveal)
	}
	if menu.Item(locales.DocOpenInFileManager()).Chosen() {
		callAction(a.OpenInFileManager)
	}
	if menu.Item(locales.DocCopyPath()).Chosen() {
		callAction(a.CopyPath)
	}
}

// callAction runs a menu action unless the shell left it nil.
func callAction(fn func()) {
	if fn != nil {
		fn()
	}
}

// scrollPendingMatch scrolls to the block the search's current match is
// in, once per value the search bar sets: it finds the row that shows
// the block and hands it to the list. It runs after the list built, so
// the row is measured when the frame lays out.
func (st *State) scrollPendingMatch(m *doc.Model) {
	if st.ScrollTargetBlock < 0 || st.ScrollTargetBlock == st.scrollTargetDone {
		return
	}
	st.scrollTargetDone = st.ScrollTargetBlock
	for r := range m.Len() {
		if m.RowBlock(r).Index == st.ScrollTargetBlock {
			st.List.ScrollIntoView(r)
			return
		}
	}
}

// Row builds the row of the r-th top-level block, binding the block's
// handle to it for the pointer's hit, then renders the block. ScrollView
// calls it; a caller that builds its own list calls it from its row
// callback the same way, so Handler's hit sees the rows. The handle is
// the block's across frames, so its Bounds answers between them. The
// column wrapper keeps the row semantics the list's row box gave the
// block: children stretch across.
func (st *State) Row(c *ui.Context, m *doc.Model, r int) {
	block := m.RowBlock(r).Index
	st.ctx = c
	if st.rowHandles == nil {
		st.rowHandles = map[int]*ui.Handle{}
	}
	handle := st.rowHandles[block]
	if handle == nil {
		handle = &ui.Handle{}
		st.rowHandles[block] = handle
	}
	// The right padding keeps the row's text clear of the scrollbar: the
	// shell pads the content's right edge 0 so the scrollbar hugs the
	// window's edge, and the clearance happens here instead. The bottom
	// padding is the gap every block-level row shares, the Rust version's
	// paragraph_gap: 1rem of air under each block. The handle binds the
	// padded column, whose width the hit shapes the text at.
	newColumn := func() ui.Element {
		col := ui.Column(c).Padding(0, rowClearance, blockGap, 0)
		col.Bind(handle)
		return col
	}
	if st.ContentWidth > 0 {
		// The row still spans the window, so the bar keeps to its edge;
		// only the column inside pulls in and centers. Grow with the cap,
		// not the cap alone: a column without a width shrinks to its
		// content, which would center each block by its own length. The
		// row is built before the column, which then attaches inside it.
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			newColumn().Grow(1).MaxWidth(st.ContentWidth).Children(func() {
				Render(c, m, st, block)
			})
		})
		return
	}
	newColumn().Children(func() {
		Render(c, m, st, block)
	})
}

// anchorsOf maps each heading block to its anchor, from the model's.
func anchorsOf(m *doc.Model) func(int) string {
	slugByBlock := map[int]string{}
	for _, h := range m.Headings() {
		slugByBlock[h.BlockIndex] = m.Anchor(h.BlockIndex)
	}
	return func(i int) string { return slugByBlock[i] }
}

// anchorIndex is the anchor lookups a linked image's "#anchor" needs:
// the model already answers it.
type anchorIndex interface {
	AnchorBySlug(slug string) (int, bool)
}

// Render builds the row of top-level block i.
func Render(c *ui.Context, m *doc.Model, st *State, i int) {
	b := m.Block(i)
	switch b.Kind {
	case parser.Heading:
		st.heading(c, m, i, b.Level)
	case parser.Paragraph:
		st.paragraph(c, m, i, style{})
	case parser.CodeBlock:
		st.codeBlock(c, m, i)
	case parser.Table:
		st.table(c, m, i)
	case parser.Quote:
		st.quote(c, m, i)
	case parser.List:
		st.list(c, m, i, b, 0)
	case parser.ThematicBreak:
		ui.Box(c).Height(2).Background(st.tokens(c).border).AlignSelf(ui.Stretch)
	case parser.HTMLBlock:
		st.htmlBlock(c, m, i)
	case parser.FootnoteDefinition:
		// The note's text renders here, its children in muted text; the
		// reference in the citing text is only the [^label] mark.
		st.mutedChildren(c, m, i)
	default:
		// A math block (no TeX typesetting, ticket #17), an unmapped or
		// unknown kind degrades to its source lines, so content never
		// vanishes into a blank row.
		st.sourceBlock(c, m, i)
	}
}

const (
	fontMono = "monospace"

	// defaultBodySize is the body's size without a setting: the Rust
	// reader's base, the font_size setting rides over it.
	defaultBodySize = float32(15)

	// blockGap is the air under every block-level row: the Rust version's
	// paragraph_gap, 1rem. A container spaces its own children, so only a
	// block-level element takes it.
	blockGap = float32(16)

	// rowClearance is the room a row keeps clear on the right for the
	// list's overlay scrollbar: the shell gives the content no right
	// padding so the bar hugs the window's edge, and the row carries the
	// clearance itself. The frontmatter card, which sits above the list
	// rather than in it, takes the same room as its right margin.
	rowClearance = float32(16)

	// codeBlockPadding is the room inside a code block's panel: the Rust
	// version's p_3.
	codeBlockPadding = float32(12)

	// inlineCodeScale shrinks inline code a little, a monospaced run
	// reading wider than the text around it at the same size;
	// inlineCodePadding and inlineCodeRadius are the chip's inline box.
	inlineCodeScale   = 0.95
	inlineCodePadding = float32(3)
	inlineCodeRadius  = float32(3)
	// inlineCodeTint is how far the accent tints the inline code's chip
	// off the page: the Rust version's accent highlight, kept readable on
	// the light and the dark page alike.
	inlineCodeTint = 0.12
	// quoteBarTint is how far the accent tints the quote's bar off the
	// border's gray: the Rust version's secondary_active, low in
	// saturation.
	quoteBarTint = 0.55

	// maxImageHeight is the height cap an image draws under, the Rust
	// version's max_h(px(2000)): a tall screenshot shows near whole
	// instead of pressing the page into a letterbox (the old 400 did).
	maxImageHeight = float32(2000)

	// tableBaseSize is the table text's size at the default body: both
	// docTokens.tableSize and tableColumnWidths measure scale from it.
	tableBaseSize = float32(14)

	// defaultMonoSize and defaultRadius stand in for a theme or a state
	// that names neither.
	defaultMonoSize = float32(13)
	defaultRadius   = float32(6)
)

// docTokens is the document's slice of the theme: the tokens the blocks
// draw with, mapped once instead of scattered over the builders. The
// Rust version reads the same roles out of gpui-component's Theme, whose
// token names differ from mygo's:
//
//	body text   theme.Text            (Rust foreground)
//	muted text  theme.TextMuted       (Rust muted_foreground)
//	panels      theme.Surface         (Rust muted: code blocks, table heads)
//	inline code accent tinted into the page (Rust accent)
//	quote bar   accent faded into the border (Rust secondary_active)
//	links       theme.Accent         (Rust link)
//	grid, rules theme.Border         (Rust border)
type docTokens struct {
	text     ui.Color
	muted    ui.Color
	panel    ui.Color
	inline   ui.Color
	quote    ui.Color
	link     ui.Color
	border   ui.Color
	radius   float32
	body     float32
	monoSize float32
	sans     string
	mono     string
}

// tokens maps the window's theme and the document's settings onto the
// document's roles.
func (st *State) tokens(c *ui.Context) docTokens {
	t := c.Theme()
	radius := t.Radius
	if radius <= 0 {
		radius = defaultRadius
	}
	monoSize := st.MonoFontSize
	if monoSize <= 0 {
		monoSize = defaultMonoSize
	}
	body := st.FontSize
	if body <= 0 {
		body = defaultBodySize
	}
	return docTokens{
		text:     t.Text,
		muted:    t.TextMuted,
		panel:    t.Surface,
		inline:   t.Background.Mix(t.Accent, inlineCodeTint),
		quote:    t.Border.Mix(t.Accent, quoteBarTint),
		link:     t.Accent,
		border:   t.Border,
		radius:   radius,
		body:     body,
		monoSize: monoSize,
		sans:     st.FontFamily,
		mono:     st.MonoFontFamily,
	}
}

// bodySize is the body's size without a context, for hit testing: the
// setting's, over the default.
func (st *State) bodySize() float32 {
	if st.FontSize > 0 {
		return st.FontSize
	}
	return defaultBodySize
}

// inlineCode dresses a span as inline code: a monospaced run in a rounded
// chip, the Rust version's highlight on the accent.
func (tk docTokens) inlineCode(sp ui.Span) ui.Span {
	sp.Font = tk.monoFamily()
	sp.Size *= inlineCodeScale
	sp.Background = tk.inline
	sp.Radius = inlineCodeRadius
	sp.PaddingX = inlineCodePadding
	return sp
}

// monoFamily is the family the mono text draws with: the setting's, or
// the hardcoded monospace default.
func (tk docTokens) monoFamily() string {
	if tk.mono != "" {
		return tk.mono
	}
	return fontMono
}

// tableSize is the table text's size: 14 at the default body, riding
// the body's scale as the headings do. tableColumnWidths measures at
// the same ratio.
func (tk docTokens) tableSize() float32 { return tableSizeAt(tk.body) }

// tableSizeAt is the table's font size for a body of the given size: the
// base table size scaled the way the headings scale, the one expression
// tableSize and tableColumnWidths share.
func tableSizeAt(body float32) float32 {
	return tableBaseSize * body / defaultBodySize
}

// style is the base style of a run of spans.
type style struct {
	size      float32
	weight    int
	color     ui.Color
	underline bool
	italic    bool
	strike    bool
}

func (st *State) heading(c *ui.Context, m *doc.Model, i, level int) {
	s := style{weight: 700}
	s.size, s.color = headingStyle(c, st, level)
	st.richParagraph(c, m, i, s, true)
}

func (st *State) paragraph(c *ui.Context, m *doc.Model, i int, s style) {
	st.richParagraph(c, m, i, s, false)
}

// headingSize is the font size of a heading's level, for a body of the
// given size. Ticket #19 keeps ticket #18's absolute 28/22/18/16/15/14
// (a body of 15) and scales them by body/15, so the whole page grows
// with the font_size setting: h1 is 1.87x, h2 1.47x, h3 1.2x, h4 1.07x,
// h5 1x and h6 0.93x of the body. A body of 15 reproduces the old
// numbers; headingStyle colors with the same function.
func headingSize(body float32, level int) float32 {
	if body <= 0 {
		body = defaultBodySize
	}
	scale := body / defaultBodySize
	var size float32
	switch level {
	case 1:
		size = 28
	case 2:
		size = 22
	case 3:
		size = 18
	case 4:
		size = 16
	case 5:
		size = 15
	default:
		size = 14
	}
	return size * scale
}

func headingStyle(c *ui.Context, st *State, level int) (float32, ui.Color) {
	size := headingSize(st.bodySize(), level)
	if level > 5 || level <= 0 {
		return size, c.Theme().TextMuted
	}
	return size, c.Theme().Text
}

// richParagraph builds block i's text as styled spans, heading when it is
// one, so it gets its role.
func (st *State) richParagraph(c *ui.Context, m *doc.Model, i int, s style, heading bool) {
	tk := st.tokens(c)
	if s.size == 0 {
		s.size = tk.body
	}
	if s.weight == 0 {
		s.weight = 400
	}
	if s.color == (ui.Color{}) {
		s.color = tk.text
	}
	if images := m.Images(i); len(images) > 0 {
		st.images(c, i, images)
		return
	}
	spans := st.spans(c, m, i, s)
	el := ui.RichText(c, spans...).Font(tk.sans).FontFeatures("tnum").AlignSelf(ui.Stretch).MinWidth(0)
	if heading {
		el.Role(ui.RoleHeading).Level(m.Block(i).Level)
	}
}

// spans walks block i's inlines into styled spans, drawing the selection
// and the matches over the text.
func (st *State) spans(c *ui.Context, m *doc.Model, i int, s style) []ui.Span {
	tk := st.tokens(c)
	spans := []ui.Span{}
	walk(m.Inlines(i), s, func(text string, kind runKind, link bool, dest string) {
		if text == "" {
			return
		}
		base := s
		if base.color == (ui.Color{}) {
			base.color = tk.text
		}
		if link {
			// A link reads as ui.Link does, the accent color underlined;
			// full link interaction is later work.
			base.color = tk.link
			base.underline = true
		}
		sp := ui.Span{Text: text, Size: base.size, Weight: base.weight, Color: base.color, Underline: base.underline}
		switch kind {
		case runCode:
			// Code is the chip: the Rust version highlights it on the
			// theme's accent.
			sp = tk.inlineCode(sp)
		case runLiteral:
			// Inline HTML, math and a footnote's mark are source, not
			// prose: muted, a touch smaller, no chip.
			sp.Color = tk.muted
			sp.Size *= inlineCodeScale
		}
		spans = append(spans, sp)
	})
	return st.overlay(c, m, i, spans)
}

// runKind says how a run of inline text reads.
type runKind uint8

const (
	// runPlain is prose, the body text.
	runPlain runKind = iota
	// runCode is inline code: a monospaced run on its chip.
	runCode
	// runLiteral is source the view shows but does not typeset: inline
	// HTML, math, a footnote's mark. It reads muted.
	runLiteral
)

// walk flattens inlines to runs of text in style s, marking code, literals
// and links. The spans' texts joined are the block's run text in doc's run
// table, rune for rune: the offsets of the selection, the cuts and the
// search all address that text. LineBreak and FootnoteRef follow
// parser.PlainText (a break is a space, a note is "[^label]"); an
// image's alt text stays in the text, as PlainText's flat walk over the
// inline array emits it too.
func walk(in []parser.Inline, s style, fn func(text string, kind runKind, link bool, dest string)) {
	walkRange(in, 0, len(in), s, fn)
}

// walkRange walks in[from:to) as one subtree in style s. A node's Next
// names the flat array the parser returned for the whole block, not the
// slice a recursion holds, so the children's range reads the absolute
// Next and clamps it to the subtree's end: a nested image inside a link,
// [![a](b)](c), keeps the flat Next of its parent's end, and slicing a
// slice off it with the raw Next both panics and loses nodes. An end
// past the subtree's own (a malformed table the streaming parser has
// not healed yet) clamps too, so a bad index can never reach out of the
// array the walk was handed.
func walkRange(in []parser.Inline, from, to int, s style, fn func(text string, kind runKind, link bool, dest string)) {
	for k := from; k < to; k++ {
		n := &in[k]
		end := n.Next
		if end > to {
			end = to
		}
		if end < k {
			end = k
		}
		switch n.Kind {
		case parser.Text:
			fn(n.Text, runPlain, false, "")
		case parser.SoftBreak:
			fn(" ", runPlain, false, "")
		case parser.LineBreak:
			fn(" ", runPlain, false, "")
		case parser.Code:
			fn(n.Text, runCode, false, "")
		case parser.Emphasis:
			walkRange(in, k+1, end, style{size: s.size, weight: s.weight, color: s.color, underline: s.underline}, fn)
		case parser.Strong:
			walkRange(in, k+1, end, style{size: s.size, weight: 700, color: s.color, underline: s.underline}, fn)
		case parser.Link:
			walkRange(in, k+1, end, style{size: s.size, weight: s.weight, color: s.color, underline: s.underline}, func(text string, kind runKind, link bool, dest string) {
				fn(text, kind, true, n.Dest)
			})
		case parser.Image:
			// Its description stands for it within the text.
			walkRange(in, k+1, end, style{size: s.size, weight: s.weight, color: s.color, underline: s.underline}, func(text string, kind runKind, link bool, dest string) {
				fn(text, kind, false, "")
			})
		case parser.HTML, parser.Math:
			fn(n.Text, runLiteral, false, "")
		case parser.FootnoteRef:
			fn("[^"+n.Text+"]", runLiteral, false, "")
		}
		if n.Next > k {
			k = n.Next - 1
		}
	}
}

// overlay splits a block's spans where the selection and the matches cut
// its text, coloring the covered runs. The block i handed in is the leaf
// the row draws, often a cell or a quote's paragraph; its run and the
// leaf's offset in it come from the run table.
func (st *State) overlay(c *ui.Context, m *doc.Model, i int, spans []ui.Span) []ui.Span {
	leaf, off := runBase(m, i)
	return st.overlayRun(c, leaf, off, spans)
}

// overlayRun recolors spans where the selection and the matches cut the
// run of top-level block leaf at rune offset off, which a nested piece
// that draws one slice of the run carries in as its own offset.
func (st *State) overlayRun(c *ui.Context, leaf, off int, spans []ui.Span) []ui.Span {
	var cuts []cut
	if st.Selection.set {
		start, end := st.Selection.start, st.Selection.end
		// A leaf between the endpoints is wholly selected: its cut spans
		// the leaf's whole text, which splitSpans clamps to the leaf's
		// ends. Only the endpoint leaves cut at their endpoint's offset.
		// Offsets shift by off first, per-leaf as before.
		if start.Block <= leaf && leaf <= end.Block && start.Before(end) {
			from, to := 0, math.MaxInt
			if leaf == start.Block {
				from = start.Offset
			}
			if leaf == end.Block {
				to = end.Offset
			}
			cuts = append(cuts, cut{from, to, cutSelection})
		}
	}
	for k, match := range st.Matches {
		if match.Block != leaf {
			continue
		}
		kind := cutMatch
		if k == st.Current {
			kind = cutCurrent
		}
		cuts = append(cuts, cut{match.Start, match.End, kind})
	}
	if len(cuts) == 0 {
		return spans
	}
	// Shift the run coordinates into the leaf's; splitSpans clamps a
	// cut that reaches past the leaf's ends to them.
	shifted := cuts[:0]
	for _, c := range cuts {
		c.from -= off
		c.to -= off
		if c.to > 0 {
			shifted = append(shifted, c)
		}
	}
	if len(shifted) == 0 {
		return spans
	}
	return splitSpans(spans, shifted, c.Theme())
}

// cut is a run of a block's text to color.
type cut struct {
	from, to int
	kind     cutKind
}

type cutKind uint8

const (
	cutSelection cutKind = iota
	cutMatch
	cutCurrent
)

// splitSpans recolors the spans where the cuts are. The spans' texts joined
// are the block's text, so cuts address runes of that joined text. Its
// spans carry no per-span TextColor: a color set on the element would
// not move with the recolored pieces.
func splitSpans(spans []ui.Span, cuts []cut, t *ui.Theme) []ui.Span {
	if len(spans) == 0 {
		return spans
	}
	// Flatten to one text with a style per span boundary, then re-emit.
	type piece struct {
		sp    ui.Span
		runes []rune
	}
	pieces := make([]piece, 0, len(spans))
	at := 0
	bounds := []int{0}
	for _, sp := range spans {
		r := []rune(sp.Text)
		pieces = append(pieces, piece{sp, r})
		at += len(r)
		bounds = append(bounds, at)
	}
	total := at
	out := make([]ui.Span, 0, len(spans)+len(cuts)*2)
	for _, c := range cuts {
		if c.from < 0 {
			c.from = 0
		}
		if c.to > total {
			c.to = total
		}
		if c.from >= c.to {
			continue
		}
	}
	segments := mergeCuts(cuts, total)
	for pi, p := range pieces {
		from, to := bounds[pi], bounds[pi+1]
		for start := from; start < to; {
			end := to
			color := p.sp.Color
			background := ui.Color{}
			for _, seg := range segments {
				if seg.to <= start || seg.from >= to {
					continue
				}
				if seg.from > start {
					end = min(end, seg.from)
					break
				}
				end = min(end, seg.to)
				color, background = seg.color(t)
				break
			}
			sp := p.sp
			sp.Text = string(p.runes[start-from : end-from])
			if background != (ui.Color{}) {
				sp.Color, sp.Background, sp.Radius = color, background, 3
			}
			out = append(out, sp)
			start = end
		}
	}
	return out
}

// segment is a colored run of the block's text.
type segment struct {
	from, to int
	kind     cutKind
}

func (s segment) color(t *ui.Theme) (ui.Color, ui.Color) {
	switch s.kind {
	case cutSelection:
		return t.AccentText, t.Selection
	case cutCurrent:
		return t.AccentText, t.Accent
	default:
		return t.Text, t.Focus
	}
}

// cutPriority ranks the overlays when they overlap: the current match
// over the selection, the selection over the other matches.
func cutPriority(k cutKind) int {
	switch k {
	case cutCurrent:
		return 3
	case cutSelection:
		return 2
	default:
		return 1
	}
}

// mergeCuts sorts the cuts by priority and clips the overlaps: where
// cuts cover the same runes the higher-priority one wins and the lower
// keeps only what the higher leaves, so a selection over the current
// match still shows the match around it.
func mergeCuts(cuts []cut, total int) []segment {
	slices.SortStableFunc(cuts, func(a, b cut) int {
		if p := cutPriority(b.kind) - cutPriority(a.kind); p != 0 {
			return p
		}
		return cmp.Compare(a.from, b.from)
	})
	var out []segment
	for _, c := range cuts {
		c.from = max(c.from, 0)
		c.to = min(c.to, total)
		if c.from >= c.to {
			continue
		}
		// The segments already taken outrank c; carve them out of it.
		at := c.from
		for _, s := range out {
			if s.to <= at || s.from >= c.to {
				continue
			}
			if s.from > at {
				out = append(out, segment{at, s.from, c.kind})
			}
			at = max(at, s.to)
		}
		if at < c.to {
			out = append(out, segment{at, c.to, c.kind})
		}
	}
	slices.SortFunc(out, func(a, b segment) int { return cmp.Compare(a.from, b.from) })
	return out
}

// images builds a row of pictures for the images of block i. An image
// a link wraps opens the link on a click, cursor and tooltip saying so;
// a bare picture does nothing.
func (st *State) images(c *ui.Context, i int, images []doc.Image) {
	ui.Row(c).Gap(8).AlignSelf(ui.Stretch).Children(func() {
		for _, img := range images {
			src := st.source(img.Dest)
			if src != nil {
				image := ui.Image(c, src).MaxWidthPercent(100).MaxHeight(maxImageHeight).Fit(ui.Contain).Radius(4)
				if img.Width > 0 && img.Height > 0 {
					// The HTML img's width/height attributes fix the box;
					// the max caps stay for the attrs' absence.
					image = image.Size(img.Width, img.Height)
				}
				if img.Link != "" {
					tip := img.Link
					image.Cursor(ui.CursorPointer).Tooltip(tip).OnClick(func() {
						st.openImageLink(img.Link)
					})
				}
				continue
			}
			label := img.Alt
			if label == "" {
				label = img.Dest
			}
			text := ui.Text(c, label).FontSize(st.tokens(c).monoSize).TextColor(st.tokens(c).muted)
			if img.Link != "" {
				text.Cursor(ui.CursorPointer).Tooltip(img.Link)
			}
		}
	})
}

// openImageLink opens a linked image's target as a text link does: the
// same dispatch as openLinkAt, minus the hit testing, which the image
// element has done.
func (st *State) openImageLink(dest string) {
	st.dispatchLink(dest)
}

// anchorLookup resolves a slug to a block through the anchor index, when
// one is in place; a bare State misses and the anchor stays dead.
func (st *State) anchorLookup(slug string) (int, bool) {
	if st.AnchorOf == nil || st.anchorModel == nil {
		return 0, false
	}
	return st.anchorModel.AnchorBySlug(slug)
}

// source loads and caches the image at url.
func (st *State) source(url string) ui.ImageSource {
	if st.LoadImage == nil {
		return nil
	}
	if src, ok := st.imageCache[url]; ok {
		return src
	}
	src, err := st.LoadImage(url)
	if st.imageCache == nil {
		st.imageCache = map[string]ui.ImageSource{}
	}
	if err != nil || src == nil {
		st.imageCache[url] = nil
		return nil
	}
	st.imageCache[url] = src
	return src
}

// codeBlock builds a fenced or indented code block, its lines tokenized by
// the language the fence names. The lines are one run in doc's run table,
// the text the search's matches address, so the matches cut the joined
// lines through the same overlay the paragraphs draw with.
func (st *State) codeBlock(c *ui.Context, m *doc.Model, i int) {
	t := c.Theme()
	ct := st.CodeTheme
	if ct == nil {
		ct = editor.DefaultTheme(t)
	}
	lang := editor.LanguageNamed(m.Language(i))
	// A mermaid block with a loader draws its diagram in the loader's
	// place; on failure it stays the source block, an error line above
	// saying why. Search hits in the source highlight only there: a
	// diagram carries no text to cut, so they stay counted but invisible
	// over the picture, a known limit. CJK tofu is fixed on Windows by
	// injecting system Microsoft YaHei (internal/mermaid/font.go); other
	// platforms may still tofu (TODO macOS PingFang, Linux fontconfig).
	if isMermaid(m.Language(i)) && st.LoadMermaid != nil {
		b, err := st.mermaidBitmap(strings.Trim(m.Lines(i), "\n"), mermaidTheme(c), mermaidScale(st))
		if err == nil {
			st.mermaidImage(c, b)
			return
		}
		// A failed diagram shows the three Rust-branch facts, the header,
		// the error and the kept source, then falls through to the block.
		mermaidErrorPanel(c, err)
	}
	lines := splitLines(m.Lines(i))
	spans := make([]ui.Span, 0, len(lines))
	for k, line := range lines {
		if k > 0 {
			spans = append(spans, ui.Span{Text: "\n"})
		}
		var toks []editor.Token
		if lang != nil && lang.Tokenizer != nil {
			toks, _ = lang.Tokenizer.Tokenize(line, 0, nil)
		}
		spans = append(spans, codeSpans(line, toks, ct, t.Text)...)
	}
	st.monoPanel(c, i, strings.TrimRight(m.Lines(i), "\n"), spans)
}

// monoPanel wraps spans in the surface box the source-like blocks share:
// a code block's tokenized lines, a math block's TeX, the source of any
// kind the view does not map. text is what the copy button copies; an
// empty text hides the button, and so does a view without CopyText.
func (st *State) monoPanel(c *ui.Context, i int, text string, spans []ui.Span) {
	tk := st.tokens(c)
	// The bottom padding sits inside the scroller rather than on the panel:
	// the horizontal scroll bar overlays the scroll box's last 9 DIPs, and
	// the block's whole bottom padding puts the last line above it instead
	// of under the thumb. The panel keeps its 12 DIPs either way.
	box := ui.Column(c).Background(tk.panel).Radius(tk.radius).
		Padding(codeBlockPadding, codeBlockPadding, 0, codeBlockPadding).
		AlignSelf(ui.Stretch).MinWidth(0)
	box.Children(func() {
		st.copyButton(c, i, text)
		// The line scrolls sideways rather than wrapping: a wrapped line
		// breaks the code's shape, and the source keeps its own line
		// breaks. A line that fits leaves the scroller idle.
		ui.ScrollHorizontal(c).Label("Code block").AlignSelf(ui.Stretch).Direction(ui.LeftToRight).Children(func() {
			col := ui.Column(c).Shrink(0).Padding(0, 0, codeBlockPadding, 0)
			col.Children(func() { st.monoBox(c, i, 0, spans) })
		})
	})
}

// monoBox lays spans out as one monospaced text in a surface box, cutting
// the overlays into them at off, the leaf's base in its run. NoWrap keeps
// every line whole: the panel around it scrolls the long ones.
func (st *State) monoBox(c *ui.Context, i, off int, spans []ui.Span) {
	if len(spans) == 0 {
		spans = append(spans, ui.Span{Text: " "})
	}
	tk := st.tokens(c)
	spans = st.overlayRun(c, i, off, spans)
	ui.RichText(c, spans...).Font(tk.monoFamily()).FontSize(tk.monoSize).
		MinWidth(0).NoWrap().Shrink(0)
}

// copyFeedback is how long a code block's copy button shows that it
// copied, as the Rust version's two seconds.
const copyFeedback = 2 * time.Second

// copyButton builds the code block's copy button in the panel's top-right
// corner, off the layout so the code keeps its place under it. It copies
// text through CopyText, shows the accent check for copyFeedback, then
// asks for the frame that puts it back. The label stays for the tester
// and assistive technology; the tooltip names the action, which the icon
// alone no longer says.
func (st *State) copyButton(c *ui.Context, i int, text string) {
	if st.CopyText == nil || text == "" {
		return
	}
	tk := st.tokens(c)
	copied := st.copied(i, c)
	label := "Copy"
	tip := locales.CopyCodeTooltip()
	color := tk.muted
	if copied {
		label, tip, color = locales.Copied(), locales.Copied(), c.Theme().AccentText
	}
	b := ui.ButtonBase(c).Label(label).Padding(4).Radius(5).
		AlignItems(ui.Center).Absolute().Right(6).Top(6).Tooltip(tip)
	if st.copyHandles == nil {
		st.copyHandles = map[int]*ui.Handle{}
	}
	if h := st.copyHandles[i]; h != nil {
		b.Bind(h)
	} else {
		h := &ui.Handle{}
		st.copyHandles[i] = h
		b.Bind(h)
	}
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	b.Children(func() {
		icon := copyIcon
		if copied {
			icon = checkIcon
		}
		ui.Icon(c, icon).Size(12, 12).TextColor(color)
	})
	if b.Clicked() {
		st.CopyText(text)
		st.copiedBlock, st.copiedAt = i, c.Now()
		c.After(copyFeedback)
	}
}

// copied reports whether block i's copy button still shows its feedback.
func (st *State) copied(i int, c *ui.Context) bool {
	return !st.copiedAt.IsZero() && st.copiedBlock == i && c.Now().Sub(st.copiedAt) < copyFeedback
}

// codeSpans splits one line of code into spans, one per token, colored
// by the code theme; a run the tokens leave takes the base color.
func codeSpans(line string, toks []editor.Token, ct *editor.Theme, base ui.Color) []ui.Span {
	spans := []ui.Span{}
	at := 0
	for _, tok := range toks {
		start, end := max(at, 0), min(tok.End, len(line))
		if end > start {
			spans = append(spans, ui.Span{Text: line[start:end], Color: ct.TokenColor(tok.Kind)})
		}
		at = end
	}
	if at < len(line) {
		spans = append(spans, ui.Span{Text: line[at:], Color: base})
	}
	return spans
}

func splitLines(s string) []string {
	s = strings.Trim(s, "\n")
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

// sourceBlock renders block i's raw lines as plain monospaced gray text:
// an unmapped or unstyled block kind falls back to its source.
func (st *State) sourceBlock(c *ui.Context, m *doc.Model, i int) {
	text := strings.TrimRight(m.Lines(i), "\n")
	spans := []ui.Span{{Text: text, Color: st.tokens(c).muted}}
	st.monoPanel(c, i, text, spans)
}

// mutedChildren draws the blocks nested in i in muted text, the note of
// a footnote definition, styled like a quote's text without its bar.
func (st *State) mutedChildren(c *ui.Context, m *doc.Model, i int) {
	tk := st.tokens(c)
	col := ui.Column(c).AlignSelf(ui.Stretch).MinWidth(0)
	col.Children(func() {
		for j := range m.Children(i) {
			if m.Block(j).Kind == parser.Paragraph {
				st.paragraph(c, m, j, style{color: tk.muted})
			} else {
				Render(c, m, st, j)
			}
		}
	})
}

// table builds a Markdown table as a grid of rows: a cell per column,
// ruled along every row and between every column, the header on a surface
// at weight 600. The grid takes the width its columns measure, not the
// row's: the Rust version draws a table at its content's width and leaves
// the page's right side empty rather than stretching short cells apart.
// A split table's slice renders as a table of its own: the header it
// carries opens bold on a surface, only the first slice carries one, and
// a "rows N-M of T" note closes a slice when the table has more slices.
// Slices measure their columns independently, so the widths can drift a
// little between slices of one table; the header repeats, which keeps the
// drift readable.
func (st *State) table(c *ui.Context, m *doc.Model, i int) {
	tk := st.tokens(c)
	widths := tableColumnWidths(st, m, i)
	// The centered column leaves the table that room; mygo does not clamp
	// a fixed-width cell inside a MaxWidth'd grid, so a 4-column table's
	// ~1205 DIPs would spill past the column's right edge (shot: the last
	// column drawn outside the centered 900). Shrink the ratios into the
	// room, and the longest columns wrap harder instead of escaping.
	if room := st.ContentWidth - 2*rowClearance; room > 0 {
		widths = shrinkWidths(widths, room-float32(len(widths))-2)
	}
	// The frame owns the radius and clips its rows to it: the header's
	// surface is one band across the row, so its square corners would
	// otherwise poke out of the frame's rounded ones.
	grid := ui.Column(c).Border(1, tk.border).Radius(tk.radius).Clip().
		MaxWidth(tableGridWidth(widths)).AlignSelf(ui.Start).MinWidth(0)
	aligns := m.Block(i).Align
	grid.Children(func() {
		rows := m.TableRows(i)
		for r, row := range rows {
			head, last := r == 0, r == len(rows)-1
			line := ui.Row(c)
			if head {
				// One background for the whole row, so the header reads as
				// a band: the cells carry no fill of their own.
				line.Background(tk.panel)
			}
			if !last {
				// The row's grid line; the last row's is the grid's own
				// bottom border.
				line.BorderWidth(0, 0, 1, 0).BorderColor(tk.border)
			}
			line.Children(func() {
				cells := m.TableCells(row)
				for k, cell := range cells {
					s := style{}
					if head {
						s.weight = 600
					}
					spans := st.spans(c, m, cell, s)
					// A Row centers its children, which would leave a cell as high
					// as its own text and break its rule at every shorter cell:
					// stretched, the cells fill the row's height instead.
					box := ui.Box(c).Width(widths[min(k, len(widths)-1)]).Padding(6, 10).
						AlignSelf(ui.Stretch)
					if k < len(cells)-1 {
						box.BorderWidth(0, 1, 0, 0).BorderColor(tk.border)
					}
					align := columnAlign(aligns, k)
					box.Children(func() {
						ui.RichText(c, spans...).Font(tk.sans).FontSize(tk.tableSize()).
							MinWidth(0).AlignSelf(ui.Stretch).TextAlign(align)
					})
				}
			})
		}
		if b := m.Block(i); b.TableSlice && b.TableSliceFirst > 0 {
			from, to := b.TableSliceFirst+1, b.TableSliceFirst+len(rows)-1
			ui.Text(c, fmt.Sprintf("rows %d-%d of %d", from, to, b.TableTotalRows)).
				TextColor(tk.muted).FontSize(tk.monoSize).Padding(4, 10).
				BorderWidth(1, 0, 0, 0).BorderColor(tk.border)
		}
	})
}

// tableGridWidth is the width a table's grid draws at: its columns plus
// their right borders, inside the grid's own two side borders.
func tableGridWidth(widths []float32) float32 {
	w := float32(2)
	for _, cw := range widths {
		w += cw + 1
	}
	return w
}

// tableColumnWidths measures a table's cells to size each column to its
// widest cell, one shared width per column, so rows line up instead of
// stretching to share the row. A cell measures its plain text at the
// table's 14 with 20 for the padding and 1 for the column's right rule,
// capped at 300; a header cell measures bold, as it renders at weight 600,
// and a table without text takes the default 80.
func tableColumnWidths(st *State, m *doc.Model, table int) []float32 {
	size := tableSizeAt(st.bodySize())
	widths := []float32{}
	for r, row := range m.TableRows(table) {
		weight := 400
		if r == 0 {
			weight = 600
		}
		for k, cell := range m.TableCells(row) {
			w := float32(81)
			if text := m.Plain(cell); text != "" {
				tw, _ := ui.ShapeText(text, ui.Font{Family: st.FontFamily, Size: size, Weight: weight}, 10000).Size()
				w = tw + 21
			}
			w = min(w, 300)
			for len(widths) <= k {
				widths = append(widths, 0)
			}
			widths[k] = max(widths[k], w)
		}
	}
	return widths
}

// shrinkWidths scales the columns down to fit the budget, keeping their
// ratios; a budget below the columns' count leaves them alone.
func shrinkWidths(widths []float32, budget float32) []float32 {
	total := float32(0)
	for _, w := range widths {
		total += w
	}
	if total <= budget || total <= 0 {
		return widths
	}
	scale := budget / total
	out := make([]float32, len(widths))
	for i, w := range widths {
		out[i] = w * scale
	}
	return out
}

// columnAlign maps a table column's delimiter row, :---: / ---: / :---,
// onto the alignment its cells draw with; a column past the delimiter row
// and one without colons read left, the markdown default.
func columnAlign(aligns []parser.Align, k int) ui.Align {
	if k >= len(aligns) {
		return ui.Start
	}
	switch aligns[k] {
	case parser.AlignCenter:
		return ui.Center
	case parser.AlignRight:
		return ui.End
	default:
		return ui.Start
	}
}

// quote builds a block quote with its bar.
func (st *State) quote(c *ui.Context, m *doc.Model, i int) {
	tk := st.tokens(c)
	box := ui.Row(c).PaddingStart(16).AlignSelf(ui.Stretch).MinWidth(0)
	box.BorderStartWidth(3).BorderColor(tk.quote)
	box.Children(func() {
		col := ui.Column(c).Gap(4).Grow(1).MinWidth(0)
		col.Children(func() {
			for _, j := range m.QuoteBlocks(i) {
				if m.Block(j).Kind == parser.Paragraph {
					st.paragraph(c, m, j, style{color: tk.muted})
				} else {
					Render(c, m, st, j)
				}
			}
		})
	})
}

// list builds a list, its items with markers or numbers, nested one level
// deeper. The nesting column already shifts each level past its parent,
// the Rust version's ml(rems(1.)) per nested list; depth drives the
// markers, as its list_item_prefix does: bullets step through
// listBullets, an ordered top level keeps numbers and its descendants
// run through letters. Task boxes take the level's marker width so the
// text lines up as it does beside a bullet.
func (st *State) list(c *ui.Context, m *doc.Model, i int, b doc.Block, depth int) {
	tk := st.tokens(c)
	col := ui.Column(c).Gap(2).AlignSelf(ui.Stretch).MinWidth(0)
	col.Children(func() {
		number := b.Number
		for _, item := range m.ListItems(i) {
			marker, _ := listPrefix(b.Ordered, &number, depth)
			row := ui.Row(c).Gap(4).AlignSelf(ui.Stretch).MinWidth(0).AlignItems(ui.Baseline)
			row.Children(func() {
				if item.Task != parser.NoTask {
					// A task item's box stands in for the bullet, as the Rust
					// version's checkbox replaces the marker.
					st.taskBox(c, item.Task, markerWidth(marker))
				} else {
					ui.Text(c, marker).Width(markerWidth(marker)).TextColor(tk.muted).TextAlign(ui.End)
				}
				content := ui.Column(c).Gap(2).Grow(1).MinWidth(0)
				content.Children(func() {
					for j := range m.Children(item.Index) {
						child := m.Block(j)
						switch child.Kind {
						case parser.Paragraph:
							spans := st.spans(c, m, j, style{})
							ui.RichText(c, spans...)
						case parser.List:
							st.list(c, m, j, child, depth+1)
						default:
							Render(c, m, st, j)
						}
					}
				})
			})
		}
	})
}

// listBullets is the bullet of each nesting level, the Rust version's
// BULLETS. A level past the last keeps the last one's bullet.
var listBullets = [5]string{"▪", "•", "◦", "‣", "⁃"}

// listUpper and listLower name an ordered list's nested levels, the Rust
// version's NUMBERED_PREFIXES.
const (
	listUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	listLower = "abcdefghijklmnopqrstuvwxyz"
)

// listPrefix is the marker of an item at depth, advancing *number: the
// item's position. Numbers at the top, cycling letters below, bullets
// elsewhere.
func listPrefix(ordered bool, number *int, depth int) (string, int) {
	if ordered {
		var s string
		ix := *number - 1
		switch {
		case depth <= 0:
			s = strconv.Itoa(*number) + "."
		case depth == 1:
			s = string(listUpper[ix%len(listUpper)]) + "."
		default:
			s = string(listLower[ix%len(listLower)]) + "."
		}
		*number++
		return s, 28
	}
	level := min(depth, len(listBullets)-1)
	return listBullets[level], 18
}

// markerWidth is the width a marker's column takes: room for the widest
// form the level's series reaches, so "10." and "27." keep their text
// lined up with single-digit rows.
func markerWidth(marker string) float32 {
	if len(marker) > 3 {
		return 28
	}
	return 18
}

// taskBoxSize is the task item's check box, a square the border frames
// when open and the accent fills with a check when done.
const taskBoxSize = float32(12)

// taskBox draws a task item's check box in the marker column, the width
// the list gives its markers so the item text lines up with a bulleted
// item's. The box aligns by its baseline, which an empty box takes from
// its bottom: it sits on the item's first line, not the row's bottom.
func (st *State) taskBox(c *ui.Context, task parser.Task, width float32) {
	tk := st.tokens(c)
	col := ui.Box(c).Width(width).AlignSelf(ui.Baseline)
	col.Children(func() {
		box := ui.Box(c).Size(taskBoxSize, taskBoxSize).Radius(3).Shrink(0)
		if task != parser.Checked {
			box.Border(1, tk.border)
			return
		}
		box.Background(tk.link).AlignItems(ui.Center).Justify(ui.Center)
		box.Children(func() {
			ui.Icon(c, checkIcon).Size(9, 9).TextColor(c.Theme().AccentText)
		})
	})
}

// pendingAnchorTries is the frames a pending cross-document anchor
// waits for its heading before the view gives up on it.
const pendingAnchorTries = 3

// ResetPendingAnchor arms slug as the cross-document anchor to scroll
// to once the document holds the heading, clearing any earlier wait.
func (st *State) ResetPendingAnchor(slug string) {
	st.PendingAnchor = slug
	st.pendingAnchorMisses = 0
}

// scrollPendingAnchor asks the list to scroll to the block the anchor
// names, once per anchor or version bump. A pending cross-document
// anchor resolves first: a hit promotes it to Anchor, a miss rides
// until pendingAnchorTries then drops.
func (st *State) scrollPendingAnchor(m *doc.Model) {
	if st.PendingAnchor != "" {
		slug := st.PendingAnchor
		if _, ok := m.AnchorBySlug(slug); ok {
			st.Anchor = slug
			st.PendingAnchor = ""
		} else if st.pendingAnchorMisses++; st.pendingAnchorMisses > pendingAnchorTries {
			st.PendingAnchor = ""
		}
	}
	if st.Anchor == "" || st.AnchorOf == nil {
		return
	}
	key := st.Anchor + "@" + strconv.Itoa(int(st.AnchorVersion))
	if st.anchorDone == key {
		return
	}
	for r, b := range m.Blocks() {
		if slug := st.AnchorOf(b.Index); slug == st.Anchor {
			st.List.ScrollIntoView(r)
			st.anchorDone = key
			return
		}
	}
}

// layoutKey is a layout cache's key: the block, the width and the font
// size the text was shaped at. The size is in the key because a heading
// reshapes at its own size, not the body's, and rows move between them
// as the document edits.
type layoutKey struct {
	block int
	width float32
	size  float32
}

// textLayout is a shaped block of text with the key it was shaped at.
type textLayout struct {
	layout *ui.TextLayout
	width  float32
}

// runText returns the text of top-level block i as the run table holds
// it, falling back to Plain for a block without a run of its own.
func runText(m *doc.Model, block int) string {
	if r := m.RunOf(block); r != nil {
		return r.Text
	}
	return m.Plain(block)
}

// layout shapes a block's text at a row's width and font size, caching
// by block, width and size: the selection addresses runes, so the text
// it shapes is the run table's.
func (st *State) layout(block int, text string, width, size float32) *ui.TextLayout {
	if st.layouts == nil {
		st.layouts = map[layoutKey]*textLayout{}
	}
	key := layoutKey{block: block, width: width, size: size}
	if entry := st.layouts[key]; entry != nil && entry.width == width {
		return entry.layout
	}
	layout := ui.ShapeText(text, ui.Font{Size: size}, width)
	st.layouts[key] = &textLayout{layout: layout, width: width}
	return layout
}

// RowRect returns the window rectangle the committed frame drew block i's
// row in, and whether there is one: the outline reads it to highlight the
// heading the view is scrolled to. Valid only while the shell builds a
// frame (the hook's context lives), like Bounds of any handle.
func (st *State) RowRect(block int) (ui.Rect, bool) {
	if st.ctx == nil || st.rowHandles == nil {
		return ui.Rect{}, false
	}
	h, ok := st.rowHandles[block]
	if !ok || h == nil {
		return ui.Rect{}, false
	}
	return h.Bounds(st.ctx), true
}

// HasSelection reports a selection to draw or copy: a non-empty one.
func (st *State) HasSelection() bool {
	return st.Selection.set && st.Selection.start != st.Selection.end
}

// SelectionText returns the selected text of the document.
func (st *State) SelectionText(m *doc.Model) string {
	if !st.Selection.set {
		return ""
	}
	s := selection.Selection{Anchor: st.Selection.start, Caret: st.Selection.end}
	return s.TextOf(m)
}

// htmlBlock renders an embedded HTML block as the semantic content its
// tags hold, the way the markdown blocks around it draw: paragraphs and
// headings as rich text, tables as grids, pre as code, hr as a rule and
// img as a picture. Tags outside the whitelist the parser prunes or
// flattens to their text. The run table holds the block's visible text,
// joined with newlines, so the overlays cut it node by node: each piece
// hands overlayRun its slice's rune offset in the run, from the leaf's
// base when the block sits nested in a quote or a list, and the run's
// block, the quote or list the matches and the selection address.
// Zero-text nodes draw but take no slot in the run: a rule's line, an
// image without alt. The block reparses once a frame, HTML blocks being
// few; the run table caches the same text through doc.RunText for the
// search.
func (st *State) htmlBlock(c *ui.Context, m *doc.Model, i int) {
	nodes := htmlx.Parse(m.Lines(i))
	if len(nodes) == 0 {
		ui.Box(c)
		return
	}
	leaf, base := runBase(m, i)
	col := ui.Column(c).AlignSelf(ui.Stretch).MinWidth(0)
	col.Children(func() {
		at := base
		parts := 0
		for _, n := range nodes {
			if n.Text != "" {
				if parts > 0 {
					at++
				}
				parts++
				st.htmlNode(c, m, leaf, n, at)
				at += runeLen(n.Text)
				continue
			}
			st.htmlNode(c, m, leaf, n, at)
		}
	})
}

// runBase returns the block the overlays address i's text under, with the
// rune offset where the piece starts: the block itself at 0 at the top
// level, or its place in the quote's or list's run when nested.
func runBase(m *doc.Model, i int) (leaf, off int) {
	if run, at := m.RunAt(i); run != nil {
		return run.Block, at
	}
	return i, 0
}

// htmlNode renders one semantic node, drawing its piece of the run's
// text, the slice at off; i is the block the run table holds it under.
func (st *State) htmlNode(c *ui.Context, m *doc.Model, i int, n htmlx.Node, off int) {
	tk := st.tokens(c)
	switch n.Kind {
	case htmlx.Para:
		var spans []ui.Span
		for _, seg := range n.Segs {
			spans = append(spans, st.htmlSeg(c, seg, n.Level))
		}
		spans = st.overlayRun(c, i, off, spans)
		el := ui.RichText(c, spans...).AlignSelf(ui.Stretch).MinWidth(0).PaddingY(4)
		if n.Level > 0 {
			el.Role(ui.RoleHeading).Level(n.Level)
		}
	case htmlx.Pre:
		// A pre shares the code block's panel, mono size and wrapping.
		box := ui.Column(c).Background(tk.panel).Radius(tk.radius).Padding(codeBlockPadding).MinWidth(0)
		box.Children(func() {
			spans := st.overlayRun(c, i, off, []ui.Span{{Text: n.Text}})
			ui.RichText(c, spans...).Font(tk.monoFamily()).FontSize(tk.monoSize).MinWidth(0)
		})
	case htmlx.Rule:
		ui.Box(c).Height(2).Background(tk.border).AlignSelf(ui.Stretch)
	case htmlx.Image:
		// No overlays over the picture, as for a markdown image block;
		// the img's width/height attributes fix the box when set.
		st.images(c, i, []doc.Image{{Dest: n.Src, Alt: n.Alt, Width: n.Width, Height: n.Height}})
	case htmlx.Table:
		st.htmlTable(c, i, n, off)
	}
}

// htmlSeg styles one inline seg of a paragraph, headings by their
// level's size, links in the accent color underlined and code as the
// markdown walk styles it: the same chip, the same tokens.
func (st *State) htmlSeg(c *ui.Context, seg htmlx.Seg, level int) ui.Span {
	tk := st.tokens(c)
	s := style{}
	if level > 0 {
		s.weight = 700
		s.size, s.color = headingStyle(c, st, level)
	} else {
		s.size, s.color = tk.body, tk.text
	}
	if seg.Bold {
		s.weight = 700
	}
	if seg.Italic {
		s.italic = true
	}
	if seg.Strike {
		s.strike = true
	}
	if seg.Underline {
		s.underline = true
	}
	if seg.Link {
		s.color = tk.link
		s.underline = true
	}
	sp := ui.Span{Text: seg.Text, Size: s.size, Weight: s.weight, Italic: s.italic, Color: s.color, Underline: s.underline, Strikethrough: s.strike}
	if seg.Code {
		sp = tk.inlineCode(sp)
	}
	return sp
}

// htmlTable renders an HTML table as a grid of rows, the header on a
// Surface background at weight 600 and the same rules the markdown table
// draws, its cells sharing the width the row gives them. The run text of
// the cells is joined with spaces, the markdown table's convention, so the
// overlays cut each cell at its rune offset in the block's run.
func (st *State) htmlTable(c *ui.Context, i int, n htmlx.Node, off int) {
	tk := st.tokens(c)
	grid := ui.Column(c).Border(1, tk.border).Radius(tk.radius).Clip().AlignSelf(ui.Stretch).MinWidth(0)
	grid.Children(func() {
		cellOff := off
		for r, row := range n.Cells {
			head, last := r == 0, r == len(n.Cells)-1
			line := ui.Row(c)
			if head {
				line.Background(tk.panel)
			}
			if !last {
				line.BorderWidth(0, 0, 1, 0).BorderColor(tk.border)
			}
			line.Children(func() {
				for k, cell := range row {
					at := cellOff
					weight := 400
					if head {
						weight = 600
					}
					spans := st.overlayRun(c, i, at, []ui.Span{{Text: cell, Size: tk.tableSize(), Weight: weight}})
					box := ui.Box(c).Grow(1).Padding(6, 10).AlignSelf(ui.Stretch)
					if k < len(row)-1 {
						box.BorderWidth(0, 1, 0, 0).BorderColor(tk.border)
					}
					box.Children(func() {
						ui.RichText(c, spans...).FontSize(tk.tableSize())
					})
					cellOff += runeLen(cell) + 1
				}
			})
		}
	})
}
