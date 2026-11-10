package doc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// A table past the split threshold becomes consecutive slice blocks, so
// the view virtualizes a giant table row by row instead of laying its
// whole text out in one List row (regression: the 6.9 GB working set of
// by-stars.md's single 1281-row block).

func bigTableDoc(n int) string {
	var b strings.Builder
	b.WriteString("| c1 | c2 |\n|---|---|\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "| r%d | x%d |\n", i, i)
	}
	return b.String()
}

func sliceBlocks(m *Model) []Block {
	var out []Block
	for _, b := range m.Blocks() {
		if b.TableSlice {
			out = append(out, b)
		}
	}
	return out
}

func TestGiantTableSplitsIntoSliceBlocks(t *testing.T) {
	m := New(bigTableDoc(200))
	for _, b := range m.Blocks() {
		if b.Kind != parser.Table {
			t.Fatalf("block %d is %v, want a table", b.Index, b.Kind)
		}
		if !b.TableSlice {
			t.Fatalf("block %d is not a slice; the giant table must split", b.Index)
		}
	}
	// A slice holds tableSliceRows rows counting the repeated header, so
	// 200 body rows make ceil(200/63) = 4 slices (63 + 63 + 63 + 11).
	if got, want := len(m.Blocks()), (200+tableSliceRows-2)/(tableSliceRows-1); got != want {
		t.Fatalf("%d slice blocks, want %d", got, want)
	}
}

func TestSmallTableStaysOneBlock(t *testing.T) {
	m := New("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if m.Len() != 1 {
		t.Fatalf("%d blocks, want 1", m.Len())
	}
	b := m.Blocks()[0]
	if b.TableSlice {
		t.Fatalf("a small table must not split")
	}
	if m.TableRows(b.Index)[0] == 0 {
		t.Fatalf("TableRows lost the header")
	}
}

func TestTableSlicesRowLayout(t *testing.T) {
	m := New(bigTableDoc(200))
	bs := m.Blocks()
	// Every slice opens with the repeated header, so 64 rows mean 63
	// body ones; the last slice carries the 11 left over.
	if n := len(m.TableRows(bs[0].Index)); n != 64 {
		t.Fatalf("first slice %d rows, want 64 (header + 63)", n)
	}
	if n := len(m.TableRows(bs[2].Index)); n != 64 {
		t.Fatalf("third slice %d rows, want 64", n)
	}
	if n := len(m.TableRows(bs[3].Index)); n != 12 {
		t.Fatalf("last slice %d rows, want 12 (header + 11)", n)
	}
	for k, b := range bs {
		if b.TableTotalRows != 200 {
			t.Fatalf("slice %d total rows %d, want 200", k, b.TableTotalRows)
		}
		if got, want := b.TableSliceFirst, k*(tableSliceRows-1); got != want {
			t.Fatalf("slice %d starts at body row %d, want %d", k, got, want)
		}
	}
	// The last slice's body rows are the table's last ones, in order.
	rows := m.TableRows(bs[3].Index)
	for k, row := range rows[1:] {
		if got, want := m.Plain(m.TableCells(row)[0]), fmt.Sprintf("r%d", 3*(tableSliceRows-1)+k); got != want {
			t.Fatalf("last slice row %d: %q, want %q", k, got, want)
		}
	}
}

func TestTableSlicesRunTextCoversAllRows(t *testing.T) {
	m := New(bigTableDoc(200))
	// Each slice's run repeats the header, so the rows must appear in
	// order across the joined run table: find "rN xN" for rising N.
	joined := strings.Join(runTextsOf(m), " ")
	at := 0
	for i := 0; i < 200; i++ {
		want := fmt.Sprintf("r%d x%d", i, i)
		k := strings.Index(joined[at:], want)
		if k < 0 {
			t.Fatalf("row %d (%q) missing or out of order in the runs %q", i, want, joined)
		}
		at += k + len(want)
	}
	// The header text opens every slice's run, so a search for it hits
	// each slice.
	hits := 0
	for _, r := range m.Runs() {
		if strings.Contains(r.Text, "c1 c2") {
			hits++
		}
	}
	if hits != len(m.Blocks()) {
		t.Fatalf("%d runs carry the header, want one per slice (%d)", hits, len(m.Blocks()))
	}
}

func TestTableSlicesReadingOrderMatchesBlocks(t *testing.T) {
	m := New("# T\n\n" + bigTableDoc(200) + "\nafter\n")
	bs := m.Blocks()
	slices := 200/tableSliceRows + 1 // ceil(200/63) = 4
	if len(bs) != 2+slices || bs[0].Kind != parser.Heading || bs[len(bs)-1].Kind != parser.Paragraph {
		t.Fatalf("%d blocks, want heading, %d table slices, paragraph", len(bs), slices)
	}
	for k, b := range bs {
		if m.RowBlock(k).Index != b.Index {
			t.Fatalf("RowBlock(%d) = %d, want %d", k, m.RowBlock(k).Index, b.Index)
		}
	}
	// One run per block: the slices' runs sit between the heading's and
	// the paragraph's, in the document's reading order.
	runs := m.Runs()
	if len(runs) != 2+slices {
		t.Fatalf("%d runs, want %d", len(runs), 2+slices)
	}
	if !strings.Contains(runs[1].Text, "r0 x0") || !strings.Contains(runs[1].Text, "r62 x62") {
		t.Fatalf("first slice run misses its rows: %q", runs[1].Text[:80])
	}
	if !strings.Contains(runs[2].Text, "r63 x63") {
		t.Fatalf("second slice run misses its rows: %q", runs[2].Text[:80])
	}
}

func TestTableSliceOfParserTableStillReads(t *testing.T) {
	// The view's table render reads a slice through TableRows, Children
	// and the cells' Plain, all of which answer for the slice's model
	// index without panicking. Lines, Info and Plain of the slice block
	// itself stay parser-only: a slice is not a parser block.
	m := New(bigTableDoc(200))
	b := m.Blocks()[1]
	for _, row := range m.TableRows(b.Index) {
		if m.Block(row).Kind != parser.TableRow {
			t.Fatalf("slice %d grows a %v row", b.Index, m.Block(row).Kind)
		}
		for _, cell := range m.TableCells(row) {
			_ = m.Plain(cell)
		}
	}
	for j := range m.Children(b.Index) {
		if m.Block(j).Kind != parser.TableRow {
			t.Fatalf("slice %d grows a %v child", b.Index, m.Block(j).Kind)
		}
	}
}

func runTextsOf(m *Model) []string {
	out := make([]string, 0, len(m.runs))
	for _, r := range m.runs {
		out = append(out, r.Text)
	}
	return out
}
