package view

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// A split table renders slice by slice: the view List never lays a whole
// giant table out in one row (regression: by-stars.md's 1281-row block,
// 54 MB allocated per frame, OOM at 60fps).

func giantTableDoc(n int) string {
	var b strings.Builder
	b.WriteString("| c1 | c2 |\n|---|---|\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "| r%d | x%d |\n", i, i)
	}
	return b.String()
}

func renderGiant(t *testing.T, src string) (*doc.Model, *ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	state := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, state, m.RowBlock(r).Index)
		})
	}, 800, 600)
	return m, tt, state
}

// TestGiantTableShowsFirstAndLastSlice scrolls a 1281-row table to its
// end and expects the last slice's rows on screen next to the "rows"
// note; the first slice's header must sit at the top of the document.
func TestGiantTableShowsFirstAndLastSlice(t *testing.T) {
	m, tt, state := renderGiant(t, giantTableDoc(1281))
	if !tt.HasText("c1") {
		t.Fatalf("texts %v: want the header row", tt.Texts())
	}
	if got, want := len(m.Blocks()), 1281/63+1; got != want {
		t.Fatalf("%d slice blocks, want %d", got, want)
	}
	state.List.ScrollToEnd()
	tt.Frame()
	found, note := false, false
	for _, s := range tt.Texts() {
		if s == "r1280" || s == "x1280" || strings.Contains(s, "r1280 x1280") {
			found = true
		}
		if strings.Contains(s, "rows 1261-1281 of 1281") {
			note = true
		}
	}
	if !found {
		t.Fatalf("texts after the end %v: want the table's last row", tt.Texts())
	}
	if !note {
		t.Fatalf("texts %v: want the last slice's rows note", tt.Texts())
	}
}

// TestGiantTableSliceSearchHighlight runs a search against the slices'
// runs, scrolls to the hit the way the search bar does and renders: the
// hit's row shows inside its slice.
func TestGiantTableSliceSearchHighlight(t *testing.T) {
	m, tt, state := renderGiant(t, giantTableDoc(1281))
	m.SetSearch("r1280", doc.SearchOptions{})
	if len(m.Matches) == 0 {
		t.Fatalf("no match for r1280 in %d runs", len(m.Runs()))
	}
	state.Matches = m.Matches
	state.Current = m.Current
	state.ScrollTargetBlock = m.Matches[m.Current].Block
	state.scrollPendingMatch(m)
	tt.Frame()
	found := false
	for _, s := range tt.Texts() {
		if s == "r1280" || s == "x1280" || strings.Contains(s, "r1280 x1280") {
			found = true
		}
	}
	if !found {
		t.Fatalf("texts %v: want the hit's row rendered", tt.Texts())
	}
}

// BenchmarkGiantTableFrame measures one frame of a virtualized document
// whose only block is a 1281-row table, by-stars.md's shape. regression:
// giant table must be virtualized per-slice; before the split it drew
// 54 MB per frame through one row callback and grew without bound.
func BenchmarkGiantTableFrame(b *testing.B) {
	if testing.Short() {
		b.Skip("memory regression: run on the full suite only")
	}
	m := doc.New(giantTableDoc(1281))
	state := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ui.List(c, &state.List, m.Len(), func(r int) {
			Render(c, m, state, m.RowBlock(r).Index)
		})
	}, 800, 600)
	// Warm one frame so the first build's allocations stay out.
	tt.Frame()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	b.ResetTimer()
	for k := 0; k < b.N; k++ {
		tt.Frame()
	}
	b.StopTimer()
	runtime.ReadMemStats(&after)
	perFrame := (after.TotalAlloc - before.TotalAlloc) / uint64(max(b.N, 1))
	b.ReportMetric(float64(perFrame)/1e6, "MB/frame")
	if perFrame > 2e6 {
		b.Fatalf("regression: giant table frame allocates %d B/frame, over the 2 MB budget", perFrame)
	}
}
