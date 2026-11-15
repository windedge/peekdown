// Package doc is the app's document model over mygo's Markdown parser: the
// blocks of the source with their headings, slugs and images, for the view
// to lay out and the selection and search to address.
package doc

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/windedge/peekdown/internal/htmlx"
)

// Model is a parsed document. It keeps the parser's incremental document and
// caches what the blocks share: the headings, their anchors and, of each
// block, whether its top-level content is one run of images.
type Model struct {
	doc *parser.Document
	// Front is the frontmatter the source opened with, nil for none. It
	// is not document text: the blocks, the runs and the search all see
	// the body alone.
	Front    *Frontmatter
	headings []Heading
	anchors  map[int]string
	images   map[int][]Image
	rows     []Block
	runs     []Run
	// tableRowBySlice maps a slice block's model index to the parser
	// indices of the rows it shows, the header repeated at the front;
	// plain (unsplit) tables answer TableRows from the parser's tree.
	// sliceByIndex holds the block each slice index carries.
	tableRowBySlice map[int][]int
	sliceByIndex    map[int]Block
	// leafRun maps each leaf block that contributes text to a run to the
	// run's index and the rune offset of its text within the run's text.
	// A split table's repeated header cells are absent: the header is a
	// visual copy each slice renders, so RunAt of a header cell would
	// answer whichever slice appended last, not the slice it shows in.
	leafRun map[int]leafOffset
	// linkTargets caches the blocks one link wraps, for LinkTargetOf; nil
	// until the first ask after a scan.
	linkTargets []linkTarget
	version     uint64

	// The search's state: the query it ran with, the matches in reading
	// order and the one the user is on, -1 for none. Matches address the
	// run table, the text the view draws.
	query   string
	opts    SearchOptions
	Matches []Match
	Current int
}

// leafOffset locates a leaf's text inside the run that holds it.
type leafOffset struct {
	run    int
	offset int
}

// Block is a top-level block of the document.
type Block struct {
	// Index is the block's in the model.
	Index int
	Kind  parser.Kind
	// Ordered, Number and Tight describe a list, Task an item's check box,
	// Level a heading's, Align a table's columns.
	Ordered bool
	Number  int
	Tight   bool
	Task    parser.Task
	Level   int
	Align   []parser.Align
	// Start and End are the block's bytes in the source. A table slice
	// reports the whole table's span: the slices share it.
	Start, End int
	// TableSlice marks a table the scan split: the block is one window
	// of a parser table too big to render in one row. TableTotalRows is
	// the whole table's body rows, the repeated header not counted, and
	// TableSliceFirst the body row the slice opens with, for a "rows
	// N-M of T" note.
	TableSlice      bool
	TableTotalRows  int
	TableSliceFirst int
}

// Heading is a heading of the document, for the outline and anchors.
type Heading struct {
	Level      int
	Text       string
	BlockIndex int
}

// Image is an image at the top level of a block's content, which the view
// shows as a picture of its own rather than within the text.
type Image struct {
	Dest string
	Alt  string
	// Link is the destination of a link wrapped around the image,
	// [![alt](img)](href), "" for none: the view opens it on a click on
	// the picture, as the Rust version does.
	Link string
	// Width and Height are the pixel sizes an HTML img's attributes
	// give, 0 for none; Markdown images have none. Set, the view draws
	// the picture at them; unset, it caps it at the Rust max sizes.
	Width, Height float32
}

// New parses src with the app's extension set, GFM plus footnotes: the
// set the shell opens files with, minus the math extensions so math
// stays literal text.
func New(src string) *Model {
	m, _ := NewWith(src, parser.GFM|parser.ExtFootnotes)
	return m
}

// NewWith parses src with extensions, as GFM with footnotes. A parse
// that panics leaves an empty document and reports it: the viewer must
// never take the process down over one file.
func NewWith(src string, ext parser.Extensions) (m *Model, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = empty(ext), fmt.Errorf("parse failed: %v", r)
		}
	}()
	front, body := stripped(src)
	d := parser.New(ext)
	d.Set(body)
	return wrap(d, front), nil
}

// empty builds an empty model with ext, what a failed parse falls back
// to.
func empty(ext parser.Extensions) *Model {
	d := parser.New(ext)
	d.Set("")
	return wrap(d, nil)
}

// emptyModel is an empty model over the same extension set, for a Set
// that panicked part way: the caller keeps its *Model, its state reset
// to a consistent one.
func emptyModel() *Model { return empty(parser.GFM | parser.ExtFootnotes) }

// stripped returns src's frontmatter and the body after it: the one
// place the header comes off the source, so the parser, the blocks and
// the runs see the body alone.
func stripped(src string) (*Frontmatter, string) {
	front, body, _ := ExtractFrontmatter(src)
	return front, body
}

// wrap finishes a document the parser made: it caches the frontmatter
// taken off its source and scans the blocks.
func wrap(d *parser.Document, front *Frontmatter) *Model {
	m := &Model{doc: d, Front: front, Current: -1}
	m.scan()
	return m
}

// Set replaces the source, parsing again only what changed. The search
// drops: its matches name text the new document need not have. A
// frontmatter is stripped first, so the parser, the blocks and the runs
// see the body alone.
func (m *Model) Set(src string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parse failed: %v", r)
		}
	}()
	front, body := stripped(src)
	return m.setBody(front, body)
}

// setBody swaps the source and its frontmatter in one step, the search
// dropping: its matches name text the new document need not have. A
// panic inside the parser's incremental Set leaves the Model half
// updated (new source and frontmatter, stale scan), so the recover
// rewraps from scratch and reports it: the shell then holds a
// self-consistent empty model that answers Len, RunOf and Plain, not one
// whose caches name the text it no longer has.
func (m *Model) setBody(front *Frontmatter, body string) (err error) {
	m.Front = front
	defer func() {
		if r := recover(); r != nil {
			*m = *emptyModel()
			err = fmt.Errorf("parse failed: %v", r)
		}
	}()
	m.doc.Set(body)
	m.scan()
	m.ClearSearch()
	return nil
}

// Offset contract: the model's source is the body, whose line 1 is the
// file's line Front.LineCount+1. Every block Start and End addresses the
// body; a viewer mapping lines back to the file adds LineCount to the
// line. The frontmatter itself is no block, contributes no run and never
// matches a search, even when shown as a code block (broken YAML or
// TOML), unlike an ordinary code block.

// Source returns the document's source.
func (m *Model) Source() string { return m.doc.Source() }

// root is the parser's block 0, which holds the document.
const root = 0

// tableSliceRows caps a table slice's rows, the header repeated at the
// front. A whole table renders inside the view List's one row callback,
// so a giant table means one giant per-frame layout: by-stars.md's
// 1281-row block allocated 54 MB per frame (mygo's text layouts, capped
// at 4096, evict and reshape every frame). 64 rows keep a slice's text
// near 10 KB, well inside the layout cache's budget.
const tableSliceRows = 64

// tableSplitRows is the row count that splits a table, and
// tableSplitBytes the byte size that does: past either, a table's text
// could crowd the layout cache in one row, so the scan cuts it into
// slices of tableSliceRows.
const (
	tableSplitRows  = 64
	tableSplitBytes = 256 << 10
)

// Len returns the number of top-level blocks.
func (m *Model) Len() int { return len(m.rows) }

// Version bumps every scan, so every Set and every fresh parse: the view
// keys its caches and its selection on it, dropping what the older
// document left.
func (m *Model) Version() uint64 { return m.version }

// Blocks returns the top-level blocks in document order.
func (m *Model) Blocks() []Block { return m.rows }

// Block returns block i.
func (m *Model) Block(i int) Block {
	if b, ok := m.sliceAt(i); ok {
		return b
	}
	return m.blockAt(i)
}

// sliceAt returns the slice block of model index i and whether i names
// one: tableRowBySlice maps the model's virtual indices to their rows,
// and sliceByIndex to the block they carry.
func (m *Model) sliceAt(i int) (Block, bool) {
	slice, ok := m.sliceByIndex[i]
	if !ok {
		return Block{}, false
	}
	slice.Index = i
	return slice, true
}

// RowBlock returns the block row r shows: the r-th top-level block. Rows
// are 0-based and dense, block indices are the parser's and skip the
// nested blocks - except a split table's slices, whose indices are the
// model's own and live in tableRowBySlice.
func (m *Model) RowBlock(r int) Block { return m.rows[r] }

func (m *Model) blockAt(i int) Block {
	b := m.doc.Block(i)
	return Block{
		Index:   i,
		Kind:    b.Kind,
		Ordered: b.Ordered,
		Number:  b.Number,
		Tight:   b.Tight,
		Task:    b.Task,
		Level:   b.Level,
		Align:   b.Align,
		Start:   b.Start,
		End:     b.End,
	}
}

// Headings returns the document's headings in document order.
func (m *Model) Headings() []Heading { return m.headings }

// Anchor returns the anchor of heading block i, unique in the document:
// the slug, then slug-1, slug-2, as GitHub numbers repeats.
func (m *Model) Anchor(i int) string { return m.anchors[i] }

// AnchorBySlug returns the block of the heading whose anchor is slug.
func (m *Model) AnchorBySlug(slug string) (int, bool) {
	for _, h := range m.headings {
		if m.anchors[h.BlockIndex] == slug {
			return h.BlockIndex, true
		}
	}
	return 0, false
}

// Plain returns the text of block i without its markup, breaks as spaces.
func (m *Model) Plain(i int) string {
	return parser.PlainText(m.doc.Inlines(i))
}

// Inlines returns the parsed inlines of block i.
func (m *Model) Inlines(i int) []parser.Inline { return m.doc.Inlines(i) }

// Images returns the images at the top level of block i's content: those
// the view shows as pictures of their own.
func (m *Model) Images(i int) []Image { return m.images[i] }

// Lines returns the lines of block i, as a code block's.
func (m *Model) Lines(i int) string { return m.doc.Lines(i) }

// Language returns the language a fenced code block names.
func (m *Model) Language(i int) string { return m.doc.Block(i).Language() }

// Children iterates the blocks nested in block i, a list, a quote or a
// table's rows. A table slice answers with its own window's rows, the
// header first: the slice is the model's block, the rows the parser's.
func (m *Model) Children(i int) func(yield func(int) bool) {
	if rows, ok := m.tableRowBySlice[i]; ok {
		return func(yield func(int) bool) {
			for _, r := range rows {
				if !yield(r) {
					return
				}
			}
		}
	}
	return m.doc.Children(i)
}

// TableRows returns the rows of table block i, the first its header. A
// split table's slice answers with its own window, the header repeated
// at the front; the rows are the parser's, so Plain and TableCells read
// them as any other.
func (m *Model) TableRows(i int) []int {
	if rows, ok := m.tableRowBySlice[i]; ok {
		return rows
	}
	var rows []int
	m.doc.Children(i)(func(j int) bool {
		if m.doc.Block(j).Kind == parser.TableRow {
			rows = append(rows, j)
		}
		return true
	})
	return rows
}

// TableCells returns the cells of table row j.
func (m *Model) TableCells(j int) []int {
	var cells []int
	m.doc.Children(j)(func(k int) bool {
		if m.doc.Block(k).Kind == parser.TableCell {
			cells = append(cells, k)
		}
		return true
	})
	return cells
}

// ListItems returns the items of list block i.
func (m *Model) ListItems(i int) []Block {
	var items []Block
	m.doc.Children(i)(func(j int) bool {
		if m.doc.Block(j).Kind == parser.Item {
			items = append(items, m.blockAt(j))
		}
		return true
	})
	return items
}

// QuoteBlocks returns the blocks nested in quote block i.
func (m *Model) QuoteBlocks(i int) []int {
	var blocks []int
	m.doc.Children(i)(func(j int) bool {
		blocks = append(blocks, j)
		return true
	})
	return blocks
}

// scan rebuilds what the model caches of the document: the headings with
// their anchors and the blocks that are only images.
func (m *Model) scan() {
	m.version++
	m.headings = nil
	m.anchors = nil
	m.images = nil
	m.rows = nil
	m.runs = nil
	m.tableRowBySlice = nil
	m.sliceByIndex = nil
	m.leafRun = nil
	m.linkTargets = nil
	seen := map[string]bool{}
	for i := range m.doc.Children(root) {
		if b := m.doc.Block(i); b.Kind == parser.Table && m.splitsTable(i, b) {
			m.appendTableSlices(i, b)
			continue
		}
		m.rows = append(m.rows, m.blockAt(i))
		b := m.doc.Block(i)
		switch b.Kind {
		case parser.Heading:
			text := strings.TrimSpace(parser.PlainText(m.doc.Inlines(i)))
			m.headings = append(m.headings, Heading{Level: b.Level, Text: text, BlockIndex: i})
			slug := Slug(text)
			cand, n := slug, 0
			for seen[cand] {
				n++
				cand = slug + "-" + strconv.Itoa(n)
			}
			seen[cand] = true
			if m.anchors == nil {
				m.anchors = map[int]string{}
			}
			m.anchors[i] = cand
		case parser.Paragraph:
			if images := topLevelImages(m.doc.Inlines(i)); len(images) > 0 {
				if m.images == nil {
					m.images = map[int][]Image{}
				}
				m.images[i] = images
			}
		}
		m.appendRuns(i)
	}
}

// splitsTable reports whether table i's text and rows crowd one List
// row enough to split it: more rows than the slice holds, or more bytes
// than the split budget.
func (m *Model) splitsTable(i int, b *parser.Block) bool {
	return b.End-b.Start >= tableSplitBytes || len(m.TableRows(i)) >= tableSplitRows
}

// appendTableSlices splits parser table i into consecutive slice blocks:
// each a window of tableSliceRows rows, the header repeated at the front
// so a slice renders as a table of its own. The slices' indices are the
// model's own (a dense range past the parser's), held in tableRowBySlice
// and sliceByIndex, which answer TableRows, Block and Children for them;
// the runs treat each slice as a table in its own right.
func (m *Model) appendTableSlices(i int, b *parser.Block) {
	all := m.TableRows(i)
	header, body := all[0], all[1:]
	base := m.nextSliceIndex()
	for first := 0; first < len(body); {
		end := min(first+tableSliceRows-1, len(body))
		window := append([]int{header}, body[first:end]...)
		idx := base + len(m.tableRowBySlice)
		if m.tableRowBySlice == nil {
			m.tableRowBySlice = map[int][]int{}
		}
		m.tableRowBySlice[idx] = window
		if m.sliceByIndex == nil {
			m.sliceByIndex = map[int]Block{}
		}
		slice := Block{
			Index: idx, Kind: parser.Table, Align: b.Align,
			Start: b.Start, End: b.End,
			TableSlice: true,
			// The body rows of the whole table, the repeated header not
			// counted.
			TableTotalRows:  len(body),
			TableSliceFirst: first,
		}
		m.rows = append(m.rows, slice)
		m.sliceByIndex[idx] = slice
		var leaves []leafText
		for _, row := range window {
			for _, cell := range m.TableCells(row) {
				leaves = append(leaves, leafText{cell, m.Plain(cell)})
			}
		}
		// The header repeats in every window but its cells are one parser
		// block apiece, so leafRun holds them once and the last write wins:
		// RunAt of a header cell would then name the last slice's run and
		// cut every other slice's header highlight at the wrong runes. The
		// header is the same visual copy every slice draws, so no slice
		// maps it: a header cell's RunAt misses and the view's overlay
		// leaves it unhighlighted rather than cutting another slice's run.
		skip := map[int]bool{}
		for _, cell := range m.TableCells(header) {
			skip[cell] = true
		}
		m.appendRunAt(idx, 0, skip, joined(leaves, " "), leaves)
		first = end
	}
}

// nextSliceIndex returns the first model index free for a slice: past
// the parser's blocks and any slices already made.
func (m *Model) nextSliceIndex() int {
	n := m.doc.Len()
	for k := range m.tableRowBySlice {
		if k >= n {
			n = k + 1
		}
	}
	return n
}

// topLevelImages returns the images of in whose run of top-level content
// they are, when the whole block is one: images and nothing else but
// breaks. A paragraph that mixes text and images keeps its images within
// the text.
func topLevelImages(in []parser.Inline) []Image {
	var images []Image
	for k := 0; k < len(in); {
		n := &in[k]
		switch n.Kind {
		case parser.Link:
			// [![alt](img)](href): the image is the link's only real
			// content, so the block stays an image block and the link's
			// destination rides on it; anything else leaves to the text path.
			if n.Next > k+1 && in[k+1].Kind == parser.Image {
				img := &in[k+1]
				images = append(images, Image{Dest: img.Dest, Alt: parser.PlainText(in[k+2 : img.Next]), Link: n.Dest})
				k = img.Next
				continue
			}
			return nil
		case parser.Image:
			images = append(images, Image{Dest: n.Dest, Alt: parser.PlainText(in[k+1 : n.Next])})
			k = n.Next
			continue
		case parser.SoftBreak, parser.LineBreak:
			k++
		default:
			return nil
		}
	}
	return images
}

// SetSearch runs query over the document and stores the state: the
// matches, with the current one reset to the first, or -1 when the
// search found nothing. The document is unchanged, so the version does
// not move; the view redraws the overlays off the new matches.
func (m *Model) SetSearch(query string, opts SearchOptions) {
	m.query = query
	m.opts = opts
	m.Matches = FindAll(m, query, opts)
	if len(m.Matches) == 0 {
		m.Current = -1
		return
	}
	m.Current = 0
}

// Query returns the text of the current search, empty for none.
func (m *Model) Query() string { return m.query }

// SearchOptions returns the options the current search ran with.
func (m *Model) SearchOptions() SearchOptions { return m.opts }

// NextMatch moves to the next match, wrapping to the first.
func (m *Model) NextMatch() {
	if len(m.Matches) == 0 {
		return
	}
	m.Current = (m.Current + 1) % len(m.Matches)
}

// PrevMatch moves to the previous match, wrapping to the last.
func (m *Model) PrevMatch() {
	if len(m.Matches) == 0 {
		return
	}
	m.Current += len(m.Matches) - 1
	m.Current %= len(m.Matches)
}

// ClearSearch drops the matches: no query, nothing to highlight.
func (m *Model) ClearSearch() {
	m.query = ""
	m.opts = SearchOptions{}
	m.Matches = nil
	m.Current = -1
}

// Slug returns the anchor of a heading of text, as the Rust implementation
// makes them: lowercased, each whitespace rune to one dash (no collapsing),
// Unicode letters, digits and dashes kept, trimmed dashes.
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte('-')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

// Run is one stretch of document text the reader sees, with its rune
// offsets: the run table is the one authority selections, search and copy
// address. A run is a leaf block's rendered text: a paragraph's or heading's
// plain text, a code block's lines, a table's cells in reading order, a
// list's or a quote's text in reading order.
type Run struct {
	// Block is the top-level block the run belongs to.
	Block int
	// Leaf is the parser block the text was taken from, a child of Block
	// for nested blocks, Block itself otherwise.
	Leaf int
	// Text is the run's rendered text, what the view draws for it.
	Text string
}

// RunText returns runes from..to of run, clamped to its length.
func (m *Model) RunText(run Run, from, to int) string {
	r := []rune(run.Text)
	if from < 0 {
		from = 0
	}
	if to > len(r) {
		to = len(r)
	}
	if from >= to {
		return ""
	}
	return string(r[from:to])
}

// Runs returns the document's runs in reading order.
func (m *Model) Runs() []Run { return m.runs }

// RunOf returns the run that holds block i's text, or nil when block i
// contributes none.
func (m *Model) RunOf(block int) *Run {
	for k := range m.runs {
		if m.runs[k].Leaf == block {
			return &m.runs[k]
		}
	}
	return nil
}

// RunAt returns the run that holds leaf block i's text and the rune
// offset of that text within the run, as the view's overlays address
// runs. nil when block i contributes no text. A leaf of a quote or a
// list shares the whole block's one run, joined with newlines; a table
// cell shares its table's run, joined with spaces; a paragraph or a
// heading run of its own at offset 0.
func (m *Model) RunAt(leaf int) (*Run, int) {
	if m.leafRun == nil {
		return nil, 0
	}
	at, ok := m.leafRun[leaf]
	if !ok {
		return nil, 0
	}
	return &m.runs[at.run], at.offset
}

// appendRuns walks block i's subtree in reading order, appending one run
// per stretch of text. Code blocks are their lines; HTML blocks the visible
// text of their semantic content; nested blocks contribute their leaves in
// order; a leaf run inherits the text the view renders for it. Each
// contributing leaf is recorded with its rune offset in the run that holds it.
func (m *Model) appendRuns(i int) {
	b := m.doc.Block(i)
	switch b.Kind {
	case parser.Paragraph, parser.Heading:
		m.appendRun(i, m.Plain(i), nil)
	case parser.CodeBlock, parser.MathBlock:
		m.appendRun(i, strings.Trim(m.Lines(i), "\n"), nil)
	case parser.HTMLBlock:
		m.appendRun(i, htmlx.RunText(m.Lines(i)), nil)
	case parser.Table:
		var leaves []leafText
		m.collectLeaves(i, &leaves)
		m.appendRun(i, joined(leaves, " "), leaves)
	case parser.Quote, parser.List, parser.FootnoteDefinition:
		// One run for the whole block: its leaves in reading order. A
		// footnote definition keeps its note text here, the reference in
		// the citing text being only the [^label] mark.
		var leaves []leafText
		m.collectLeaves(i, &leaves)
		m.appendRun(i, joined(leaves, "\n"), leaves)
	}
}

// leafText is one contributing leaf with its text.
type leafText struct {
	block int
	text  string
}

// joined returns the leaves' texts joined for one run, table cells with
// spaces, a quote's or a list's leaves with newlines.
func joined(leaves []leafText, sep string) string {
	parts := make([]string, len(leaves))
	for k, l := range leaves {
		parts[k] = l.text
	}
	return strings.Join(parts, sep)
}

// collectLeaves gathers the text of block i's leaf descendants.
func (m *Model) collectLeaves(i int, leaves *[]leafText) {
	b := m.doc.Block(i)
	switch b.Kind {
	case parser.Paragraph, parser.Heading:
		*leaves = append(*leaves, leafText{i, m.Plain(i)})
		return
	case parser.CodeBlock, parser.MathBlock:
		*leaves = append(*leaves, leafText{i, strings.Trim(m.Lines(i), "\n")})
		return
	case parser.HTMLBlock:
		*leaves = append(*leaves, leafText{i, htmlx.RunText(m.Lines(i))})
		return
	case parser.Table:
		for _, row := range m.TableRows(i) {
			for _, cell := range m.TableCells(row) {
				*leaves = append(*leaves, leafText{cell, m.Plain(cell)})
			}
		}
		return
	}
	for j := range m.doc.Children(i) {
		m.collectLeaves(j, leaves)
	}
}

func (m *Model) appendRun(block int, text string, leaves []leafText) {
	m.appendRunAt(block, 0, nil, text, leaves)
}

// appendRunAt is appendRun with the run's rune offset in an enclosing
// run, the offset a nested table slice's leaves already carry: the text
// still lands in the run table whole, the leaves record where in it
// their own text starts. skip records the leaves whose mapping the run
// does not take: a split table's repeated header cells, which the view
// renders as a visual copy and overlays address per slice through the
// slice's own run (see appendTableSlices).
func (m *Model) appendRunAt(block, at int, skip map[int]bool, text string, leaves []leafText) {
	if text == "" {
		return
	}
	m.runs = append(m.runs, Run{Block: block, Leaf: block, Text: text})
	// The run's own text starts at offset 0; the leaves nested in block i
	// share its run, joined, each starting where its text does.
	if m.leafRun == nil {
		m.leafRun = map[int]leafOffset{}
	}
	run := len(m.runs) - 1
	for k, l := range leaves {
		if k > 0 {
			at++
		}
		if !skip[l.block] {
			m.leafRun[l.block] = leafOffset{run: run, offset: at}
		}
		at += len([]rune(l.text))
	}
}
