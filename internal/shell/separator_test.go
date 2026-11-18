package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// pixelIsShell reports whether the pixel at x, y is exactly c.
func pixelIsShell(img *image.RGBA, x, y int, c ui.Color) bool {
	r, g, b, a := img.RGBAAt(x, y).RGBA()
	return uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(b>>8) == c.B && uint8(a>>8) == c.A
}

// accentColumns scans the divider band's columns and returns which hold an
// accent pixel along the divider's run.
func accentColumns(img *image.RGBA, x0, x1 int, accent ui.Color) map[int]bool {
	out := map[int]bool{}
	for x := x0; x <= x1; x++ {
		for y := 60; y < 500; y++ {
			if pixelIsShell(img, x, y, accent) {
				out[x] = true
				break
			}
		}
	}
	return out
}

// TestDividerHiddenUntilHover: idle, the gutter band holds no accent
// column anywhere (the native edge line is covered, no hover line);
// hovering the gap lights exactly one column, centered in the gutter
// (card edge + 4); moving off the gap drops it again.
func TestDividerHiddenUntilHover(t *testing.T) {
	s, tt := openOutline(t, 1200)
	accent := s.accentTheme().of(false).Accent
	w := int(s.Sidebar.Width)
	got := accentColumns(tt.Image(), w-8, w+9, accent)
	if len(got) != 0 {
		t.Fatalf("idle band holds accent columns %v", got)
	}
	tt.Move(float32(w)+4, 300)
	tt.Frame()
	got = accentColumns(tt.Image(), w-8, w+9, accent)
	if len(got) != 1 || !got[w+4] {
		t.Fatalf("hover band columns %v, want only the centered %d", got, w+4)
	}
	tt.Move(600, 300)
	tt.Frame()
	got = accentColumns(tt.Image(), w-8, w+9, accent)
	if len(got) != 0 {
		t.Fatalf("moved-off band still holds accent columns %v", got)
	}
}

// TestOutlineDividerCentersOnTheGutter does the same on the right side:
// the pane boundary (the outline card's left edge) is a pane width from
// the window's right; the centered line stands 4 left of it.

// TestDividerIdleHasNoBorderColumn pins the user's report: with no pointer
// in either gutter, no column anywhere in the two gutter bands may differ
// from the window's own gradient at that spot, native line included.
func TestDividerIdleHasNoBorderColumn(t *testing.T) {
	s, tt := openOutline(t, 1200)
	k := s.accentTheme().of(false)
	w := int(s.Sidebar.Width)
	edge := 1200 - int(s.Outline.Width)
	img := tt.Image()
	for _, band := range [][2]int{{w, w + 7}, {edge - 4, edge + 3}} {
		for x := band[0]; x <= band[1]; x++ {
			for _, y := range []int{120, 300, 500} {
				r, g, b, _ := img.At(x, y).RGBA()
				cr, cg, cb, _ := img.At(2, y).RGBA() // gutter strip at same y
				// The native line reads ~30 units off the gradient; AA and
				// gradient rounding differ by a few. Eight splits them.
				if abs8(r, cr) > 8 || abs8(g, cg) > 8 || abs8(b, cb) > 8 {
					t.Fatalf("column %d at y=%d shows a line (%d %d %d) the gradient does not hold (%d %d %d)", x, y, r>>8, g>>8, b>>8, cr>>8, cg>>8, cb>>8)
				}
			}
		}
	}
	_ = k
}

func abs8(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}
func TestOutlineDividerCentersOnTheGutter(t *testing.T) {
	s, tt := openOutline(t, 1200)
	accent := s.accentTheme().of(false).Accent
	edge := 1200 - int(s.Outline.Width)
	tt.Move(float32(edge)-4, 300)
	tt.Frame()
	got := accentColumns(tt.Image(), edge-9, edge+8, accent)
	// paneGrip puts the pane boundary one pixel left of the card edge, so
	// the centered line lands on edge-2; the hover pointer still aims at
	// the gutter's middle.
	if len(got) != 1 || !got[edge-4] {
		t.Fatalf("hover band columns %v, want only the centered %d", got, edge-4)
	}
}
