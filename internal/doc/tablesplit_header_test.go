package doc

import (
	"strings"
	"testing"
)

// A split table's header repeats in every slice's window, but a header
// cell is one parser block: appending a run per slice used to overwrite
// the cell's leafRun entry, leaving RunAt of the header naming the last
// slice's run. The fix drops the header from leafRun: the header is a
// visual copy each slice renders, and the view overlays it through the
// slice's own run, so search and selection never address another
// slice's runes through a header cell.

func TestSplitHeaderLeafRunNotRemapped(t *testing.T) {
	m := New(bigTableDoc(200))
	bs := sliceBlocks(m)
	if len(bs) < 3 {
		t.Fatalf("%d slices, want at least 3", len(bs))
	}
	// The parser's header row is block 0 (the doc opens with the table).
	header := m.TableRows(bs[0].Index)[0]
	cells := m.TableCells(header)
	if len(cells) == 0 {
		t.Fatal("header row has no cells")
	}
	for _, cell := range cells {
		if run, _ := m.RunAt(cell); run != nil {
			t.Fatalf("header cell %d mapped to run %q: the repeated header must stay out of leafRun", cell, run.Text[:20])
		}
	}
}

func TestSplitSliceLeafRunsPointAtOwnRun(t *testing.T) {
	m := New(bigTableDoc(200))
	bs := sliceBlocks(m)
	if len(bs) < 2 {
		t.Fatal("the giant table must split")
	}
	// Every slice's body cells resolve to that slice's run, not the last
	// one's: the first slice's first body cell "r0" sits just past the
	// repeated header the run leads with, inside the run whose Leaf is the
	// first slice's model index.
	first := bs[0].Index
	rows := m.TableRows(first)
	cell := m.TableCells(rows[1])[0]
	run, off := m.RunAt(cell)
	if run == nil {
		t.Fatal("first slice's body cell has no run")
	}
	if run.Leaf != first {
		t.Fatalf("body cell's run leaf = %d, want the first slice %d", run.Leaf, first)
	}
	// The run opens with the header row, cells joined by spaces.
	headerRun := 0
	for _, hc := range m.TableCells(rows[0]) {
		headerRun += len([]rune(m.Plain(hc))) + 1
	}
	if off != headerRun {
		t.Fatalf("body cell offset = %d, want %d, past the repeated header", off, headerRun)
	}
	// And the second slice's first body cell names the second slice's
	// run, which the old last-write-wins mapping could not do.
	second := bs[1].Index
	rows2 := m.TableRows(second)
	cell2 := m.TableCells(rows2[1])[0]
	run2, off2 := m.RunAt(cell2)
	if run2 == nil || run2.Leaf != second || off2 != headerRun {
		t.Fatalf("second slice's body cell: run leaf %v offset %d, want leaf %d at %d", run2, off2, second, headerRun)
	}
}

func TestSplitHeaderSearchStaysPerSlice(t *testing.T) {
	// The run text keeps the header per slice (the view repeats it), so
	// a query for the header still hits once per slice, while RunAt of
	// the header cells stays unset.
	m := New(bigTableDoc(200))
	m.SetSearch("c1", SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatal("header text missed by the search")
	}
	slices := len(sliceBlocks(m))
	if len(m.Matches) < slices {
		t.Fatalf("%d header matches, want at least one per slice (%d)", len(m.Matches), slices)
	}
	// Every match's block is a slice whose run carries the header text.
	for _, mt := range m.Matches {
		run := m.RunOf(mt.Block)
		if run == nil {
			t.Fatalf("match block %d has no run", mt.Block)
		}
		if !strings.Contains(run.Text, "c1") {
			t.Fatalf("match run %q lacks the header text", run.Text[:30])
		}
	}
}
