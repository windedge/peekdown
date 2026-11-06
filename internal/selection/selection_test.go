package selection

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/windedge/peekdown/internal/doc"
)

const sample = `# Title

Alpha beta gamma delta.

More text here.

## Next

End paragraph.

` + "```go\nx := 1\n```\n\n" + `| a | b |
|---|---|
| 1 | 2 |
`

func model(t *testing.T) *doc.Model {
	t.Helper()
	return doc.New(sample)
}

func TestEndpointsCompare(t *testing.T) {
	a := Point{Block: 2, Offset: 5}
	if !a.Before(Point{Block: 2, Offset: 9}) {
		t.Fatal("same block: earlier offset should be before")
	}
	if !a.Before(Point{Block: 3, Offset: 0}) {
		t.Fatal("earlier block should be before")
	}
	if a.Before(a) {
		t.Fatal("a point is not before itself")
	}
	if (Point{Block: 2, Offset: 9}).Before(Point{Block: 2, Offset: 4}) {
		t.Fatal("a later offset is before an earlier one")
	}
}

func TestCollapsedSelection(t *testing.T) {
	s := Collapsed(Point{Block: 2, Offset: 3})
	if !s.Collapsed() {
		t.Fatal("collapsed")
	}
	if s.Text("alpha beta") != "" {
		t.Fatal("a collapsed selection selects nothing")
	}
}

func TestNormalization(t *testing.T) {
	s := Selection{Anchor: Point{Block: 3, Offset: 4}, Caret: Point{Block: 2, Offset: 3}}
	start, end := s.Order()
	if start != (Point{Block: 2, Offset: 3}) || end != (Point{Block: 3, Offset: 4}) {
		t.Fatalf("order: %v %v", start, end)
	}
}

func TestTextWithinOneBlock(t *testing.T) {
	text := "Alpha beta gamma delta."
	s := Selection{
		Anchor: Point{Block: 2, Offset: 6},
		Caret:  Point{Block: 2, Offset: 16},
	}
	if got := s.Text(text); got != "beta gamma" {
		t.Fatalf("text: %q", got)
	}
}

func TestTextAcrossBlocks(t *testing.T) {
	m := model(t)
	// From the middle of the first paragraph to the middle of the second.
	s := Selection{
		Anchor: Point{Block: 2, Offset: 11},
		Caret:  Point{Block: 3, Offset: 5},
	}
	got := s.TextOf(m)
	want := "gamma delta.\nMore "
	if got != want {
		t.Fatalf("across: %q, want %q", got, want)
	}
}

func TestTextIncludesFullEndsOfMiddleBlocks(t *testing.T) {
	m := doc.New("# T\n\none two\n\nthree four\n\nfive six\n")
	s := Selection{
		Anchor: Point{Block: 2, Offset: 4},
		Caret:  Point{Block: 4, Offset: 4},
	}
	got := s.TextOf(m)
	want := "two\nthree four\nfive"
	if got != want {
		t.Fatalf("middle blocks: %q, want %q", got, want)
	}
}

func TestSelectWord(t *testing.T) {
	m := model(t)
	text := m.Plain(m.Blocks()[1].Index)
	s := SelectWord(Point{Block: 2, Offset: 8}, text)
	if got := s.Text(text); got != "beta" {
		t.Fatalf("word: %q", got)
	}
	s2 := SelectWord(Point{Block: 2, Offset: 0}, text)
	if got := s2.Text(text); got != "Alpha" {
		t.Fatalf("first word: %q", got)
	}
	s3 := SelectWord(Point{Block: 2, Offset: 100}, text)
	if !s3.Collapsed() {
		t.Fatalf("offset past the end: %v", s3)
	}
}

func TestSelectLine(t *testing.T) {
	m := model(t)
	text := m.Plain(m.Blocks()[1].Index)
	s := SelectLine(Point{Block: 2, Offset: 7}, text)
	if got := s.Text(text); got != text {
		t.Fatalf("line: %q, want %q", got, text)
	}
}

func TestAll(t *testing.T) {
	m := model(t)
	s := All(m)
	want := "Title\nAlpha beta gamma delta.\nMore text here.\nNext\nEnd paragraph.\nx := 1\na b 1 2"
	if got := s.TextOf(m); got != want {
		t.Fatalf("all: %q, want %q", got, want)
	}
}

// A selection crossing a code block copies the code's text, which the run
// table holds where the old per-block Plain gave the block nothing.
func TestTextOfCrossesACodeBlock(t *testing.T) {
	m := model(t)
	blocks := m.Blocks()
	var code, table int
	for _, b := range blocks {
		switch b.Kind {
		case parser.CodeBlock:
			code = b.Index
		case parser.Table:
			table = b.Index
		}
	}
	if code == 0 || table == 0 {
		t.Fatalf("blocks %+v: no code or table", blocks)
	}
	s := Selection{
		Anchor: Point{Block: blocks[1].Index, Offset: 6},
		Caret:  Point{Block: table, Offset: 3},
	}
	got := s.TextOf(m)
	want := "beta gamma delta.\nMore text here.\nNext\nEnd paragraph.\nx := 1\na b"
	if got != want {
		t.Fatalf("across code and table: %q, want %q", got, want)
	}
	if !strings.Contains(got, "x := 1") {
		t.Fatalf("the code's text is missing from %q", got)
	}
}

func TestShiftExtend(t *testing.T) {
	s := Collapsed(Point{Block: 2, Offset: 0})
	s2 := s.Shift(Point{Block: 2, Offset: 5})
	if s2.Anchor != (Point{Block: 2, Offset: 0}) || s2.Caret != (Point{Block: 2, Offset: 5}) {
		t.Fatalf("shift keeps the anchor: %+v", s2)
	}
	s3 := s2.Shift(Point{Block: 2, Offset: 2})
	if s3.Caret != (Point{Block: 2, Offset: 2}) {
		t.Fatalf("shift back: %+v", s3)
	}
	if s3.Collapsed() {
		t.Fatal("anchor 0, caret 2 selects")
	}
	s4 := s3.Shift(Point{Block: 2, Offset: 0})
	if !s4.Collapsed() {
		t.Fatal("caret back at the anchor collapsed")
	}
}

func TestParagraphOffsets(t *testing.T) {
	m := model(t)
	plain := m.Plain(m.Blocks()[1].Index)
	if plain != "Alpha beta gamma delta." {
		t.Fatalf("plain: %q", plain)
	}
	// The model text of a block is what offsets address.
	if len(plain) != 23 {
		t.Fatalf("length: %d", len(plain))
	}
}
