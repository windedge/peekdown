package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// A row's cells rarely hold the same number of lines: the column rules
// must span the row, not stop at each cell's own content (shot: every
// rule broke into a short segment per cell, leaving a gap at each row
// boundary).

// ruleTableDoc is a 3 column table whose first body row pairs one very
// tall cell with two one-line ones.
func ruleTableDoc() string {
	long := strings.Repeat("alpha beta gamma delta ", 30)
	return "| Key | Evidence | State |\n" +
		"|---|---|---|\n" +
		"| short | " + long + " | ok |\n" +
		"| also short | one line | yes |\n"
}

// tableFrame renders src with the column budget of a wide window, so the
// columns keep their measured integer widths and the rules stay crisp.
func tableFrame(t *testing.T, src string) (*ui.Tester, *State) {
	t.Helper()
	m := doc.New(src)
	st := &State{AnchorOf: anchorsOf(m), ContentWidth: 2000}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, st)
	}, 1600, 700)
	tt.Frame()
	return tt, st
}

// TestTableColumnRulesSpanTheRow scans the column rules from the top of
// the table's text to its bottom and expects no gap: every cell is as
// high as its row.
func TestTableColumnRulesSpanTheRow(t *testing.T) {
	theme := ui.LightTheme()
	tt, _ := tableFrame(t, ruleTableDoc())
	img := tt.Image()
	_, _, band := surfaceRows(img, panelColor(theme))
	if band < 0 {
		t.Fatal("the header's surface did not draw")
	}
	runs := colorRuns(img, band, panelColor(theme))
	if len(runs) != 3 {
		t.Fatalf("y=%d: %v, want one surface run per column", band, runs)
	}
	var rules [][2]int
	for k := 1; k < len(runs); k++ {
		rules = append(rules, [2]int{runs[k-1][1] + 1, runs[k][0] - 1})
	}
	// The table's vertical extent: its texts' union, which sits inside the
	// cells' boxes.
	top, bottom := -1, -1
	for _, label := range tt.Texts() {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", clip(label))
		}
		if top < 0 || int(r.Y) < top {
			top = int(r.Y)
		}
		bottom = max(bottom, int(r.Y+r.H))
	}
	if top < 0 {
		t.Fatalf("no table text: %v", tt.Texts())
	}
	for _, span := range rules {
		for y := top; y < bottom; y++ {
			hit := false
			for x := span[0]; x <= span[1]; x++ {
				if pixelIs(img, x, y, theme.Border) {
					hit = true
					break
				}
			}
			if !hit {
				t.Fatalf("rule x=%d..%d breaks at y=%d (text spans y %d..%d): the cells are not as high as the row",
					span[0], span[1], y, top, bottom)
			}
		}
	}
}

// TestHTMLTableColumnRulesSpanTheRow holds the HTML table - the grid the
// reader shares with markdown tables - to the same rule.
func TestHTMLTableColumnRulesSpanTheRow(t *testing.T) {
	long := strings.Repeat("alpha beta gamma delta ", 30)
	src := "before\n\n<table>\n" +
		"<tr><th>Key</th><th>Evidence</th><th>State</th></tr>\n" +
		"<tr><td>short</td><td>" + long + "</td><td>ok</td></tr>\n" +
		"<tr><td>also</td><td>one</td><td>yes</td></tr>\n" +
		"</table>\n\nafter\n"
	theme := ui.LightTheme()
	tt, _ := tableFrame(t, src)
	img := tt.Image()
	_, _, band := surfaceRows(img, panelColor(theme))
	if band < 0 {
		t.Fatal("the header's surface did not draw")
	}
	runs := colorRuns(img, band, panelColor(theme))
	if len(runs) != 3 {
		t.Fatalf("y=%d: %v, want one surface run per column", band, runs)
	}
	var rules [][2]int
	for k := 1; k < len(runs); k++ {
		rules = append(rules, [2]int{runs[k-1][1] + 1, runs[k][0] - 1})
	}
	top, bottom := -1, -1
	for _, label := range tt.Texts() {
		if label == "before" || label == "after" {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", clip(label))
		}
		if top < 0 || int(r.Y) < top {
			top = int(r.Y)
		}
		bottom = max(bottom, int(r.Y+r.H))
	}
	if top < 0 {
		t.Fatalf("no table text: %v", tt.Texts())
	}
	for _, span := range rules {
		for y := top; y < bottom; y++ {
			hit := false
			for x := span[0]; x <= span[1]; x++ {
				if pixelIs(img, x, y, theme.Border) {
					hit = true
					break
				}
			}
			if !hit {
				t.Fatalf("rule x=%d..%d breaks at y=%d (text spans y %d..%d): the cells are not as high as the row",
					span[0], span[1], y, top, bottom)
			}
		}
	}
}
