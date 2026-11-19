package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

const narrowTableDoc = "| Name | Value |\n|---|---|\n| alpha | beta |\n"

// tableGridSpan reads the table's drawn width from the header's surface:
// the text start gives the grid's left edge after its border and cell
// padding, and the surface's last pixel gives its right edge.
func tableGridSpan(t *testing.T, tt *ui.Tester, header string) (float32, float32) {
	t.Helper()
	r, ok := tt.Find(header)
	if !ok {
		t.Fatalf("%q not found in %v", header, tt.Texts())
	}
	right := rightmostColorX(tt.Image(), panelColor(ui.LightTheme()))
	if right < 0 {
		t.Fatal("the table's header surface did not draw")
	}
	return r.X - 11, float32(right) + 2
}

func TestNarrowTableFillsReadingColumn(t *testing.T) {
	const windowWidth = 1200
	_, tt, _ := frame(t, narrowTableDoc, windowWidth, 400)
	left, right := tableGridSpan(t, tt, "Name")
	want := float32(windowWidth - rowClearance)
	if absF(right-left-want) > 2 {
		t.Fatalf("table spans %.1f..%.1f (width %.1f), want the %.1f reading column", left, right, right-left, want)
	}
	if absF(left) > 2 {
		t.Fatalf("table starts at %.1f, want the column's left edge", left)
	}
}

func TestWideTableStaysWithinReadingColumn(t *testing.T) {
	const windowWidth = 1200
	src := "| A | B |\n|---|---|\n| " + strings.Repeat("x", 200) + " | " + strings.Repeat("y", 200) + " |\n"
	_, tt, _ := frame(t, src, windowWidth, 500)
	left, right := tableGridSpan(t, tt, "A")
	want := float32(windowWidth - rowClearance)
	if right-left > want+2 {
		t.Fatalf("table spans %.1f..%.1f (width %.1f), past the %.1f reading column", left, right, right-left, want)
	}
}

func TestNarrowTableFillsCenteredColumn(t *testing.T) {
	const windowWidth = 1600
	m := doc.New(narrowTableDoc)
	st := &State{AnchorOf: anchorsOf(m), ContentWidth: 900}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, windowWidth, 900)
	tt.Frame()

	left, right := tableGridSpan(t, tt, "Name")
	want := st.ContentWidth - rowClearance
	if absF(right-left-want) > 2 {
		t.Fatalf("table spans %.1f..%.1f (width %.1f), want the %.1f centered column", left, right, right-left, want)
	}
	if absF(left-float32(windowWidth-st.ContentWidth)/2) > 2 {
		t.Fatalf("table starts at %.1f, want the centered column's left edge", left)
	}
}

func TestTableColumnRatiosFollowMeasuredContent(t *testing.T) {
	const windowWidth = 1200
	src := "| A | BB | CCC |\n|---|---|---|\n| " + strings.Repeat("a", 12) + " | " +
		strings.Repeat("b", 36) + " | " + strings.Repeat("c", 12) + " |\n"
	m, tt, st := frame(t, src, windowWidth, 400)
	ratios, _ := tableColumnRatios(tableColumnWidths(st, m, m.Blocks()[0].Index))

	first, ok := tt.Find(strings.Repeat("a", 12))
	if !ok {
		t.Fatalf("first column text not found in %v", tt.Texts())
	}
	second, ok := tt.Find(strings.Repeat("b", 36))
	if !ok {
		t.Fatalf("second column text not found in %v", tt.Texts())
	}
	third, ok := tt.Find(strings.Repeat("c", 12))
	if !ok {
		t.Fatalf("third column text not found in %v", tt.Texts())
	}
	// Each cell's left padding cancels in the differences between starts;
	// one pixel between them is the column's right rule.
	firstCol := second.X - first.X - 1
	secondCol := third.X - second.X - 1
	got := firstCol / secondCol
	want := ratios[0] / ratios[1]
	if absF(got-want) > want*0.12 {
		t.Fatalf("column width ratio %.3f (%.1f/%.1f), want %.3f from weights %v", got, firstCol, secondCol, want, ratios)
	}
	if third.X <= second.X+second.W || second.X <= first.X+first.W {
		t.Fatalf("columns at %v, %v, %v: cells overlap", first, second, third)
	}
}

func TestTableWidthShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the table fill screenshot")
	}
	out := os.Getenv("PEEKDOWN_SHOT_OUT")
	if out == "" {
		out = "C:/tmp/table-fill.png"
	}
	m := doc.New(narrowTableDoc)
	st := &State{AnchorOf: anchorsOf(m)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 1440, 880)
	tt.Frame()
	writeShot(t, tt.Image(), out)
}
