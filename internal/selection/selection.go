// Package selection is the app's text selection model: two endpoints over
// the document, each a block and an offset within its text, independent of
// the elements on screen, so it survives scrolling and virtualization.
package selection

import (
	"strings"
	"unicode"

	"github.com/windedge/peekdown/internal/doc"
)

// Point is one end of a selection: block i's text, offset runes in.
type Point struct {
	Block  int
	Offset int
}

// Before reports whether p sorts before q in reading order.
func (p Point) Before(q Point) bool {
	if p.Block != q.Block {
		return p.Block < q.Block
	}
	return p.Offset < q.Offset
}

// Selection is a text selection between two endpoints, or a caret when the
// two are equal.
type Selection struct {
	Anchor Point
	Caret  Point
}

// Collapsed returns a selection whose anchor and caret are p: an insertion
// point selecting nothing.
func Collapsed(p Point) Selection { return Selection{Anchor: p, Caret: p} }

// All returns a selection of the whole document.
func All(m *doc.Model) Selection {
	blocks := m.Blocks()
	if len(blocks) == 0 {
		return Collapsed(Point{})
	}
	first, last := blocks[0], blocks[len(blocks)-1]
	return Selection{
		Anchor: Point{Block: first.Index, Offset: 0},
		Caret:  Point{Block: last.Index, Offset: runeLen(m.Plain(last.Index))},
	}
}

// Order returns the selection's endpoints in reading order.
func (s Selection) Order() (Point, Point) {
	if s.Anchor.Before(s.Caret) {
		return s.Anchor, s.Caret
	}
	return s.Caret, s.Anchor
}

// Collapsed reports whether the selection selects nothing.
func (s Selection) Collapsed() bool { return s.Anchor == s.Caret }

// Shift moves the caret to p, keeping the anchor.
func (s Selection) Shift(p Point) Selection { return Selection{Anchor: s.Anchor, Caret: p} }

// Text returns the selected runes of text, a single block's text.
func (s Selection) Text(text string) string {
	start, end := s.Order()
	if start.Block != end.Block {
		return ""
	}
	r := []rune(text)
	if start.Offset >= len(r) || end.Offset > len(r) || start.Offset < 0 || end.Offset < 0 {
		return ""
	}
	return string(r[start.Offset:end.Offset])
}

// TextOf returns the selected text of the document: one block's slice, or
// the blocks in between joined with newlines.
func (s Selection) TextOf(m *doc.Model) string {
	start, end := s.Order()
	blocks := m.Blocks()
	if len(blocks) == 0 {
		return ""
	}
	first, last := blocks[0].Index, blocks[len(blocks)-1].Index
	if start.Block < first || end.Block > last || start.Block < 0 {
		return ""
	}
	if start.Block == end.Block {
		return s.Text(m.Plain(start.Block))
	}
	var b strings.Builder
	b.WriteString(textFrom(m, start.Block, start.Offset, runeLen(m.Plain(start.Block))))
	for i := start.Block + 1; i < end.Block; i++ {
		b.WriteString("\n")
		b.WriteString(m.Plain(i))
	}
	b.WriteString("\n")
	b.WriteString(textFrom(m, end.Block, 0, end.Offset))
	return b.String()
}

// SelectWord returns the selection of the word at p within text.
func SelectWord(p Point, text string) Selection {
	r := []rune(text)
	if p.Offset < 0 || p.Offset >= len(r) {
		return Collapsed(p)
	}
	if !isWord(r[p.Offset]) {
		return Collapsed(p)
	}
	start := p.Offset
	for start > 0 && isWord(r[start-1]) {
		start--
	}
	end := p.Offset + 1
	for end < len(r) && isWord(r[end]) {
		end++
	}
	return Selection{Anchor: Point{Block: p.Block, Offset: start}, Caret: Point{Block: p.Block, Offset: end}}
}

// SelectLine returns the selection of the whole text of the block p is in.
func SelectLine(p Point, text string) Selection {
	return Selection{
		Anchor: Point{Block: p.Block, Offset: 0},
		Caret:  Point{Block: p.Block, Offset: runeLen(text)},
	}
}

func textFrom(m *doc.Model, block, from, to int) string {
	r := []rune(m.Plain(block))
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

func runeLen(text string) int { return len([]rune(text)) }

func isWord(r rune) bool {
	return !unicode.IsSpace(r) && !unicode.IsPunct(r)
}
