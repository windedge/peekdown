package selection

import (
	"strings"

	"github.com/windedge/peekdown/internal/doc"
)

// isSemanticBlank reports whether r carries no content the reader picks
// up: whitespace and the markdown wrappers the selection addresses as
// their inner text. Lines keep their markup - a heading's "#", a
// list's "-", a table's pipes ride along, as the Rust version copies
// them.
func isSemanticBlank(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '*', '_', '`', '~', '#', '-', '+', '>', '|', '=', '[', ']', '(', ')', '!', '<', '"':
		return true
	}
	return false
}

// trimToContent walks i forward past blank runes to the first content.
func trimToContent(r []rune, i int) int {
	for i >= 0 && i < len(r) && isSemanticBlank(r[i]) {
		i++
	}
	return i
}

// trimPastContent walks i back past blank runes to just after the last
// content, so the cut keeps the wrapper of what was selected.
func trimPastContent(r []rune, i int) int {
	for i > 0 && i <= len(r) && isSemanticBlank(r[i-1]) {
		i--
	}
	return i
}

// MarkdownOf returns the selected text as the document's markdown
// source, matching the Rust viewer's copy: a block copies its source
// lines, markup and all, and a block the selection only cuts into is
// cut by rune offsets widened to the nearest content rune. The offsets
// name the run's rendered text; the source chunk holds the same words
// with the syntax between them, and the run's rune count bounds where
// they sit. Blocks in between copy whole. Blocks in reading order join
// with one newline; Select All ends up with the source's blocks each on
// its line.
func (s Selection) MarkdownOf(m *doc.Model) string {
	runs := m.Runs()
	start, end := s.Order()
	startAt, endAt := clampRun(runs, start.Block), clampRun(runs, end.Block)
	if startAt > endAt {
		startAt, endAt = endAt, startAt
	}
	src := m.Source()
	var parts []string
	for k := startAt; k <= endAt; k++ {
		run := runs[k]
		from, to := 0, runeLen(run.Text)
		// An endpoint's own run cuts at its offset; a runless block the
		// endpoint named leaves the run beside it whole, as TextOf does.
		if k == startAt && run.Block == start.Block {
			from = max(from, start.Offset)
		}
		if k == endAt && run.Block == end.Block {
			to = min(to, max(0, end.Offset))
		}
		chunk := sourceChunk(src, m, run.Block)
		if chunk == "" {
			if text := m.RunText(run, from, to); text != "" {
				parts = append(parts, text)
			}
			continue
		}
		if from > 0 || to < runeLen(run.Text) {
			chunk = partialSourceChunk(chunk, run, from, to)
		}
		if chunk != "" {
			parts = append(parts, chunk)
		}
	}
	return strings.Join(parts, "\n")
}

// sourceChunk returns a block's source text, the bytes Start..End of
// the body the model parsed. A table slice reports the whole table's
// span, so it cuts its own window out of the table instead; any block
// past the source's length yields "".
func sourceChunk(src string, m *doc.Model, block int) string {
	b := m.Block(block)
	if b.TableSlice {
		return tableSliceChunk(src, m, b)
	}
	if b.Start < 0 || b.End > len(src) || b.Start >= b.End {
		return ""
	}
	return src[b.Start:b.End]
}

// tableSliceChunk returns the source lines one slice of a split table
// shows. Slicing the slice's own Start..End would copy the whole table
// once per slice: the slices share the table's span. The header renders
// at the front of every slice but the source holds it once, so only the
// first slice opens with it; a whole table's selection then copies the
// table's source once, every row on its line.
func tableSliceChunk(src string, m *doc.Model, b doc.Block) string {
	rows := m.TableRows(b.Index)
	if len(rows) == 0 {
		return ""
	}
	start := b.Start
	if b.TableSliceFirst > 0 {
		// rows[0] is the header the render repeats here, rows[1] the
		// slice's first body row: the source starts there.
		if len(rows) < 2 {
			return ""
		}
		start = m.Block(rows[1]).Start
	}
	end := m.Block(rows[len(rows)-1]).End
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return src[start:end]
}

// partialSourceChunk cuts the bytes of runes from..to out of the
// block's source, widening each end to the nearest content rune: the
// selection's rune offsets count the run's rendered text, whose
// wrappers and line paddings sit between the words in the source. to
// at the run's end reads as the chunk's end - inline markup pads the
// source past the run's count, an index there would cut mid-word. The
// chunk's rune count runs from the block's first rune on, so the same
// index reads the same word - unless the run holds text the source
// rewrites (an indented code's tab padding); the wrap then widens to
// the whole chunk, copying more, never mangling.
func partialSourceChunk(chunk string, run doc.Run, from, to int) string {
	r := []rune(chunk)
	if to > len(r) {
		// The chunk holds fewer runes than the run named: the run's text
		// came from unwrapped lines (a code block's). Take the block
		// whole rather than cut at a lie.
		return chunk
	}
	if to == runeLen(run.Text) {
		to = len(r)
	}
	from = trimToContent(r, from)
	if from >= len(r) {
		return ""
	}
	to = trimPastContent(r, min(to, len(r)))
	if to <= from {
		return ""
	}
	return string(r[from:to])
}
