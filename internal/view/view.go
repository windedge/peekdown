// Package view renders a parsed document as virtual list rows: one row per
// top-level block, styled spans for the text, highlight for the selection
// and the search, and an anchor that scrolls a heading into view.
package view

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/doc"
	"github.com/windedge/peekdown/internal/selection"
)

// State is the state of one document view: its list, the selection, the
// search's matches and the anchor to scroll to. Keep one per open tab.
type State struct {
	// List is the virtual list of the document's blocks.
	List ui.ListState

	// Selection is the text selection over the document's blocks.
	Selection selectionRange
	// Caret blinks at the selection's caret while the view has the focus.
	Focused bool

	// dragging marks a press that selects text: the element that took the
	// press gets the pointer's moves and its release wherever they go.
	dragging bool
	// press is where the drag began.
	press selection.Point

	// rowHandles bind each block's row as it builds, so the handler's
	// hit reads the committed frame's rectangles out of band, after the
	// frame and across scrolls; ctx is the frame's context they take.
	rowHandles map[int]*ui.Handle
	ctx        *ui.Context

	// Matches are the search's matches, in document order; Current is the
	// one the user is on, -1 for none. A match addresses a run of a block's
	// text, in runes.
	Matches []Match
	Current int

	// Anchor scrolls the heading of this slug into view; AnchorVersion
	// asks for the same anchor again.
	Anchor         string
	AnchorVersion  uint64
	anchorDone     string
	anchorVersions uint64

	// AnchorOf returns the anchor of top-level block i, as the model
	// makes them, so the view can find the row to scroll to.
	AnchorOf func(i int) string

	// LoadImage loads the image at url; nil shows its description.
	LoadImage func(url string) (ui.ImageSource, error)

	// imageCache caches the sources the loader returned, by URL.
	imageCache map[string]ui.ImageSource

	// layouts cache the shaped text of the blocks the pointer has hit.
	layouts map[int]*textLayout

	// Theme of the view's code: the window's by default.
	CodeTheme *editor.Theme
}

// selectionRange is the selection as the view sees it: a half-open run of
// each block's text, in runes.
type selectionRange struct {
	start, end selection.Point
	set        bool
}

// Point is a block and a rune offset in its text.
type Point = selection.Point

// Match is one search match: block i's text, runes from to.
type Match struct {
	Block int
	From  int
	To    int
}

// SetSelection stores the selection the view draws.
func (st *State) SetSelection(anchor, caret selection.Point) {
	st.Selection = selectionRange{start: anchor, end: caret, set: true}
	if caret.Before(anchor) {
		st.Selection.start, st.Selection.end = st.Selection.end, st.Selection.start
	}
}

// ClearSelection collapses the selection.
func (st *State) ClearSelection() { st.Selection.set = false }

// Handler returns the input handler for the document area. Wire it to the
// element that wraps the view (ScrollView's list or a Box around it): the
// element pressed takes the moves and the release wherever they go, so one
// handler covers every block. It returns false for events it does not
// take, scrolling among them. The events' position is relative to the
// element wired, which is to wrap the list without padding of its own.
func (st *State) Handler(m *doc.Model) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			// A press above the window's top comes clamped below zero
			// (the engine keeps the element hit), as the first row.
			p, ok := st.pointAt(m, ev, true)
			if !ok {
				return false
			}
			st.press = p
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
			st.SetSelection(st.press, p)
			return true
		case ui.InputPointerUp:
			if !st.dragging || ev.Button != 0 {
				return false
			}
			st.dragging = false
			return true
		}
		return false
	}
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
	layout := st.layout(block, text, width)
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
// list keeps its own scroll place; use State.List to scroll it. Each row
// binds its block's handle, which the Handler the caller wires reads.
func ScrollView(c *ui.Context, m *doc.Model, st *State) {
	ui.List(c, &st.List, m.Len(), func(r int) {
		st.Row(c, m, r)
	}).Grow(1)
	if st.AnchorOf == nil {
		st.AnchorOf = anchorsOf(m)
	}
	st.scrollPendingAnchor(m)
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
	row := ui.Column(c)
	row.Bind(handle)
	row.Children(func() {
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
		ui.Box(c).Height(1).Background(c.Theme().Border).AlignSelf(ui.Stretch).MarginY(8)
	case parser.HTMLBlock:
		ui.Text(c, strings.TrimRight(m.Lines(i), "\n")).
			Font(fontMono).FontSize(13).
			TextColor(c.Theme().TextMuted).PaddingY(4)
	default:
		ui.Box(c)
	}
}

const (
	fontMono = "monospace"
)

// style is the base style of a run of spans.
type style struct {
	size   float32
	weight int
	color  ui.Color
}

func (st *State) heading(c *ui.Context, m *doc.Model, i, level int) {
	s := style{weight: 700}
	s.size, s.color = headingStyle(c, level)
	st.richParagraph(c, m, i, s, true)
}

func (st *State) paragraph(c *ui.Context, m *doc.Model, i int, s style) {
	st.richParagraph(c, m, i, s, false)
}

func headingStyle(c *ui.Context, level int) (float32, ui.Color) {
	switch level {
	case 1:
		return 28, c.Theme().Text
	case 2:
		return 22, c.Theme().Text
	case 3:
		return 18, c.Theme().Text
	case 4:
		return 16, c.Theme().Text
	case 5:
		return 15, c.Theme().Text
	default:
		return 14, c.Theme().TextMuted
	}
}

// richParagraph builds block i's text as styled spans, heading when it is
// one, so it gets its role.
func (st *State) richParagraph(c *ui.Context, m *doc.Model, i int, s style, heading bool) {
	t := c.Theme()
	if s.size == 0 {
		s.size = 15
	}
	if s.weight == 0 {
		s.weight = 400
	}
	if s.color == (ui.Color{}) {
		s.color = t.Text
	}
	if images := m.Images(i); len(images) > 0 {
		st.images(c, i, images)
		return
	}
	spans := st.spans(c, m, i, s)
	el := ui.RichText(c, spans...).FontFeatures("tnum").AlignSelf(ui.Stretch).MinWidth(0)
	if heading {
		el.Role(ui.RoleHeading).Level(m.Block(i).Level)
	}
}

// spans walks block i's inlines into styled spans, drawing the selection
// and the matches over the text.
func (st *State) spans(c *ui.Context, m *doc.Model, i int, s style) []ui.Span {
	t := c.Theme()
	spans := []ui.Span{}
	walk(m.Inlines(i), s, func(text string, mark, link bool, dest string) {
		if text == "" {
			return
		}
		base := s
		base.color = t.Text
		if mark {
			base.color = t.TextMuted
			base.size = s.size * 0.95
		}
		spans = append(spans, ui.Span{Text: text, Size: base.size, Weight: base.weight, Color: base.color})
	})
	spans = st.overlay(c, i, spans, s)
	return spans
}

// walk flattens inlines to runs of text in style s, marking code and
// links. The spans' texts joined are the block's run text in doc's run
// table, rune for rune: the offsets of the selection, the cuts and the
// search all address that text. LineBreak and FootnoteRef follow
// parser.PlainText (a break is a space, a note is "[^label]"); an
// image's alt text stays in the text, as PlainText's flat walk over the
// inline array emits it too.
func walk(in []parser.Inline, s style, fn func(text string, code, link bool, dest string)) {
	for k := 0; k < len(in); k++ {
		n := &in[k]
		switch n.Kind {
		case parser.Text:
			fn(n.Text, false, false, "")
		case parser.SoftBreak:
			fn(" ", false, false, "")
		case parser.LineBreak:
			fn(" ", false, false, "")
		case parser.Code:
			fn(n.Text, true, false, "")
		case parser.Emphasis:
			walk(in[k+1:n.Next], style{s.size, s.weight, s.color}, fn)
		case parser.Strong:
			walk(in[k+1:n.Next], style{s.size, 700, s.color}, fn)
		case parser.Link:
			walk(in[k+1:n.Next], style{s.size, s.weight, s.color}, func(text string, code, link bool, dest string) {
				fn(text, code, true, n.Dest)
			})
		case parser.Image:
			// Its description stands for it within the text.
			walk(in[k+1:n.Next], style{s.size, s.weight, s.color}, func(text string, code, link bool, dest string) {
				fn(text, code, false, "")
			})
		case parser.HTML, parser.Math:
			fn(n.Text, true, false, "")
		case parser.FootnoteRef:
			fn("[^"+n.Text+"]", true, false, "")
		}
		if n.Next > k {
			k = n.Next - 1
		}
	}
}

// overlay splits a block's spans where the selection and the matches cut
// its text, coloring the covered runs. The spans carry the text of the
// block joined, which the overlays address by rune.
func (st *State) overlay(c *ui.Context, i int, spans []ui.Span, s style) []ui.Span {
	var cuts []cut
	if st.Selection.set {
		start, end := st.Selection.start, st.Selection.end
		if start.Block == end.Block && start.Block == i && start.Offset < end.Offset {
			cuts = append(cuts, cut{start.Offset, end.Offset, cutSelection})
		}
	}
	for k, match := range st.Matches {
		if match.Block != i {
			continue
		}
		kind := cutMatch
		if k == st.Current {
			kind = cutCurrent
		}
		cuts = append(cuts, cut{match.From, match.To, kind})
	}
	if len(cuts) == 0 {
		return spans
	}
	return splitSpans(spans, cuts, c.Theme())
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
// are the block's text, so cuts address runes of that joined text.
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

// mergeCuts sorts the cuts and drops overlaps, first match wins.
func mergeCuts(cuts []cut, total int) []segment {
	sortCuts(cuts)
	var out []segment
	for _, c := range cuts {
		if c.to <= total && (len(out) == 0 || out[len(out)-1].to <= c.from) {
			out = append(out, segment{c.from, c.to, c.kind})
		}
	}
	return out
}

func sortCuts(cuts []cut) {
	for i := 1; i < len(cuts); i++ {
		for j := i; j > 0 && cuts[j].from < cuts[j-1].from; j-- {
			cuts[j], cuts[j-1] = cuts[j-1], cuts[j]
		}
	}
}

// images builds a row of pictures for the images of block i.
func (st *State) images(c *ui.Context, i int, images []doc.Image) {
	t := c.Theme()
	ui.Row(c).Gap(8).PaddingY(6).AlignSelf(ui.Stretch).Children(func() {
		for _, img := range images {
			src := st.source(img.Dest)
			if src != nil {
				ui.Image(c, src).MaxHeight(400).MaxWidthPercent(100).Radius(4)
				continue
			}
			label := img.Alt
			if label == "" {
				label = img.Dest
			}
			ui.Text(c, label).FontSize(13).TextColor(t.TextMuted)
		}
	})
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
// the language the fence names.
func (st *State) codeBlock(c *ui.Context, m *doc.Model, i int) {
	t := c.Theme()
	ct := st.CodeTheme
	if ct == nil {
		ct = editor.DefaultTheme(t)
	}
	lang := editor.LanguageNamed(m.Language(i))
	lines := splitLines(m.Lines(i))
	box := ui.Column(c).Background(t.Surface).Radius(6).Padding(10).MarginY(6).Gap(0).AlignSelf(ui.Stretch).MinWidth(0)
	box.Children(func() {
		for _, line := range lines {
			var toks []editor.Token
			if lang != nil && lang.Tokenizer != nil {
				toks, _ = lang.Tokenizer.Tokenize(line, 0, nil)
			}
			spans := codeSpans(line, toks, ct, t.Text)
			if len(spans) == 0 {
				spans = append(spans, ui.Span{Text: " "})
			}
			el := ui.RichText(c, spans...).Font(fontMono).FontSize(13)
			el.NoWrap()
		}
	})
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

// table builds a Markdown table as a grid of rows.
func (st *State) table(c *ui.Context, m *doc.Model, i int) {
	t := c.Theme()
	grid := ui.Column(c).Border(1, t.Border).Radius(4).MarginY(6).AlignSelf(ui.Stretch).MinWidth(0)
	grid.Children(func() {
		first := true
		for _, row := range m.TableRows(i) {
			r := ui.Row(c)
			if first {
				r.Background(t.Surface)
			}
			r.Children(func() {
				for _, cell := range m.TableCells(row) {
					s := style{}
					if first {
						s.weight = 600
					}
					spans := st.spans(c, m, cell, s)
					ui.RichText(c, spans...).FontSize(14).Padding(6, 10).Grow(1).MinWidth(0)
				}
			})
			first = false
		}
	})
}

// quote builds a block quote with its bar.
func (st *State) quote(c *ui.Context, m *doc.Model, i int) {
	t := c.Theme()
	box := ui.Row(c).PaddingStart(12).MarginY(6).AlignSelf(ui.Stretch).MinWidth(0)
	box.BorderStartWidth(3).BorderColor(t.Border)
	box.Children(func() {
		col := ui.Column(c).Gap(4).Grow(1).MinWidth(0)
		col.Children(func() {
			for _, j := range m.QuoteBlocks(i) {
				if m.Block(j).Kind == parser.Paragraph {
					st.paragraph(c, m, j, style{color: t.TextMuted})
				} else {
					Render(c, m, st, j)
				}
			}
		})
	})
}

// list builds a list, its items with markers or numbers, nested one level
// deeper.
func (st *State) list(c *ui.Context, m *doc.Model, i int, b doc.Block, depth int) {
	t := c.Theme()
	col := ui.Column(c).Gap(2).MarginY(6).AlignSelf(ui.Stretch).MinWidth(0)
	col.Children(func() {
		number := b.Number
		for _, item := range m.ListItems(i) {
			marker := "•"
			width := float32(18)
			if b.Ordered {
				marker = itoa(number) + "."
				number++
				width = 28
			}
			row := ui.Row(c).Gap(4).AlignSelf(ui.Stretch).MinWidth(0)
			row.Children(func() {
				ui.Text(c, marker).Width(width).TextColor(t.TextMuted).TextAlign(ui.End)
				content := ui.Column(c).Gap(2).Grow(1).MinWidth(0)
				content.Children(func() {
					for j := range m.Children(item.Index) {
						child := m.Block(j)
						switch child.Kind {
						case parser.Paragraph:
							spans := st.spans(c, m, j, style{})
							spans = st.taskSpan(c, child, spans)
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

// taskSpan prefixes a task item's spans with its check box.
func (st *State) taskSpan(c *ui.Context, item doc.Block, spans []ui.Span) []ui.Span {
	switch item.Task {
	case parser.Unchecked:
		return append([]ui.Span{{Text: "☐ "}}, spans...)
	case parser.Checked:
		return append([]ui.Span{{Text: "☑ "}}, spans...)
	}
	return spans
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// scrollPendingAnchor asks the list to scroll to the block the anchor
// names, once per anchor or version bump.
func (st *State) scrollPendingAnchor(m *doc.Model) {
	if st.Anchor == "" || st.AnchorOf == nil {
		return
	}
	key := st.Anchor + "@" + itoa(int(st.AnchorVersion))
	if st.anchorDone == key && st.anchorVersions == st.AnchorVersion {
		return
	}
	for r, b := range m.Blocks() {
		if slug := st.AnchorOf(b.Index); slug == st.Anchor {
			st.List.ScrollIntoView(r)
			st.anchorDone = key
			st.anchorVersions = st.AnchorVersion
			return
		}
	}
}

// textLayout is a shaped block of text with the width it was shaped at.
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

// layout shapes a block's text at a row's width, caching by block and
// width: the selection addresses runes, so the text it shapes is the run
// table's.
func (st *State) layout(block int, text string, width float32) *ui.TextLayout {
	if st.layouts == nil {
		st.layouts = map[int]*textLayout{}
	}
	if entry := st.layouts[block]; entry != nil && entry.width == width {
		return entry.layout
	}
	layout := ui.ShapeText(text, ui.Font{Size: 15}, width)
	st.layouts[block] = &textLayout{layout: layout, width: width}
	return layout
}

// HasCaret reports a caret: a selection set, a drag or a click among
// them, whether it selects anything or not.
func (st *State) HasCaret() bool { return st.Selection.set }

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

// FindAll returns the matches of substr in block i's text, in runes.
func FindAll(m *doc.Model, block int, substr string) []Match {
	text := []rune(runText(m, block))
	var out []Match
	if substr == "" {
		return out
	}
	needle := []rune(substr)
	for i := 0; i+len(needle) <= len(text); {
		if string(text[i:i+len(needle)]) == substr {
			out = append(out, Match{Block: block, From: i, To: i + len(needle)})
			i += len(needle)
			continue
		}
		i++
	}
	return out
}
