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

// All returns a selection of the whole document: the first run's start to
// the last run's end.
func All(m *doc.Model) Selection {
	runs := m.Runs()
	if len(runs) == 0 {
		return Collapsed(Point{})
	}
	first, last := runs[0], runs[len(runs)-1]
	return Selection{
		Anchor: Point{Block: first.Block, Offset: 0},
		Caret:  Point{Block: last.Block, Offset: runeLen(last.Text)},
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

// TextOf returns the selected text of the document, from the run table:
// the endpoints are (top-level block, rune offset in that block's run
// text). Runs in between join whole; a run's text joins the others with a
// newline. Endpoints whose block carries no run (say, a lone image) count
// as empty; offsets past a run's end clamp to it.
func (s Selection) TextOf(m *doc.Model) string {
	start, end := s.Order()
	var b strings.Builder
	startRun := true
	for _, run := range m.Runs() {
		if run.Block < start.Block {
			continue
		}
		if run.Block > end.Block {
			break
		}
		from, to := 0, runeLen(run.Text)
		if run.Block == start.Block {
			from = max(from, start.Offset)
		}
		if run.Block == end.Block {
			to = min(to, end.Offset)
		}
		if startRun {
			startRun = false
		} else {
			b.WriteString("\n")
		}
		b.WriteString(m.RunText(run, from, to))
	}
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

func runeLen(text string) int { return len([]rune(text)) }

func isWord(r rune) bool {
	return !unicode.IsSpace(r) && !unicode.IsPunct(r)
}
