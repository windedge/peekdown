package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// TestContentWidthCentersTextColumn pins State.ContentWidth: the list and
// its rows span the window, so the overlay bar keeps to the window's edge,
// while the text column alone caps at ContentWidth and centers.
func TestContentWidthCentersTextColumn(t *testing.T) {
	src := strings.Repeat("word ", 200) + "\n"
	m := doc.New(src)

	render := func(width int, content float32) *ui.Tester {
		var st State
		st.ContentWidth = content
		tt := ui.NewTester(func(c *ui.Context) {
			c.DisableAnimations()
			ScrollView(c, m, &st)
		}, width, 900)
		tt.Frame()
		return tt
	}

	// Wide window, capped: the column is 900 DIPs, centered in the 1600 the
	// list spans; its text keeps the right clearance inside it.
	tt := render(1600, 900)
	r := rectOf(t, tt, "word")
	if r.W > 900 {
		t.Fatalf("the text is %v wide, want at most the 900 DIP cap", r.W)
	}
	if center := r.X + r.W/2; center < 800-20 || center > 800+20 {
		t.Fatalf("the text centers at %v, want the window's 800", center)
	}

	// The same window, uncapped: the column follows the list.
	full := render(1600, 0)
	fr := rectOf(t, full, "word")
	if fr.W < 1500 {
		t.Fatalf("uncapped: the text is %v wide, want the list's width", fr.W)
	}
	if fr.X >= r.X {
		t.Fatalf("uncapped: the text starts at x=%v, want it left of the capped column's %v", fr.X, r.X)
	}

	// A window narrower than the cap: the column follows the room.
	narrow := render(700, 900)
	nr := rectOf(t, narrow, "word")
	if nr.X != 0 {
		t.Fatalf("narrow: the text starts at x=%v, want the list's left edge", nr.X)
	}
	// The right clearance the rows carry keeps the overlay bar's 9 DIPs
	// off the glyphs, and a column wider than the window would clip the
	// line instead.
	if got := 700 - (nr.X + nr.W); got < 12 {
		t.Fatalf("narrow: the text ends %v DIPs from the window's edge, want >= 12", got)
	}
}

// TestContentWidthKeepsDragSelectionInTheColumn checks the pointer's hit in
// a capped column: the rows span the window but the text sits centered
// inside them, so a drag over the centered lines must still select them.
func TestContentWidthKeepsDragSelectionInTheColumn(t *testing.T) {
	m := doc.New("First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n")
	var st State
	st.ContentWidth = 900
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &st)
	}, 1600, 900)

	// Press in the empty band beside the column first: it must not select,
	// and must not break the drag that follows.
	tt.Press(60, 40)
	tt.Release(60, 40)
	tt.Frame()
	if st.HasSelection() {
		t.Fatalf("a press beside the column selected %q", st.SelectionText(m))
	}

	x0, y0 := findText(t, tt, "First", false)
	x1, y1 := findText(t, tt, "Third", true)
	if x0 < 300 || x1 > 1300 {
		t.Fatalf("the lines run %v..%v, want them inside the 900 DIP centered column", x0, x1)
	}
	dragFromTo(tt, x0, y0, x1, y1)
	if got := st.SelectionText(m); got != "First paragraph.\nSecond paragraph.\nThird paragraph." {
		t.Fatalf("selected %q", got)
	}
}
