package selection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/windedge/peekdown/internal/doc"
)

// MarkdownOf copies the document's markdown source, so the wrappers ride
// along: a heading's "#", a list's "-", a table's pipes, a paragraph's
// "**" stay in the text the way the source line has them. Blocks join
// with one newline; a block the selection only cuts into is cut by the
// run's rune offsets, widened at each end to the nearest content rune,
// which drops the wrappers wholly inside the cut's blank flanks and
// keeps the ones sitting between content ("a link](url" from a cut
// opening inside the label).

func wholeDoc(m *doc.Model) Selection {
	runs := m.Runs()
	return Selection{
		Anchor: Point{Block: runs[0].Block, Offset: 0},
		Caret:  Point{Block: runs[len(runs)-1].Block, Offset: runeLen(runs[len(runs)-1].Text)},
	}
}

func TestMarkdownOfKeepsMarkup(t *testing.T) {
	src := "Para with **bold** text.\n\n## Title\n\n- item one\n- item two\n"
	m := doc.New(src)
	got := wholeDoc(m).MarkdownOf(m)
	want := "Para with **bold** text.\n## Title\n- item one\n- item two"
	if got != want {
		t.Fatalf("MarkdownOf = %q, want %q", got, want)
	}
	for _, mark := range []string{"**bold**", "## Title", "- item one", "- item two"} {
		if !strings.Contains(got, mark) {
			t.Fatalf("MarkdownOf %q lost the %q markup", got, mark)
		}
	}
}

func TestMarkdownOfPlainBlockMatchesTextOf(t *testing.T) {
	m := doc.New("plain text only\n")
	s := wholeDoc(m)
	got, want := s.MarkdownOf(m), s.TextOf(m)
	if got != want {
		t.Fatalf("MarkdownOf = %q, TextOf = %q: a plain block copies without markup either way", got, want)
	}
}

func TestMarkdownOfPartialBlocks(t *testing.T) {
	src := "Alpha beta gamma delta.\n"
	bold := "Alpha **bold** omega."
	cases := []struct {
		name       string
		src        string
		start, end int
		want       string
	}{
		{"second half of one block", src, 11, 23, "gamma delta."},
		{"mid word", src, 6, 11, "beta"},
		{"from inside the bold's marks", bold, 6, 15, "bold"},
		// 18 is past the run's 17 runes: it clamps to the run's end, and
		// an end at the run's end widens to the chunk's end, the cut
		// keeping the closing marks rather than slicing mid-word.
		{"through the closing marks", bold, 6, 18, "bold** omega."},
		{"reversed endpoints", bold, 12, 6, "bold"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := doc.New(tc.src)
			b := m.Blocks()[0].Index
			s := Selection{
				Anchor: Point{Block: b, Offset: tc.start},
				Caret:  Point{Block: b, Offset: tc.end},
			}
			if got := s.MarkdownOf(m); got != tc.want {
				t.Fatalf("MarkdownOf = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMarkdownOfReversedAcrossBlocks(t *testing.T) {
	m := doc.New("Alpha beta.\n\nGamma delta.\n")
	blocks := m.Blocks()
	fwd := Selection{
		Anchor: Point{Block: blocks[0].Index, Offset: 6},
		Caret:  Point{Block: blocks[1].Index, Offset: 5},
	}
	rev := Selection{Anchor: fwd.Caret, Caret: fwd.Anchor}
	if got, want := fwd.MarkdownOf(m), rev.MarkdownOf(m); got != want || got != "beta.\nGamma" {
		t.Fatalf("forward %q, reversed %q, want both %q", fwd.MarkdownOf(m), rev.MarkdownOf(m), "beta.\nGamma")
	}
}

func TestMarkdownOfTable(t *testing.T) {
	src := "| a | b |\n|---|---|\n| 1 | 2 |\n\nAfter table.\n"
	m := doc.New(src)
	got := wholeDoc(m).MarkdownOf(m)
	want := "| a | b |\n|---|---|\n| 1 | 2\nAfter table."
	if got != want {
		t.Fatalf("MarkdownOf = %q, want %q", got, want)
	}
	if !strings.Contains(got, "| a | b |") || !strings.Contains(got, "| 1 | 2") {
		t.Fatalf("MarkdownOf %q lost the table's source lines", got)
	}
}

// A table past the split threshold becomes slice blocks that all report
// the whole table's span. The copy must cut each slice's own row window,
// so every source line appears exactly once: selecting all must not
// replay the table once per slice.
func TestMarkdownOfSplitTableCopiesEachRowOnce(t *testing.T) {
	var b strings.Builder
	b.WriteString("| c1 | c2 |\n|---|---|\n")
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&b, "| r%d | x%d |\n", i, i)
	}
	m := doc.New(b.String())
	var slices int
	for _, blk := range m.Blocks() {
		if blk.TableSlice {
			slices++
		}
	}
	if slices < 2 {
		t.Fatalf("%d slice blocks, want the table split", slices)
	}
	got := wholeDoc(m).MarkdownOf(m)
	if !strings.HasPrefix(got, "| c1 | c2 |\n|---|---|\n") {
		t.Fatalf("MarkdownOf %q does not open with the table's header", got)
	}
	if n := strings.Count(got, "| c1 | c2 |"); n != 1 {
		t.Fatalf("the header row copied %d times, want once", n)
	}
	// Rows straddling the slices' seams, first and last body row: each
	// line of the source appears exactly once.
	for _, i := range []int{0, 1, 62, 63, 69} {
		row := fmt.Sprintf("| r%d | x%d", i, i)
		if n := strings.Count(got, row); n != 1 {
			t.Fatalf("row %q copied %d times, want once", row, n)
		}
	}
}
