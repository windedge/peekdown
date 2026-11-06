// Package doc is the app's document model over mygo's Markdown parser: the
// blocks of the source with their headings, slugs and images, for the view
// to lay out and the selection and search to address.
package doc

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// Model is a parsed document. It keeps the parser's incremental document and
// caches what the blocks share: the headings, their anchors and, of each
// block, whether its top-level content is one run of images.
type Model struct {
	doc      *parser.Document
	headings []Heading
	anchors  map[int]string
	images   map[int][]Image
	rows     []Block
	runs     []Run
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
	// Start and End are the block's bytes in the source.
	Start, End int
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
}

// New parses src.
func New(src string) *Model {
	return wrap(parser.Parse(src))
}

// NewWith parses src with extensions, as GFM with footnotes.
func NewWith(src string, ext parser.Extensions) *Model {
	d := parser.New(ext)
	d.Set(src)
	return wrap(d)
}

// wrap finishes a document the parser made: it scans the blocks the model
// caches.
func wrap(d *parser.Document) *Model {
	m := &Model{doc: d}
	m.scan()
	return m
}

// Set replaces the source, parsing again only what changed.
func (m *Model) Set(src string) {
	m.doc.Set(src)
	m.scan()
}

// Source returns the document's source.
func (m *Model) Source() string { return m.doc.Source() }

// root is the parser's block 0, which holds the document.
const root = 0

// Len returns the number of top-level blocks.
func (m *Model) Len() int { return len(m.rows) }

// Blocks returns the top-level blocks in document order.
func (m *Model) Blocks() []Block { return m.rows }

// Block returns block i.
func (m *Model) Block(i int) Block { return m.blockAt(i) }

// RowBlock returns the block row r shows: the r-th top-level block. Rows
// are 0-based and dense, block indices are the parser's and skip the
// nested blocks.
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

// HeadingBySlug returns the block of the heading whose anchor is slug.
func (m *Model) HeadingBySlug(slug string) (int, bool) {
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

// LineCount returns the number of lines of block i.
func (m *Model) LineCount(i int) int { return m.doc.LineCount(i) }

// Line returns line k of block i.
func (m *Model) Line(i, k int) (string, int) { return m.doc.Line(i, k) }

// Info returns a fenced code block's info string.
func (m *Model) Info(i int) string { return m.doc.Block(i).Info() }

// Language returns the language a fenced code block names.
func (m *Model) Language(i int) string { return m.doc.Block(i).Language() }

// Children iterates the blocks nested in block i, a list, a quote or a
// table's rows.
func (m *Model) Children(i int) func(yield func(int) bool) {
	return m.doc.Children(i)
}

// TableRows returns the rows of table block i, the first its header.
func (m *Model) TableRows(i int) []int {
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
	m.headings = nil
	m.anchors = nil
	m.images = nil
	m.rows = nil
	m.runs = nil
	seen := map[string]bool{}
	for i := range m.doc.Children(root) {
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

// topLevelImages returns the images of in whose run of top-level content
// they are, when the whole block is one: images and nothing else but
// breaks. A paragraph that mixes text and images keeps its images within
// the text.
func topLevelImages(in []parser.Inline) []Image {
	var images []Image
	for k := 0; k < len(in); {
		n := &in[k]
		switch n.Kind {
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

// appendRuns walks block i's subtree in reading order, appending one run
// per stretch of text. Code blocks are their lines; HTML blocks are their
// source; nested blocks contribute their leaves in order; a leaf run
// inherits the text the view renders for it.
func (m *Model) appendRuns(i int) {
	b := m.doc.Block(i)
	switch b.Kind {
	case parser.Paragraph, parser.Heading:
		m.appendRun(i, m.Plain(i))
	case parser.CodeBlock:
		m.appendRun(i, strings.Trim(m.Lines(i), "\n"))
	case parser.HTMLBlock:
		m.appendRun(i, strings.TrimRight(m.Lines(i), "\n"))
	case parser.Table:
		var cells []string
		for _, row := range m.TableRows(i) {
			for _, cell := range m.TableCells(row) {
				cells = append(cells, m.Plain(cell))
			}
		}
		m.appendRun(i, strings.Join(cells, " "))
	case parser.Quote, parser.List:
		// One run for the whole block: its leaves in reading order.
		var parts []string
		m.collectLeafText(i, &parts)
		m.appendRun(i, strings.Join(parts, "\n"))
	}
}

// collectLeafText gathers the text of block i's leaf descendants.
func (m *Model) collectLeafText(i int, parts *[]string) {
	b := m.doc.Block(i)
	switch b.Kind {
	case parser.Paragraph, parser.Heading:
		*parts = append(*parts, m.Plain(i))
		return
	case parser.CodeBlock:
		*parts = append(*parts, strings.Trim(m.Lines(i), "\n"))
		return
	case parser.HTMLBlock:
		*parts = append(*parts, strings.TrimRight(m.Lines(i), "\n"))
		return
	case parser.Table:
		for _, row := range m.TableRows(i) {
			for _, cell := range m.TableCells(row) {
				*parts = append(*parts, m.Plain(cell))
			}
		}
		return
	}
	for j := range m.doc.Children(i) {
		m.collectLeafText(j, parts)
	}
}

func (m *Model) appendRun(block int, text string) {
	if text == "" {
		return
	}
	m.runs = append(m.runs, Run{Block: block, Leaf: block, Text: text})
}
