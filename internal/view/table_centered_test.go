package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// longCell is 60+ chars per column, the shape of the real-window shot
// that started this: every column caps at 300, four columns want ~1205
// DIPs, more than the 900 the centered column allows. Each cell carries
// its column's prefix, so the tester's Find (first match only) tells the
// columns apart.
const longCell = "Keep the build green on every supported platform, including windows, linux and darwin, on every single run. "

func reproDoc() *doc.Model {
	head := "| Rule ID | Description | Owner | Status |\n|---|---|---|---|\n"
	var b strings.Builder
	b.WriteString(head)
	for r := 1; r <= 3; r++ {
		cells := make([]string, 4)
		for k := range cells {
			cells[k] = fmt.Sprintf("C%d %s", k+1, longCell)
		}
		fmt.Fprintf(&b, "| PR-%03d | %s |\n", r, strings.Join(cells, " | "))
	}
	b.WriteString("\nParagraph after the table.\n")
	return doc.New(b.String())
}

func clip(s string) string {
	if len(s) > 30 {
		return s[:30]
	}
	return s
}

// TestWideTableStaysInCenteredColumn pins the fix: mygo does not clamp a
// fixed-width cell inside a MaxWidth'd grid, so the table's columns must
// shrink into the room the centered column leaves. Every table text sits
// inside the column (335..1265 with slack), not spilled toward the
// window's right as the user's shot showed.
func TestWideTableStaysInCenteredColumn(t *testing.T) {
	m := reproDoc()
	var st State
	st.ContentWidth = 900
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &st)
	}, 1600, 900)
	tt.Frame()

	lo, hi := float32(305), float32(1295) // column 335..1265 plus 30 slack
	loX, hiX := float32(1600), float32(0)
	found := 0
	for _, label := range tt.Texts() {
		header := label == "Rule ID" || label == "Description" ||
			label == "Owner" || label == "Status"
		if !header && !strings.HasPrefix(label, "C") && !strings.Contains(label, "PR-") {
			continue
		}
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", label)
		}
		found++
		loX = min(loX, r.X)
		hiX = max(hiX, r.X+r.W)
		if r.X < lo {
			t.Errorf("text %q starts at %.1f, want >= %.1f", clip(label), r.X, lo)
		}
		if r.X+r.W > hi {
			t.Errorf("text %q ends at %.1f, want <= %.1f", clip(label), r.X+r.W, hi)
		}
	}
	// 4 headers + 3 rows x 4 cells; a wrapped cell is one text node.
	if found != 16 {
		t.Fatalf("found %d table texts, want 16; texts %v", found, tt.Texts())
	}
	t.Logf("table spans %.1f..%.1f (centered column 335..1265)", loX, hiX)
}
