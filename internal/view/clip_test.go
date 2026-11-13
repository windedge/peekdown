package view

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The reported bug: a long paragraph's line ran past the window's right
// edge instead of wrapping, so the reader saw the line hard cut mid-word.
// The row carries the scrollbar's clearance as a right padding, and the
// text must wrap inside what that padding leaves, not under the bar.

// countColorIn counts the pixels of rect r that are exactly c.
func countColorIn(img *image.RGBA, c ui.Color, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if pixelIs(img, x, y, c) {
				n++
			}
		}
	}
	return n
}

func TestLongParagraphWrapsInsideTheContentArea(t *testing.T) {
	theme := ui.LightTheme()
	long := strings.Repeat("word ", 120)
	_, tt, _ := frame(t, long+"\n", 640, 400)
	body := rectOf(t, tt, "word")

	// The text box ends where the row's clearance lets it, not at the
	// window's edge: 640 - 24? no left padding in the view alone, so the
	// row pads the right 16 of 640.
	if got := 640 - (body.X + body.W); !near(float32(got), rowClearance) {
		t.Fatalf("paragraph box ends %v DIPs from the window's edge, want %v (box %v)", got, rowClearance, body)
	}
	// It wrapped rather than drawing one long line.
	if body.H < 2*20 {
		t.Fatalf("paragraph draws %v tall in a 640 wide window: one line, not wrapped", body.H)
	}
	// No glyph reaches into the clearance the scrollbar takes.
	if n := countColorIn(tt.Image(), theme.Text, ui.Rect{X: 640 - rowClearance, Y: body.Y, W: rowClearance, H: body.H}); n != 0 {
		t.Fatalf("%d text pixels inside the scrollbar's %v DIPs,"+
			" the line runs under the bar", n, rowClearance)
	}
}
