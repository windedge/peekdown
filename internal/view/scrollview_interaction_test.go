package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

// ScrollView wires the handler itself: dragging in a view built by
// ScrollView selects, as in the app, without the caller wiring anything.
func TestScrollViewDragSelects(t *testing.T) {
	m := doc.New("First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n")
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)

	x0, y0 := findText(t, tt, "First", false)
	x1, y1 := findText(t, tt, "Third", true)
	dragFromTo(tt, x0, y0, x1, y1)

	if got := state.SelectionText(m); got != "First paragraph.\nSecond paragraph.\nThird paragraph." {
		t.Fatalf("selected %q", got)
	}
	if !state.HasSelection() {
		t.Fatal("the selection was not kept")
	}
}

// The handler ScrollView wires scrolls the wheel itself now (the
// scroll_speed multiplier lives there): the default speed glides one
// notch's worth, the same place the engine's own glide would land.
func TestScrollViewWheelStillScrolls(t *testing.T) {
	long := strings.Repeat("Paragraph of filler text to scroll past.\n\n", 60)
	m := doc.New(long)
	var state State
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	first := strings.Join(tt.Texts(), " ")
	tt.Wheel(200, 150, 0, 120)
	tt.Advance(200 * time.Millisecond)
	if strings.Join(tt.Texts(), " ") == first {
		t.Fatal("the wired handler stopped the wheel from scrolling")
	}
	if state.dragging {
		t.Fatal("the wheel started a drag")
	}
}

// At a speed past 1 the handler scrolls the notch itself, the multiplier
// riding the offset write: a 2x speed lands twice a notch's travel.
func TestScrollViewWheelSpeedMultiplies(t *testing.T) {
	long := strings.Repeat("Paragraph of filler text to scroll past.\n\n", 60)
	m := doc.New(long)
	var state State
	state.ScrollSpeed = 2
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		ScrollView(c, m, &state)
	}, 400, 300)
	tt.Frame()
	tt.Wheel(200, 150, 0, 120)
	tt.Advance(200 * time.Millisecond)
	if got := state.Scroll.Y; !near(got, 240) {
		t.Fatalf("scroll y = %v after one notch, want the 2x 240", got)
	}
}
