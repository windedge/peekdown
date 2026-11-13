package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Ticket #23 F: the delimiter row's alignment reaches the cells, and a
// task item's check box is drawn rather than typed.

// alignedTable's header cells are long enough to size the columns, so a
// short body cell has room to sit left, right or centered inside its
// column; every row of a column aligns alike.
const alignedTable = "| llllllllll | rrrrrrrrrr | cccccccccc |\n" +
	"| :--------- | ---------: | :--------: |\n" +
	"| L          | R          | C          |\n"

// TestTableColumnAlignment checks each column honors its delimiter row:
// the body cell lines up with its header on the aligned side.
func TestTableColumnAlignment(t *testing.T) {
	_, tt, _ := frame(t, alignedTable, 640, 400)
	lHead := rectOf(t, tt, "llllllllll")
	rHead := rectOf(t, tt, "rrrrrrrrrr")
	cHead := rectOf(t, tt, "cccccccccc")
	left := rectOf(t, tt, "L")
	right := rectOf(t, tt, "R")
	center := rectOf(t, tt, "C")

	// :---: left, the default: the text starts at the column's padding.
	if got := left.X - lHead.X; !near(got, 0) {
		t.Fatalf("left cell starts %v DIPs from the column's left edge (cell %v, header %v)", got, left, lHead)
	}
	// ---: right: the text ends at the column's right padding.
	if got := (right.X + right.W) - (rHead.X + rHead.W); !near(got, 0) {
		t.Fatalf("right cell ends %v DIPs from the column's right edge (cell %v, header %v)", got, right, rHead)
	}
	// :---: center: the text centers on the column's center.
	if got := (center.X + center.W/2) - (cHead.X + cHead.W/2); !near(got, 0) {
		t.Fatalf("center cell is %v DIPs off the column's center (cell %v, header %v)", got, center, cHead)
	}
}

// TestTaskListCheckboxDrawn checks a task item's check box is a drawn
// square: filled with the accent while checked, framed while not, and no
// box-drawing glyph is left in the text.
func TestTaskListCheckboxDrawn(t *testing.T) {
	theme := ui.LightTheme()
	_, tt, _ := frame(t, "- [ ] todo one\n- [x] done two\n", 640, 400)
	if !tt.HasText("todo one") || !tt.HasText("done two") {
		t.Fatalf("texts %v: an item's text is gone", tt.Texts())
	}
	for _, glyph := range []string{"\u2610", "\u2611"} {
		if tt.HasText(glyph) {
			t.Fatalf("texts %v: the checkbox is still the %q glyph", tt.Texts(), glyph)
		}
	}
	if n := countColor(tt.Image(), theme.Accent); n == 0 {
		t.Fatal("no accent pixels: the checked box is not filled")
	}
	if n := countColor(tt.Image(), theme.Border); n == 0 {
		t.Fatal("no border pixels: the unchecked box is not framed")
	}
}

// TestTaskListBoxSitsOnTheFirstLine checks the box shares the item's
// first line, not the bottom of a wrapped item.
func TestTaskListBoxSitsOnTheFirstLine(t *testing.T) {
	theme := ui.LightTheme()
	long := "- [x] " + longTaskLine()
	_, tt, _ := frame(t, long, 400, 400)
	text := rectOf(t, tt, "wrapped task")
	if text.H < 2*20 {
		t.Skipf("the item did not wrap: %v", text)
	}
	box, ok := accentBounds(tt, theme.Accent)
	if !ok {
		t.Fatal("no accent pixels: the checked box is not filled")
	}
	if box.Y < text.Y-4 || box.Y+box.H > text.Y+text.H {
		t.Fatalf("the box %v sits outside the first line of the item %v", box, text)
	}
}

// accentBounds returns the bounding box of the accent pixels.
func accentBounds(tt *ui.Tester, c ui.Color) (ui.Rect, bool) {
	img := tt.Image()
	b := img.Bounds()
	left, top, right, bottom := b.Max.X, b.Max.Y, -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !pixelIs(img, x, y, c) {
				continue
			}
			left, top = min(left, x), min(top, y)
			right, bottom = max(right, x), max(bottom, y)
		}
	}
	if right < 0 {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(left), Y: float32(top), W: float32(right - left + 1), H: float32(bottom - top + 1)}, true
}

// longTaskLine is a task item long enough to wrap in a 400 wide window.
func longTaskLine() string {
	words := ""
	for range 30 {
		words += "wrapped task "
	}
	return words + "\n"
}
